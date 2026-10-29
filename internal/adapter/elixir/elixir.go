// Package elixir is the adapter for Elixir Distribution (Elixir Gestion). It talks to the ERP
// only through the ERP's own HTTP API (/api/v1), with a user session like any other client.
package elixir

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
)

type Client struct {
	BaseURL string // e.g. http://127.0.0.1:8543
	HTTP    *http.Client

	mu       sync.Mutex
	token    string
	login    func() (user, pass string) // asked again when the session expires
	onToken  func(string)
	routes   map[string]route
	routesAt time.Time
}

type route struct {
	Name     string
	Weekdays []int
}

// New returns an adapter using an existing session token. login, when set, is used to open a new
// session if the token is missing or expired; onToken persists the new token.
func New(baseURL, token string, login func() (string, string), onToken func(string)) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), token: token, login: login, onToken: onToken,
		HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Name() string { return "Elixir Distribution" }

// Login opens a session with a username and password and returns the token.
func Login(ctx context.Context, baseURL, user, pass string) (string, string, error) {
	body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var out struct {
		Token string `json:"token"`
		Error string `json:"error"`
		User  struct {
			Name     string `json:"name"`
			Username string `json:"username"`
		} `json:"user"`
	}
	json.Unmarshal(raw, &out)
	if res.StatusCode != http.StatusOK || out.Token == "" {
		if out.Error == "" {
			out.Error = fmt.Sprintf("HTTP %d", res.StatusCode)
		}
		return "", "", errors.New(out.Error)
	}
	name := out.User.Name
	if name == "" {
		name = out.User.Username
	}
	return out.Token, name, nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	for attempt := 0; attempt < 2; attempt++ {
		c.mu.Lock()
		tok := c.token
		c.mu.Unlock()
		if tok == "" {
			if err := c.relogin(ctx); err != nil {
				return err
			}
			continue
		}
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequestWithContext(ctx, method, c.BaseURL+"/api/v1"+path, rd)
		req.Header.Set("Authorization", "Bearer "+tok)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := c.HTTP.Do(req)
		if err != nil {
			return fmt.Errorf("ERP unreachable: %w", err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.mu.Lock()
			c.token = ""
			c.mu.Unlock()
			continue
		}
		if res.StatusCode >= 300 {
			var e struct {
				Error string `json:"error"`
			}
			json.Unmarshal(raw, &e)
			if e.Error == "" {
				e.Error = string(raw)
			}
			return fmt.Errorf("ERP %s %s: HTTP %d: %s", method, path, res.StatusCode, e.Error)
		}
		if out != nil {
			return json.Unmarshal(raw, out)
		}
		return nil
	}
	return adapter.ErrNotConnected
}

func (c *Client) relogin(ctx context.Context) error {
	if c.login == nil {
		return adapter.ErrNotConnected
	}
	u, p := c.login()
	if u == "" {
		return adapter.ErrNotConnected
	}
	tok, _, err := Login(ctx, c.BaseURL, u, p)
	if err != nil {
		return fmt.Errorf("ERP login: %w", err)
	}
	c.mu.Lock()
	c.token = tok
	c.mu.Unlock()
	if c.onToken != nil {
		c.onToken(tok)
	}
	return nil
}

func (c *Client) Ping(ctx context.Context) error {
	var me map[string]any
	return c.do(ctx, http.MethodGet, "/auth/me", nil, &me)
}

func (c *Client) Products(ctx context.Context) ([]catalog.Product, error) {
	var out []catalog.Product
	for offset := 0; ; offset += 500 {
		var page struct {
			Items []struct {
				ID         string  `json:"id"`
				Name       string  `json:"name"`
				NameAr     *string `json:"name_ar"`
				Brand      string  `json:"brand"`
				Barcode    *string `json:"barcode"`
				Unit       string  `json:"unit"`
				Kind       string  `json:"kind"`
				VatRate    int64   `json:"vat_rate"`
				Packagings []struct {
					Name        string `json:"name"`
					FactorMilli int64  `json:"factor_milli"`
				} `json:"packagings"`
			} `json:"items"`
		}
		if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/products?limit=500&offset=%d", offset), nil, &page); err != nil {
			return nil, err
		}
		for _, it := range page.Items {
			if it.Kind == "service" {
				continue
			}
			p := catalog.Product{ID: it.ID, Name: it.Name, Brand: it.Brand, Unit: it.Unit, VatRate: it.VatRate}
			if it.NameAr != nil {
				p.NameAr = *it.NameAr
			}
			if it.Barcode != nil {
				p.Barcode = *it.Barcode
			}
			for _, k := range it.Packagings {
				p.Packs = append(p.Packs, catalog.Pack{Name: k.Name, FactorMilli: k.FactorMilli})
			}
			out = append(out, p)
		}
		if len(page.Items) < 500 {
			return out, nil
		}
	}
}

func (c *Client) Customers(ctx context.Context) ([]catalog.Customer, error) { return c.partners(ctx, "customer") }

func (c *Client) Suppliers(ctx context.Context) ([]catalog.Customer, error) { return c.partners(ctx, "supplier") }

