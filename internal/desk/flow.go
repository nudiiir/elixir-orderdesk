package desk

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
	"github.com/nudiiir/elixir-orderdesk/internal/telegram"
)

// next asks the first open question of the draft, or shows the summary to confirm.
func (d *Desk) next(ctx context.Context, chat int64, draft string) {
	var kind, supplierID string
	d.DB.QueryRow(`SELECT kind, supplier_id FROM drafts WHERE id = ?`, draft).Scan(&kind, &supplierID)
	if kind == "invoice" && supplierID == "" {
		text, _ := d.InvoiceSummary(draft)
		var rows [][]telegram.Button
		for _, s := range catalog.Suppliers(d.DB.DB) {
			rows = append(rows, []telegram.Button{{Text: s.Name, Data: "sup:" + draft + ":" + s.ID}})
		}
		d.TG.Send(ctx, chat, text+"\n\nQuel est le fournisseur ?", rows)
		return
	}
	for _, l := range DraftLines(d.DB, draft, false) {
		if l.Status == "ask" || l.Status == "unknown" {
			d.ask(ctx, chat, l)
			return
		}
	}
	if kind == "invoice" {
		text, _ := d.InvoiceSummary(draft)
		d.TG.Send(ctx, chat, text, [][]telegram.Button{{{Text: "✅ Valider la réception", Data: "ok:" + draft}},
			{{Text: "✏️ Corriger une ligne", Data: "fix:" + draft}, {Text: "❌ Annuler", Data: "no:" + draft}}})
		return
	}
	d.summary(ctx, chat, draft, 0)
}

func (d *Desk) ask(ctx context.Context, chat int64, l Line) {
	var rows [][]telegram.Button
	for i, id := range l.Options {
		if p := d.Cat.Get(id); p != nil {
			rows = append(rows, []telegram.Button{{Text: p.Name, Data: fmt.Sprintf("p:%s:%d", l.ID, i)}})
		}
	}
	rows = append(rows, []telegram.Button{{Text: "✖ Retirer cette ligne", Data: "x:" + l.ID}})
	q := l.Question
	if l.Status == "unknown" || len(l.Options) == 0 {
		q = fmt.Sprintf("Je ne trouve pas « %s » dans le catalogue.", l.Said)
	}
	d.TG.Send(ctx, chat, "❓ "+esc(q), rows)
}

// Summary renders the draft as the shop sees it.
func (d *Desk) Summary(draft string) (text string, total int64, ok int) {
	var b strings.Builder
	var shop string
	d.DB.QueryRow(`SELECT COALESCE(c.name,'') FROM drafts d LEFT JOIN customers c ON c.id = d.customer_id WHERE d.id = ?`, draft).Scan(&shop)
	fmt.Fprintf(&b, "🧾 <b>Votre commande</b> — %s\n\n", esc(shop))
	n := 0
	for _, l := range DraftLines(d.DB, draft, false) {
		if l.Status != "ok" {
			continue
		}
		p := d.Cat.Get(l.ProductID)
		if p == nil {
			continue
		}
		n++
		fmt.Fprintf(&b, "%d. %s · %s", n, qtyLabel(l), esc(p.Name))
		if l.LineTTC > 0 {
			fmt.Fprintf(&b, " — %s", Money(l.LineTTC))
		}
		b.WriteString("\n")
		for _, k := range []string{"offer", "stock", "usual", "pack", "unit"} {
			if v := l.Checks[k]; v != "" {
				icon := "⚠️"
				if k == "offer" {
					icon = "🎁"
				}
				fmt.Fprintf(&b, "   %s %s\n", icon, esc(v))
			}
		}
		total += l.LineTTC
	}
	if total > 0 {
		fmt.Fprintf(&b, "\nTotal : <b>%s TTC</b>", Money(total))
	}
	return b.String(), total, n
}

