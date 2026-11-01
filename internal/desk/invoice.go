package desk

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// The supply side: a supplier invoice (photo or PDF) becomes a goods receipt in the ERP.
// The paper is already structured, so the only open question per line is identity: which of
// our products is "SEL 2000ML PET X6" in this supplier's words.

// InvoiceDoc is one reading of a supplier invoice.
type InvoiceDoc struct {
	Supplier string  `json:"supplier"`
	Number   string  `json:"number"`
	Date     string  `json:"date"`
	TotalHT  float64 `json:"total_ht"`
	Lines    []InvoiceLine `json:"lines"`
}

type InvoiceLine struct {
	Text      string  `json:"text"`
	Qty       float64 `json:"qty"`
	UnitPrice float64 `json:"unit_price"`
	Total     float64 `json:"total"`
}

const invoicePrompt = `You read supplier invoices (factures, bons de livraison) received by a distributor in Algeria.
Copy what is printed; never guess products. Return JSON only:
{"supplier":"the SELLER's company name as printed (not the buyer)","number":"invoice number","date":"YYYY-MM-DD",
 "total_ht":12345.67,
 "lines":[{"text":"the designation exactly as printed, with its codes and pack marks (e.g. SEL 2000ML PET X6)",
   "qty":12,"unit_price":1140.00,"total":13680.00}]}
Amounts are in dinars as printed (HT when both HT and TTC are printed). Skip headers, subtotals, taxes and stamps.`

// IngestInvoice reads an invoice image or PDF, matches its supplier and lines, and opens an
// invoice draft for approval.
func (d *Desk) IngestInvoice(ctx context.Context, key string, data []byte, media string) (string, error) {
	msgID := store.NewID("msg")
	doc, err := ReadInvoice(ctx, d.Meta, meta.Spark, data, msgID)
	if err != nil {
		return "", err
	}
	// Muse Wonder reads the same paper on its own; a line the two readers do not agree on is shown to
	// the person (the identity of the product is a separate question, settled by the resolver)
	var second *InvoiceDoc
	if d.DB.SettingInt("invoice_second_reader", 1) > 0 {
		if w, err := ReadInvoice(ctx, d.Meta, meta.Wonder, data, msgID); err == nil {
			second = &w
		}
	}
	var text strings.Builder
	for _, l := range doc.Lines {
		fmt.Fprintf(&text, "%s | %g | %.2f | %.2f\n", l.Text, l.Qty, l.UnitPrice, l.Total)
	}
	d.DB.Exec(`INSERT INTO messages (id, chat_key, kind, text, media_path, extraction, created_at) VALUES (?,?,?,?,?,?,?)`,
		msgID, key, "invoice", fmt.Sprintf("%s · n° %s · %s\n%s", doc.Supplier, doc.Number, doc.Date, text.String()), media, "ai", store.Now())

	supplierID, score := d.Cat.MatchSupplier(doc.Supplier)
	if score < 0.8 {
		supplierID = ""
	}
	draft := store.NewID("inv")
	now := store.Now()
	d.DB.Exec(`INSERT INTO drafts (id, chat_key, customer_id, status, kind, supplier_id, supplier_said, doc_number, doc_date, doc_total,
		created_at, updated_at) VALUES (?, ?, '', 'open', 'invoice', ?, ?, ?, ?, ?, ?, ?)`,
		draft, key, supplierID, doc.Supplier, doc.Number, doc.Date, cents(doc.TotalHT), now, now)
	var ms []resolve.Mention
	var kept []int // doc.Lines index of each mention: a blank row must not shift the prices
	for i, l := range doc.Lines {
		if strings.TrimSpace(l.Text) == "" {
			continue
		}
		ms = append(ms, resolve.Mention{Said: strings.TrimSpace(l.Text), Qty: l.Qty})
		kept = append(kept, i)
	}
	lines := d.resolveInvoice(ctx, draft, msgID, supplierID, ms)
	var agree []bool
	if second != nil {
		agree = Agree(doc, *second)
	}
	for i := range lines {
		if i < len(kept) {
			l := doc.Lines[kept[i]]
			lines[i].UnitPrice, lines[i].LineTTC = cents(l.UnitPrice), cents(l.Total)
			if agree != nil && !agree[kept[i]] {
				lines[i].Checks["second_reader"] = "Muse Wonder ne lit pas la même ligne : vérifiez la désignation, la quantité et le prix sur la facture"
				if lines[i].Status == "ok" {
					lines[i].Status = "ask" // never booked on a reading only one model saw
					lines[i].Options = []string{lines[i].ProductID}
					lines[i].Question = "Les deux lecteurs ne lisent pas la même ligne : est-ce bien ce produit ?"
				}
			}
		}
	}
	d.checkInvoice(ctx, draft, supplierID, lines, cents(doc.TotalHT))
	return draft, nil
}