func (c *Client) partners(ctx context.Context, kind string) ([]catalog.Customer, error) {
	var out []catalog.Customer
	for offset := 0; ; offset += 500 {
		var page struct {
			Items []struct {
				ID      string  `json:"id"`
				Kind    string  `json:"kind"`
				Name    string  `json:"name"`
				Phone   *string `json:"phone"`
				RouteID *string `json:"route_id"`
			} `json:"items"`
		}
		if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/partners?kind=%s&limit=500&offset=%d", kind, offset), nil, &page); err != nil {
			return nil, err
		}
		for _, it := range page.Items {
			if it.Kind != "" && it.Kind != kind && it.Kind != "both" {
				continue
			}
			cu := catalog.Customer{ID: it.ID, Name: it.Name}
			if it.Phone != nil {
				cu.Phone = *it.Phone
			}
			if it.RouteID != nil {
				cu.RouteID = *it.RouteID
			}
			out = append(out, cu)
		}
		if len(page.Items) < 500 {
			return out, nil
		}
	}
}

func (c *Client) Price(ctx context.Context, customerID, productID string, qtyMilli int64) (adapter.Quote, error) {
	q := url.Values{"partner_id": {customerID}, "product_id": {productID}, "qty_milli": {fmt.Sprint(qtyMilli)},
		"date": {time.Now().Format("2006-01-02")}}
	var out struct {
		Quote struct {
			UnitPrice     int64  `json:"unit_price"`
			LineTTC       int64  `json:"line_ttc"`
			FreeQtyMilli  int64  `json:"qty_free_milli"`
			Source        string `json:"price_source"`
			PromoRuleName string `json:"promo_rule_name"`
		} `json:"quote"`
	}
	if err := c.do(ctx, http.MethodGet, "/pricing/resolve?"+q.Encode(), nil, &out); err != nil {
		return adapter.Quote{}, err
	}
	return adapter.Quote{UnitPriceHT: out.Quote.UnitPrice, LineTTC: out.Quote.LineTTC, FreeQtyMilli: out.Quote.FreeQtyMilli,
		Source: out.Quote.Source, Offer: out.Quote.PromoRuleName}, nil
}

func (c *Client) Stock(ctx context.Context) (map[string]int64, error) {
	var out struct {
		Items []struct {
			ProductID string `json:"product_id"`
			QtyMilli  int64  `json:"qty_milli"`
		} `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, "/stock/levels?limit=5000", nil, &out); err != nil {
		return nil, err
	}
	m := map[string]int64{}
	for _, it := range out.Items {
		m[it.ProductID] += it.QtyMilli
	}
	return m, nil
}

func (c *Client) History(ctx context.Context, customerID string, n int) ([]adapter.PastOrder, error) {
	var list struct {
		Items []struct {
			ID        string `json:"id"`
			Number    string `json:"number"`
			OrderDate string `json:"order_date"`
			Status    string `json:"status"`
		} `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/orders?partner_id=%s&limit=%d", url.QueryEscape(customerID), n), nil, &list); err != nil {
		return nil, err
	}
	var out []adapter.PastOrder
	for _, it := range list.Items {
		if it.Status == "cancelled" {
			continue
		}
		var o struct {
			Lines []struct {
				ProductID       string `json:"product_id"`
				QtyOrderedMilli int64  `json:"qty_ordered_milli"`
			} `json:"lines"`
		}
		if err := c.do(ctx, http.MethodGet, "/orders/"+url.PathEscape(it.ID), nil, &o); err != nil {
			continue
		}
		po := adapter.PastOrder{ID: it.ID, Number: it.Number, Date: it.OrderDate}
		for _, l := range o.Lines {
			po.Lines = append(po.Lines, adapter.PastLine{ProductID: l.ProductID, QtyMilli: l.QtyOrderedMilli})
		}
		out = append(out, po)
	}
	return out, nil
}

func (c *Client) CreateOrder(ctx context.Context, o adapter.Order) (adapter.OrderRef, error) {
	lines := []map[string]any{}
	for _, l := range o.Lines {
		lines = append(lines, map[string]any{"product_id": l.ProductID, "qty_milli": l.QtyMilli, "notes": l.Notes})
	}
	body := map[string]any{"partner_id": o.CustomerID, "source": "whatsapp", "lines": lines, "notes": o.Notes}
	if o.ExternalRef != "" {
		body["external_ref"] = o.ExternalRef
	}
	var out struct {
		ID       string `json:"id"`
		Number   string `json:"number"`
		TotalTTC int64  `json:"total_ttc"`
		Order    *struct {
			ID       string `json:"id"`
			Number   string `json:"number"`
			TotalTTC int64  `json:"total_ttc"`
		} `json:"order"`
	}
	if err := c.do(ctx, http.MethodPost, "/orders", body, &out); err != nil {
		return adapter.OrderRef{}, err
	}
	if out.ID == "" && out.Order != nil {
		return adapter.OrderRef{ID: out.Order.ID, Number: out.Order.Number, TotalTTC: out.Order.TotalTTC}, nil
	}
	return adapter.OrderRef{ID: out.ID, Number: out.Number, TotalTTC: out.TotalTTC}, nil
}

