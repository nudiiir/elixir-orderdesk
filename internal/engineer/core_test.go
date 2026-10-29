package engineer

import (
	"path/filepath"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// A small cosmetics catalogue; no AI is called anywhere in these tests.
func fixture(t *testing.T) (*store.DB, *catalog.Catalog) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	prods := []catalog.Product{
		{ID: "P1", Name: "Head & Shoulders Supreme Shampooing 400 ml", Brand: "Head & Shoulders", Packs: []catalog.Pack{{Name: "Carton", FactorMilli: 12000}}},
		{ID: "P2", Name: "Dove Color Care Shampooing 250 ml", Brand: "Dove", Packs: []catalog.Pack{{Name: "Carton", FactorMilli: 24000}}},
		{ID: "P3", Name: "Dove Gel douche 500 ml", Brand: "Dove"},
		{ID: "P4", Name: "Nivea Soft Crème hydratante 200 ml", Brand: "Nivea", Packs: []catalog.Pack{{Name: "Fardeau", FactorMilli: 6000}, {Name: "Carton", FactorMilli: 24000}}},
		{ID: "P5", Name: "Nivea Soft Crème hydratante 100 ml", Brand: "Nivea"},
	}
	if err := catalog.ReplaceProducts(db, prods); err != nil {
		t.Fatal(err)
	}
	catalog.ReplaceCustomers(db, []catalog.Customer{{ID: "C1", Name: "Épicerie Nour"}})
	return db, catalog.New(db)
}

func TestDeterministicMatchAndRules(t *testing.T) {
	_, cat := fixture(t)
	none := rules.NewSet(nil)
	cx := resolve.Context{CustomerID: "C1"}

	if r := resolve.Deterministic(cat, none, resolve.Mention{Said: "dove shampooing 250ml"}, cx); r.ProductID != "P2" || r.Level != "rules" {
		t.Fatalf("exact match: got %+v", r)
	}
	if r := resolve.Deterministic(cat, none, resolve.Mention{Said: "hns 400"}, cx); r.Status == "ok" {
		t.Fatalf("hns should not resolve without a rule: %+v", r)
	}
	syn := &rules.Rule{ID: "R-0001", Type: rules.Synonym, Scope: rules.Scope{Kind: "global"}, Pattern: "hns|h&s", Action: rules.Action{Replace: "head shoulders"}}
	set := rules.NewSet([]*rules.Rule{syn})
	if r := resolve.Deterministic(cat, set, resolve.Mention{Said: "HNS shampooing"}, cx); r.ProductID != "P1" {
		t.Fatalf("synonym: got %+v", r)
	}
	// "nivea creme" fits two sizes: a customer default picks one.
	if r := resolve.Deterministic(cat, none, resolve.Mention{Said: "nivea creme"}, cx); r.Status == "ok" {
		t.Fatalf("ambiguous size must not resolve: %+v", r)
	}
	def := &rules.Rule{ID: "R-0002", Type: rules.Default, Scope: rules.Scope{Kind: "customer", ID: "C1"}, Pattern: "nivea creme", Action: rules.Action{Product: "P4"}}
	if r := resolve.Deterministic(cat, rules.NewSet([]*rules.Rule{def}), resolve.Mention{Said: "Nivea crème"}, cx); r.ProductID != "P4" {
		t.Fatalf("default: got %+v", r)
	}
	if r := resolve.Deterministic(cat, rules.NewSet([]*rules.Rule{def}), resolve.Mention{Said: "Nivea crème"}, resolve.Context{CustomerID: "C9"}); r.Status == "ok" {
		t.Fatalf("a customer default must not apply to another customer")
	}
	// pack rule
	pk := &rules.Rule{ID: "R-0003", Type: rules.PackT, Scope: rules.Scope{Kind: "global"}, Pattern: "x{n}$", Action: rules.Action{PackUnit: "carton", PackQty: "{n}"}}
	r := resolve.Deterministic(cat, rules.NewSet([]*rules.Rule{pk}), resolve.Mention{Said: "dove shampooing 250 ml x24"}, cx)
	if r.ProductID != "P2" || r.Pack == nil || r.Pack.Qty != 24 {
		t.Fatalf("pack rule: got %+v pack %+v", r, r.Pack)
	}
	if name, f := resolve.ChoosePack(cat.Get("P4"), "pack", nil); name != "Fardeau" || f != 6000 {
		t.Fatalf("pack choice: %s %d", name, f)
	}
}

