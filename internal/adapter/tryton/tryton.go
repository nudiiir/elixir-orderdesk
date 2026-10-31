// Package tryton is the adapter for Tryton (6.0 LTS to 7.x) through its JSON-RPC API, with a user's
// session. Login with a password happens once, to open the session; the password is never kept and
// the session is revoked like any other one (logout, password change, or deleting it in Tryton).
// Nothing is installed in Tryton: Sales, Purchases and Stock are used as they are.
//
//	products   product.product (+ the template's sale and purchase units as packs, the accounting
//	           category's customer taxes; brand = the product category, under "Marques" if any)
//	customers  parties in a "Clients"/"Customers" category or with sales; suppliers: in a
//	           "Fournisseurs"/"Suppliers" category, with supplier prices or purchases
//	order      sale.sale, quoted and confirmed (Tryton then creates its shipment and invoice)
//	receipt    purchase.purchase, confirmed, and a supplier shipment holding its moves, received
//	           and done when posted (stock in)
package tryton

import (
	"bytes"
	"context"
	"encoding/base64"
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

// Client talks to one Tryton database. Token is "<user id>:<session key>", as Login returns it.
type Client struct {
	BaseURL, DB, Login, Token string
	HTTP                      *http.Client

	mu   sync.Mutex
	env  *env
	uoms map[int]uom
}

// env is what every call needs and Tryton does not infer: the user's company and its currency,
// and the warehouses.
type env struct {
	company, currency int
	warehouses        []int
	warehouse         location
}

type location struct{ id, input, storage int }

type uom struct {
	name     string
	category int
	factor   float64
}

func New(baseURL, database, login, token string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), DB: database, Login: login, Token: token, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) Name() string { return "Tryton" }

// Login opens a session with a password once and returns its token ("<user id>:<session key>")
// and the user's name. A session lasts what the server's [session] max_age allows (30 days by
// default); logging out or changing the password revokes it.
func Login(ctx context.Context, baseURL, database, user, password string) (token, userName string, err error) {
	c := New(baseURL, database, user, "")
	var res []any
	if err := c.rpc(ctx, "common.db.login", []any{user, map[string]any{"password": password}, "en"}, &res); err != nil {
		var he *httpError
		if errors.As(err, &he) && (he.code == http.StatusUnauthorized || he.code == http.StatusForbidden) {
			return "", "", errors.New("Tryton refused the login (database, user or password)")
		}
		if errors.As(err, &he) && he.code == http.StatusTooManyRequests {
			return "", "", errors.New("Tryton refused the login: too many attempts, wait and retry")
		}
		return "", "", err
	}
	if len(res) < 2 {
		return "", "", errors.New("Tryton refused the login (database, user or password)")
	}
	uid := int(num(res[0]))
	c.Token = fmt.Sprintf("%d:%s", uid, str(res[1]))
	userName = user
	var users []map[string]any
	if err := c.call(ctx, "res.user", "read", []any{[]int{uid}, []string{"name"}}, &users); err == nil && len(users) > 0 && str(users[0]["name"]) != "" {
		userName = str(users[0]["name"])
	}
	return c.Token, userName, nil
}

type httpError struct {
	code int
	body string
}

func (e *httpError) Error() string {
	switch e.code {
	case http.StatusUnauthorized:
		return "Tryton refused the session (expired or revoked): log in again"
	case http.StatusForbidden:
		return "Tryton: not allowed for this user"
	}
	return fmt.Sprintf("Tryton: HTTP %d: %.200s", e.code, e.body)
}

