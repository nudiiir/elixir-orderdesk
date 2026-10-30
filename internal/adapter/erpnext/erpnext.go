// Package erpnext is the adapter for ERPNext (v14–v16) through Frappe's REST API, with a user's
// API key and secret. Login with a password happens once, to generate the keys; the password is
// never kept.
//
//	products   Item (+ UOM conversions as packs, barcodes, brand)
//	customers  Customer; suppliers: Supplier
//	order      Sales Order, submitted; the shop's words as a comment
//	receipt    Purchase Receipt, submitted when posted (stock in)
package erpnext

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
)

type Client struct {
	BaseURL, Key, Secret string
	HTTP                 *http.Client

	mu       sync.Mutex
	defaults map[string]string
}

func New(baseURL, key, secret string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Key: key, Secret: secret, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) Name() string { return "ERPNext" }

// Login opens a session with a password once and generates the user's API key and secret.
// (Generating a new secret replaces the user's previous one.)
func Login(ctx context.Context, baseURL, user, pass string) (key, secret, name string, err error) {
	base := strings.TrimRight(baseURL, "/")
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Timeout: 60 * time.Second, Jar: jar}
	form := func(path string, v url.Values) (map[string]any, error) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+path, strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		res, err := hc.Do(req)
		if err != nil {
			return nil, fmt.Errorf("ERP unreachable: %w", err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var out map[string]any
		json.Unmarshal(raw, &out)
		if res.StatusCode >= 300 {
			return out, fmt.Errorf("ERPNext %s: HTTP %d: %s", path, res.StatusCode, frappeMessage(out, raw))
		}
		return out, nil
	}
	r, err := form("/api/method/login", url.Values{"usr": {user}, "pwd": {pass}})
	if err != nil {
		return "", "", "", fmt.Errorf("ERPNext refused the login: %w", err)
	}
	name, _ = r["full_name"].(string)
	g, err := form("/api/method/frappe.core.doctype.user.user.generate_keys", url.Values{"user": {user}})
	if err != nil {
		return "", "", "", err
	}
	if m, ok := g["message"].(map[string]any); ok {
		secret, _ = m["api_secret"].(string)
		key, _ = m["api_key"].(string)
	}
	if key == "" {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/resource/User/"+url.PathEscape(user), nil)
		req.Header.Set("Accept", "application/json")
		if res, err := hc.Do(req); err == nil {
			var u struct {
				Data struct {
					APIKey string `json:"api_key"`
				} `json:"data"`
			}
			json.NewDecoder(res.Body).Decode(&u)
			res.Body.Close()
			key = u.Data.APIKey
		}
	}
	if key == "" || secret == "" {
		return "", "", "", errors.New("ERPNext did not return an API key for this user")
	}
	if name == "" {
		name = user
	}
	return key, secret, name, nil
}

func frappeMessage(out map[string]any, raw []byte) string {
	for _, k := range []string{"exception", "message", "_server_messages"} {
		if s, ok := out[k].(string); ok && s != "" {
			if len(s) > 300 {
				s = s[:300]
			}
			return s
		}
	}
	if len(raw) > 200 {
		raw = raw[:200]
	}
	return string(raw)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rd)
	req.Header.Set("Authorization", "token "+c.Key+":"+c.Secret)
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
		var m map[string]any
		json.Unmarshal(raw, &m)
		return fmt.Errorf("ERPNext %s %s: not allowed (%d): %s", method, path, res.StatusCode, frappeMessage(m, raw))
	}
	if res.StatusCode >= 300 {
		var m map[string]any
		json.Unmarshal(raw, &m)
		return fmt.Errorf("ERPNext %s %s: HTTP %d: %s", method, path, res.StatusCode, frappeMessage(m, raw))
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// list reads a doctype (or, with parent set, one of its child tables) through frappe.client.get_list.
func (c *Client) list(ctx context.Context, doctype, parent string, fields []string, filters []any, orderBy string, limit int) ([]map[string]any, error) {
	q := url.Values{"doctype": {doctype}, "fields": {jsonStr(fields)}, "limit_page_length": {fmt.Sprint(limit)}}
	if parent != "" {
		q.Set("parent", parent)
	}
	if len(filters) > 0 {
		q.Set("filters", jsonStr(filters))
	}
	if orderBy != "" {
		q.Set("order_by", orderBy)
	}
	var out struct {
		Message []map[string]any `json:"message"`
	}
	err := c.do(ctx, http.MethodGet, "/api/method/frappe.client.get_list?"+q.Encode(), nil, &out)
	return out.Message, err
}

func (c *Client) value(ctx context.Context, doctype, field string) string {
	c.mu.Lock()
	if v, ok := c.defaults[doctype+"."+field]; ok {
		c.mu.Unlock()
		return v
	}
	c.mu.Unlock()
	q := url.Values{"doctype": {doctype}, "fieldname": {field}}
	var out struct {
		Message map[string]any `json:"message"`
	}
	c.do(ctx, http.MethodGet, "/api/method/frappe.client.get_value?"+q.Encode(), nil, &out)
	v, _ := out.Message[field].(string)
	c.mu.Lock()
	if c.defaults == nil {
		c.defaults = map[string]string{}
	}
	c.defaults[doctype+"."+field] = v
	c.mu.Unlock()
	return v
}

func jsonStr(v any) string { b, _ := json.Marshal(v); return string(b) }

func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/api/method/frappe.auth.get_logged_user", nil, nil)
}

