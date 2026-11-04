package engineer

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta/metatest"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

func truthDesk(t *testing.T, prods []catalog.Product, lines [][3]string) (*store.DB, *catalog.Catalog) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "e.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := catalog.ReplaceProducts(db, prods); err != nil {
		t.Fatal(err)
	}
	catalog.ReplaceCustomers(db, []catalog.Customer{{ID: "C1", Name: "Superette Amine"}})
	for i, l := range lines { // said, product, kind
		db.Exec(`INSERT OR REPLACE INTO ground_truth (line_id, said, norm, unit, customer_id, supplier_id, kind, product_id, level, origin, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`, "L"+string(rune('a'+i)), l[0], catalog.Normalize(l[0]), "", "C1", "", l[2], l[1], "person", "test",
			"2026-11-01 10:00:"+string(rune('0'+i/10))+string(rune('0'+i%10)))
	}
	db.SetSetting("engineer_fallback_model", "") // Muse Spark only
	return db, catalog.New(db)
}

// The gate's promise, checked on whatever the model wrote: every rule that went live or to shadow books no
// known line wrong, and the breaker broke none of them.
func checkAdmitted(t *testing.T, db *store.DB, cat *catalog.Catalog) (admitted int) {
	t.Helper()
	list, _ := rules.List(db, false)
	truth := GroundTruth(db)
	for _, r := range list {
		var b Bench
		json.Unmarshal(r.Bench, &b)
		t.Logf("%s %-8s %s (%s)", r.ID, r.Status, r.Sentence, r.Reason)
		if r.Status != "shadow" && r.Status != "live" {
			continue
		}
		admitted++
		if again := RunBench(cat, rules.NewSet(nil), r, truth); again.Wrong != 0 || b.Wrong != 0 {
			t.Errorf("%s was admitted but books %d known lines wrong", r.ID, again.Wrong)
		}
		for _, a := range b.Attacks {
			if a.Broken {
				t.Errorf("%s was admitted although the breaker broke it with %q", r.ID, a.Line)
			}
		}
	}
	return admitted
}

// A recorded rule-engineer session (Muse Spark with its tools, then the breaker on each rule that passed the
// gate) on lines a supermarket clerk confirmed: abbreviations no catalogue name carries.
func TestRecordedEngineerSession(t *testing.T) {
	db, cat := truthDesk(t, []catalog.Product{
		{ID: "S2", Name: "HAMOUD BOUALEM SELECTO 2L", Brand: "HAMOUD BOUALEM"},
		{ID: "S1", Name: "HAMOUD BOUALEM SELECTO 1L", Brand: "HAMOUD BOUALEM"},
		{ID: "H2", Name: "HAMOUD BOUALEM CHARBAT 2L", Brand: "HAMOUD BOUALEM"},
		{ID: "I15", Name: "IFRI EAU MINERALE 1.5L", Brand: "IFRI"},
		{ID: "I05", Name: "IFRI EAU MINERALE 0.5L", Brand: "IFRI"},
		{ID: "IG1", Name: "IFRI GAZEIFIEE 1L", Brand: "IFRI"},
	}, [][3]string{
		{"slkt 2l", "S2", "order"}, {"slkt 1l", "S1", "order"}, {"hb slkt 2l", "S2", "invoice"}, {"slkt 2 l", "S2", "order"},
		{"hb charbat 2l", "H2", "invoice"}, {"ifri gaz 1l", "IG1", "order"}, {"ifri gazeuse 1l", "IG1", "invoice"},
		{"ifri 1.5l", "I15", "order"}, {"ifri 0.5l", "I05", "order"},
	})
	mc, tape := metatest.Client(t, db, "engineer-session")
	e := &Engineer{DB: db, Cat: cat, Meta: mc}
	rep := e.Run(context.Background(), "test")
	t.Logf("residue %d, accepted %v, rejected %v, $%.4f: %s", rep.Residue, rep.Accepted, rep.Rejected, rep.Cost, rep.Summary)
	if rep.Error != "" {
		t.Fatal(rep.Error)
	}
	n := checkAdmitted(t, db, cat)
	if !metatest.Recording() {
		if want := 1; n != want {
			t.Errorf("%d rules admitted, the recording admits %d", n, want)
		}
		if u := tape.Unused(); u != 0 {
			t.Errorf("%d recorded calls were never made", u)
		}
	}
	if rep.Cost <= 0 {
		t.Error("no cost metered")
	}
	for _, id := range rep.Accepted { // the breaker attacked every rule the gate let through
		r, _ := rules.Get(db, id)
		var b Bench
		json.Unmarshal(r.Bench, &b)
		if len(b.Attacks) == 0 {
			t.Errorf("%s went to shadow without the breaker's attacks", id)
		}
		t.Logf("breaker vs %s: %d look-alike lines, e.g. %+v", id, len(b.Attacks), b.Attacks[:min(2, len(b.Attacks))])
	}
}

// A hand-written session: the model submits a rule that would book "dove gel" as a shampoo. The gate refuses
// it, nothing goes live, and the model's summary ends the session.
func TestHandwrittenEngineerBadRule(t *testing.T) {
	db, cat := truthDesk(t, []catalog.Product{
		{ID: "P2", Name: "Dove Color Care Shampooing 250 ml", Brand: "Dove"},
		{ID: "P3", Name: "Dove Gel douche 500 ml", Brand: "Dove"},
		{ID: "P1", Name: "Head & Shoulders Supreme Shampooing 400 ml", Brand: "Head & Shoulders"},
	}, [][3]string{{"hns 400", "P1", "order"}, {"hns shampooing", "P1", "order"}, {"dove gel", "P3", "order"}, {"dove shampoing", "P2", "order"}})
	mc, tape := metatest.Client(t, db, "handwritten-engineer-bad-rule")
	rep := (&Engineer{DB: db, Cat: cat, Meta: mc}).Run(context.Background(), "test")
	if len(rep.Accepted) != 0 || len(rep.Rejected) != 1 {
		t.Fatalf("accepted %v rejected %v, want the bad rule rejected", rep.Accepted, rep.Rejected)
	}
	if checkAdmitted(t, db, cat) != 0 {
		t.Fatal("a rule was admitted")
	}
	if len(tape.Requests) != 2 {
		t.Fatalf("%d model turns, want 2 (the submission, then the summary)", len(tape.Requests))
	}
}