func qtyLabel(l Line) string {
	if l.PackFactor > 0 {
		name := strings.ToLower(l.PackName)
		if l.Qty > 1 && !strings.HasSuffix(name, "s") && !strings.HasSuffix(name, "x") {
			if strings.HasSuffix(name, "eau") {
				name += "x"
			} else {
				name += "s"
			}
		}
		return fmt.Sprintf("%s %s ×%s", trimFloat(l.Qty), name, trimQty(l.PackFactor))
	}
	return trimFloat(l.Qty) + " u."
}

func tokensOf(said, hint string) ([]string, string) {
	return catalog.Tokens(catalog.Normalize(said + " " + hint))
}

func trimFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func (d *Desk) summary(ctx context.Context, chat int64, draft string, editID int) {
	text, _, n := d.Summary(draft)
	if n == 0 {
		d.say(ctx, chat, "La commande est vide. Envoyez vos produits.")
		return
	}
	var customerID string
	d.DB.QueryRow(`SELECT customer_id FROM drafts WHERE id = ?`, draft).Scan(&customerID)
	if e := d.erp(); e != nil {
		if del, err := e.NextDelivery(ctx, customerID); err == nil && del.Day != "" {
			text += fmt.Sprintf("\nLivraison : <b>%s</b> (tournée %s)", del.Day, esc(del.Route))
			d.DB.Exec(`UPDATE drafts SET delivery = ? WHERE id = ?`, del.Day+" · "+del.Route, draft)
		}
	}
	rows := [][]telegram.Button{{{Text: "✅ Confirmer", Data: "ok:" + draft}},
		{{Text: "✏️ Corriger une ligne", Data: "fix:" + draft}, {Text: "❌ Annuler", Data: "no:" + draft}}}
	if editID > 0 {
		d.TG.Edit(ctx, chat, editID, text, rows)
		return
	}
	id, _ := d.TG.Send(ctx, chat, text, rows)
	d.DB.Exec(`UPDATE drafts SET summary_msg_id = ? WHERE id = ?`, id, draft)
}

