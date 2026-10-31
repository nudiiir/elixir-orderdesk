// Package inventree is the adapter for InvenTree (1.x) through its REST API, with a user's API
// token. Login with a password happens once, to issue a named token (revocable in InvenTree under
// the user's settings → Access tokens); the password is never kept.
//
//	products   Part, salable (+ supplier parts' pack quantities as packs, manufacturer as brand,
//	           parameters "TVA" and "EAN")
//	customers  Company with is_customer; suppliers: is_supplier
//	order      Sales Order, issued; VAT as an extra line, since InvenTree has no taxes
//	receipt    Purchase Order, placed and received into stock when posted
package inventree

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
)

type Client struct {
	BaseURL, Token string
	HTTP           *http.Client

	mu       sync.Mutex
	location int // default stock location, looked up once
}

func New(baseURL, token string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Token: token, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) Name() string { return "InvenTree" }

// Login exchanges a password, once, for a named API token ("orderdesk"). Asking again revokes the
// previous token of that name and issues a new one.
func Login(ctx context.Context, baseURL, user, pass string) (token, name string, err error) {
	base := strings.TrimRight(baseURL, "/")
	hc := &http.Client{Timeout: 60 * time.Second}
	get := func(path, auth string, out any) error {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		req.Header.Set("Authorization", auth)
		req.Header.Set("Accept", "application/json")
		res, err := hc.Do(req)
		if err != nil {
			return fmt.Errorf("ERP unreachable: %w", err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode >= 300 {
			return fmt.Errorf("InvenTree %s: HTTP %d: %s", path, res.StatusCode, apiMessage(raw))
		}
		return json.Unmarshal(raw, out)
	}
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth(user, pass)
	var t struct {
		Token string `json:"token"`
	}
	if err := get("/api/user/me/token/?name=orderdesk", req.Header.Get("Authorization"), &t); err != nil {
		return "", "", fmt.Errorf("InvenTree refused the login: %w", err)
	}
	if t.Token == "" {
		return "", "", errors.New("InvenTree did not return an API token for this user")
	}
	var me struct {
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	get("/api/user/me/", "Token "+t.Token, &me)
	name = strings.TrimSpace(me.FirstName + " " + me.LastName)
	if name == "" {
		name = me.Username
	}
	if name == "" {
		name = user
	}
	return t.Token, name, nil
}

// apiMessage shortens a DRF error body ({"detail": …} or {"field": ["…"]}).
func apiMessage(raw []byte) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) == nil {
		if s, ok := m["detail"].(string); ok {
			return s
		}
	}
	s := string(raw)
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rd)
	req.Header.Set("Authorization", "Token "+c.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("ERP unreachable: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return fmt.Errorf("InvenTree %s %s: not allowed (%d): %s", method, path, res.StatusCode, apiMessage(raw))
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("InvenTree %s %s: HTTP %d: %s", method, path, res.StatusCode, apiMessage(raw))
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// list reads a list endpoint whole: without a limit InvenTree returns a plain array, with one a
// page ({"results": …}); both are accepted.
func (c *Client) list(ctx context.Context, path string, q url.Values) ([]map[string]any, error) {
	if q != nil {
		path += "?" + q.Encode()
	}
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	var rows []map[string]any
	if json.Unmarshal(raw, &rows) == nil {
		return rows, nil
	}
	var page struct {
		Results []map[string]any `json:"results"`
	}
	err := json.Unmarshal(raw, &page)
	return page.Results, err
}

func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/api/user/me/", nil, nil)
}

// ---- reading ---------------------------------------------------------------------------------

func (c *Client) Products(ctx context.Context) ([]catalog.Product, error) {
	parts, err := c.list(ctx, "/api/part/", url.Values{"active": {"true"}, "salable": {"true"}, "ordering": {"name"}})
	if err != nil {
		return nil, err
	}
	// packs: what the part is bought in (a supplier part's pack quantity), one per factor
	packs := map[string][]catalog.Pack{}
	if rows, err := c.list(ctx, "/api/company/part/", url.Values{"active": {"true"}}); err == nil {
		seen := map[string]bool{}
		for _, r := range rows {
			f := num(r["pack_quantity_native"])
			if f <= 1 {
				continue
			}
			id, milli := idStr(r["part"]), int64(math.Round(f*1000))
			if key := fmt.Sprintf("%s|%d", id, milli); !seen[key] {
				seen[key] = true
				name := str(r["packaging"])
				if name == "" {
					name = "x" + str(r["pack_quantity"])
				}
				packs[id] = append(packs[id], catalog.Pack{Name: name, FactorMilli: milli})
			}
		}
	}
	brands := map[string]string{}
	if rows, err := c.list(ctx, "/api/company/part/manufacturer/", url.Values{"manufacturer_detail": {"true"}}); err == nil {
		for _, r := range rows {
			if m, ok := r["manufacturer_detail"].(map[string]any); ok && brands[idStr(r["part"])] == "" {
				brands[idStr(r["part"])] = str(m["name"])
			}
		}
	}
	params, _ := c.partParameters(ctx)
	out := make([]catalog.Product, 0, len(parts))
	for _, p := range parts {
		id := idStr(p["pk"])
		pr := catalog.Product{ID: id, Name: str(p["name"]), Brand: brands[id], Unit: str(p["units"]), Packs: packs[id],
			Barcode: params[id]["EAN"], VatRate: vatRate(params[id]["TVA"])}
		sort.Slice(pr.Packs, func(i, j int) bool { return pr.Packs[i].FactorMilli < pr.Packs[j].FactorMilli })
		out = append(out, pr)
	}
	return out, nil
}

// partParameters returns part → parameter name → value (e.g. "TVA" → "19").
func (c *Client) partParameters(ctx context.Context) (map[string]map[string]string, error) {
	rows, err := c.list(ctx, "/api/parameter/", url.Values{"model_type": {"part.part"}})
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]string{}
	for _, r := range rows {
		t, _ := r["template_detail"].(map[string]any)
		id := idStr(r["model_id"])
		if out[id] == nil {
			out[id] = map[string]string{}
		}
		out[id][str(t["name"])] = str(r["data"])
	}
	return out, nil
}

// vatRate reads a "TVA" parameter ("19", "19 %", "9,5") as an Order Desk rate (19 % → 1900).
func vatRate(s string) int64 {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(f * 100))
}

