// Package adaptertest is the conformance test every ERP adapter passes against a live ERP: the
// catalogue reads the way the desk needs it, and an order and a goods receipt really land in the ERP.
package adaptertest

import (
	"context"
	"testing"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
)

func Run(t *testing.T, e adapter.ERP) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := e.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	prods, err := e.Products(ctx)
	if err != nil || len(prods) == 0 {
		t.Fatalf("products: %d, %v", len(prods), err)
	}
	var withPack, withBrand, withVat int
	var pick *catalog.Product
	for i := range prods {
		p := &prods[i]
		if len(p.Packs) > 0 {
			withPack++
			if pick == nil {
				pick = p
			}
		}
		if p.Brand != "" {
			withBrand++
		}
		if p.VatRate > 0 {
			withVat++
		}
	}
	t.Logf("%s: %d products, %d with packs, %d with a brand, %d with VAT", e.Name(), len(prods), withPack, withBrand, withVat)
	if pick == nil {
		pick = &prods[0]
	}
	custs, err := e.Customers(ctx)
	if err != nil || len(custs) == 0 {
		t.Fatalf("customers: %d, %v", len(custs), err)
	}
	sups, err := e.Suppliers(ctx)
	if err != nil || len(sups) == 0 {
		t.Fatalf("suppliers: %d, %v", len(sups), err)
	}
	t.Logf("%d customers, %d suppliers; product %q (%s)", len(custs), len(sups), pick.Name, pick.ID)

	q, err := e.Price(ctx, custs[0].ID, pick.ID, 6000)
	if err != nil {
		t.Fatalf("price: %v", err)
	}
	t.Logf("price of 6: %+v", q)
	stock, err := e.Stock(ctx)
	if err != nil {
		t.Fatalf("stock: %v", err)
	}
	before := stock[pick.ID]
	t.Logf("stock of the product: %d (milli), %d products with stock", before, len(stock))

	ref, err := e.CreateOrder(ctx, adapter.Order{CustomerID: custs[0].ID, ExternalRef: "orderdesk:conformance-" + time.Now().Format("150405.000"),
		Notes: "Commande reçue par Order Desk (test)", Lines: []adapter.OrderLine{{ProductID: pick.ID, QtyMilli: 6000, Notes: "« test ligne »"}}})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if ref.Number == "" || ref.TotalTTC <= 0 {
		t.Errorf("order ref: %+v", ref)
	}
	t.Logf("order %s, total %d, %s", ref.Number, ref.TotalTTC, e.OrderURL(ref))
	hist, err := e.History(ctx, custs[0].ID, 5)
	if err != nil || len(hist) == 0 {
		t.Fatalf("history: %d, %v", len(hist), err)
	}
	found := false
	for _, l := range hist[0].Lines {
		found = found || l.ProductID == pick.ID
	}
	if !found {
		t.Errorf("the newest order in the history does not hold the product: %+v", hist[0])
	}
	if _, err := e.NextDelivery(ctx, custs[0].ID); err != nil && err != adapter.ErrUnsupported {
		t.Errorf("next delivery: %v", err)
	}

	// a supplier invoice: 2 packs of the product (or 12 units), approved and posted
	factor := int64(1000)
	if len(pick.Packs) > 0 {
		factor = pick.Packs[0].FactorMilli
	}
	qty := int64(2000)
	unitCost := int64(12345) // per pack
	rr, err := e.CreateReceipt(ctx, adapter.Receipt{SupplierID: sups[0].ID, Date: time.Now().Format("2006-01-02"), SupplierRef: "FAC-TEST-001",
		Notes: "Facture lue par Order Desk (test)", Post: true,
		Lines: []adapter.ReceiptLine{{ProductID: pick.ID, Description: "PRODUIT TEST X6", QtyMilli: qty, UnitCostHT: unitCost, VatRate: pick.VatRate, PackFactorMilli: factor}}})
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	t.Logf("receipt %s, total %d", rr.Number, rr.TotalTTC)
	stock2, err := e.Stock(ctx)
	if err != nil {
		t.Fatalf("stock after: %v", err)
	}
	want := before + qty*factor/1000
	if got := stock2[pick.ID]; got != want {
		t.Errorf("stock after the receipt: %d, want %d (before %d + %d units)", got, want, before, qty*factor/1000000)
	}
	last, err := e.LastPurchasePrices(ctx, sups[0].ID)
	if err != nil {
		t.Fatalf("last purchase prices: %v", err)
	}
	perUnit := unitCost * 1000 / factor
	if got := last[pick.ID]; got < perUnit-1 || got > perUnit+1 {
		t.Errorf("last purchase price per base unit: %d, want %d", got, perUnit)
	}
	t.Logf("last purchase price per unit: %d (%d prices known for this supplier)", last[pick.ID], len(last))
}
