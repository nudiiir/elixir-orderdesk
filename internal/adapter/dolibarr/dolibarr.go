// Package dolibarr is the adapter for Dolibarr (17+) through its REST API, with a user's API key
// (DOLAPIKEY). Login with a password happens once, through /login, to read or create the key; the
// password is never kept.
//
//	products   Product, to sell (packs = extrafield `packs` "Carton x6"; brand = extrafield `brand`
//	           or a category under "Marques"/"Brands"; VAT = tva_tx)
//	customers  third parties that are customers; suppliers: third parties that are suppliers
//	order      customer order (Commande), validated; the shop's words in the private note
//	receipt    supplier order, validated, approved and ordered, then a reception validated and
//	           closed when posted (stock in); without the Reception module, the order is dispatched
package dolibarr

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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
)

type Client struct {
	BaseURL, Key string
	HTTP         *http.Client

	mu               sync.Mutex
	warehouse        int // default warehouse, found once
	reception        int // Reception module: 0 unknown, 1 on, -1 off
	noCustomerPrices bool
}

// New takes the Dolibarr root URL (with or without /api/index.php) and a user's API key.
func New(baseURL, apiKey string) *Client {
	return &Client{BaseURL: root(baseURL), Key: apiKey, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

func root(u string) string {
	u = strings.TrimRight(strings.TrimSpace(u), "/")
	u = strings.TrimSuffix(u, "/api/index.php")
	return strings.TrimRight(u, "/")
}

func (c *Client) Name() string { return "Dolibarr" }

// Login turns a login and password into the user's API key, once: Dolibarr returns the user's key,
// or creates one when the user has none (an existing key is kept, so other integrations go on
// working). The login API is off by default (constant API_ENABLE_LOGIN_API); then the key is
// copied from the user card instead.
func Login(ctx context.Context, baseURL, user, password string) (apiKey, userName string, err error) {
	base := root(baseURL)
	hc := &http.Client{Timeout: 60 * time.Second}
	b, _ := json.Marshal(map[string]any{"login": user, "password": password})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/index.php/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := hc.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("ERP unreachable: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		msg := apiMessage(raw)
		if strings.Contains(msg, "API_ENABLE_LOGIN_API") {
			return "", "", errors.New("Dolibarr's login API is turned off (constant API_ENABLE_LOGIN_API): paste the user's API key from the user card instead")
		}
		return "", "", fmt.Errorf("Dolibarr refused the login: HTTP %d: %s", res.StatusCode, msg)
	}
	var out struct {
		Success struct {
			Token any `json:"token"`
		} `json:"success"`
	}
	json.Unmarshal(raw, &out)
	apiKey, _ = out.Success.Token.(string)
	if apiKey == "" {
		return "", "", errors.New("Dolibarr did not return an API key for this user")
	}
	userName = user
	var me map[string]any
	if err := New(base, apiKey).do(ctx, http.MethodGet, "/users/info", nil, &me); err == nil {
		if n := strings.TrimSpace(str(me["firstname"]) + " " + str(me["lastname"])); n != "" {
			userName = n
		}
	}
	return apiKey, userName, nil
}

// httpError keeps the status, so a list that Dolibarr answers with 404 ("No … found") reads as empty.
type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

func statusOf(err error) int {
	var he *httpError
	if errors.As(err, &he) {
		return he.status
	}
	return 0
}

func apiMessage(raw []byte) string {
	var m struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &m) == nil && m.Error.Message != "" {
		s := m.Error.Message
		if len(s) > 300 {
			s = s[:300]
		}
		return s
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
	req, _ := http.NewRequestWithContext(ctx, method, c.BaseURL+"/api/index.php"+path, rd)
	req.Header.Set("DOLAPIKEY", c.Key)
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
	short := path
	if i := strings.IndexByte(short, '?'); i >= 0 {
		short = short[:i]
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return &httpError{res.StatusCode, fmt.Sprintf("Dolibarr %s %s: not allowed (%d): %s", method, short, res.StatusCode, apiMessage(raw))}
	}
	if res.StatusCode >= 300 {
		return &httpError{res.StatusCode, fmt.Sprintf("Dolibarr %s %s: HTTP %d: %s", method, short, res.StatusCode, apiMessage(raw))}
	}
	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// list reads every page of a list endpoint. Dolibarr answers an empty list with [] or with a 404.
func (c *Client) list(ctx context.Context, path string, q url.Values, max int) ([]map[string]any, error) {
	const page = 500
	var out []map[string]any
	for p := 0; ; p++ {
		v := url.Values{}
		for k, vs := range q {
			v[k] = vs
		}
		limit := page
		if max > 0 && max < page {
			limit = max
		}
		v.Set("limit", strconv.Itoa(limit))
		v.Set("page", strconv.Itoa(p))
		var rows []map[string]any
		if err := c.do(ctx, http.MethodGet, path+"?"+v.Encode(), nil, &rows); err != nil {
			if statusOf(err) == http.StatusNotFound {
				return out, nil
			}
			return out, err
		}
		out = append(out, rows...)
		if len(rows) < limit || (max > 0 && len(out) >= max) {
			return out, nil
		}
	}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/users/info", nil, nil)
}

// ---- reading ---------------------------------------------------------------------------------

func (c *Client) Products(ctx context.Context) ([]catalog.Product, error) {
	rows, err := c.list(ctx, "/products", url.Values{"mode": {"1"}, "sortfield": {"t.rowid"}, "sqlfilters": {"(t.tosell:=:1)"}}, 0)
	if err != nil {
		return nil, err
	}
	brands := c.brandCategories(ctx)
	units := c.units(ctx)
	out := make([]catalog.Product, 0, len(rows))
	for _, r := range rows {
		id := str(r["id"])
		p := catalog.Product{ID: id, Name: str(r["label"]), Barcode: str(r["barcode"]), Unit: units[str(r["fk_unit"])],
			VatRate: int64(math.Round(num(r["tva_tx"]) * 100))}
		if p.Name == "" {
			p.Name = str(r["ref"])
		}
		opts, _ := r["array_options"].(map[string]any) // [] when the product has no extrafield
		for _, k := range []string{"options_brand", "options_marque"} {
			if p.Brand == "" {
				p.Brand = strings.TrimSpace(str(opts[k]))
			}
		}
		if p.Brand == "" {
			p.Brand = brands[id]
		}
		for _, k := range []string{"options_packs", "options_pack", "options_conditionnement"} {
			p.Packs = append(p.Packs, parsePacks(str(opts[k]))...)
		}
		sort.Slice(p.Packs, func(i, j int) bool { return p.Packs[i].FactorMilli < p.Packs[j].FactorMilli })
		out = append(out, p)
	}
	return out, nil
}

// "Carton x6", "Pack de 12", "Palette = 480": the number at the end is the units in the pack.
var packRe = regexp.MustCompile(`(?i)(?:^|[\s(x×*=:])(\d+(?:[.,]\d+)?)\s*(?:u|unités?|pcs?)?\s*\)?\s*$`)

func parsePacks(s string) []catalog.Pack {
	var out []catalog.Pack
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == '|' || r == '\n' }) {
		part = strings.TrimSpace(part)
		m := packRe.FindStringSubmatch(part)
		if m == nil {
			continue
		}
		f, _ := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
		if f > 1 {
			out = append(out, catalog.Pack{Name: part, FactorMilli: int64(math.Round(f * 1000))})
		}
	}
	return out
}

var brandRoots = map[string]bool{"marque": true, "marques": true, "brand": true, "brands": true}

// brandCategories maps product → brand when brands are product categories under a "Marques" (or
// "Brands") category, the usual way to keep brands in Dolibarr.
func (c *Client) brandCategories(ctx context.Context) map[string]string {
	out := map[string]string{}
	cats, err := c.list(ctx, "/categories", url.Values{"type": {"product"}}, 0)
	if err != nil {
		return out
	}
	roots := map[string]bool{}
	for _, k := range cats {
		if brandRoots[strings.ToLower(strings.TrimSpace(str(k["label"])))] {
			roots[str(k["id"])] = true
		}
	}
	for _, k := range cats {
		if !roots[str(k["fk_parent"])] {
			continue
		}
		var ids []any
		if err := c.do(ctx, http.MethodGet, "/categories/"+str(k["id"])+"/objects?type=product&onlyids=1", nil, &ids); err != nil {
			continue
		}
		for _, id := range ids {
			if _, seen := out[str(id)]; !seen {
				out[str(id)] = strings.TrimSpace(str(k["label"]))
			}
		}
	}
	return out
}

// units maps fk_unit to its short label (used when the instance sets PRODUCT_USE_UNITS).
func (c *Client) units(ctx context.Context) map[string]string {
	out := map[string]string{}
	rows, _ := c.list(ctx, "/setup/dictionary/units", nil, 0)
	for _, r := range rows {
		out[str(r["rowid"])] = str(r["short_label"])
	}
	return out
}

func (c *Client) Customers(ctx context.Context) ([]catalog.Customer, error) {
	return c.parties(ctx, "1")
}
func (c *Client) Suppliers(ctx context.Context) ([]catalog.Customer, error) {
	return c.parties(ctx, "4")
}

// parties: mode 1 = customers, 4 = suppliers; active ones only.
func (c *Client) parties(ctx context.Context, mode string) ([]catalog.Customer, error) {
	rows, err := c.list(ctx, "/thirdparties", url.Values{"mode": {mode}, "sortfield": {"t.nom"}, "sqlfilters": {"(t.status:=:1)"}}, 0)
	if err != nil {
		return nil, err
	}
	out := make([]catalog.Customer, 0, len(rows))
	for _, r := range rows {
		phone := str(r["phone_mobile"])
		if phone == "" {
			phone = str(r["phone"])
		}
		out = append(out, catalog.Customer{ID: str(r["id"]), Name: str(r["name"]), Phone: phone})
	}
	return out, nil
}

type price struct {
	subprice, vat, discount float64 // HT per unit, VAT %, line discount %
	source                  string
}

// price applies Dolibarr's own selling rules: a price for this customer (PRODUIT_CUSTOMER_PRICES),
// else the customer's price level (PRODUIT_MULTIPRICES), else the product's price; and the
// customer's standing discount.
func (c *Client) price(ctx context.Context, customerID, productID string) (price, error) {
	var p, cust map[string]any
	if err := c.do(ctx, http.MethodGet, "/products/"+url.PathEscape(productID), nil, &p); err != nil {
		return price{}, err
	}
	pr := price{subprice: num(p["price"]), vat: num(p["tva_tx"]), source: "Dolibarr prix de vente"}
	if customerID != "" {
		if err := c.do(ctx, http.MethodGet, "/thirdparties/"+url.PathEscape(customerID), nil, &cust); err == nil {
			pr.discount = num(cust["remise_percent"])
			if lvl := str(cust["price_level"]); lvl != "" && lvl != "1" {
				if mp, ok := p["multiprices"].(map[string]any); ok && num(mp[lvl]) > 0 {
					pr.subprice, pr.source = num(mp[lvl]), "Dolibarr niveau de prix "+lvl
					if tv, ok := p["multiprices_tva_tx"].(map[string]any); ok && tv[lvl] != nil {
						pr.vat = num(tv[lvl])
					}
				}
			}
		}
		c.mu.Lock()
		off := c.noCustomerPrices
		c.mu.Unlock()
		if !off {
			var rows []map[string]any
			err := c.do(ctx, http.MethodGet, "/products/"+url.PathEscape(productID)+"/selling_multiprices/per_customer?thirdparty_id="+url.QueryEscape(customerID), nil, &rows)
			if s := statusOf(err); s == http.StatusBadRequest || s == http.StatusNotImplemented {
				c.mu.Lock()
				c.noCustomerPrices = true // "this mode of pricing is not enabled by setup"
				c.mu.Unlock()
			}
			for _, r := range rows {
				if str(r["fk_soc"]) == customerID && num(r["price"]) > 0 {
					pr.subprice, pr.vat, pr.source = num(r["price"]), num(r["tva_tx"]), "Dolibarr prix client"
					break
				}
			}
		}
	}
	return pr, nil
}

func (c *Client) Price(ctx context.Context, customerID, productID string, qtyMilli int64) (adapter.Quote, error) {
	pr, err := c.price(ctx, customerID, productID)
	if err != nil {
		return adapter.Quote{}, err
	}
	net := pr.subprice * (1 - pr.discount/100)
	q := adapter.Quote{UnitPriceHT: int64(math.Round(net * 100)), Source: pr.source,
		LineTTC: int64(math.Round(net * 100 * float64(qtyMilli) / 1000 * (1 + pr.vat/100)))}
	if pr.discount > 0 {
		q.Offer = fmt.Sprintf("remise %s %%", strconv.FormatFloat(pr.discount, 'f', -1, 64))
	}
	return q, nil
}

// Stock is the physical stock of every product, all warehouses together.
func (c *Client) Stock(ctx context.Context) (map[string]int64, error) {
	rows, err := c.list(ctx, "/products", url.Values{"mode": {"1"}, "sortfield": {"t.rowid"}}, 0)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[str(r["id"])] = int64(math.Round(num(r["stock_reel"]) * 1000))
	}
	return out, nil
}