func (c *Client) vatOf(ctx context.Context, partID string) int64 {
	rows, err := c.list(ctx, "/api/parameter/", url.Values{"model_type": {"part.part"}, "model_id": {partID}})
	if err != nil {
		return 0
	}
	for _, r := range rows {
		if t, _ := r["template_detail"].(map[string]any); str(t["name"]) == "TVA" {
			return vatRate(str(r["data"]))
		}
	}
	return 0
}

func (c *Client) Customers(ctx context.Context) ([]catalog.Customer, error) {
	return c.companies(ctx, "is_customer")
}

func (c *Client) Suppliers(ctx context.Context) ([]catalog.Customer, error) {
	return c.companies(ctx, "is_supplier")
}

func (c *Client) companies(ctx context.Context, flag string) ([]catalog.Customer, error) {
	rows, err := c.list(ctx, "/api/company/", url.Values{flag: {"true"}, "active": {"true"}, "ordering": {"name"}})
	if err != nil {
		return nil, err
	}
	out := make([]catalog.Customer, 0, len(rows))
	for _, r := range rows {
		out = append(out, catalog.Customer{ID: idStr(r["pk"]), Name: str(r["name"]), Phone: str(r["phone"])})
	}
	return out, nil
}

// Price is the part's sale price break for the quantity (InvenTree has no customer prices) and
// its "TVA" parameter.
func (c *Client) Price(ctx context.Context, customerID, productID string, qtyMilli int64) (adapter.Quote, error) {
	unit, err := c.salePrice(ctx, productID, qtyMilli)
	if err != nil || unit == 0 {
		return adapter.Quote{}, err
	}
	rate := c.vatOf(ctx, productID)
	ht := float64(unit) * float64(qtyMilli) / 1000
	return adapter.Quote{UnitPriceHT: unit, LineTTC: int64(math.Round(ht * (1 + float64(rate)/10000))), Source: "InvenTree sale price"}, nil
}