func (d *Desk) onCallback(ctx context.Context, cb *telegram.TgCallback) {
	if cb.Message == nil {
		return
	}
	chat := cb.Message.Chat.ID
	key := chatKey(chat)
	customerID, _ := d.linked(key)
	parts := strings.Split(cb.Data, ":")
	d.TG.Answer(ctx, cb.ID, "")
	if (customerID == "" && !d.isOwner(key)) || len(parts) < 2 {
		return
	}
	switch parts[0] {
	case "p", "c": // pick an option for an asked line (p) or for a corrected line (c)
		if len(parts) != 3 {
			return
		}
		idx, _ := strconv.Atoi(parts[2])
		l, err := GetLine(d.DB, parts[1])
		if err != nil || idx < 0 || idx >= len(l.Options) || !d.ownsLine(key, l) {
			return
		}
		d.Choose(ctx, &l, l.Options[idx], parts[0] == "c")
		d.TG.Edit(ctx, chat, cb.Message.MessageID, "✔️ « "+esc(l.Said)+" » → "+esc(d.Cat.Get(l.ProductID).Name), nil)
		d.next(ctx, chat, l.DraftID)
	case "x":
		l, err := GetLine(d.DB, parts[1])
		if err != nil || !d.ownsLine(key, l) {
			return
		}
		l.Status, l.Outcome = "removed", "removed"
		saveLine(d.DB, &l)
		d.TG.Edit(ctx, chat, cb.Message.MessageID, "✖ Ligne retirée : « "+esc(l.Said)+" »", nil)
		d.next(ctx, chat, l.DraftID)
	case "fix": // list the lines to correct
		var rows [][]telegram.Button
		for _, l := range DraftLines(d.DB, parts[1], false) {
			if p := d.Cat.Get(l.ProductID); p != nil && l.Status == "ok" {
				rows = append(rows, []telegram.Button{{Text: p.Name, Data: "fl:" + l.ID}})
			}
		}
		rows = append(rows, []telegram.Button{{Text: "↩️ Retour", Data: "back:" + parts[1]}})
		d.TG.Edit(ctx, chat, cb.Message.MessageID, "Quelle ligne corriger ?", rows)
	case "fl": // show alternatives for one line
		l, err := GetLine(d.DB, parts[1])
		if err != nil || !d.ownsLine(key, l) {
			return
		}
		alts := d.alternatives(l)
		l.Options = alts
		saveLine(d.DB, &l)
		var rows [][]telegram.Button
		for i, id := range alts {
			rows = append(rows, []telegram.Button{{Text: d.Cat.Get(id).Name, Data: fmt.Sprintf("c:%s:%d", l.ID, i)}})
		}
		rows = append(rows, []telegram.Button{{Text: "✖ Retirer cette ligne", Data: "x:" + l.ID}},
			[]telegram.Button{{Text: "↩️ Retour", Data: "back:" + l.DraftID}})
		d.TG.Edit(ctx, chat, cb.Message.MessageID, fmt.Sprintf("« %s » : quel produit ?", esc(l.Said)), rows)
	case "sup": // the owner names the supplier of an invoice
		if len(parts) == 3 && d.ownsLine(key, Line{DraftID: parts[1]}) {
			d.SetSupplier(ctx, parts[1], parts[2])
			d.TG.Edit(ctx, chat, cb.Message.MessageID, "✔️ Fournisseur : "+esc(catalog.SupplierName(d.DB.DB, parts[2])), nil)
			d.next(ctx, chat, parts[1])
		}
	case "back":
		d.summary(ctx, chat, parts[1], cb.Message.MessageID)
	case "ok":
		ref, err := d.Confirm(ctx, parts[1], "shop")
		if err != nil {
			d.fail(ctx, chat, "la commande n'a pas pu être enregistrée chez le distributeur", err)
			return
		}
		var kind string
		d.DB.QueryRow(`SELECT kind FROM drafts WHERE id = ?`, parts[1]).Scan(&kind)
		if kind == "invoice" {
			text, _ := d.InvoiceSummary(parts[1])
			d.TG.Edit(ctx, chat, cb.Message.MessageID, text+fmt.Sprintf("\n\n✅ Bon de réception <b>%s</b> enregistré dans l'ERP.", esc(ref.Number)), nil)
			return
		}
		text, _, _ := d.Summary(parts[1])
		var delivery string
		d.DB.QueryRow(`SELECT delivery FROM drafts WHERE id = ?`, parts[1]).Scan(&delivery)
		msg := text + fmt.Sprintf("\n\n✅ Commande <b>%s</b> enregistrée.", esc(ref.Number))
		if delivery != "" {
			msg += "\n🚚 " + esc(delivery)
		}
		d.TG.Edit(ctx, chat, cb.Message.MessageID, msg, nil)
	case "no":
		d.cancel(ctx, chat, parts[1])
		d.TG.Edit(ctx, chat, cb.Message.MessageID, "❌ Commande annulée.", nil)
	}
}

func (d *Desk) ownsLine(key string, l Line) bool {
	var k string
	d.DB.QueryRow(`SELECT chat_key FROM drafts WHERE id = ? AND status = 'open'`, l.DraftID).Scan(&k)
	return k == key
}

// alternatives lists other products a line could be (for a correction).
func (d *Desk) alternatives(l Line) []string {
	var out []string
	words, size := tokensOf(l.Said, l.Hint)
	for _, p := range d.Cat.Shortlist(words, size, nil, nil, map[string]bool{l.ProductID: true}, 5) {
		out = append(out, p.ID)
	}
	return out
}