func (c *Client) History(ctx context.Context, customerID string, n int) ([]adapter.PastOrder, error) {
	orders, err := c.list(ctx, "/orders", url.Values{"thirdparty_ids": {customerID}, "sortfield": {"t.date_commande,t.rowid"}, "sortorder": {"DESC,DESC"},
		"sqlfilters": {"(t.fk_statut:>:0)"}}, n)
	if err != nil {
		return nil, err
	}
	var out []adapter.PastOrder
	for _, o := range orders {
		po := adapter.PastOrder{ID: str(o["id"]), Number: str(o["ref"]), Date: day(o["date_commande"])}
		if po.Date == "" {
			po.Date = day(o["date"])
		}
		lines, _ := o["lines"].([]any)
		for _, x := range lines {
			l, _ := x.(map[string]any)
			if id := str(l["fk_product"]); id != "" && id != "0" {
				po.Lines = append(po.Lines, adapter.PastLine{ProductID: id, QtyMilli: int64(math.Round(num(l["qty"]) * 1000))})
			}
		}
		out = append(out, po)
	}
	return out, nil
}

// day reads Dolibarr's date (a Unix timestamp at the server's midnight) as YYYY-MM-DD; the half day
// added keeps the date right whatever the server's time zone.
func day(v any) string {
	ts := num(v)
	if ts <= 0 {
		return ""
	}
	return time.Unix(int64(ts)+12*3600, 0).UTC().Format("2006-01-02")
}