// salePrice: the price of the largest sale price break not above the quantity, centimes HT.
func (c *Client) salePrice(ctx context.Context, partID string, qtyMilli int64) (int64, error) {
	rows, err := c.list(ctx, "/api/part/sale-price/", url.Values{"part": {partID}})
	if err != nil {
		return 0, err
	}
	best, bestQ := -1.0, -1.0
	for _, r := range rows {
		q := num(r["quantity"])
		if (q*1000 <= float64(qtyMilli) || bestQ < 0) && q > bestQ {
			best, bestQ = num(r["price"]), q
		}
	}
	if best < 0 {
		return 0, nil
	}
	return int64(math.Round(best * 100)), nil
}

func (c *Client) Stock(ctx context.Context) (map[string]int64, error) {
	rows, err := c.list(ctx, "/api/part/", url.Values{"active": {"true"}, "salable": {"true"}})
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[idStr(r["pk"])] = int64(math.Round(num(r["total_in_stock"]) * 1000))
	}
	return out, nil
}

// Sales order statuses that count as a real order: in progress, shipped, complete.
var soDone = map[int]bool{15: true, 20: true, 30: true}

func (c *Client) History(ctx context.Context, customerID string, n int) ([]adapter.PastOrder, error) {
	orders, err := c.list(ctx, "/api/order/so/", url.Values{"customer": {customerID}, "ordering": {"-reference"}, "limit": {strconv.Itoa(4 * n)}})
	if err != nil {
		return nil, err
	}
	var out []adapter.PastOrder
	for _, o := range orders {
		if !soDone[int(num(o["status"]))] {
			continue
		}
		id := idStr(o["pk"])
		date := str(o["issue_date"])
		if date == "" {
			date = str(o["creation_date"])
		}
		po := adapter.PastOrder{ID: id, Number: str(o["reference"]), Date: date}
		lines, err := c.list(ctx, "/api/order/so-line/", url.Values{"order": {id}})
		if err != nil {
			return out, err
		}
		for _, l := range lines {
			po.Lines = append(po.Lines, adapter.PastLine{ProductID: idStr(l["part"]), QtyMilli: int64(math.Round(num(l["quantity"]) * 1000))})
		}
		out = append(out, po)
		if len(out) == n {
			break
		}
	}
	return out, nil
}

func (c *Client) NextDelivery(ctx context.Context, customerID string) (adapter.Delivery, error) {
	return adapter.Delivery{}, nil // no delivery rounds in InvenTree
}

func (c *Client) OrderURL(ref adapter.OrderRef) string {
	if ref.ID == "" {
		return ""
	}
	return c.BaseURL + "/web/sales/sales-order/" + ref.ID
}

// ---- writing ---------------------------------------------------------------------------------

func (c *Client) create(ctx context.Context, path string, body map[string]any) (int, map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return 0, nil, err
	}
	return int(num(out["pk"])), out, nil
}

// vatLines adds the VAT of an order as extra lines, one per rate ("TVA 19 %"), so that InvenTree's
// own order total is the TTC total. base: rate → HT total in centimes.
func (c *Client) vatLines(ctx context.Context, path string, order int, base map[int64]float64) error {
	rates := make([]int64, 0, len(base))
	for r := range base {
		rates = append(rates, r)
	}
	sort.Slice(rates, func(i, j int) bool { return rates[i] < rates[j] })
	for _, r := range rates {
		vat := math.Round(base[r] * float64(r) / 10000)
		if r <= 0 || vat == 0 {
			continue
		}
		label := "TVA " + strconv.FormatFloat(float64(r)/100, 'f', -1, 64) + " %"
		if _, _, err := c.create(ctx, path, map[string]any{"order": order, "reference": label, "quantity": 1,
			"price": vat / 100, "description": label + " sur " + money(base[r])}); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) total(ctx context.Context, path string, id int) (string, int64, error) {
	var o map[string]any
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s%d/", path, id), nil, &o); err != nil {
		return "", 0, err
	}
	return str(o["reference"]), int64(math.Round(num(o["total_price"]) * 100)), nil
}