// ---- reading ---------------------------------------------------------------------------------

func (c *Client) Products(ctx context.Context) ([]catalog.Product, error) {
	items, err := c.list(ctx, "Item", "", []string{"name", "item_name", "brand", "stock_uom"},
		[]any{[]any{"disabled", "=", 0}, []any{"is_sales_item", "=", 1}}, "item_name asc", 0)
	if err != nil {
		return nil, err
	}
	packs := map[string][]catalog.Pack{}
	if rows, err := c.list(ctx, "UOM Conversion Detail", "Item", []string{"parent", "uom", "conversion_factor"}, nil, "", 0); err == nil {
		for _, r := range rows {
			if f := num(r["conversion_factor"]); f > 1 {
				p := str(r["parent"])
				packs[p] = append(packs[p], catalog.Pack{Name: str(r["uom"]), FactorMilli: int64(math.Round(f * 1000))})
			}
		}
	}
	barcodes := map[string]string{}
	if rows, err := c.list(ctx, "Item Barcode", "Item", []string{"parent", "barcode"}, nil, "", 0); err == nil {
		for _, r := range rows {
			if barcodes[str(r["parent"])] == "" {
				barcodes[str(r["parent"])] = str(r["barcode"])
			}
		}
	}
	out := make([]catalog.Product, 0, len(items))
	for _, it := range items {
		id := str(it["name"])
		p := catalog.Product{ID: id, Name: str(it["item_name"]), Brand: str(it["brand"]), Unit: str(it["stock_uom"]), Barcode: barcodes[id], Packs: packs[id]}
		if p.Name == "" {
			p.Name = id
		}
		sort.Slice(p.Packs, func(i, j int) bool { return p.Packs[i].FactorMilli < p.Packs[j].FactorMilli })
		out = append(out, p)
	}
	return out, nil
}

func (c *Client) Customers(ctx context.Context) ([]catalog.Customer, error) {
	return c.parties(ctx, "Customer", "customer_name")
}

func (c *Client) Suppliers(ctx context.Context) ([]catalog.Customer, error) {
	return c.parties(ctx, "Supplier", "supplier_name")
}

func (c *Client) parties(ctx context.Context, doctype, nameField string) ([]catalog.Customer, error) {
	rows, err := c.list(ctx, doctype, "", []string{"name", nameField, "mobile_no"}, []any{[]any{"disabled", "=", 0}}, nameField+" asc", 0)
	if err != nil {
		return nil, err
	}
	out := make([]catalog.Customer, 0, len(rows))
	for _, r := range rows {
		out = append(out, catalog.Customer{ID: str(r["name"]), Name: str(r[nameField]), Phone: str(r["mobile_no"])})
	}
	return out, nil
}

// Price reads the item's rate in the customer's price list (else the default selling list).
func (c *Client) Price(ctx context.Context, customerID, productID string, qtyMilli int64) (adapter.Quote, error) {
	list := ""
	if rows, err := c.list(ctx, "Customer", "", []string{"default_price_list"}, []any{[]any{"name", "=", customerID}}, "", 1); err == nil && len(rows) > 0 {
		list = str(rows[0]["default_price_list"])
	}
	if list == "" {
		list = c.value(ctx, "Selling Settings", "selling_price_list")
	}
	if list == "" {
		list = "Standard Selling"
	}
	rows, err := c.list(ctx, "Item Price", "", []string{"price_list_rate"}, []any{[]any{"item_code", "=", productID}, []any{"price_list", "=", list}}, "valid_from desc", 1)
	if err != nil || len(rows) == 0 {
		return adapter.Quote{}, err
	}
	unit := int64(math.Round(num(rows[0]["price_list_rate"]) * 100))
	return adapter.Quote{UnitPriceHT: unit, LineTTC: unit * qtyMilli / 1000, Source: "ERPNext " + list}, nil
}