func (c *Client) NextDelivery(ctx context.Context, customerID string) (adapter.Delivery, error) {
	return adapter.Delivery{}, nil // no delivery rounds in standard Dolibarr
}

func (c *Client) OrderURL(ref adapter.OrderRef) string {
	if ref.ID == "" {
		return ""
	}
	return c.BaseURL + "/commande/card.php?id=" + url.QueryEscape(ref.ID)
}

// ---- writing ---------------------------------------------------------------------------------

// defaultWarehouse: the user's default warehouse, else the first open one.
func (c *Client) defaultWarehouse(ctx context.Context) (int, error) {
	c.mu.Lock()
	wh := c.warehouse
	c.mu.Unlock()
	if wh > 0 {
		return wh, nil
	}
	var me map[string]any
	if err := c.do(ctx, http.MethodGet, "/users/info", nil, &me); err == nil {
		wh = int(num(me["fk_warehouse"]))
	}
	if wh <= 0 {
		rows, err := c.list(ctx, "/warehouses", url.Values{"sortfield": {"t.rowid"}, "sqlfilters": {"(t.statut:=:1)"}}, 1)
		if err != nil {
			return 0, err
		}
		if len(rows) == 0 {
			return 0, errors.New("Dolibarr has no open warehouse to receive the goods in")
		}
		wh = int(num(rows[0]["id"]))
	}
	c.mu.Lock()
	c.warehouse = wh
	c.mu.Unlock()
	return wh, nil
}