// ReadInvoice has one model read an invoice image or PDF.
func ReadInvoice(ctx context.Context, m *meta.Client, model string, data []byte, ref string) (InvoiceDoc, error) {
	var doc InvoiceDoc
	var err error
	if strings.HasPrefix(http.DetectContentType(data), "application/pdf") {
		_, err = m.JSONFile(ctx, model, "invoice", ref, invoicePrompt, "Read this invoice.", data, "application/pdf", 6000, &doc)
	} else {
		_, err = m.JSON(ctx, model, "invoice", ref, invoicePrompt, "Read this invoice.", [][]byte{data}, 6000, &doc)
	}
	return doc, err
}

// Agree tells, for each line of the first reading, whether the second reading has the same line: the same
// quantity and unit price, and a designation with the same words (each reader may drop a stray mark).
func Agree(a, b InvoiceDoc) []bool {
	out := make([]bool, len(a.Lines))
	used := make([]bool, len(b.Lines))
	for i, l := range a.Lines {
		wa, _ := catalog.Tokens(catalog.Normalize(l.Text))
		for j, m := range b.Lines {
			if used[j] || !sameAmount(l.Qty, m.Qty) || !sameAmount(l.UnitPrice, m.UnitPrice) {
				continue
			}
			wb, _ := catalog.Tokens(catalog.Normalize(m.Text))
			if overlap(wa, wb) >= 0.8 {
				out[i], used[j] = true, true
				break
			}
		}
	}
	return out
}

func sameAmount(a, b float64) bool { return math.Abs(a-b) <= math.Max(0.01, 0.005*math.Abs(a)) }

// overlap is the share of words the two designations have in common (over the longer one).
func overlap(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	set := map[string]bool{}
	for _, w := range b {
		set[w] = true
	}
	n := 0
	for _, w := range a {
		if set[w] {
			n++
		}
	}
	return float64(n) / float64(max(len(a), len(b)))
}

// resolveInvoice resolves invoice lines in the supplier's scope (its own wordings and rules).
func (d *Desk) resolveInvoice(ctx context.Context, draft, msgID, supplierID string, ms []resolve.Mention) []Line {
	usual := map[string]int{}
	if e := d.erp(); e != nil && supplierID != "" {
		if prices, err := e.LastPurchasePrices(ctx, supplierID); err == nil {
			for id := range prices {
				usual[id] = 1 // bought from this supplier before
			}
		}
	}
	live, shadow := d.ruleSets()
	rv := d.resolver(live, shadow)
	res := rv.Resolve(ctx, ms, resolve.Context{SupplierID: supplierID, Usual: usual}, msgID)
	var out []Line
	for i, r := range res {
		l := Line{DraftID: draft, MessageID: msgID, Position: i + 1, Said: r.Said, Norm: r.Norm, Qty: r.Qty, ProductID: r.ProductID,
			Level: r.Level, Used: r.Used, Confidence: r.Confidence, Options: r.Options, Question: r.Question, Status: r.Status,
			Checks: map[string]string{}, Shadow: r.Shadow}
		if r.Check != "" {
			l.Checks["spark"] = r.Check
		}
		if r.CheckWhy != "" {
			l.Checks["check_why"] = r.CheckWhy
		}
		if r.Det != "" {
			l.Checks["det_product"], l.Checks["det_used"] = r.Det, strings.Join(r.DetUsed, ",")
		}
		if r.Pack != nil {
			l.Unit = r.Pack.Unit
			l.Checks["pack_rule"] = r.Pack.Unit
			if r.Pack.Qty > 0 {
				l.Checks["pack_qty"] = strconv.Itoa(r.Pack.Qty)
			}
		}
		if l.Status == "unknown" || (l.Status == "ask" && len(l.Options) == 0) {
			l.Checks["new"] = "produit inconnu du catalogue : créez-le dans l'ERP puis synchronisez"
		}
		out = append(out, l)
	}
	return out
}

