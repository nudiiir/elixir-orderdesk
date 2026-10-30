// Package odoo is the adapter for Odoo (16, 17, 18) through its standard JSON-RPC API, with a
// user's login and an API key (Settings → Users → API keys) or password. Nothing is installed in
// Odoo: Sales, Purchase and Inventory are used as they are.
//
//	products   product.product (+ product.packaging, sale taxes; brand = product category)
//	customers  res.partner with customer_rank > 0; suppliers with supplier_rank > 0
//	order      sale.order, confirmed
//	receipt    purchase.order, confirmed, and its incoming transfer validated (stock in)
package odoo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
)

type Client struct {
	BaseURL, DB, Login, Key string
	HTTP                    *http.Client

	mu  sync.Mutex
	uid int
}

func New(baseURL, db, login, key string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), DB: db, Login: login, Key: key, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) Name() string { return "Odoo" }

// Authenticate checks the login and returns the user's display name.
func (c *Client) Authenticate(ctx context.Context) (string, error) {
	var uid any
	if err := c.call(ctx, "common", "authenticate", []any{c.DB, c.Login, c.Key, map[string]any{}}, &uid); err != nil {
		return "", err
	}
	id, ok := uid.(float64)
	if !ok || id == 0 {
		return "", errors.New("Odoo refused the login (database, login or API key)")
	}
	c.mu.Lock()
	c.uid = int(id)
	c.mu.Unlock()
	var users []map[string]any
	if err := c.exec(ctx, "res.users", "read", []any{[]int{int(id)}}, map[string]any{"fields": []string{"name"}}, &users); err == nil && len(users) > 0 {
		return str(users[0]["name"]), nil
	}
	return c.Login, nil
}