func noteWith(notes string, extra []string) string {
	return strings.TrimSpace(strings.TrimSpace(notes) + "\n" + strings.Join(extra, "\n"))
}

// CreateOrder creates and validates a customer order, priced by Dolibarr's rules for this customer.
// The shop's own words go in the order's private note, line by line, so the order keeps its evidence.
func (c *Client) CreateOrder(ctx context.Context, o adapter.Order) (adapter.OrderRef, error) {
	lines := []any{}
	var words []string
	for _, l := range o.Lines {
		pr, err := c.price(ctx, o.CustomerID, l.ProductID)
		if err != nil {
			return adapter.OrderRef{}, fmt.Errorf("product %s: %w", l.ProductID, err)
		}
		lines = append(lines, map[string]any{"fk_product": l.ProductID, "qty": float64(l.QtyMilli) / 1000, "subprice": pr.subprice,
			"tva_tx": pr.vat, "remise_percent": pr.discount, "product_type": 0})
		if l.Notes != "" {
			words = append(words, l.Notes)
		}
	}
	body := map[string]any{"socid": o.CustomerID, "date": time.Now().Unix(), "lines": lines, "note_private": noteWith(o.Notes, words)}
	if o.ExternalRef != "" {
		body["ref_client"] = o.ExternalRef
	}
	var id any
	if err := c.do(ctx, http.MethodPost, "/orders", body, &id); err != nil {
		return adapter.OrderRef{}, err
	}
	oid := str(id)
	wh, _ := c.defaultWarehouse(ctx) // used only when the stock moves on order validation
	var v map[string]any
	if err := c.do(ctx, http.MethodPost, "/orders/"+oid+"/validate", map[string]any{"idwarehouse": wh, "notrigger": 0}, &v); err != nil {
		return adapter.OrderRef{ID: oid}, fmt.Errorf("order %s created as a draft, but validating failed: %w", oid, err)
	}
	return adapter.OrderRef{ID: oid, Number: str(v["ref"]), TotalTTC: int64(math.Round(num(v["total_ttc"]) * 100))}, nil
}