func (c *Client) NextDelivery(ctx context.Context, customerID string) (adapter.Delivery, error) {
	var p struct {
		RouteID *string `json:"route_id"`
	}
	if err := c.do(ctx, http.MethodGet, "/partners/"+url.PathEscape(customerID), nil, &p); err != nil || p.RouteID == nil {
		return adapter.Delivery{}, err
	}
	c.mu.Lock()
	stale := time.Since(c.routesAt) > 10*time.Minute
	c.mu.Unlock()
	if stale {
		var rs struct {
			Items []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Weekdays []int  `json:"weekdays"`
			} `json:"items"`
		}
		if err := c.do(ctx, http.MethodGet, "/delivery-routes", nil, &rs); err == nil {
			m := map[string]route{}
			for _, r := range rs.Items {
				m[r.ID] = route{Name: r.Name, Weekdays: r.Weekdays}
			}
			c.mu.Lock()
			c.routes, c.routesAt = m, time.Now()
			c.mu.Unlock()
		}
	}
	c.mu.Lock()
	r, ok := c.routes[*p.RouteID]
	c.mu.Unlock()
	if !ok {
		return adapter.Delivery{}, nil
	}
	names := []string{"", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi", "dimanche"}
	for i := 1; i <= 7; i++ {
		t := time.Now().AddDate(0, 0, i)
		iso := (int(t.Weekday())+6)%7 + 1
		for _, w := range r.Weekdays {
			if w == iso {
				return adapter.Delivery{Day: names[iso] + " " + t.Format("02/01"), Route: r.Name}, nil
			}
		}
	}
	return adapter.Delivery{Route: r.Name}, nil
}

func (c *Client) OrderURL(ref adapter.OrderRef) string { return "" }

func (c *Client) LastPurchasePrices(ctx context.Context, supplierID string) (map[string]int64, error) {
	var out struct {
		Prices map[string]int64 `json:"prices"`
	}
	if err := c.do(ctx, http.MethodGet, "/purchases/last-prices?partner_id="+url.QueryEscape(supplierID), nil, &out); err != nil {
		return nil, err
	}
	return out.Prices, nil
}

func (c *Client) defaultWarehouse(ctx context.Context) (string, error) {
	var out struct {
		Items []struct {
			ID        string `json:"id"`
			IsDefault bool   `json:"is_default"`
			Active    bool   `json:"active"`
		} `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, "/warehouses", nil, &out); err != nil {
		return "", err
	}
	first := ""
	for _, w := range out.Items {
		if w.IsDefault {
			return w.ID, nil
		}
		if first == "" && w.Active {
			first = w.ID
		}
	}
	return first, nil
}

// CreateReceipt books a goods receipt (bon de réception, PURCHASE_RECEIPT) and posts it when asked:
// the ERP's own posting moves the stock and records the purchase prices.
func (c *Client) CreateReceipt(ctx context.Context, r adapter.Receipt) (adapter.OrderRef, error) {
	wh, err := c.defaultWarehouse(ctx)
	if err != nil {
		return adapter.OrderRef{}, err
	}
	lines := []map[string]any{}
	for i, l := range r.Lines {
		lines = append(lines, map[string]any{"position": i + 1, "product_id": l.ProductID, "description": l.Description,
			"qty_milli": l.QtyMilli, "unit_price": l.UnitCostHT, "vat_rate": l.VatRate, "pack_factor_milli": l.PackFactorMilli})
	}
	body := map[string]any{"doc_type": "PURCHASE_RECEIPT", "doc_date": r.Date, "partner_id": r.SupplierID, "notes": r.Notes, "lines": lines}
	if wh != "" {
		body["warehouse_id"] = wh
	}
	if r.SupplierRef != "" {
		body["supplier_ref"] = r.SupplierRef
	}
	var doc struct {
		ID       string  `json:"id"`
		Number   *string `json:"number"`
		TotalTTC int64   `json:"total_ttc"`
	}
	if err := c.do(ctx, http.MethodPost, "/documents", body, &doc); err != nil {
		return adapter.OrderRef{}, err
	}
	ref := adapter.OrderRef{ID: doc.ID, TotalTTC: doc.TotalTTC}
	if doc.Number != nil {
		ref.Number = *doc.Number
	}
	if r.Post {
		var posted struct {
			Number   *string `json:"number"`
			TotalTTC int64   `json:"total_ttc"`
		}
		if err := c.do(ctx, http.MethodPost, "/documents/"+url.PathEscape(doc.ID)+"/post", map[string]any{}, &posted); err != nil {
			return ref, fmt.Errorf("receipt created as a draft, but posting failed: %w", err)
		}
		if posted.Number != nil {
			ref.Number = *posted.Number
		}
	}
	if ref.Number == "" {
		ref.Number = "brouillon " + doc.ID[:8]
	}
	return ref, nil
}