func (c *Client) call(ctx context.Context, service, method string, args []any, out any) error {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "call", "id": 1,
		"params": map[string]any{"service": service, "method": method, "args": args}})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/jsonrpc", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("ERP unreachable: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var r struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
			Data    struct {
				Name    string `json:"name"`
				Message string `json:"message"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return fmt.Errorf("Odoo %s.%s: HTTP %d: %.200s", service, method, res.StatusCode, raw)
	}
	if r.Error != nil {
		msg := r.Error.Data.Message
		if msg == "" {
			msg = r.Error.Message
		}
		if strings.Contains(r.Error.Data.Name, "AccessDenied") {
			c.mu.Lock()
			c.uid = 0
			c.mu.Unlock()
		}
		return fmt.Errorf("Odoo %s: %s", method, msg)
	}
	if out == nil || len(r.Result) == 0 {
		return nil
	}
	return json.Unmarshal(r.Result, out)
}

// exec runs a model method (execute_kw) as the connected user.
func (c *Client) exec(ctx context.Context, model, method string, args []any, kw map[string]any, out any) error {
	c.mu.Lock()
	uid := c.uid
	c.mu.Unlock()
	if uid == 0 {
		if _, err := c.Authenticate(ctx); err != nil {
			return err
		}
		c.mu.Lock()
		uid = c.uid
		c.mu.Unlock()
	}
	if kw == nil {
		kw = map[string]any{}
	}
	return c.call(ctx, "object", "execute_kw", []any{c.DB, uid, c.Key, model, method, args, kw}, out)
}

func (c *Client) searchRead(ctx context.Context, model string, domain []any, fields []string, extra map[string]any) ([]map[string]any, error) {
	kw := map[string]any{"fields": fields}
	for k, v := range extra {
		kw[k] = v
	}
	var out []map[string]any
	err := c.exec(ctx, model, "search_read", []any{domain}, kw, &out)
	return out, err
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Authenticate(ctx)
	return err
}

// ---- reading ---------------------------------------------------------------------------------

var genericCategories = map[string]bool{"all": true, "saleable": true, "goods": true, "deliveries": true, "expenses": true, "services": true}

func (c *Client) Products(ctx context.Context) ([]catalog.Product, error) {
	var out []catalog.Product
	for offset := 0; ; offset += 500 {
		rows, err := c.searchRead(ctx, "product.product", []any{[]any{"sale_ok", "=", true}, []any{"type", "!=", "service"}},
			[]string{"name", "barcode", "default_code", "categ_id", "uom_id", "taxes_id"}, map[string]any{"limit": 500, "offset": offset, "order": "id"})
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return out, nil
		}
		ids := make([]any, 0, len(rows))
		taxIDs := map[int]bool{}
		for _, r := range rows {
			ids = append(ids, r["id"])
			for _, t := range intList(r["taxes_id"]) {
				taxIDs[t] = true
			}
		}
		packs := map[int][]catalog.Pack{}
		if pk, err := c.searchRead(ctx, "product.packaging", []any{[]any{"product_id", "in", ids}}, []string{"product_id", "name", "qty"}, nil); err == nil {
			for _, p := range pk {
				if q := f64(p["qty"]); q > 1 {
					id := m2oID(p["product_id"])
					packs[id] = append(packs[id], catalog.Pack{Name: str(p["name"]), FactorMilli: int64(math.Round(q * 1000))})
				}
			}
		}
		rates := c.taxRates(ctx, taxIDs)
		for _, r := range rows {
			id := int(f64(r["id"]))
			p := catalog.Product{ID: strconv.Itoa(id), Name: str(r["name"]), Barcode: str(r["barcode"]), Unit: m2oName(r["uom_id"])}
			cat := m2oName(r["categ_id"])
			if i := strings.LastIndex(cat, "/"); i >= 0 {
				cat = strings.TrimSpace(cat[i+1:])
			}
			if !genericCategories[strings.ToLower(cat)] {
				p.Brand = cat
			}
			for _, t := range intList(r["taxes_id"]) {
				p.VatRate += rates[t]
			}
			p.Packs = packs[id]
			sort.Slice(p.Packs, func(i, j int) bool { return p.Packs[i].FactorMilli < p.Packs[j].FactorMilli })
			out = append(out, p)
		}
		if len(rows) < 500 {
			return out, nil
		}
	}
}

// taxRates returns percent taxes as Order Desk rates (19 % → 1900).
func (c *Client) taxRates(ctx context.Context, ids map[int]bool) map[int]int64 {
	out := map[int]int64{}
	if len(ids) == 0 {
		return out
	}
	list := []any{}
	for id := range ids {
		list = append(list, id)
	}
	rows, _ := c.searchRead(ctx, "account.tax", []any{[]any{"id", "in", list}}, []string{"amount", "amount_type"}, map[string]any{"context": map[string]any{"active_test": false}})
	for _, t := range rows {
		if str(t["amount_type"]) == "percent" {
			out[int(f64(t["id"]))] = int64(math.Round(f64(t["amount"]) * 100))
		}
	}
	return out
}

func (c *Client) Customers(ctx context.Context) ([]catalog.Customer, error) { return c.partners(ctx, "customer_rank") }
func (c *Client) Suppliers(ctx context.Context) ([]catalog.Customer, error) { return c.partners(ctx, "supplier_rank") }

func (c *Client) partners(ctx context.Context, rank string) ([]catalog.Customer, error) {
	rows, err := c.searchRead(ctx, "res.partner", []any{[]any{rank, ">", 0}}, []string{"name", "phone", "mobile"}, map[string]any{"order": "name"})
	if err != nil {
		return nil, err
	}
	out := make([]catalog.Customer, 0, len(rows))
	for _, r := range rows {
		phone := str(r["mobile"])
		if phone == "" {
			phone = str(r["phone"])
		}
		out = append(out, catalog.Customer{ID: strconv.Itoa(int(f64(r["id"]))), Name: str(r["name"]), Phone: phone})
	}
	return out, nil
}

// Price is the product's sales price and its taxes. (A partner's pricelist rules are applied by
// Odoo itself when the order is created.)
func (c *Client) Price(ctx context.Context, customerID, productID string, qtyMilli int64) (adapter.Quote, error) {
	pid, err := strconv.Atoi(productID)
	if err != nil {
		return adapter.Quote{}, err
	}
	var rows []map[string]any
	if err := c.exec(ctx, "product.product", "read", []any{[]int{pid}}, map[string]any{"fields": []string{"lst_price", "taxes_id"}}, &rows); err != nil || len(rows) == 0 {
		return adapter.Quote{}, err
	}
	ids := map[int]bool{}
	for _, t := range intList(rows[0]["taxes_id"]) {
		ids[t] = true
	}
	var rate int64
	for _, r := range c.taxRates(ctx, ids) {
		rate += r
	}
	unit := int64(math.Round(f64(rows[0]["lst_price"]) * 100))
	ht := float64(unit) * float64(qtyMilli) / 1000
	return adapter.Quote{UnitPriceHT: unit, LineTTC: int64(math.Round(ht * (1 + float64(rate)/10000))), Source: "Odoo sales price"}, nil
}

func (c *Client) Stock(ctx context.Context) (map[string]int64, error) {
	rows, err := c.searchRead(ctx, "product.product", []any{[]any{"sale_ok", "=", true}, []any{"type", "!=", "service"}}, []string{"qty_available"}, nil)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[strconv.Itoa(int(f64(r["id"])))] = int64(math.Round(f64(r["qty_available"]) * 1000))
	}
	return out, nil
}

func (c *Client) History(ctx context.Context, customerID string, n int) ([]adapter.PastOrder, error) {
	cid, err := strconv.Atoi(customerID)
	if err != nil {
		return nil, err
	}
	orders, err := c.searchRead(ctx, "sale.order", []any{[]any{"partner_id", "=", cid}, []any{"state", "in", []string{"sale", "done"}}},
		[]string{"name", "date_order", "order_line"}, map[string]any{"limit": n, "order": "date_order desc"})
	if err != nil {
		return nil, err
	}
	var out []adapter.PastOrder
	for _, o := range orders {
		po := adapter.PastOrder{ID: strconv.Itoa(int(f64(o["id"]))), Number: str(o["name"]), Date: strings.SplitN(str(o["date_order"]), " ", 2)[0]}
		lines, _ := c.searchRead(ctx, "sale.order.line", []any{[]any{"order_id", "=", int(f64(o["id"]))}, []any{"display_type", "=", false}},
			[]string{"product_id", "product_uom_qty"}, nil)
		for _, l := range lines {
			if id := m2oID(l["product_id"]); id > 0 {
				po.Lines = append(po.Lines, adapter.PastLine{ProductID: strconv.Itoa(id), QtyMilli: int64(math.Round(f64(l["product_uom_qty"]) * 1000))})
			}
		}
		out = append(out, po)
	}
	return out, nil
}

func (c *Client) NextDelivery(ctx context.Context, customerID string) (adapter.Delivery, error) {
	return adapter.Delivery{}, nil // Odoo has no delivery rounds out of the box
}

func (c *Client) OrderURL(ref adapter.OrderRef) string {
	if ref.ID == "" {
		return ""
	}
	return c.BaseURL + "/odoo/sales/" + ref.ID
}

// ---- writing ---------------------------------------------------------------------------------

// CreateOrder creates and confirms a sales order. The shop's own words go in the order's note,
// line by line, so the order keeps its evidence.
func (c *Client) CreateOrder(ctx context.Context, o adapter.Order) (adapter.OrderRef, error) {
	cid, err := strconv.Atoi(o.CustomerID)
	if err != nil {
		return adapter.OrderRef{}, fmt.Errorf("customer id: %w", err)
	}
	lines := []any{}
	note := o.Notes
	for _, l := range o.Lines {
		pid, err := strconv.Atoi(l.ProductID)
		if err != nil {
			return adapter.OrderRef{}, fmt.Errorf("product id %q: %w", l.ProductID, err)
		}
		lines = append(lines, []any{0, 0, map[string]any{"product_id": pid, "product_uom_qty": float64(l.QtyMilli) / 1000}})
		if l.Notes != "" {
			note += "\n" + l.Notes
		}
	}
	vals := map[string]any{"partner_id": cid, "order_line": lines, "note": note}
	if o.ExternalRef != "" {
		vals["client_order_ref"] = o.ExternalRef
	}
	var id float64
	if err := c.exec(ctx, "sale.order", "create", []any{vals}, nil, &id); err != nil {
		return adapter.OrderRef{}, err
	}
	if err := c.exec(ctx, "sale.order", "action_confirm", []any{[]int{int(id)}}, nil, nil); err != nil {
		return adapter.OrderRef{}, fmt.Errorf("order %d created as a quotation, but confirming failed: %w", int(id), err)
	}
	return c.ref(ctx, "sale.order", int(id), "")
}

func (c *Client) ref(ctx context.Context, model string, id int, suffix string) (adapter.OrderRef, error) {
	var rows []map[string]any
	if err := c.exec(ctx, model, "read", []any{[]int{id}}, map[string]any{"fields": []string{"name", "amount_total"}}, &rows); err != nil || len(rows) == 0 {
		return adapter.OrderRef{ID: strconv.Itoa(id)}, err
	}
	return adapter.OrderRef{ID: strconv.Itoa(id), Number: str(rows[0]["name"]) + suffix, TotalTTC: int64(math.Round(f64(rows[0]["amount_total"]) * 100))}, nil
}

// LastPurchasePrices: the last price paid to this supplier per product (confirmed purchase lines,
// newest first), else the supplier's price list. Centimes per base unit.
func (c *Client) LastPurchasePrices(ctx context.Context, supplierID string) (map[string]int64, error) {
	sid, err := strconv.Atoi(supplierID)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	lines, err := c.searchRead(ctx, "purchase.order.line", []any{[]any{"partner_id", "=", sid}, []any{"state", "in", []string{"purchase", "done"}}},
		[]string{"product_id", "price_unit", "product_uom", "product_id"}, map[string]any{"order": "id desc", "limit": 2000})
	if err != nil {
		return nil, err
	}
	for _, l := range lines {
		id := strconv.Itoa(m2oID(l["product_id"]))
		if _, seen := out[id]; !seen && id != "0" {
			out[id] = int64(math.Round(f64(l["price_unit"]) * 100))
		}
	}
	infos, err := c.searchRead(ctx, "product.supplierinfo", []any{[]any{"partner_id", "=", sid}}, []string{"product_tmpl_id", "product_id", "price"}, nil)
	if err != nil {
		return out, nil
	}
	tmpls := []any{}
	for _, s := range infos {
		tmpls = append(tmpls, m2oID(s["product_tmpl_id"]))
	}
	variants := map[int][]int{}
	if len(tmpls) > 0 {
		vs, _ := c.searchRead(ctx, "product.product", []any{[]any{"product_tmpl_id", "in", tmpls}}, []string{"product_tmpl_id"}, nil)
		for _, v := range vs {
			t := m2oID(v["product_tmpl_id"])
			variants[t] = append(variants[t], int(f64(v["id"])))
		}
	}
	for _, s := range infos {
		ids := variants[m2oID(s["product_tmpl_id"])]
		if p := m2oID(s["product_id"]); p > 0 {
			ids = []int{p}
		}
		for _, p := range ids {
			if _, seen := out[strconv.Itoa(p)]; !seen {
				out[strconv.Itoa(p)] = int64(math.Round(f64(s["price"]) * 100))
			}
		}
	}
	return out, nil
}

// CreateReceipt books the approved supplier invoice as a confirmed purchase order and, when asked,
// validates its incoming transfer: Odoo moves the stock and keeps the purchase price.
func (c *Client) CreateReceipt(ctx context.Context, r adapter.Receipt) (adapter.OrderRef, error) {
	sid, err := strconv.Atoi(r.SupplierID)
	if err != nil {
		return adapter.OrderRef{}, fmt.Errorf("supplier id: %w", err)
	}
	date := r.Date + " 12:00:00"
	lines := []any{}
	for _, l := range r.Lines {
		pid, err := strconv.Atoi(l.ProductID)
		if err != nil {
			return adapter.OrderRef{}, fmt.Errorf("product id %q: %w", l.ProductID, err)
		}
		// Order Desk counts a receipt line in packs when a pack factor is set; Odoo here in base units.
		factor := float64(l.PackFactorMilli) / 1000
		if factor <= 0 {
			factor = 1
		}
		units := float64(l.QtyMilli) / 1000 * factor
		price := float64(l.UnitCostHT) / 100 / factor
		vals := map[string]any{"product_id": pid, "product_qty": units, "price_unit": math.Round(price*10000) / 10000, "date_planned": date}
		if l.Description != "" {
			vals["name"] = l.Description
		}
		lines = append(lines, []any{0, 0, vals})
	}
	po := map[string]any{"partner_id": sid, "date_order": date, "order_line": lines}
	if r.SupplierRef != "" {
		po["partner_ref"] = r.SupplierRef
	}
	if r.Notes != "" {
		po["notes"] = r.Notes
	}
	var id float64
	if err := c.exec(ctx, "purchase.order", "create", []any{po}, nil, &id); err != nil {
		return adapter.OrderRef{}, err
	}
	pid := int(id)
	if err := c.exec(ctx, "purchase.order", "button_confirm", []any{[]int{pid}}, nil, nil); err != nil {
		return adapter.OrderRef{ID: strconv.Itoa(pid)}, fmt.Errorf("purchase %d created as an RFQ, but confirming failed: %w", pid, err)
	}
	suffix := ""
	if r.Post {
		names, err := c.validateIncoming(ctx, pid)
		if err != nil {
			ref, _ := c.ref(ctx, "purchase.order", pid, "")
			return ref, fmt.Errorf("purchase confirmed, but the receipt was not validated: %w", err)
		}
		if len(names) > 0 {
			suffix = " · " + strings.Join(names, ", ")
		}
	}
	return c.ref(ctx, "purchase.order", pid, suffix)
}

// validateIncoming marks every move of the order's incoming transfers as received and validates them.
func (c *Client) validateIncoming(ctx context.Context, poID int) ([]string, error) {
	picks, err := c.searchRead(ctx, "stock.picking", []any{[]any{"purchase_id", "=", poID}, []any{"state", "not in", []string{"done", "cancel"}}},
		[]string{"name"}, nil)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, pk := range picks {
		pkID := int(f64(pk["id"]))
		moves, err := c.searchRead(ctx, "stock.move", []any{[]any{"picking_id", "=", pkID}}, []string{"product_uom_qty"}, nil)
		if err != nil {
			return names, err
		}
		for _, m := range moves {
			mid, q := int(f64(m["id"])), f64(m["product_uom_qty"])
			// Odoo 17+: quantity + picked; Odoo 16: quantity_done
			if err := c.exec(ctx, "stock.move", "write", []any{[]int{mid}, map[string]any{"quantity": q, "picked": true}}, nil, nil); err != nil {
				if err2 := c.exec(ctx, "stock.move", "write", []any{[]int{mid}, map[string]any{"quantity_done": q}}, nil, nil); err2 != nil {
					return names, err
				}
			}
		}
		var res any
		if err := c.exec(ctx, "stock.picking", "button_validate", []any{[]int{pkID}}, nil, &res); err != nil {
			return names, err
		}
		if wiz, ok := res.(map[string]any); ok && wiz["res_model"] != nil {
			return names, fmt.Errorf("Odoo asks a question to validate %s (%v): finish it in Odoo", str(pk["name"]), wiz["res_model"])
		}
		names = append(names, str(pk["name"]))
	}
	return names, nil
}

// ---- JSON helpers: Odoo returns false for empty values and [id, "name"] for relations --------

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func f64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return 0
}

func m2oID(v any) int {
	if a, ok := v.([]any); ok && len(a) > 0 {
		return int(f64(a[0]))
	}
	return 0
}

func m2oName(v any) string {
	if a, ok := v.([]any); ok && len(a) > 1 {
		return str(a[1])
	}
	return ""
}

func intList(v any) []int {
	a, _ := v.([]any)
	out := make([]int, 0, len(a))
	for _, x := range a {
		out = append(out, int(f64(x)))
	}
	return out
}

var _ adapter.ERP = (*Client)(nil)