// LastPurchasePrices: the last price paid to this supplier per product (supplier orders from
// approved to received, newest first), else the supplier's prices. Centimes HT per base unit.
func (c *Client) LastPurchasePrices(ctx context.Context, supplierID string) (map[string]int64, error) {
	out := map[string]int64{}
	orders, err := c.list(ctx, "/supplierorders", url.Values{"thirdparty_ids": {supplierID}, "sortfield": {"t.rowid"}, "sortorder": {"DESC"},
		"sqlfilters": {"(t.fk_statut:>=:2) and (t.fk_statut:<=:5)"}}, 100)
	if err != nil {
		return nil, err
	}
	for _, o := range orders {
		lines, _ := o["lines"].([]any)
		for _, x := range lines {
			l, _ := x.(map[string]any)
			id := str(l["fk_product"])
			if _, seen := out[id]; seen || id == "" || id == "0" {
				continue
			}
			out[id] = int64(math.Round(num(l["subprice"]) * (1 - num(l["remise_percent"])/100) * 100))
		}
	}
	// the supplier's price list: product id → its supplier prices
	for p := 0; ; p++ {
		var prices map[string][]map[string]any
		err := c.do(ctx, http.MethodGet, fmt.Sprintf("/products/purchase_prices?supplier=%s&mode=1&limit=500&page=%d", url.QueryEscape(supplierID), p), nil, &prices)
		if err != nil {
			break // 404 when there is none (or no more)
		}
		for id, list := range prices {
			for _, pp := range list {
				if _, seen := out[id]; !seen && str(pp["fourn_id"]) == supplierID {
					out[id] = int64(math.Round(num(pp["fourn_unitprice"]) * 100))
				}
			}
		}
		if len(prices) < 500 {
			break
		}
	}
	return out, nil
}

