// Package demo is a stand-in ERP for trying Order Desk with nothing else installed: the catalogue,
// shops and suppliers are the ones already in the desk's own database, and what the desk books is
// written to local files, one JSON line per document.
//
//	products   the desk's products and packs tables (a trained desk ships with its catalogue)
//	customers  the desk's customers table (see AddShops); suppliers likewise
//	order      appended to <data>/demo-orders.jsonl, numbered DEMO-0001, DEMO-0002...
//	receipt    appended to <data>/demo-receipts.jsonl, numbered DEMO-R-0001...; stock goes up
//
// Prices are fake but stable (the same product always costs the same), stock is plenty and the next
// delivery is tomorrow.
package demo

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// Plenty is the stock of every product before any receipt: 1,000 units (milli).
const Plenty = 1000 * 1000

// Shops are the fictional shops a demo desk offers in the panel's "Try a message".
var Shops = []catalog.Customer{
	{ID: "demo-shop-1", Name: "Supérette Nour (démo)"},
	{ID: "demo-shop-2", Name: "Épicerie du Marché (démo)"},
	{ID: "demo-shop-3", Name: "Alimentation Les Palmiers (démo)"},
}

// AddShops adds the demo shops the desk does not have yet and returns how many were added.
func AddShops(db *store.DB) (int, error) {
	n := 0
	for _, s := range Shops {
		res, err := db.Exec(`INSERT OR IGNORE INTO customers (id, name, phone, route_id) VALUES (?, ?, '', '')`, s.ID, s.Name)
		if err != nil {
			return n, err
		}
		if k, _ := res.RowsAffected(); k > 0 {
			n++
		}
	}
	return n, nil
}

type Client struct {
	DB  *store.DB
	Dir string // where demo-orders.jsonl and demo-receipts.jsonl are written

	mu sync.Mutex // one writer per file
}

func New(db *store.DB, dir string) *Client { return &Client{DB: db, Dir: dir} }

func (c *Client) Name() string { return "Demo ERP (local files)" }

func (c *Client) Ping(ctx context.Context) error { return c.DB.PingContext(ctx) }

// OrdersFile and ReceiptsFile are where booked documents land.
func (c *Client) OrdersFile() string   { return filepath.Join(c.Dir, "demo-orders.jsonl") }
func (c *Client) ReceiptsFile() string { return filepath.Join(c.Dir, "demo-receipts.jsonl") }

// ---- reading ---------------------------------------------------------------------------------

func (c *Client) Products(ctx context.Context) ([]catalog.Product, error) {
	rows, err := c.DB.QueryContext(ctx, `SELECT id, name, name_ar, brand, barcode, unit, vat_rate FROM products WHERE active = 1 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var out []catalog.Product
	at := map[string]int{}
	for rows.Next() {
		var p catalog.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.NameAr, &p.Brand, &p.Barcode, &p.Unit, &p.VatRate); err != nil {
			rows.Close()
			return nil, err
		}
		at[p.ID] = len(out)
		out = append(out, p)
	}
	rows.Close()
	pr, err := c.DB.QueryContext(ctx, `SELECT product_id, name, factor_milli FROM packs ORDER BY factor_milli`)
	if err != nil {
		return nil, err
	}
	defer pr.Close()
	for pr.Next() {
		var id string
		var k catalog.Pack
		if pr.Scan(&id, &k.Name, &k.FactorMilli) == nil {
			if i, ok := at[id]; ok {
				out[i].Packs = append(out[i].Packs, k)
			}
		}
	}
	return out, nil
}

func (c *Client) Customers(ctx context.Context) ([]catalog.Customer, error) {
	rows, err := c.DB.QueryContext(ctx, `SELECT id, name, phone, route_id FROM customers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []catalog.Customer{}
	for rows.Next() {
		var cu catalog.Customer
		if rows.Scan(&cu.ID, &cu.Name, &cu.Phone, &cu.RouteID) == nil {
			out = append(out, cu)
		}
	}
	return out, nil
}

func (c *Client) Suppliers(ctx context.Context) ([]catalog.Customer, error) {
	return catalog.Suppliers(c.DB.DB), nil
}

// Price is a fake sales price that never changes for a product: 50 to 999 DA HT per unit, plus the
// product's VAT.
func (c *Client) Price(ctx context.Context, customerID, productID string, qtyMilli int64) (adapter.Quote, error) {
	var vat int64
	if err := c.DB.QueryRowContext(ctx, `SELECT vat_rate FROM products WHERE id = ?`, productID).Scan(&vat); err != nil {
		return adapter.Quote{}, fmt.Errorf("unknown product %q", productID)
	}
	unit := unitPrice(productID)
	return adapter.Quote{UnitPriceHT: unit, LineTTC: lineTTC(unit, qtyMilli, vat), Source: "demo price (fixed per product)"}, nil
}

func unitPrice(productID string) int64 {
	h := fnv.New32a()
	h.Write([]byte(productID))
	return (50 + int64(h.Sum32()%950)) * 100
}

func lineTTC(unit, qtyMilli, vat int64) int64 {
	ht := float64(unit) * float64(qtyMilli) / 1000
	return int64(math.Round(ht * (1 + float64(vat)/10000)))
}

// Stock is plenty of everything, plus what the demo receipts brought in. Orders do not take stock out.
func (c *Client) Stock(ctx context.Context) (map[string]int64, error) {
	rows, err := c.DB.QueryContext(ctx, `SELECT id FROM products WHERE active = 1`)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			out[id] = Plenty
		}
	}
	rows.Close()
	c.mu.Lock()
	defer c.mu.Unlock()
	var recs []receipt
	if err := readLines(c.ReceiptsFile(), &recs); err != nil {
		return nil, err
	}
	for _, r := range recs {
		for _, l := range r.Lines {
			if _, ok := out[l.ProductID]; ok {
				out[l.ProductID] += l.QtyMilli * factor(l.PackFactorMilli) / 1000
			}
		}
	}
	return out, nil
}