// checkInvoice converts quantities with the real packs and checks prices against the last purchase
// price and the line totals against the invoice total.
func (d *Desk) checkInvoice(ctx context.Context, draft, supplierID string, lines []Line, totalHT int64) {
	var last map[string]int64
	if e := d.erp(); e != nil && supplierID != "" {
		last, _ = e.LastPurchasePrices(ctx, supplierID)
	}
	var sum int64
	for i := range lines {
		l := &lines[i]
		sum += l.LineTTC
		if l.Status == "ok" {
			d.invoiceQty(l, last[l.ProductID])
		}
		saveLine(d.DB, l)
	}
	checks := map[string]string{}
	if totalHT > 0 && sum > 0 && math.Abs(float64(sum-totalHT)) > math.Max(100, float64(totalHT)*0.005) {
		checks["total"] = fmt.Sprintf("la somme des lignes (%s) ne correspond pas au total imprimé (%s)", Money(sum), Money(totalHT))
	}
	if supplierID == "" {
		checks["supplier"] = "fournisseur non reconnu : choisissez-le"
	}
	d.DB.Exec(`UPDATE drafts SET checks = ?, updated_at = ? WHERE id = ?`, store.JSON(checks), store.Now(), draft)
}

// invoiceQty decides whether the printed quantity counts packs or base units: a pack mark read by a
// rule ("X6") decides first, then the printed unit word, then whichever reading gives a cost per base
// unit closest to the last purchase price from this supplier.
func (d *Desk) invoiceQty(l *Line, lastCost int64) {
	p := d.Cat.Get(l.ProductID)
	if p == nil {
		return
	}
	delete(l.Checks, "pack")
	delete(l.Checks, "price")
	delete(l.Checks, "unit")
	perUnit := func(f int64) int64 { // cost per base unit if the printed qty counts packs of f
		if l.Qty <= 0 || l.LineTTC <= 0 {
			return 0
		}
		base := l.Qty
		if f > 0 {
			base = l.Qty * float64(f) / 1000
		}
		return int64(float64(l.LineTTC) / base)
	}
	var name string
	var factor int64
	printed, _ := strconv.Atoi(l.Checks["pack_qty"])
	switch {
	case printed > 0:
		name, factor = resolve.ChoosePack(p, "carton", &rules.PackSpec{Unit: "carton", Qty: printed})
		if factor != int64(printed)*1000 {
			// the supplier delivered packs of `printed`: count what was delivered, and flag it
			l.Checks["pack"] = fmt.Sprintf("la facture dit ×%d, le catalogue a %s", printed, packsText(p))
			name, factor = fmt.Sprintf("×%d", printed), int64(printed)*1000
		}
	case l.Unit == "carton" || l.Unit == "pack":
		name, factor = resolve.ChoosePack(p, l.Unit, nil)
	case l.Unit == "unit" || len(p.Packs) == 0:
	default:
		_, f := resolve.ChoosePack(p, "carton", nil)
		if lastCost > 0 && diff(perUnit(f), lastCost) < diff(perUnit(0), lastCost) {
			name, factor = resolve.ChoosePack(p, "carton", nil)
		} else if lastCost == 0 {
			l.Checks["unit"] = "quantité lue en unités (aucun conditionnement imprimé)"
		}
	}
	l.PackName, l.PackFactor = name, factor
	if factor > 0 {
		l.QtyMilli = int64(l.Qty*float64(factor) + 0.5)
	} else {
		l.QtyMilli = int64(l.Qty*1000 + 0.5)
	}
	if cost := perUnit(factor); lastCost > 0 && cost > 0 && diff(cost, lastCost) > 0.15 {
		l.Checks["price"] = fmt.Sprintf("prix unitaire %s, dernier achat %s (%+.0f %%)", Money(cost), Money(lastCost),
			(float64(cost)/float64(lastCost)-1)*100)
	}
}