// Choose sets the product of an asked line (or corrects a resolved one) from a person's answer.
func (d *Desk) Choose(ctx context.Context, l *Line, productID string, correction bool) {
	prev := l.ProductID
	if correction && prev != "" && prev != productID {
		l.Checks["corrected_from"] = prev
		l.Outcome = "corrected"
		d.strike(*l, prev)
	}
	l.ProductID, l.Status, l.Confidence = productID, "ok", 1
	if !correction {
		l.Outcome = "confirmed"
	}
	l.Level = resolve.LPerson
	var customerID, supplierID, kind string
	d.DB.QueryRow(`SELECT customer_id, supplier_id, kind FROM drafts WHERE id = ?`, l.DraftID).Scan(&customerID, &supplierID, &kind)
	if kind == "invoice" {
		delete(l.Checks, "new")
		var last map[string]int64
		if e := d.erp(); e != nil && supplierID != "" {
			last, _ = e.LastPurchasePrices(ctx, supplierID)
		}
		d.invoiceQty(l, last[productID])
		saveLine(d.DB, l)
		return
	}
	d.setPack(l, nil)
	d.price(ctx, customerID, l, d.history(ctx, customerID), d.stock(ctx))
	saveLine(d.DB, l)
}

// Relearn applies a person's correction to a line of an order already confirmed: the ground truth
// changes, the wrong wording is blocked and the right one is witnessed. (The ERP order itself is
// corrected in the ERP.)
func (d *Desk) Relearn(l Line, wrong string) {
	var customerID, supplierID, kind string
	d.DB.QueryRow(`SELECT customer_id, supplier_id, kind FROM drafts WHERE id = ?`, l.DraftID).Scan(&customerID, &supplierID, &kind)
	scopeKind, scopeID := "customer", customerID
	if kind == "invoice" {
		scopeKind, scopeID = "supplier", supplierID
	}
	d.DB.Exec(`UPDATE ground_truth SET product_id = ?, origin = 'clerk' WHERE line_id = ?`, l.ProductID, l.ID)
	if l.Norm == "" {
		return
	}
	if wrong != "" {
		d.Cat.Block(l.Norm, wrong, scopeKind, scopeID)
	}
	a, _ := d.Cat.Witness(l.Norm, l.ProductID, scopeKind, scopeID, "clerk", d.DB.SettingInt("alias_trust_after", 2))
	d.DB.Event("line.corrected", l.ProductID, "clerk", map[string]any{"line": l.ID, "said": l.Said, "wrong": wrong, "alias": a.ID})
}

// strike: a person corrected a line resolved by a live rule or a trusted alias.
func (d *Desk) strike(l Line, wrongProduct string) {
	for _, u := range l.Used {
		if strings.HasPrefix(u, "R-") {
			rules.Bump(d.DB, u, "strikes", 1)
			d.DB.Event("rule.strike", u, "shop", map[string]string{"line": l.ID, "said": l.Said, "wrong": wrongProduct})
			if r, err := rules.Get(d.DB, u); err == nil && r.Status == "live" && r.Strikes >= d.DB.SettingInt("rollback_strikes", 2) {
				rules.SetStatus(d.DB, u, "rolled_back", fmt.Sprintf("%d corrections by people", r.Strikes))
				d.DB.Event("rule.rolled_back", u, "system", map[string]any{"strikes": r.Strikes})
			}
		}
	}
}

func (d *Desk) cancel(ctx context.Context, chat int64, draft string) {
	d.DB.Exec(`UPDATE drafts SET status = 'cancelled', updated_at = ? WHERE id = ?`, store.Now(), draft)
}