// rpc sends one JSON-RPC call to the database. Tryton encodes decimals and dates as objects
// ({"__class__": "Decimal", "decimal": "1.50"}), and answers errors as [type, [message, ...]].
func (c *Client) rpc(ctx context.Context, method string, params []any, out any) error {
	body, err := json.Marshal(map[string]any{"method": method, "params": params, "id": 1})
	if err != nil {
		return err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/"+c.DB+"/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if uid, key, ok := strings.Cut(c.Token, ":"); ok {
		req.Header.Set("Authorization", "Session "+base64.StdEncoding.EncodeToString([]byte(c.Login+":"+uid+":"+key)))
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("ERP unreachable: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return &httpError{code: res.StatusCode, body: string(raw)}
	}
	var r struct {
		Result json.RawMessage `json:"result"`
		Error  []any           `json:"error"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return fmt.Errorf("Tryton %s: HTTP %d: %.200s", method, res.StatusCode, raw)
	}
	if r.Error != nil {
		return fmt.Errorf("Tryton %s: %s", method, trytonMessage(r.Error))
	}
	if out == nil || len(r.Result) == 0 {
		return nil
	}
	return json.Unmarshal(r.Result, out)
}

// trytonMessage keeps the user-facing part of an error: UserError carries [message, description].
func trytonMessage(e []any) string {
	kind := ""
	if len(e) > 0 {
		kind = str(e[0])
	}
	if len(e) > 1 {
		switch a := e[1].(type) {
		case []any:
			var parts []string
			for _, p := range a {
				if s := str(p); s != "" {
					parts = append(parts, s)
				}
				if len(parts) == 2 {
					break
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, " — ")
			}
		case string:
			if a != "" {
				return a
			}
		}
	}
	return kind
}

// call runs a model method; the context (company, …) is appended, as Tryton expects it last.
func (c *Client) call(ctx context.Context, model, method string, args []any, out any) error {
	return c.callCtx(ctx, model, method, args, nil, out)
}

func (c *Client) callCtx(ctx context.Context, model, method string, args []any, extra map[string]any, out any) error {
	tctx := map[string]any{}
	c.mu.Lock()
	if c.env != nil && c.env.company > 0 {
		tctx["company"] = c.env.company
		tctx["companies"] = []int{c.env.company}
	}
	c.mu.Unlock()
	for k, v := range extra {
		tctx[k] = v
	}
	return c.rpc(ctx, "model."+model+"."+method, append(args, tctx), out)
}

func (c *Client) searchRead(ctx context.Context, model string, domain []any, fields []string, order []any, limit int, extra map[string]any) ([]map[string]any, error) {
	var lim any
	if limit > 0 {
		lim = limit
	}
	var ord any
	if len(order) > 0 {
		ord = order
	}
	var out []map[string]any
	err := c.callCtx(ctx, model, "search_read", []any{domain, 0, lim, ord, fields}, extra, &out)
	return out, err
}

func (c *Client) read(ctx context.Context, model string, ids []int, fields []string) (map[int]map[string]any, error) {
	out := map[int]map[string]any{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []map[string]any
	if err := c.call(ctx, model, "read", []any{uniq(ids), fields}, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[int(num(r["id"]))] = r
	}
	return out, nil
}

// environment reads the user's company and currency and the warehouses once.
func (c *Client) environment(ctx context.Context) (*env, error) {
	c.mu.Lock()
	e := c.env
	c.mu.Unlock()
	if e != nil {
		return e, nil
	}
	var prefs map[string]any
	if err := c.rpc(ctx, "model.res.user.get_preferences", []any{true, map[string]any{}}, &prefs); err != nil {
		return nil, err
	}
	e = &env{company: int(num(prefs["company"]))}
	if e.company == 0 {
		return nil, errors.New("Tryton: the user has no current company")
	}
	c.mu.Lock()
	c.env = e
	c.mu.Unlock()
	comps, err := c.read(ctx, "company.company", []int{e.company}, []string{"currency"})
	if err != nil {
		return nil, err
	}
	e.currency = int(num(comps[e.company]["currency"]))
	whs, err := c.searchRead(ctx, "stock.location", []any{[]any{"type", "=", "warehouse"}}, []string{"input_location", "storage_location"}, []any{[]any{"id", "ASC"}}, 0, nil)
	if err != nil {
		return nil, err
	}
	for _, w := range whs {
		e.warehouses = append(e.warehouses, int(num(w["id"])))
	}
	if len(whs) == 0 {
		return nil, errors.New("Tryton: no warehouse")
	}
	e.warehouse = location{id: int(num(whs[0]["id"])), input: int(num(whs[0]["input_location"])), storage: int(num(whs[0]["storage_location"]))}
	return e, nil
}

func (c *Client) units(ctx context.Context) (map[int]uom, error) {
	c.mu.Lock()
	u := c.uoms
	c.mu.Unlock()
	if u != nil {
		return u, nil
	}
	rows, err := c.searchRead(ctx, "product.uom", []any{}, []string{"name", "category", "factor"}, nil, 0, map[string]any{"active_test": false})
	if err != nil {
		return nil, err
	}
	u = map[int]uom{}
	for _, r := range rows {
		u[int(num(r["id"]))] = uom{name: str(r["name"]), category: int(num(r["category"])), factor: num(r["factor"])}
	}
	c.mu.Lock()
	c.uoms = u
	c.mu.Unlock()
	return u, nil
}

// ratio is how many base units one unit holds (Carton x6 → 6).
func ratio(units map[int]uom, unit, base int) float64 {
	u, b := units[unit], units[base]
	if u.factor <= 0 || b.factor <= 0 || u.category != b.category {
		return 1
	}
	return u.factor / b.factor
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.environment(ctx)
	return err
}

// ---- reading ---------------------------------------------------------------------------------

var genericCategories = map[string]bool{"all": true, "tous": true, "saleable": true, "goods": true, "marchandises": true, "products": true, "produits": true}
var brandParents = map[string]bool{"marques": true, "marque": true, "brands": true, "brand": true}

type template struct {
	defaultUOM, saleUOM, purchaseUOM, accountCategory int
	categories                                        []int
}

func (c *Client) templates(ctx context.Context, ids []int) (map[int]template, error) {
	rows, err := c.read(ctx, "product.template", ids, []string{"default_uom", "sale_uom", "purchase_uom", "account_category", "categories"})
	if err != nil {
		return nil, err
	}
	out := map[int]template{}
	for id, r := range rows {
		out[id] = template{defaultUOM: int(num(r["default_uom"])), saleUOM: int(num(r["sale_uom"])), purchaseUOM: int(num(r["purchase_uom"])),
			accountCategory: int(num(r["account_category"])), categories: intList(r["categories"])}
	}
	return out, nil
}

// taxRates returns, per accounting category, the sum of its percentage taxes as Order Desk rates
// (19 % → 1900). field is customer_taxes_used or supplier_taxes_used.
func (c *Client) taxRates(ctx context.Context, categories []int, field string) map[int]int64 {
	out := map[int]int64{}
	cats, err := c.read(ctx, "product.category", categories, []string{field})
	if err != nil {
		return out
	}
	var taxIDs []int
	for _, r := range cats {
		taxIDs = append(taxIDs, intList(r[field])...)
	}
	taxes, _ := c.read(ctx, "account.tax", taxIDs, []string{"type", "rate"})
	for id, r := range cats {
		for _, t := range intList(r[field]) {
			if tx := taxes[t]; tx != nil && str(tx["type"]) == "percentage" {
				out[id] += int64(math.Round(num(tx["rate"]) * 10000))
			}
		}
	}
	return out
}

func (c *Client) Products(ctx context.Context) ([]catalog.Product, error) {
	if _, err := c.environment(ctx); err != nil {
		return nil, err
	}
	units, err := c.units(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := c.searchRead(ctx, "product.product", []any{[]any{"template.salable", "=", true}, []any{"template.type", "!=", "service"}},
		[]string{"name", "code", "template", "identifiers"}, []any{[]any{"id", "ASC"}}, 0, nil)
	if err != nil {
		return nil, err
	}
	var tmplIDs, identIDs []int
	for _, r := range rows {
		tmplIDs = append(tmplIDs, int(num(r["template"])))
		identIDs = append(identIDs, intList(r["identifiers"])...)
	}
	tmpls, err := c.templates(ctx, tmplIDs)
	if err != nil {
		return nil, err
	}
	var catIDs, accIDs []int
	for _, t := range tmpls {
		catIDs = append(catIDs, t.categories...)
		if t.accountCategory > 0 {
			accIDs = append(accIDs, t.accountCategory)
		}
	}
	cats, _ := c.read(ctx, "product.category", catIDs, []string{"name", "parent"})
	var parentIDs []int
	for _, r := range cats {
		if p := int(num(r["parent"])); p > 0 {
			parentIDs = append(parentIDs, p)
		}
	}
	parents, _ := c.read(ctx, "product.category", parentIDs, []string{"name"})
	rates := c.taxRates(ctx, accIDs, "customer_taxes_used")
	codes := map[int]string{}
	if idents, err := c.read(ctx, "product.identifier", identIDs, []string{"product", "code"}); err == nil {
		for _, id := range uniq(identIDs) { // the first identifier of the product is its barcode
			if r := idents[id]; r != nil && codes[int(num(r["product"]))] == "" {
				codes[int(num(r["product"]))] = str(r["code"])
			}
		}
	}
	out := make([]catalog.Product, 0, len(rows))
	for _, r := range rows {
		id := int(num(r["id"]))
		t := tmpls[int(num(r["template"]))]
		p := catalog.Product{ID: strconv.Itoa(id), Name: str(r["name"]), Barcode: codes[id], Unit: units[t.defaultUOM].name, VatRate: rates[t.accountCategory]}
		p.Brand = brandOf(t.categories, cats, parents)
		seen := map[int64]bool{}
		for _, u := range []int{t.purchaseUOM, t.saleUOM} {
			if f := ratio(units, u, t.defaultUOM); u > 0 && f > 1 && !seen[int64(math.Round(f*1000))] {
				seen[int64(math.Round(f*1000))] = true
				p.Packs = append(p.Packs, catalog.Pack{Name: units[u].name, FactorMilli: int64(math.Round(f * 1000))})
			}
		}
		sort.Slice(p.Packs, func(i, j int) bool { return p.Packs[i].FactorMilli < p.Packs[j].FactorMilli })
		out = append(out, p)
	}
	return out, nil
}

// brandOf prefers a category filed under "Marques"/"Brands", else the first category that is not
// a generic one.
func brandOf(ids []int, cats, parents map[int]map[string]any) string {
	fallback := ""
	for _, id := range ids {
		r := cats[id]
		if r == nil {
			continue
		}
		name := strings.TrimSpace(str(r["name"]))
		if p := parents[int(num(r["parent"]))]; p != nil && brandParents[strings.ToLower(strings.TrimSpace(str(p["name"])))] {
			return name
		}
		if fallback == "" && !genericCategories[strings.ToLower(name)] && !brandParents[strings.ToLower(name)] {
			fallback = name
		}
	}
	return fallback
}

func (c *Client) Customers(ctx context.Context) ([]catalog.Customer, error) {
	return c.parties(ctx, []string{"client", "customer"}, [][2]string{{"sale.sale", "party"}})
}

func (c *Client) Suppliers(ctx context.Context) ([]catalog.Customer, error) {
	return c.parties(ctx, []string{"fournisseur", "supplier", "vendor"}, [][2]string{{"purchase.product_supplier", "party"}, {"purchase.purchase", "party"}})
}

// parties: Tryton has no customer or supplier flag. A party is one when it sits in a category so
// named, or has the documents of one. Without either, every customer-side party is a customer.
func (c *Client) parties(ctx context.Context, words []string, docs [][2]string) ([]catalog.Customer, error) {
	e, err := c.environment(ctx)
	if err != nil {
		return nil, err
	}
	ids := map[int]bool{}
	var catDomain []any
	catDomain = append(catDomain, "OR")
	for _, w := range words {
		catDomain = append(catDomain, []any{"name", "ilike", w + "%"})
	}
	var cats []int
	if err := c.call(ctx, "party.category", "search", []any{catDomain}, &cats); err != nil {
		return nil, err
	}
	if len(cats) > 0 {
		var ps []int
		if err := c.call(ctx, "party.party", "search", []any{[]any{[]any{"categories", "in", cats}}}, &ps); err != nil {
			return nil, err
		}
		for _, p := range ps {
			ids[p] = true
		}
	}
	for _, d := range docs {
		rows, err := c.searchRead(ctx, d[0], []any{}, []string{d[1]}, []any{[]any{"id", "DESC"}}, 5000, nil)
		if err != nil {
			continue
		}
		for _, r := range rows {
			if p := int(num(r[d[1]])); p > 0 {
				ids[p] = true
			}
		}
	}
	comps, _ := c.read(ctx, "company.company", []int{e.company}, []string{"party"})
	own := int(num(comps[e.company]["party"]))
	domain := []any{[]any{"id", "in", keys(ids)}}
	if len(ids) == 0 && words[0] == "client" {
		sups, err := c.Suppliers(ctx)
		if err != nil {
			return nil, err
		}
		not := []int{own}
		for _, s := range sups {
			id, _ := strconv.Atoi(s.ID)
			not = append(not, id)
		}
		domain = []any{[]any{"id", "not in", not}}
	}
	rows, err := c.searchRead(ctx, "party.party", domain, []string{"name", "phone", "mobile"}, []any{[]any{"name", "ASC"}}, 0, nil)
	if err != nil {
		return nil, err
	}
	out := make([]catalog.Customer, 0, len(rows))
	for _, r := range rows {
		if int(num(r["id"])) == own {
			continue
		}
		phone := str(r["mobile"])
		if phone == "" {
			phone = str(r["phone"])
		}
		out = append(out, catalog.Customer{ID: strconv.Itoa(int(num(r["id"]))), Name: str(r["name"]), Phone: phone})
	}
	return out, nil
}

// Price asks Tryton for the sale price of the quantity for this customer (its price list rules,
// sale_price_list) and applies the product's customer taxes.
func (c *Client) Price(ctx context.Context, customerID, productID string, qtyMilli int64) (adapter.Quote, error) {
	cid, err := strconv.Atoi(customerID)
	if err != nil {
		return adapter.Quote{}, err
	}
	pid, err := strconv.Atoi(productID)
	if err != nil {
		return adapter.Quote{}, err
	}
	price, rate, source, err := c.salePrice(ctx, cid, pid, qtyMilli)
	if err != nil {
		return adapter.Quote{}, err
	}
	ht := price * float64(qtyMilli) / 1000
	return adapter.Quote{UnitPriceHT: int64(math.Round(price * 100)), LineTTC: int64(math.Round(ht * 100 * (1 + float64(rate)/10000))), Source: source}, nil
}

// salePrice is the unit price per base unit (in currency units), the VAT rate and where the price
// came from: the customer's price list (else the default sale price list) when the module is
// active, else the product's list price.
func (c *Client) salePrice(ctx context.Context, cid, pid int, qtyMilli int64) (float64, int64, string, error) {
	e, err := c.environment(ctx)
	if err != nil {
		return 0, 0, "", err
	}
	prods, err := c.read(ctx, "product.product", []int{pid}, []string{"template"})
	if err != nil || prods[pid] == nil {
		return 0, 0, "", fmt.Errorf("Tryton: product %d not found (%v)", pid, err)
	}
	tmpls, err := c.templates(ctx, []int{int(num(prods[pid]["template"]))})
	if err != nil {
		return 0, 0, "", err
	}
	t := tmpls[int(num(prods[pid]["template"]))]
	var taxes []int
	if cats, err := c.read(ctx, "product.category", []int{t.accountCategory}, []string{"customer_taxes_used"}); err == nil && cats[t.accountCategory] != nil {
		taxes = intList(cats[t.accountCategory]["customer_taxes_used"])
	}
	pctx := map[string]any{"customer": cid, "quantity": float64(qtyMilli) / 1000, "currency": e.currency, "uom": t.defaultUOM, "taxes": taxes}
	source := "Tryton list price"
	if pl, name := c.priceList(ctx, cid); pl > 0 {
		pctx["price_list"], source = pl, "Tryton "+name
	}
	var rows []map[string]any
	if err := c.callCtx(ctx, "product.product", "read", []any{[]int{pid}, []string{"sale_price_uom"}}, pctx, &rows); err != nil || len(rows) == 0 {
		return 0, 0, "", err
	}
	return num(rows[0]["sale_price_uom"]), c.taxRates(ctx, []int{t.accountCategory}, "customer_taxes_used")[t.accountCategory], source, nil
}

// priceList is the customer's sale price list, else the default one (module sale_price_list).
func (c *Client) priceList(ctx context.Context, cid int) (int, string) {
	pl := 0
	if parties, err := c.read(ctx, "party.party", []int{cid}, []string{"sale_price_list"}); err == nil && parties[cid] != nil {
		pl = int(num(parties[cid]["sale_price_list"]))
	}
	if pl == 0 {
		if conf, err := c.read(ctx, "sale.configuration", []int{1}, []string{"sale_price_list"}); err == nil && conf[1] != nil {
			pl = int(num(conf[1]["sale_price_list"]))
		}
	}
	if pl == 0 {
		return 0, ""
	}
	lists, err := c.read(ctx, "product.price_list", []int{pl}, []string{"name"})
	if err != nil || lists[pl] == nil {
		return 0, ""
	}
	return pl, str(lists[pl]["name"])
}

// Stock is the quantity on hand in the warehouses, in base units.
func (c *Client) Stock(ctx context.Context) (map[string]int64, error) {
	e, err := c.environment(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := c.searchRead(ctx, "product.product", []any{[]any{"template.type", "=", "goods"}}, []string{"quantity"}, nil, 0,
		map[string]any{"locations": e.warehouses})
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[strconv.Itoa(int(num(r["id"])))] = int64(math.Round(num(r["quantity"]) * 1000))
	}
	return out, nil
}

func (c *Client) History(ctx context.Context, customerID string, n int) ([]adapter.PastOrder, error) {
	cid, err := strconv.Atoi(customerID)
	if err != nil {
		return nil, err
	}
	units, err := c.units(ctx)
	if err != nil {
		return nil, err
	}
	orders, err := c.searchRead(ctx, "sale.sale", []any{[]any{"party", "=", cid}, []any{"state", "in", []string{"confirmed", "processing", "done"}}},
		[]string{"number", "sale_date"}, []any{[]any{"sale_date", "DESC"}, []any{"id", "DESC"}}, n, nil)
	if err != nil || len(orders) == 0 {
		return nil, err
	}
	var ids []int
	for _, o := range orders {
		ids = append(ids, int(num(o["id"])))
	}
	lines, err := c.searchRead(ctx, "sale.line", []any{[]any{"sale", "in", ids}, []any{"type", "=", "line"}, []any{"product", "!=", nil}},
		[]string{"sale", "product", "quantity", "unit"}, []any{[]any{"sequence", "ASC"}, []any{"id", "ASC"}}, 0, nil)
	if err != nil {
		return nil, err
	}
	var prodIDs []int
	for _, l := range lines {
		prodIDs = append(prodIDs, int(num(l["product"])))
	}
	baseUOM := map[int]int{}
	if prods, err := c.read(ctx, "product.product", prodIDs, []string{"default_uom"}); err == nil {
		for id, p := range prods {
			baseUOM[id] = int(num(p["default_uom"]))
		}
	}
	byOrder := map[int][]adapter.PastLine{}
	for _, l := range lines {
		pid := int(num(l["product"]))
		q := num(l["quantity"]) * ratio(units, int(num(l["unit"])), baseUOM[pid])
		byOrder[int(num(l["sale"]))] = append(byOrder[int(num(l["sale"]))], adapter.PastLine{ProductID: strconv.Itoa(pid), QtyMilli: int64(math.Round(q * 1000))})
	}
	var out []adapter.PastOrder
	for _, o := range orders {
		id := int(num(o["id"]))
		out = append(out, adapter.PastOrder{ID: strconv.Itoa(id), Number: str(o["number"]), Date: date(o["sale_date"]), Lines: byOrder[id]})
	}
	return out, nil
}

func (c *Client) NextDelivery(ctx context.Context, customerID string) (adapter.Delivery, error) {
	return adapter.Delivery{}, nil // Tryton has no delivery rounds out of the box
}

// OrderURL opens the sale in Tryton's web client.
func (c *Client) OrderURL(ref adapter.OrderRef) string {
	if ref.ID == "" {
		return ""
	}
	return c.BaseURL + "/#" + c.DB + "/model/sale.sale/" + ref.ID
}

// ---- writing ---------------------------------------------------------------------------------

// addresses picks the party's invoice and delivery addresses (flagged ones first, else the first).
func (c *Client) addresses(ctx context.Context, party int) (invoice, delivery int, err error) {
	rows, err := c.searchRead(ctx, "party.address", []any{[]any{"party", "=", party}}, []string{"invoice", "delivery"}, []any{[]any{"sequence", "ASC"}, []any{"id", "ASC"}}, 0, nil)
	if err != nil {
		return 0, 0, err
	}
	if len(rows) == 0 {
		return 0, 0, fmt.Errorf("Tryton: party %d has no address", party)
	}
	invoice, delivery = int(num(rows[0]["id"])), int(num(rows[0]["id"]))
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i]["invoice"] == true {
			invoice = int(num(rows[i]["id"]))
		}
		if rows[i]["delivery"] == true {
			delivery = int(num(rows[i]["id"]))
		}
	}
	return invoice, delivery, nil
}

// CreateOrder creates a sale priced by Tryton, quotes and confirms it. The shop's words go in the
// sale's comment and each line's note, so the order keeps its evidence.
func (c *Client) CreateOrder(ctx context.Context, o adapter.Order) (adapter.OrderRef, error) {
	e, err := c.environment(ctx)
	if err != nil {
		return adapter.OrderRef{}, err
	}
	cid, err := strconv.Atoi(o.CustomerID)
	if err != nil {
		return adapter.OrderRef{}, fmt.Errorf("customer id: %w", err)
	}
	inv, del, err := c.addresses(ctx, cid)
	if err != nil {
		return adapter.OrderRef{}, err
	}
	var pids []int
	for _, l := range o.Lines {
		pid, err := strconv.Atoi(l.ProductID)
		if err != nil {
			return adapter.OrderRef{}, fmt.Errorf("product id %q: %w", l.ProductID, err)
		}
		pids = append(pids, pid)
	}
	prods, err := c.read(ctx, "product.product", pids, []string{"template", "default_uom"})
	if err != nil {
		return adapter.OrderRef{}, err
	}
	var tmplIDs []int
	for _, p := range prods {
		tmplIDs = append(tmplIDs, int(num(p["template"])))
	}
	tmpls, err := c.templates(ctx, tmplIDs)
	if err != nil {
		return adapter.OrderRef{}, err
	}
	var accIDs []int
	for _, t := range tmpls {
		accIDs = append(accIDs, t.accountCategory)
	}
	cats, _ := c.read(ctx, "product.category", accIDs, []string{"customer_taxes_used"})
	lines := []any{}
	note := o.Notes
	for i, l := range o.Lines {
		p := prods[pids[i]]
		if p == nil {
			return adapter.OrderRef{}, fmt.Errorf("Tryton: product %d not found", pids[i])
		}
		t := tmpls[int(num(p["template"]))]
		price, _, _, err := c.salePrice(ctx, cid, pids[i], l.QtyMilli)
		if err != nil {
			return adapter.OrderRef{}, err
		}
		line := map[string]any{"type": "line", "product": pids[i], "quantity": float64(l.QtyMilli) / 1000, "unit": int(num(p["default_uom"])),
			"unit_price": decimal(price, 4), "taxes": []any{[]any{"add", intList(cats[t.accountCategory]["customer_taxes_used"])}}}
		if l.Notes != "" {
			line["note"] = l.Notes
			note += "\n" + l.Notes
		}
		lines = append(lines, line)
	}
	vals := map[string]any{"party": cid, "company": e.company, "currency": e.currency, "warehouse": e.warehouse.id,
		"invoice_address": inv, "shipment_address": del, "comment": note, "lines": []any{[]any{"create", lines}}}
	if o.ExternalRef != "" {
		vals["reference"] = o.ExternalRef
	}
	if pl, _ := c.priceList(ctx, cid); pl > 0 {
		vals["price_list"] = pl
	}
	var ids []int
	if err := c.call(ctx, "sale.sale", "create", []any{[]any{vals}}, &ids); err != nil {
		return adapter.OrderRef{}, err
	}
	if len(ids) == 0 {
		return adapter.OrderRef{}, errors.New("Tryton did not return the sale")
	}
	id := ids[0]
	for _, step := range []string{"quote", "confirm"} {
		if err := c.call(ctx, "sale.sale", step, []any{[]int{id}}, nil); err != nil {
			ref, _ := c.ref(ctx, "sale.sale", id, "")
			return ref, fmt.Errorf("sale %d created as a draft, but %s failed: %w", id, step, err)
		}
	}
	return c.ref(ctx, "sale.sale", id, "")
}

func (c *Client) ref(ctx context.Context, model string, id int, suffix string) (adapter.OrderRef, error) {
	rows, err := c.read(ctx, model, []int{id}, []string{"number", "total_amount"})
	if err != nil || rows[id] == nil {
		return adapter.OrderRef{ID: strconv.Itoa(id)}, err
	}
	return adapter.OrderRef{ID: strconv.Itoa(id), Number: str(rows[id]["number"]) + suffix, TotalTTC: int64(math.Round(num(rows[id]["total_amount"]) * 100))}, nil
}

// LastPurchasePrices: the last price paid to this supplier per product (confirmed purchase lines,
// newest first), else the supplier's prices. Centimes per base unit.
func (c *Client) LastPurchasePrices(ctx context.Context, supplierID string) (map[string]int64, error) {
	sid, err := strconv.Atoi(supplierID)
	if err != nil {
		return nil, err
	}
	units, err := c.units(ctx)
	if err != nil {
		return nil, err
	}
	lines, err := c.searchRead(ctx, "purchase.line", []any{[]any{"purchase.party", "=", sid}, []any{"purchase.state", "in", []string{"confirmed", "processing", "done"}},
		[]any{"type", "=", "line"}, []any{"product", "!=", nil}}, []string{"product", "unit_price", "unit"}, []any{[]any{"id", "DESC"}}, 2000, nil)
	if err != nil {
		return nil, err
	}
	infos, _ := c.searchRead(ctx, "purchase.product_supplier", []any{[]any{"party", "=", sid}}, []string{"product", "template", "prices", "unit"}, nil, 0, nil)
	var prodIDs, tmplIDs, priceIDs []int
	for _, l := range lines {
		prodIDs = append(prodIDs, int(num(l["product"])))
	}
	for _, s := range infos {
		if p := int(num(s["product"])); p > 0 {
			prodIDs = append(prodIDs, p)
		} else {
			tmplIDs = append(tmplIDs, int(num(s["template"])))
		}
		priceIDs = append(priceIDs, intList(s["prices"])...)
	}
	variants := map[int][]int{}
	if len(tmplIDs) > 0 {
		vs, _ := c.searchRead(ctx, "product.product", []any{[]any{"template", "in", uniq(tmplIDs)}}, []string{"template"}, nil, 0, nil)
		for _, v := range vs {
			variants[int(num(v["template"]))] = append(variants[int(num(v["template"]))], int(num(v["id"])))
			prodIDs = append(prodIDs, int(num(v["id"])))
		}
	}
	baseUOM := map[int]int{}
	if prods, err := c.read(ctx, "product.product", prodIDs, []string{"default_uom"}); err == nil {
		for id, p := range prods {
			baseUOM[id] = int(num(p["default_uom"]))
		}
	}
	out := map[string]int64{}
	for _, l := range lines {
		pid := int(num(l["product"]))
		if _, seen := out[strconv.Itoa(pid)]; !seen {
			out[strconv.Itoa(pid)] = int64(math.Round(num(l["unit_price"]) / ratio(units, int(num(l["unit"])), baseUOM[pid]) * 100))
		}
	}
	prices, _ := c.read(ctx, "purchase.product_supplier.price", priceIDs, []string{"quantity", "unit_price"})
	for _, s := range infos {
		ids := variants[int(num(s["template"]))]
		if p := int(num(s["product"])); p > 0 {
			ids = []int{p}
		}
		// the price for the smallest quantity, in the supplier's unit (the purchase unit)
		best, bestQty := -1.0, math.MaxFloat64
		for _, pr := range intList(s["prices"]) {
			if r := prices[pr]; r != nil && num(r["quantity"]) < bestQty {
				best, bestQty = num(r["unit_price"]), num(r["quantity"])
			}
		}
		if best < 0 {
			continue
		}
		for _, p := range ids {
			if _, seen := out[strconv.Itoa(p)]; !seen {
				out[strconv.Itoa(p)] = int64(math.Round(best / ratio(units, int(num(s["unit"])), baseUOM[p]) * 100))
			}
		}
	}
	return out, nil
}

// CreateReceipt books the approved supplier invoice as a confirmed purchase and puts its moves in
// a supplier shipment; when Post is set the shipment is received and done: Tryton moves the stock
// and updates the cost price. A line counted in packs is booked in the matching pack unit at the
// pack price, as printed.
func (c *Client) CreateReceipt(ctx context.Context, r adapter.Receipt) (adapter.OrderRef, error) {
	e, err := c.environment(ctx)
	if err != nil {
		return adapter.OrderRef{}, err
	}
	sid, err := strconv.Atoi(r.SupplierID)
	if err != nil {
		return adapter.OrderRef{}, fmt.Errorf("supplier id: %w", err)
	}
	inv, _, err := c.addresses(ctx, sid)
	if err != nil {
		return adapter.OrderRef{}, err
	}
	units, err := c.units(ctx)
	if err != nil {
		return adapter.OrderRef{}, err
	}
	var pids []int
	for _, l := range r.Lines {
		pid, err := strconv.Atoi(l.ProductID)
		if err != nil {
			return adapter.OrderRef{}, fmt.Errorf("product id %q: %w", l.ProductID, err)
		}
		pids = append(pids, pid)
	}
	prods, err := c.read(ctx, "product.product", pids, []string{"template", "default_uom"})
	if err != nil {
		return adapter.OrderRef{}, err
	}
	var tmplIDs []int
	for _, p := range prods {
		tmplIDs = append(tmplIDs, int(num(p["template"])))
	}
	tmpls, err := c.templates(ctx, tmplIDs)
	if err != nil {
		return adapter.OrderRef{}, err
	}
	var accIDs []int
	for _, t := range tmpls {
		accIDs = append(accIDs, t.accountCategory)
	}
	cats, _ := c.read(ctx, "product.category", accIDs, []string{"supplier_taxes_used"})
	lines := []any{}
	for i, l := range r.Lines {
		p := prods[pids[i]]
		if p == nil {
			return adapter.OrderRef{}, fmt.Errorf("Tryton: product %d not found", pids[i])
		}
		t := tmpls[int(num(p["template"]))]
		base := int(num(p["default_uom"]))
		factor := float64(l.PackFactorMilli) / 1000
		if factor <= 0 {
			factor = 1
		}
		line := map[string]any{"type": "line", "product": pids[i]}
		if pack := packUnit(units, base, t.purchaseUOM, factor); pack > 0 && factor > 1 {
			line["unit"], line["quantity"], line["unit_price"] = pack, float64(l.QtyMilli)/1000, decimal(float64(l.UnitCostHT)/100, 4)
		} else {
			line["unit"], line["quantity"], line["unit_price"] = base, float64(l.QtyMilli)/1000*factor, decimal(float64(l.UnitCostHT)/100/factor, 4)
		}
		taxes := intList(cats[t.accountCategory]["supplier_taxes_used"])
		if len(taxes) == 0 && l.VatRate > 0 {
			c.call(ctx, "account.tax", "search", []any{[]any{[]any{"type", "=", "percentage"}, []any{"rate", "=", decimal(float64(l.VatRate)/10000, 4)},
				[]any{"parent", "=", nil}}, 0, 1}, &taxes)
		}
		line["taxes"] = []any{[]any{"add", taxes}}
		if l.Description != "" {
			line["description"] = l.Description
		}
		lines = append(lines, line)
	}
	po := map[string]any{"party": sid, "company": e.company, "currency": e.currency, "warehouse": e.warehouse.id, "invoice_address": inv,
		"lines": []any{[]any{"create", lines}}}
	if d, err := time.Parse("2006-01-02", r.Date); err == nil {
		po["purchase_date"] = dateValue(d)
	}
	if r.SupplierRef != "" {
		po["reference"] = r.SupplierRef
	}
	if r.Notes != "" {
		po["comment"] = r.Notes
	}
	var ids []int
	if err := c.call(ctx, "purchase.purchase", "create", []any{[]any{po}}, &ids); err != nil {
		return adapter.OrderRef{}, err
	}
	if len(ids) == 0 {
		return adapter.OrderRef{}, errors.New("Tryton did not return the purchase")
	}
	id := ids[0]
	for _, step := range []string{"quote", "confirm"} {
		if err := c.call(ctx, "purchase.purchase", step, []any{[]int{id}}, nil); err != nil {
			ref, _ := c.ref(ctx, "purchase.purchase", id, "")
			return ref, fmt.Errorf("purchase %d created as a draft, but %s failed: %w", id, step, err)
		}
	}
	number, err := c.receive(ctx, e, id, sid, r)
	if err != nil {
		ref, _ := c.ref(ctx, "purchase.purchase", id, "")
		return ref, fmt.Errorf("purchase confirmed, but the receipt was not booked: %w", err)
	}
	suffix := " · " + number
	if !r.Post {
		suffix += " (brouillon)"
	}
	return c.ref(ctx, "purchase.purchase", id, suffix)
}

// packUnit finds the unit holding factor base units: the product's purchase unit first, else any
// unit of the same category.
func packUnit(units map[int]uom, base, purchase int, factor float64) int {
	if purchase > 0 && math.Abs(ratio(units, purchase, base)-factor) < 1e-6 {
		return purchase
	}
	ids := make([]int, 0, len(units))
	for id := range units {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		if units[id].category == units[base].category && math.Abs(ratio(units, id, base)-factor) < 1e-6 {
			return id
		}
	}
	return 0
}

// receive puts the purchase's incoming moves in a supplier shipment, received and done when posted.
func (c *Client) receive(ctx context.Context, e *env, purchase, supplier int, r adapter.Receipt) (string, error) {
	moves, err := c.purchaseMoves(ctx, purchase)
	if err != nil {
		return "", err
	}
	if len(moves) == 0 { // processing is queued on this server: run it now
		if err := c.call(ctx, "purchase.purchase", "process", []any{[]int{purchase}}, nil); err != nil {
			return "", err
		}
		if moves, err = c.purchaseMoves(ctx, purchase); err != nil {
			return "", err
		}
	}
	if len(moves) == 0 {
		return "", errors.New("Tryton created no stock move for the purchase")
	}
	sh := map[string]any{"supplier": supplier, "company": e.company, "warehouse": e.warehouse.id, "warehouse_input": e.warehouse.input,
		"warehouse_storage": e.warehouse.storage, "incoming_moves": []any{[]any{"add", moves}}}
	if r.SupplierRef != "" {
		sh["reference"] = r.SupplierRef
	}
	var ids []int
	if err := c.call(ctx, "stock.shipment.in", "create", []any{[]any{sh}}, &ids); err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", errors.New("Tryton did not return the shipment")
	}
	if r.Post {
		for _, step := range []string{"receive", "done"} {
			if err := c.call(ctx, "stock.shipment.in", step, []any{ids}, nil); err != nil {
				return "", fmt.Errorf("shipment %d: %s failed: %w", ids[0], step, err)
			}
		}
	}
	rows, err := c.read(ctx, "stock.shipment.in", ids, []string{"number"})
	if err != nil {
		return "", err
	}
	if n := str(rows[ids[0]]["number"]); n != "" {
		return n, nil
	}
	return "shipment " + strconv.Itoa(ids[0]), nil
}

func (c *Client) purchaseMoves(ctx context.Context, purchase int) ([]int, error) {
	rows, err := c.read(ctx, "purchase.purchase", []int{purchase}, []string{"moves"})
	if err != nil {
		return nil, err
	}
	moves := intList(rows[purchase]["moves"])
	if len(moves) == 0 {
		return nil, nil
	}
	// only the moves still waiting for a shipment
	var free []int
	if err := c.call(ctx, "stock.move", "search", []any{[]any{[]any{"id", "in", moves}, []any{"shipment", "=", nil}, []any{"state", "=", "draft"}}}, &free); err != nil {
		return nil, err
	}
	return free, nil
}

// ---- JSON helpers: Tryton sends decimals and dates as tagged objects, relations as ids ---------

func decimal(v float64, digits int) map[string]any {
	return map[string]any{"__class__": "Decimal", "decimal": strconv.FormatFloat(v, 'f', digits, 64)}
}

func dateValue(t time.Time) map[string]any {
	return map[string]any{"__class__": "date", "year": t.Year(), "month": int(t.Month()), "day": t.Day()}
}

func date(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", int(num(m["year"])), int(num(m["month"])), int(num(m["day"])))
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
	case map[string]any:
		if x["__class__"] == "Decimal" {
			f, _ := strconv.ParseFloat(str(x["decimal"]), 64)
			return f
		}
	}
	return 0
}

func intList(v any) []int {
	a, _ := v.([]any)
	out := make([]int, 0, len(a))
	for _, x := range a {
		out = append(out, int(num(x)))
	}
	return out
}

func uniq(ids []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func keys(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

var _ adapter.ERP = (*Client)(nil)