func (c *Client) Stock(ctx context.Context) (map[string]int64, error) {
	rows, err := c.list(ctx, "Bin", "", []string{"item_code", "actual_qty"}, nil, "", 0)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[str(r["item_code"])] += int64(math.Round(num(r["actual_qty"]) * 1000))
	}
	return out, nil
}

func (c *Client) History(ctx context.Context, customerID string, n int) ([]adapter.PastOrder, error) {
	orders, err := c.list(ctx, "Sales Order", "", []string{"name", "transaction_date"}, []any{[]any{"customer", "=", customerID}, []any{"docstatus", "=", 1}},
		"transaction_date desc, creation desc", n)
	if err != nil || len(orders) == 0 {
		return nil, err
	}
	names := []any{}
	for _, o := range orders {
		names = append(names, o["name"])
	}
	items, err := c.list(ctx, "Sales Order Item", "Sales Order", []string{"parent", "item_code", "stock_qty"}, []any{[]any{"parent", "in", names}}, "", 0)
	if err != nil {
		return nil, err
	}
	byOrder := map[string][]adapter.PastLine{}
	for _, it := range items {
		byOrder[str(it["parent"])] = append(byOrder[str(it["parent"])], adapter.PastLine{ProductID: str(it["item_code"]), QtyMilli: int64(math.Round(num(it["stock_qty"]) * 1000))})
	}
	var out []adapter.PastOrder
	for _, o := range orders {
		out = append(out, adapter.PastOrder{ID: str(o["name"]), Number: str(o["name"]), Date: str(o["transaction_date"]), Lines: byOrder[str(o["name"])]})
	}
	return out, nil
}

func (c *Client) NextDelivery(ctx context.Context, customerID string) (adapter.Delivery, error) {
	return adapter.Delivery{}, nil // no delivery rounds in standard ERPNext
}

func (c *Client) OrderURL(ref adapter.OrderRef) string {
	if ref.ID == "" {
		return ""
	}
	return c.BaseURL + "/app/sales-order/" + url.PathEscape(ref.ID)
}

// ---- writing ---------------------------------------------------------------------------------

type doc struct {
	Name       string  `json:"name"`
	GrandTotal float64 `json:"grand_total"`
}

func (c *Client) insertSubmit(ctx context.Context, doctype string, body map[string]any, submit bool) (doc, error) {
	var created struct {
		Data doc `json:"data"`
	}
	path := "/api/resource/" + url.PathEscape(doctype)
	if err := c.do(ctx, http.MethodPost, path, body, &created); err != nil {
		return doc{}, err
	}
	if !submit {
		return created.Data, nil
	}
	var sub struct {
		Data doc `json:"data"`
	}
	if err := c.do(ctx, http.MethodPut, path+"/"+url.PathEscape(created.Data.Name), map[string]any{"docstatus": 1}, &sub); err != nil {
		return created.Data, fmt.Errorf("%s %s saved as a draft, but submitting failed: %w", doctype, created.Data.Name, err)
	}
	return sub.Data, nil
}

func (c *Client) comment(ctx context.Context, doctype, name, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	c.do(ctx, http.MethodPost, "/api/resource/Comment", map[string]any{"comment_type": "Comment", "reference_doctype": doctype,
		"reference_name": name, "content": strings.ReplaceAll(text, "\n", "<br>")}, nil)
}

// CreateOrder creates and submits a sales order; the shop's words are added as a comment.
func (c *Client) CreateOrder(ctx context.Context, o adapter.Order) (adapter.OrderRef, error) {
	delivery := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	items := []any{}
	note := o.Notes
	for _, l := range o.Lines {
		items = append(items, map[string]any{"item_code": l.ProductID, "qty": float64(l.QtyMilli) / 1000, "delivery_date": delivery})
		if l.Notes != "" {
			note += "\n" + l.Notes
		}
	}
	body := map[string]any{"customer": o.CustomerID, "transaction_date": time.Now().Format("2006-01-02"), "delivery_date": delivery, "items": items}
	if wh := c.value(ctx, "Stock Settings", "default_warehouse"); wh != "" {
		body["set_warehouse"] = wh
	}
	if o.ExternalRef != "" {
		body["po_no"] = o.ExternalRef
	}
	d, err := c.insertSubmit(ctx, "Sales Order", body, true)
	if err != nil {
		return adapter.OrderRef{ID: d.Name, Number: d.Name}, err
	}
	c.comment(ctx, "Sales Order", d.Name, note)
	return adapter.OrderRef{ID: d.Name, Number: d.Name, TotalTTC: int64(math.Round(d.GrandTotal * 100))}, nil
}