// Confirm books the draft in the ERP and teaches the catalogue from every confirmed line.
func (d *Desk) Confirm(ctx context.Context, draft, origin string) (adapter.OrderRef, error) {
	e := d.erp()
	if e == nil {
		return adapter.OrderRef{}, adapter.ErrNotConnected
	}
	var customerID, status, key, kind string
	d.DB.QueryRow(`SELECT customer_id, status, chat_key, kind FROM drafts WHERE id = ?`, draft).Scan(&customerID, &status, &key, &kind)
	if status != "open" {
		return adapter.OrderRef{}, fmt.Errorf("draft is %s", status)
	}
	if kind == "invoice" {
		return d.confirmInvoice(ctx, e, draft, origin)
	}
	lines := DraftLines(d.DB, draft, false)
	o := adapter.Order{CustomerID: customerID, ExternalRef: "orderdesk:" + draft,
		Notes: "Commande reçue par Order Desk (" + strings.SplitN(key, ":", 2)[0] + ")"}
	for _, l := range lines {
		if l.Status == "ok" && l.QtyMilli > 0 {
			note := ""
			if l.Said != "" {
				note = "« " + l.Said + " »"
			}
			o.Lines = append(o.Lines, adapter.OrderLine{ProductID: l.ProductID, QtyMilli: l.QtyMilli, Notes: note})
		}
	}
	if len(o.Lines) == 0 {
		return adapter.OrderRef{}, fmt.Errorf("no confirmed line")
	}
	ref, err := e.CreateOrder(ctx, o)
	if err != nil {
		return ref, err
	}
	d.DB.Exec(`UPDATE drafts SET status = 'confirmed', erp_order_id = ?, erp_order_number = ?, total_ttc = ?, updated_at = ? WHERE id = ?`,
		ref.ID, ref.Number, ref.TotalTTC, store.Now(), draft)
	d.DB.Event("order.created", draft, origin, map[string]any{"number": ref.Number, "lines": len(o.Lines), "erp": e.Name()})
	d.hist.Delete(customerID)
	d.learnLines(lines, "order", "customer", customerID, origin)
	if d.OnLearn != nil {
		go d.OnLearn()
	}
	return ref, nil
}

// learn: every confirmed line becomes ground truth and a witness for its wording; shadow rules
// are compared with the answer; rules that resolved lines get credit.
func (d *Desk) learnLines(lines []Line, kind, scopeKind, scopeID, origin string) {
	customerID, supplierID := "", ""
	if scopeKind == "customer" {
		customerID = scopeID
	} else {
		supplierID = scopeID
	}
	trustAfter := d.DB.SettingInt("alias_trust_after", 2)
	shadowNeeded := d.DB.SettingInt("shadow_agreements", 5)
	for _, l := range lines {
		if l.Status != "ok" || l.Level == resolve.LHistory || l.Norm == "" {
			continue
		}
		if l.Outcome == "" {
			if l.Level == resolve.LAlias || l.Level == resolve.LRules {
				l.Outcome = "auto"
			} else {
				l.Outcome = "confirmed"
			}
			d.DB.Exec(`UPDATE lines SET outcome = ? WHERE id = ?`, l.Outcome, l.ID)
		}
		d.DB.Exec(`INSERT OR REPLACE INTO ground_truth (line_id, said, norm, unit, customer_id, supplier_id, kind, product_id, level, origin, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`, l.ID, l.Said, l.Norm, l.Unit, customerID, supplierID, kind, l.ProductID, l.Level, origin, store.Now())
		var source string
		d.DB.QueryRow(`SELECT kind FROM messages WHERE id = ?`, l.MessageID).Scan(&source)
		if source == "" {
			source = "text"
		}
		if a, change := d.Cat.Witness(l.Norm, l.ProductID, scopeKind, scopeID, source, trustAfter); change != "" {
			d.DB.Event("alias."+change, l.ProductID, origin, map[string]any{"text": a.Text, "alias": a.ID, "witnesses": a.Witnesses})
		}
		for _, u := range l.Used {
			if strings.HasPrefix(u, "R-") {
				rules.Bump(d.DB, u, "lines_resolved", 1)
			}
		}
		for rid, pid := range l.Shadow {
			r, err := rules.Get(d.DB, rid)
			if err != nil || r.Status != "shadow" {
				continue
			}
			if pid == l.ProductID {
				rules.Bump(d.DB, rid, "shadow_agree", 1)
				if r.ShadowOK+1 >= shadowNeeded {
					rules.SetStatus(d.DB, rid, "live", "")
					d.DB.Event("rule.live", rid, "system", map[string]any{"agreements": r.ShadowOK + 1})
				}
			} else {
				rules.SetStatus(d.DB, rid, "rejected", fmt.Sprintf("disagreed in shadow mode on « %s »", l.Said))
				d.DB.Event("rule.rejected", rid, "system", map[string]any{"line": l.ID, "said": l.Said, "stage": "shadow"})
			}
		}
	}
	log.Printf("[desk] learned from %d line(s)", len(lines))
}