// History is the shop's orders this adapter wrote, newest first.
func (c *Client) History(ctx context.Context, customerID string, n int) ([]adapter.PastOrder, error) {
	c.mu.Lock()
	var all []order
	err := readLines(c.OrdersFile(), &all)
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	var out []adapter.PastOrder
	for i := len(all) - 1; i >= 0 && (n <= 0 || len(out) < n); i-- {
		o := all[i]
		if o.CustomerID != customerID {
			continue
		}
		po := adapter.PastOrder{ID: o.ID, Number: o.Number, Date: o.Date}
		for _, l := range o.Lines {
			po.Lines = append(po.Lines, adapter.PastLine{ProductID: l.ProductID, QtyMilli: l.QtyMilli})
		}
		out = append(out, po)
	}
	return out, nil
}

var weekdays = []string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"}

// NextDelivery is always tomorrow, on the demo round.
func (c *Client) NextDelivery(ctx context.Context, customerID string) (adapter.Delivery, error) {
	t := time.Now().AddDate(0, 0, 1)
	return adapter.Delivery{Day: weekdays[t.Weekday()] + " " + t.Format("02/01"), Route: "Démo"}, nil
}

func (c *Client) OrderURL(ref adapter.OrderRef) string { return "" } // no ERP screens: the order is a line in OrdersFile

// LastPurchasePrices: the last cost per base unit this supplier's demo receipts carried, centimes HT.
func (c *Client) LastPurchasePrices(ctx context.Context, supplierID string) (map[string]int64, error) {
	c.mu.Lock()
	var recs []receipt
	err := readLines(c.ReceiptsFile(), &recs)
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range recs { // oldest first: the newest price wins
		if r.SupplierID != supplierID {
			continue
		}
		for _, l := range r.Lines {
			out[l.ProductID] = l.UnitCostHT * 1000 / factor(l.PackFactorMilli)
		}
	}
	return out, nil
}

// ---- writing ---------------------------------------------------------------------------------

type order struct {
	ID          string      `json:"id"`
	Number      string      `json:"number"`
	Date        string      `json:"date"`
	CreatedAt   string      `json:"created_at"`
	CustomerID  string      `json:"customer_id"`
	Customer    string      `json:"customer"`
	ExternalRef string      `json:"external_ref"`
	Notes       string      `json:"notes"`
	Lines       []orderLine `json:"lines"`
	TotalTTC    int64       `json:"total_ttc"`
}

type orderLine struct {
	ProductID   string `json:"product_id"`
	Name        string `json:"name"`
	QtyMilli    int64  `json:"qty_milli"`
	UnitPriceHT int64  `json:"unit_price_ht"`
	LineTTC     int64  `json:"line_ttc"`
	Notes       string `json:"notes"`
}

