package demo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/adaptertest"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// desk is a small desk database: three products (one sold by the carton), the demo shops, a supplier.
func desk(t *testing.T) (*Client, string) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "orderdesk.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := catalog.ReplaceProducts(db, []catalog.Product{
		{ID: "SM-1", Name: "MOLLE MOUTARDE 200G", Brand: "MOLLE"},
		{ID: "SM-2", Name: "AMOUR COUSCOUS", Brand: "AMOUR", VatRate: 900, Packs: []catalog.Pack{{Name: "carton", FactorMilli: 12000}}},
		{ID: "SM-3", Name: "BETTOUCHE YAOURT AROMATISE", Brand: "BETTOUCHE"},
	}); err != nil {
		t.Fatal(err)
	}
	if n, err := AddShops(db); err != nil || n != len(Shops) {
		t.Fatalf("AddShops: %d, %v", n, err)
	}
	if n, _ := AddShops(db); n != 0 {
		t.Fatalf("AddShops twice added %d shops", n)
	}
	catalog.ReplaceSuppliers(db, []catalog.Customer{{ID: "S-1", Name: "SARL GROSSISTE TEST"}})
	return New(db, dir), dir
}

func TestConformance(t *testing.T) {
	c, _ := desk(t)
	adaptertest.Run(t, c)
}

func TestOrdersFile(t *testing.T) {
	c, dir := desk(t)
	ctx := context.Background()
	q1, _ := c.Price(ctx, "demo-shop-1", "SM-2", 1000)
	q2, _ := c.Price(ctx, "demo-shop-2", "SM-2", 1000)
	if q1.UnitPriceHT <= 0 || q1 != q2 {
		t.Fatalf("the price must be stable and positive: %+v %+v", q1, q2)
	}
	if q, _ := c.Price(ctx, "", "SM-2", 2000); q.LineTTC != int64(float64(q1.UnitPriceHT)*2*1.09+0.5) {
		t.Errorf("2 units with 9 %% VAT: %+v (unit %d)", q, q1.UnitPriceHT)
	}
	if _, err := c.Price(ctx, "", "NOPE", 1000); err == nil {
		t.Error("an unknown product has no price")
	}

	r1, err := c.CreateOrder(ctx, adapter.Order{CustomerID: "demo-shop-1", ExternalRef: "orderdesk:d1",
		Lines: []adapter.OrderLine{{ProductID: "SM-1", QtyMilli: 3000, Notes: "« 3 moutarde »"}, {ProductID: "SM-2", QtyMilli: 24000}}})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := c.CreateOrder(ctx, adapter.Order{CustomerID: "demo-shop-2", Lines: []adapter.OrderLine{{ProductID: "SM-3", QtyMilli: 6000}}})
	if err != nil {
		t.Fatal(err)
	}
	if r1.Number != "DEMO-0001" || r2.Number != "DEMO-0002" || r1.TotalTTC <= 0 {
		t.Errorf("refs: %+v %+v", r1, r2)
	}
	b, err := os.ReadFile(filepath.Join(dir, "demo-orders.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"MOLLE MOUTARDE 200G"`) || !strings.Contains(lines[0], `"Supérette Nour (démo)"`) {
		t.Errorf("orders file:\n%s", b)
	}
	h, err := c.History(ctx, "demo-shop-1", 10)
	if err != nil || len(h) != 1 || h[0].Number != "DEMO-0001" || len(h[0].Lines) != 2 || h[0].Lines[1].QtyMilli != 24000 {
		t.Errorf("history of shop 1: %+v, %v", h, err)
	}

	// a new client on the same folder (a restart) keeps counting
	r3, err := New(c.DB, dir).CreateOrder(ctx, adapter.Order{CustomerID: "demo-shop-1", Lines: []adapter.OrderLine{{ProductID: "SM-1", QtyMilli: 1000}}})
	if err != nil || r3.Number != "DEMO-0003" {
		t.Errorf("after a restart: %+v, %v", r3, err)
	}
	if h, _ := c.History(ctx, "demo-shop-1", 1); len(h) != 1 || h[0].Number != "DEMO-0003" {
		t.Errorf("newest first: %+v", h)
	}
	if d, err := c.NextDelivery(ctx, "demo-shop-1"); err != nil || d.Day == "" {
		t.Errorf("next delivery: %+v, %v", d, err)
	}
}