// note attaches the text as the object's primary note (since InvenTree 1.6 notes are their own model).
func (c *Client) note(ctx context.Context, modelType string, id int, title, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	c.do(ctx, http.MethodPost, "/api/note/", map[string]any{"model_type": modelType, "model_id": id, "primary": true,
		"title": title, "content": text}, nil)
}

// CreateOrder creates and issues a sales order at InvenTree's sale prices, with the VAT as an extra
// line. The shop's words go in the order's notes and on each line.
func (c *Client) CreateOrder(ctx context.Context, o adapter.Order) (adapter.OrderRef, error) {
	cid, err := strconv.Atoi(o.CustomerID)
	if err != nil {
		return adapter.OrderRef{}, fmt.Errorf("customer id: %w", err)
	}
	type line struct {
		part int
		unit int64
		vat  int64
	}
	lines := make([]line, 0, len(o.Lines))
	for _, l := range o.Lines {
		pid, err := strconv.Atoi(l.ProductID)
		if err != nil {
			return adapter.OrderRef{}, fmt.Errorf("product id %q: %w", l.ProductID, err)
		}
		unit, err := c.salePrice(ctx, l.ProductID, l.QtyMilli)
		if err != nil {
			return adapter.OrderRef{}, err
		}
		lines = append(lines, line{pid, unit, c.vatOf(ctx, l.ProductID)})
	}
	so := map[string]any{"customer": cid, "description": cut(firstLine(o.Notes), 250), "target_date": time.Now().AddDate(0, 0, 1).Format("2006-01-02")}
	if o.ExternalRef != "" {
		so["customer_reference"] = cut(o.ExternalRef, 64)
	}
	id, _, err := c.create(ctx, "/api/order/so/", so)
	if err != nil {
		return adapter.OrderRef{}, err
	}
	ref := adapter.OrderRef{ID: strconv.Itoa(id)}
	base := map[int64]float64{}
	for i, l := range o.Lines {
		body := map[string]any{"order": id, "part": lines[i].part, "quantity": float64(l.QtyMilli) / 1000}
		if lines[i].unit > 0 {
			body["sale_price"] = float64(lines[i].unit) / 100
			base[lines[i].vat] += float64(lines[i].unit) * float64(l.QtyMilli) / 1000
		}
		if l.Notes != "" {
			body["notes"] = cut(l.Notes, 500)
		}
		if _, _, err := c.create(ctx, "/api/order/so-line/", body); err != nil {
			return ref, fmt.Errorf("sales order %d created, but a line was refused: %w", id, err)
		}
	}
	if err := c.vatLines(ctx, "/api/order/so-extra-line/", id, base); err != nil {
		return ref, fmt.Errorf("sales order %d created, but its VAT line was refused: %w", id, err)
	}
	c.note(ctx, "order.salesorder", id, "Order Desk", o.Notes)
	if err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/order/so/%d/issue/", id), map[string]any{}, nil); err != nil {
		return ref, fmt.Errorf("sales order %d created as pending, but issuing failed: %w", id, err)
	}
	ref.Number, ref.TotalTTC, err = c.total(ctx, "/api/order/so/", id)
	return ref, err
}

// LastPurchasePrices: the last price received from this supplier per part (purchase order lines,
// newest order first), else the supplier's price breaks. Centimes HT per base unit (a pack price
// divided by the supplier part's pack quantity).
func (c *Client) LastPurchasePrices(ctx context.Context, supplierID string) (map[string]int64, error) {
	out := map[string]int64{}
	packOf := map[string]float64{} // supplier part → pack quantity
	sparts, err := c.list(ctx, "/api/company/part/", url.Values{"supplier": {supplierID}})
	if err != nil {
		return nil, err
	}
	partOf := map[string]string{}
	for _, s := range sparts {
		packOf[idStr(s["pk"])] = num(s["pack_quantity_native"])
		partOf[idStr(s["pk"])] = idStr(s["part"])
	}
	perUnit := func(price float64, sp string) int64 {
		f := packOf[sp]
		if f <= 0 {
			f = 1
		}
		return int64(math.Round(price / f * 100))
	}
	orders, err := c.list(ctx, "/api/order/po/", url.Values{"supplier": {supplierID}, "ordering": {"-reference"}, "limit": {"30"}})
	if err != nil {
		return nil, err
	}
	for _, o := range orders {
		if s := int(num(o["status"])); s != 20 && s != 30 { // placed or complete
			continue
		}
		lines, err := c.list(ctx, "/api/order/po-line/", url.Values{"order": {idStr(o["pk"])}})
		if err != nil {
			return out, err
		}
		for _, l := range lines {
			sp := idStr(l["part"])
			if p := partOf[sp]; p != "" && num(l["received"]) > 0 && l["purchase_price"] != nil {
				if _, seen := out[p]; !seen {
					out[p] = perUnit(num(l["purchase_price"]), sp)
				}
			}
		}
	}
	breaks, err := c.list(ctx, "/api/company/price-break/", url.Values{"supplier": {supplierID}, "ordering": {"quantity"}})
	if err == nil {
		for _, b := range breaks {
			sp := idStr(b["part"])
			if p := partOf[sp]; p != "" {
				if _, seen := out[p]; !seen {
					out[p] = perUnit(num(b["price"]), sp)
				}
			}
		}
	}
	return out, nil
}