func packsText(p *catalog.Product) string {
	if len(p.Packs) == 0 {
		return "l'unité seulement"
	}
	var parts []string
	for _, k := range p.Packs {
		parts = append(parts, fmt.Sprintf("%s ×%s", strings.ToLower(k.Name), trimQty(k.FactorMilli)))
	}
	return strings.Join(parts, ", ")
}

func diff(a, b int64) float64 {
	if b == 0 {
		return math.Inf(1)
	}
	return math.Abs(float64(a)/float64(b) - 1)
}

func cents(da float64) int64 { return int64(math.Round(da * 100)) }

// SetSupplier sets (or corrects) the supplier of an invoice draft and re-resolves its lines in that
// supplier's scope. The printed name is learned as a wording for that supplier.
func (d *Desk) SetSupplier(ctx context.Context, draft, supplierID string) error {
	var said, status string
	d.DB.QueryRow(`SELECT supplier_said, status FROM drafts WHERE id = ? AND kind = 'invoice'`, draft).Scan(&said, &status)
	if status != "open" {
		return fmt.Errorf("receipt is %s", status)
	}
	d.DB.Exec(`UPDATE drafts SET supplier_id = ? WHERE id = ? AND status = 'open'`, supplierID, draft)
	if said != "" {
		d.Cat.Witness("supplier: "+catalog.Normalize(said), supplierID, "global", "", "invoice", 1)
	}
	old := DraftLines(d.DB, draft, false)
	var ms []resolve.Mention
	for _, l := range old {
		ms = append(ms, resolve.Mention{Said: l.Said, Qty: l.Qty})
	}
	d.DB.Exec(`DELETE FROM lines WHERE draft_id = ?`, draft)
	msgID := ""
	if len(old) > 0 {
		msgID = old[0].MessageID
	}
	lines := d.resolveInvoice(ctx, draft, msgID, supplierID, ms)
	for i := range lines {
		if i < len(old) {
			lines[i].UnitPrice, lines[i].LineTTC = old[i].UnitPrice, old[i].LineTTC
		}
	}
	var total int64
	d.DB.QueryRow(`SELECT doc_total FROM drafts WHERE id = ?`, draft).Scan(&total)
	d.checkInvoice(ctx, draft, supplierID, lines, total)
	return nil
}