// LastPurchasePrices: the newest receipt price per item from this supplier, else its buying price
// list. Centimes per stock unit.
func (c *Client) LastPurchasePrices(ctx context.Context, supplierID string) (map[string]int64, error) {
	out := map[string]int64{}
	recs, err := c.list(ctx, "Purchase Receipt", "", []string{"name"}, []any{[]any{"supplier", "=", supplierID}, []any{"docstatus", "=", 1}},
		"posting_date desc, creation desc", 100)
	if err != nil {
		return nil, err
	}
	if len(recs) > 0 {
		names := []any{}
		order := map[string]int{}
		for i, r := range recs {
			names = append(names, r["name"])
			order[str(r["name"])] = i
		}
		items, err := c.list(ctx, "Purchase Receipt Item", "Purchase Receipt", []string{"parent", "item_code", "rate", "conversion_factor"},
			[]any{[]any{"parent", "in", names}}, "", 0)
		if err == nil {
			sort.SliceStable(items, func(i, j int) bool { return order[str(items[i]["parent"])] < order[str(items[j]["parent"])] })
			for _, it := range items {
				code := str(it["item_code"])
				if _, seen := out[code]; seen {
					continue
				}
				f := num(it["conversion_factor"])
				if f <= 0 {
					f = 1
				}
				out[code] = int64(math.Round(num(it["rate"]) / f * 100))
			}
		}
	}
	prices, err := c.list(ctx, "Item Price", "", []string{"item_code", "price_list_rate"}, []any{[]any{"supplier", "=", supplierID}, []any{"buying", "=", 1}}, "", 0)
	if err == nil {
		for _, p := range prices {
			if _, seen := out[str(p["item_code"])]; !seen {
				out[str(p["item_code"])] = int64(math.Round(num(p["price_list_rate"]) * 100))
			}
		}
	}
	return out, nil
}

// CreateReceipt books the approved supplier invoice as a purchase receipt, submitted when Post is
// set: ERPNext moves the stock and values it at the invoice's prices.
func (c *Client) CreateReceipt(ctx context.Context, r adapter.Receipt) (adapter.OrderRef, error) {
	items := []any{}
	wh := c.value(ctx, "Stock Settings", "default_warehouse")
	// A line counted in packs is booked in the item's pack unit at the pack price, as printed: a price
	// per unit would be rounded to the currency's precision (123.45 / 24 → 5.14).
	codes := []any{}
	for _, l := range r.Lines {
		codes = append(codes, l.ProductID)
	}
	packUOM := map[string]string{} // item + factor → UOM
	if rows, err := c.list(ctx, "UOM Conversion Detail", "Item", []string{"parent", "uom", "conversion_factor"}, []any{[]any{"parent", "in", codes}}, "", 0); err == nil {
		for _, u := range rows {
			packUOM[fmt.Sprintf("%s|%d", str(u["parent"]), int64(math.Round(num(u["conversion_factor"])*1000)))] = str(u["uom"])
		}
	}
	for _, l := range r.Lines {
		factor := float64(l.PackFactorMilli) / 1000
		if factor <= 0 {
			factor = 1
		}
		var it map[string]any
		if uom := packUOM[fmt.Sprintf("%s|%d", l.ProductID, l.PackFactorMilli)]; uom != "" && factor > 1 {
			it = map[string]any{"item_code": l.ProductID, "qty": float64(l.QtyMilli) / 1000, "uom": uom, "conversion_factor": factor,
				"rate": float64(l.UnitCostHT) / 100}
		} else {
			it = map[string]any{"item_code": l.ProductID, "qty": float64(l.QtyMilli) / 1000 * factor,
				"rate": math.Round(float64(l.UnitCostHT)/100/factor*10000) / 10000}
		}
		if wh != "" {
			it["warehouse"] = wh
		}
		items = append(items, it)
	}
	body := map[string]any{"supplier": r.SupplierID, "posting_date": r.Date, "set_posting_time": 1, "items": items}
	if wh != "" {
		body["set_warehouse"] = wh
	}
	if r.SupplierRef != "" {
		body["supplier_delivery_note"] = r.SupplierRef
	}
	d, err := c.insertSubmit(ctx, "Purchase Receipt", body, r.Post)
	if err != nil {
		return adapter.OrderRef{ID: d.Name, Number: d.Name}, err
	}
	var lines []string
	for _, l := range r.Lines {
		if l.Description != "" {
			lines = append(lines, "« "+l.Description+" »")
		}
	}
	c.comment(ctx, "Purchase Receipt", d.Name, strings.TrimSpace(r.Notes+"\n"+strings.Join(lines, "\n")))
	ref := adapter.OrderRef{ID: d.Name, Number: d.Name, TotalTTC: int64(math.Round(d.GrandTotal * 100))}
	if !r.Post {
		ref.Number += " (brouillon)"
	}
	return ref, nil
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return 0
}

var _ adapter.ERP = (*Client)(nil)