// supplierPart finds the supplier's part for this product and pack quantity, else creates it: a
// line counted in packs is ordered in packs at the pack price, as printed, and InvenTree turns the
// packs into units when it receives them.
func (c *Client) supplierPart(ctx context.Context, supplier int, partID string, factorMilli int64, desc string) (int, error) {
	rows, err := c.list(ctx, "/api/company/part/", url.Values{"supplier": {strconv.Itoa(supplier)}, "part": {partID}})
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		if int64(math.Round(num(r["pack_quantity_native"])*1000)) == factorMilli {
			return int(num(r["pk"])), nil
		}
	}
	pack := strconv.FormatFloat(float64(factorMilli)/1000, 'f', -1, 64)
	var p map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/part/"+partID+"/", nil, &p); err != nil {
		return 0, err
	}
	sku := str(p["IPN"])
	if sku == "" {
		sku = "P" + partID
	}
	body := map[string]any{"part": partID, "supplier": supplier, "SKU": cut(sku+"-X"+pack, 100), "pack_quantity": pack, "description": cut(desc, 250)}
	if factorMilli > 1000 {
		body["packaging"] = "Colis x" + pack
	}
	id, _, err := c.create(ctx, "/api/company/part/", body)
	return id, err
}

// defaultLocation is where received stock goes: the part's default location, else the first one.
func (c *Client) defaultLocation(ctx context.Context, partID string) int {
	var p map[string]any
	if c.do(ctx, http.MethodGet, "/api/part/"+partID+"/", nil, &p) == nil && num(p["default_location"]) > 0 {
		return int(num(p["default_location"]))
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.location == 0 {
		if rows, err := c.list(ctx, "/api/stock/location/", url.Values{"ordering": {"pk"}, "limit": {"1"}}); err == nil && len(rows) > 0 {
			c.location = int(num(rows[0]["pk"]))
		}
	}
	return c.location
}

// CreateReceipt books the approved supplier invoice as a purchase order (VAT as an extra line) and,
// when Post is set, places it, receives every line into stock and completes it: InvenTree moves the
// stock and keeps the purchase price per unit on the stock items.
func (c *Client) CreateReceipt(ctx context.Context, r adapter.Receipt) (adapter.OrderRef, error) {
	sid, err := strconv.Atoi(r.SupplierID)
	if err != nil {
		return adapter.OrderRef{}, fmt.Errorf("supplier id: %w", err)
	}
	type line struct {
		spart, location int
		qty, price      float64
	}
	lines := make([]line, 0, len(r.Lines))
	base := map[int64]float64{}
	for _, l := range r.Lines {
		if _, err := strconv.Atoi(l.ProductID); err != nil {
			return adapter.OrderRef{}, fmt.Errorf("product id %q: %w", l.ProductID, err)
		}
		factor := l.PackFactorMilli
		if factor <= 1000 {
			factor = 1000 // the line is counted in base units
		}
		sp, err := c.supplierPart(ctx, sid, l.ProductID, factor, l.Description)
		if err != nil {
			return adapter.OrderRef{}, err
		}
		lines = append(lines, line{sp, c.defaultLocation(ctx, l.ProductID), float64(l.QtyMilli) / 1000, float64(l.UnitCostHT) / 100})
		base[l.VatRate] += float64(l.UnitCostHT) * float64(l.QtyMilli) / 1000
	}
	desc := "Facture fournisseur"
	if r.SupplierRef != "" {
		desc += " " + r.SupplierRef
	}
	if r.Date != "" {
		desc += " du " + r.Date
	}
	po := map[string]any{"supplier": sid, "description": cut(desc, 250)}
	if r.SupplierRef != "" {
		po["supplier_reference"] = cut(r.SupplierRef, 64)
	}
	if r.Date != "" {
		po["target_date"] = r.Date
	}
	if len(lines) > 0 && lines[0].location > 0 {
		po["destination"] = lines[0].location
	}
	id, _, err := c.create(ctx, "/api/order/po/", po)
	if err != nil {
		return adapter.OrderRef{}, err
	}
	ref := adapter.OrderRef{ID: strconv.Itoa(id)}
	lineIDs := make([]int, 0, len(lines))
	for i, l := range lines {
		body := map[string]any{"order": id, "part": l.spart, "quantity": l.qty, "purchase_price": l.price, "auto_pricing": false}
		if d := r.Lines[i].Description; d != "" {
			body["reference"] = cut(d, 100)
		}
		lid, _, err := c.create(ctx, "/api/order/po-line/", body)
		if err != nil {
			return ref, fmt.Errorf("purchase order %d created, but a line was refused: %w", id, err)
		}
		lineIDs = append(lineIDs, lid)
	}
	if err := c.vatLines(ctx, "/api/order/po-extra-line/", id, base); err != nil {
		return ref, fmt.Errorf("purchase order %d created, but its VAT line was refused: %w", id, err)
	}
	var notes []string
	if r.Notes != "" {
		notes = append(notes, r.Notes)
	}
	for _, l := range r.Lines {
		if l.Description != "" {
			notes = append(notes, "« "+l.Description+" »")
		}
	}
	c.note(ctx, "order.purchaseorder", id, "Facture lue par Order Desk", strings.Join(notes, "\n\n"))
	suffix := " (brouillon)"
	if r.Post {
		if err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/order/po/%d/issue/", id), map[string]any{}, nil); err != nil {
			return ref, fmt.Errorf("purchase order %d created as pending, but placing it failed: %w", id, err)
		}
		items := make([]any, 0, len(lines))
		for i, l := range lines {
			it := map[string]any{"line_item": lineIDs[i], "quantity": l.qty, "status": 10} // 10 = OK
			if l.location > 0 {
				it["location"] = l.location
			}
			items = append(items, it)
		}
		recv := map[string]any{"items": items}
		if len(lines) > 0 && lines[0].location > 0 {
			recv["location"] = lines[0].location
		}
		if err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/order/po/%d/receive/", id), recv, nil); err != nil {
			ref.Number, ref.TotalTTC, _ = c.total(ctx, "/api/order/po/", id)
			return ref, fmt.Errorf("purchase order placed, but the goods were not received: %w", err)
		}
		c.do(ctx, http.MethodPost, fmt.Sprintf("/api/order/po/%d/complete/", id), map[string]any{"accept_incomplete": true}, nil)
		suffix = ""
	}
	ref.Number, ref.TotalTTC, err = c.total(ctx, "/api/order/po/", id)
	if r.SupplierRef != "" {
		ref.Number += " · " + r.SupplierRef
	}
	ref.Number += suffix
	return ref, err
}

// ---- helpers -----------------------------------------------------------------------------------

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string: // decimals may come back as strings
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

// idStr renders a primary key (a JSON number) as Order Desk's string id.
func idStr(v any) string {
	if f, ok := v.(float64); ok {
		return strconv.FormatInt(int64(f), 10)
	}
	return str(v)
}

func money(centimes float64) string { return strconv.FormatFloat(centimes/100, 'f', 2, 64) }

func firstLine(s string) string {
	return strings.TrimSpace(strings.SplitN(strings.TrimSpace(s), "\n", 2)[0])
}

// cut shortens s to n characters (not bytes), for InvenTree's length-limited fields.
func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

var _ adapter.ERP = (*Client)(nil)