// CreateOrder appends the order to OrdersFile, priced line by line.
func (c *Client) CreateOrder(ctx context.Context, o adapter.Order) (adapter.OrderRef, error) {
	if o.CustomerID == "" || len(o.Lines) == 0 {
		return adapter.OrderRef{}, fmt.Errorf("an order needs a customer and a line")
	}
	now := time.Now()
	rec := order{Date: now.Format("2006-01-02"), CreatedAt: store.Now(), CustomerID: o.CustomerID,
		Customer: catalog.CustomerName(c.DB.DB, o.CustomerID), ExternalRef: o.ExternalRef, Notes: o.Notes}
	for _, l := range o.Lines {
		q, err := c.Price(ctx, o.CustomerID, l.ProductID, l.QtyMilli)
		if err != nil {
			return adapter.OrderRef{}, err
		}
		var name string
		c.DB.QueryRowContext(ctx, `SELECT name FROM products WHERE id = ?`, l.ProductID).Scan(&name)
		rec.Lines = append(rec.Lines, orderLine{ProductID: l.ProductID, Name: name, QtyMilli: l.QtyMilli,
			UnitPriceHT: q.UnitPriceHT, LineTTC: q.LineTTC, Notes: l.Notes})
		rec.TotalTTC += q.LineTTC
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var all []order
	if err := readLines(c.OrdersFile(), &all); err != nil {
		return adapter.OrderRef{}, err
	}
	rec.ID = fmt.Sprint(len(all) + 1)
	rec.Number = fmt.Sprintf("DEMO-%04d", len(all)+1)
	if err := appendLine(c.OrdersFile(), rec); err != nil {
		return adapter.OrderRef{}, err
	}
	return adapter.OrderRef{ID: rec.ID, Number: rec.Number, TotalTTC: rec.TotalTTC}, nil
}

type receipt struct {
	ID          string                `json:"id"`
	Number      string                `json:"number"`
	CreatedAt   string                `json:"created_at"`
	SupplierID  string                `json:"supplier_id"`
	Supplier    string                `json:"supplier"`
	Date        string                `json:"date"`
	SupplierRef string                `json:"supplier_ref"`
	Notes       string                `json:"notes"`
	Posted      bool                  `json:"posted"`
	Lines       []adapter.ReceiptLine `json:"lines"`
	TotalTTC    int64                 `json:"total_ttc"`
}

// CreateReceipt appends the approved invoice to ReceiptsFile; its quantities count in Stock.
func (c *Client) CreateReceipt(ctx context.Context, r adapter.Receipt) (adapter.OrderRef, error) {
	if r.SupplierID == "" || len(r.Lines) == 0 {
		return adapter.OrderRef{}, fmt.Errorf("a receipt needs a supplier and a line")
	}
	rec := receipt{CreatedAt: store.Now(), SupplierID: r.SupplierID, Supplier: catalog.SupplierName(c.DB.DB, r.SupplierID),
		Date: r.Date, SupplierRef: r.SupplierRef, Notes: r.Notes, Posted: r.Post, Lines: r.Lines}
	for _, l := range r.Lines {
		rec.TotalTTC += lineTTC(l.UnitCostHT, l.QtyMilli, l.VatRate)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var all []receipt
	if err := readLines(c.ReceiptsFile(), &all); err != nil {
		return adapter.OrderRef{}, err
	}
	rec.ID = fmt.Sprint(len(all) + 1)
	rec.Number = fmt.Sprintf("DEMO-R-%04d", len(all)+1)
	if err := appendLine(c.ReceiptsFile(), rec); err != nil {
		return adapter.OrderRef{}, err
	}
	return adapter.OrderRef{ID: rec.ID, Number: rec.Number, TotalTTC: rec.TotalTTC}, nil
}

// factor is a receipt line's base units per line unit (milli), 1 when none is given.
func factor(f int64) int64 {
	if f <= 0 {
		return 1000
	}
	return f
}

// ---- files -----------------------------------------------------------------------------------

// readLines reads a JSON-lines file into out (a pointer to a slice); a missing file is empty.
func readLines[T any](path string, out *[]T) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16<<20)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var v T
		if err := json.Unmarshal(sc.Bytes(), &v); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		*out = append(*out, v)
	}
	return sc.Err()
}

func appendLine(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

var _ adapter.ERP = (*Client)(nil)