// CreateReceipt books the approved supplier invoice as a supplier order (validated, approved and
// marked ordered) and its reception: validated and closed when Post is set, so Dolibarr moves the
// stock in at the invoice's prices; left as a draft reception otherwise.
func (c *Client) CreateReceipt(ctx context.Context, r adapter.Receipt) (adapter.OrderRef, error) {
	wh, err := c.defaultWarehouse(ctx)
	if err != nil {
		return adapter.OrderRef{}, err
	}
	date := time.Now()
	if t, err := time.ParseInLocation("2006-01-02", r.Date, time.Local); err == nil {
		date = t.Add(12 * time.Hour)
	}
	type unitLine struct {
		product     string
		units, cost float64 // base units and HT cost per base unit
	}
	var want []unitLine
	lines := []any{}
	var words []string
	for _, l := range r.Lines {
		// Order Desk counts a receipt line in packs when a pack factor is set; Dolibarr in base units,
		// at the pack price divided down (unit prices keep 5 decimals: 123.45 / 6 = 20.575).
		factor := float64(l.PackFactorMilli) / 1000
		if factor <= 0 {
			factor = 1
		}
		u := unitLine{product: l.ProductID, units: float64(l.QtyMilli) / 1000 * factor,
			cost: math.Round(float64(l.UnitCostHT)/100/factor*100000) / 100000}
		want = append(want, u)
		line := map[string]any{"fk_product": l.ProductID, "qty": u.units, "subprice": u.cost, "tva_tx": float64(l.VatRate) / 100, "product_type": 0}
		if l.Description != "" {
			line["desc"] = l.Description
			words = append(words, "« "+l.Description+" »")
		}
		lines = append(lines, line)
	}
	body := map[string]any{"socid": r.SupplierID, "date": date.Unix(), "lines": lines, "note_private": noteWith(r.Notes, words)}
	if r.SupplierRef != "" {
		body["ref_supplier"] = r.SupplierRef
	}
	var id any
	if err := c.do(ctx, http.MethodPost, "/supplierorders", body, &id); err != nil {
		return adapter.OrderRef{}, err
	}
	poID := str(id)
	po := "/supplierorders/" + poID
	steps := []struct {
		path string
		body map[string]any
		what string
	}{
		{po + "/validate", map[string]any{"idwarehouse": 0, "notrigger": 0}, "validating"},
		{po + "/approve", map[string]any{"idwarehouse": 0, "secondlevel": 0}, "approving"},
		{po + "/makeorder", map[string]any{"date": date.Unix(), "method": 1, "comment": "Order Desk"}, "marking it ordered"},
	}
	for _, s := range steps {
		if err := c.do(ctx, http.MethodPost, s.path, s.body, nil); err != nil && statusOf(err) != http.StatusNotModified {
			return adapter.OrderRef{ID: poID}, fmt.Errorf("supplier order %s created, but %s failed: %w", poID, s.what, err)
		}
	}
	var order map[string]any
	if err := c.do(ctx, http.MethodGet, po, nil, &order); err != nil {
		return adapter.OrderRef{ID: poID}, err
	}
	ref := adapter.OrderRef{ID: poID, Number: str(order["ref"]), TotalTTC: int64(math.Round(num(order["total_ttc"]) * 100))}
	// the order's lines, in creation order, to tie each received quantity to its line
	got, _ := order["lines"].([]any)
	lineIDs := make([]string, len(want))
	used := map[int]bool{}
	for i, w := range want {
		for j, x := range got {
			l, _ := x.(map[string]any)
			if !used[j] && str(l["fk_product"]) == w.product {
				lineIDs[i], used[j] = str(l["id"]), true
				break
			}
		}
	}

	if c.receptionModule(ctx) {
		rec := []any{}
		for i, w := range want {
			rec = append(rec, map[string]any{"fk_product": w.product, "fk_entrepot": wh, "origin_id": poID, "origin_line_id": lineIDs[i],
				"origin_type": "order_supplier", "qty": w.units, "cost_price": w.cost})
		}
		rb := map[string]any{"socid": r.SupplierID, "origin_id": poID, "origin_type": "order_supplier", "origin": "order_supplier",
			"date_reception": date.Unix(), "date_delivery": date.Unix(), "lines": rec, "note_private": noteWith(r.Notes, words)}
		if r.SupplierRef != "" {
			rb["ref_supplier"] = r.SupplierRef
		}
		var rid any
		if err := c.do(ctx, http.MethodPost, "/receptions", rb, &rid); err != nil {
			return ref, fmt.Errorf("supplier order %s ordered, but creating its reception failed: %w", ref.Number, err)
		}
		recID := str(rid)
		if !r.Post {
			ref.Number += " (réception brouillon)"
			return ref, nil
		}
		var v map[string]any
		if err := c.do(ctx, http.MethodPost, "/receptions/"+recID+"/validate", map[string]any{"notrigger": 0}, &v); err != nil {
			return ref, fmt.Errorf("supplier order %s ordered, but validating its reception failed (stock not in): %w", ref.Number, err)
		}
		// closing moves the stock on instances set to move it at closing; it also marks the order received
		if err := c.do(ctx, http.MethodPost, "/receptions/"+recID+"/close", map[string]any{"notrigger": 0}, nil); err != nil && statusOf(err) != http.StatusNotModified {
			return ref, fmt.Errorf("reception %s validated, but closing it failed: %w", str(v["ref"]), err)
		}
		if n := str(v["ref"]); n != "" {
			ref.Number += " · " + n
		}
		return ref, nil
	}

	// No Reception module: the order is dispatched into the warehouse (the stock moves when the
	// instance is set to move it on dispatch, STOCK_CALCULATE_ON_SUPPLIER_DISPATCH_ORDER).
	if !r.Post {
		ref.Number += " (à réceptionner)"
		return ref, nil
	}
	disp := []any{}
	for i, w := range want {
		disp = append(disp, map[string]any{"id": lineIDs[i], "fk_product": w.product, "qty": w.units, "warehouse": wh, "price": w.cost,
			"comment": r.SupplierRef, "notrigger": 0})
	}
	if err := c.do(ctx, http.MethodPost, po+"/receive", map[string]any{"closeopenorder": 1, "comment": r.SupplierRef, "lines": disp}, nil); err != nil {
		return ref, fmt.Errorf("supplier order %s ordered, but receiving it failed (stock not in): %w", ref.Number, err)
	}
	return ref, nil
}

// receptionModule: is Dolibarr's Reception module on (its API answers)?
func (c *Client) receptionModule(ctx context.Context) bool {
	c.mu.Lock()
	r := c.reception
	c.mu.Unlock()
	if r != 0 {
		return r > 0
	}
	var rows []any
	err := c.do(ctx, http.MethodGet, "/receptions?limit=1", nil, &rows)
	r = -1
	if err == nil || strings.Contains(err.Error(), "No reception") {
		r = 1
	}
	c.mu.Lock()
	c.reception = r
	c.mu.Unlock()
	return r > 0
}

// ---- JSON helpers: Dolibarr sends numbers as strings ("19.0000"), numbers or null ------------

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "1"
		}
	}
	return ""
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	case bool:
		if x {
			return 1
		}
	}
	return 0
}

var _ adapter.ERP = (*Client)(nil)