func TestBenchmarkGate(t *testing.T) {
	db, cat := fixture(t)
	truth := []Truth{
		{LineID: "a", Said: "hns 400", CustomerID: "C1", ProductID: "P1"},
		{LineID: "b", Said: "hns shampooing", CustomerID: "C1", ProductID: "P1"},
		{LineID: "c", Said: "dove gel", CustomerID: "C1", ProductID: "P3"},
	}
	_ = db
	live := rules.NewSet(nil)
	good := &rules.Rule{ID: "X", Type: rules.Synonym, Scope: rules.Scope{Kind: "global"}, Pattern: "hns", Action: rules.Action{Replace: "head shoulders"}}
	b := RunBench(cat, live, good, truth)
	if b.NewlyResolved != 2 || b.Wrong != 0 {
		t.Fatalf("good rule bench: %+v", b)
	}
	if ok, _ := Gate(b, 2); !ok {
		t.Fatalf("good rule should pass")
	}
	bad := &rules.Rule{ID: "Y", Type: rules.Synonym, Scope: rules.Scope{Kind: "global"}, Pattern: "gel", Action: rules.Action{Replace: "shampooing 250ml"}}
	b = RunBench(cat, live, bad, truth)
	if ok, why := Gate(b, 1); ok {
		t.Fatalf("bad rule must fail: %+v %s", b, why)
	}
	if res := Residue(cat, live, truth); len(res) != 3-0 && len(res) != 2 {
		t.Fatalf("residue: %d", len(res))
	}
}

func TestSentences(t *testing.T) {
	r := rules.Rule{Type: rules.PackT, Scope: rules.Scope{Kind: "supplier", ID: "S1", Name: "Sahel Distribution"}, Pattern: "x{n}$",
		Action: rules.Action{PackUnit: "carton", PackQty: "{n}"}}
	if s := r.MakeSentence(func(string) string { return "" }); s != `On invoices from Sahel Distribution, a trailing "x{n}" means a carton of {n}.` {
		t.Fatalf("sentence: %s", s)
	}
	d := rules.Rule{Type: rules.Default, Scope: rules.Scope{Kind: "customer", ID: "C1", Name: "Épicerie Nour"}, Pattern: "ifri", Action: rules.Action{Product: "P"}}
	if s := d.MakeSentence(func(string) string { return "Ifri 1.5 L" }); s != `For Épicerie Nour, "ifri" with no size means Ifri 1.5 L.` {
		t.Fatalf("sentence: %s", s)
	}
}

func TestAliasLifecycle(t *testing.T) {
	_, cat := fixture(t)
	a, _ := cat.Witness("shampoing hs", "P1", "customer", "C1", "text", 2)
	if a.State != "candidate" {
		t.Fatalf("first witness must be a candidate: %+v", a)
	}
	if _, ok := cat.Trusted("shampoing hs", "C1", ""); ok {
		t.Fatal("candidate must not resolve")
	}
	a, _ = cat.Witness("shampoing hs", "P1", "customer", "C1", "text", 2)
	if a.State != "trusted" {
		t.Fatalf("second witness must trust: %+v", a)
	}
	r := resolve.Deterministic(cat, rules.NewSet(nil), resolve.Mention{Said: "Shampoing HS"}, resolve.Context{CustomerID: "C1"})
	if r.ProductID != "P1" || r.Level != "alias" {
		t.Fatalf("alias resolve: %+v", r)
	}
	cat.Witness("shampoing hs", "P2", "customer", "C1", "text", 2) // a confirmation for another product blocks it
	if _, ok := cat.Trusted("shampoing hs", "C1", ""); ok {
		t.Fatal("contradicted alias must be blocked")
	}
}