// ConfirmInvoice books the approved invoice as a goods receipt and teaches the catalogue.
func (d *Desk) confirmInvoice(ctx context.Context, e adapter.ERP, draft, origin string) (adapter.OrderRef, error) {
	var supplierID, number, date string
	d.DB.QueryRow(`SELECT supplier_id, doc_number, doc_date FROM drafts WHERE id = ?`, draft).Scan(&supplierID, &number, &date)
	if supplierID == "" {
		return adapter.OrderRef{}, fmt.Errorf("choose the supplier first")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		date = time.Now().Format("2006-01-02")
	}
	lines := DraftLines(d.DB, draft, false)
	r := adapter.Receipt{SupplierID: supplierID, Date: date, SupplierRef: number, Post: d.DB.Setting("receipt_post", "1") == "1",
		Notes: "Facture lue par Order Desk"}
	for _, l := range lines {
		if l.Status != "ok" || l.QtyMilli <= 0 {
			continue
		}
		p := d.Cat.Get(l.ProductID)
		var vat int64
		if p != nil {
			vat = p.VatRate
		}
		// The ERP counts a receipt line in its own unit (packs when a pack factor is given) and prices it per that unit.
		qty, factor := l.QtyMilli, int64(1000)
		if l.PackFactor > 0 {
			qty, factor = l.QtyMilli*1000/l.PackFactor, l.PackFactor
		}
		cost := int64(0)
		if qty > 0 {
			cost = int64(math.Round(float64(l.LineTTC) / (float64(qty) / 1000)))
		}
		r.Lines = append(r.Lines, adapter.ReceiptLine{ProductID: l.ProductID, Description: l.Said, QtyMilli: qty,
			UnitCostHT: cost, VatRate: vat, PackFactorMilli: factor})
	}
	if len(r.Lines) == 0 {
		return adapter.OrderRef{}, fmt.Errorf("no approved line")
	}
	for _, l := range lines {
		if l.Status == "ask" || l.Status == "unknown" {
			return adapter.OrderRef{}, fmt.Errorf("%d line(s) still need an answer", countOpen(lines))
		}
	}
	ref, err := e.CreateReceipt(ctx, r)
	if err != nil {
		return ref, err
	}
	d.DB.Exec(`UPDATE drafts SET status = 'confirmed', erp_order_id = ?, erp_order_number = ?, total_ttc = ?, updated_at = ? WHERE id = ?`,
		ref.ID, ref.Number, ref.TotalTTC, store.Now(), draft)
	d.DB.Event("receipt.created", draft, origin, map[string]any{"number": ref.Number, "lines": len(r.Lines), "erp": e.Name(),
		"supplier": catalog.SupplierName(d.DB.DB, supplierID)})
	d.learnLines(lines, "invoice", "supplier", supplierID, origin)
	if d.OnLearn != nil {
		go d.OnLearn()
	}
	return ref, nil
}

func countOpen(lines []Line) int {
	n := 0
	for _, l := range lines {
		if l.Status == "ask" || l.Status == "unknown" {
			n++
		}
	}
	return n
}

// InvoiceSummary is the owner's view of an invoice in the chat.
func (d *Desk) InvoiceSummary(draft string) (string, int) {
	var said, supplierID, number string
	var checks string
	d.DB.QueryRow(`SELECT supplier_said, supplier_id, doc_number, checks FROM drafts WHERE id = ?`, draft).Scan(&said, &supplierID, &number, &checks)
	name := catalog.SupplierName(d.DB.DB, supplierID)
	if name == "" {
		name = said + " (non reconnu)"
	}
	lines := DraftLines(d.DB, draft, false)
	auto, ai, open, flagged := 0, 0, 0, 0
	for _, l := range lines {
		switch {
		case l.Status != "ok":
			open++
		case l.Level == resolve.LAlias || l.Level == resolve.LRules:
			auto++
		default:
			ai++
		}
		if l.Checks["price"] != "" || l.Checks["pack"] != "" {
			flagged++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "🧾 <b>Facture %s</b> — %s\n%d ligne(s) : %d reconnues par les règles, %d par l'IA, %d à vérifier",
		esc(number), esc(name), len(lines), auto, ai, open)
	if flagged > 0 {
		fmt.Fprintf(&b, ", %d prix ou conditionnement à contrôler", flagged)
	}
	var c map[string]string
	json.Unmarshal([]byte(checks), &c)
	for _, v := range c {
		b.WriteString("\n⚠️ " + esc(v))
	}
	return b.String(), open
}
