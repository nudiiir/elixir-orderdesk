package resolve_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta/metatest"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// Products from the benchmark's supermarket catalogue (bench/catalogs/supermarket.csv).
func pickDesk(t *testing.T, tape string) (*resolve.Resolver, *metatest.Tape) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "r.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	catalog.ReplaceProducts(db, []catalog.Product{
		{ID: "M140", Name: "MOLLE MOUTARDE 140G", Brand: "MOLLE"},
		{ID: "M200", Name: "MOLLE MOUTARDE 200G", Brand: "MOLLE"},
		{ID: "M370", Name: "MOLLE MOUTARDE 370G", Brand: "MOLLE"},
		{ID: "H760", Name: "AVRIL HRISSA 760G", Brand: "AVRIL"},
		{ID: "H135", Name: "AVRIL HRISSA 135G", Brand: "AVRIL"},
		{ID: "AC1", Name: "ARWA SODA COLA 1L", Brand: "ARWA"},
		{ID: "AP1", Name: "ARWA SODA PECHE 1L", Brand: "ARWA"},
	})
	mc, tp := metatest.Client(t, db, tape)
	return &resolve.Resolver{Cat: catalog.New(db), Live: rules.NewSet(nil), Meta: mc, Threshold: 0.9}, tp
}

// Lines the code cannot settle alone ("harissa" is not a catalogue word, "peach" is English) go to Muse
// Spark, which may only choose among the shortlist the code built. Recorded answers: harissa 760 g and the
// peach soda are booked; harissa with no size (the catalogue sells 760 g and 135 g) is asked, never guessed.
func TestRecordedSparkPick(t *testing.T) {
	rv, tape := pickDesk(t, "spark-pick")
	ms := []resolve.Mention{
		{Said: "harissa avril 760g", Qty: 3},
		{Said: "arwa soda peach 1l", Qty: 2, Unit: "carton"},
		{Said: "avril harissa", Qty: 6},
	}
	res := rv.Resolve(context.Background(), ms, resolve.Context{}, "msg-1")
	right := map[int]string{0: "H760", 1: "AP1"}
	for i, r := range res {
		t.Logf("%q → %s %s (%s, %.2f) options %v", ms[i].Said, r.Status, r.ProductID, r.Level, r.Confidence, r.Options)
		if r.Status == "ok" && r.ProductID != right[i] {
			t.Errorf("%q booked to %s", ms[i].Said, r.ProductID)
		}
		if r.Status == "ok" && r.Confidence < 0.9 {
			t.Errorf("%q booked under the 0.9 bar (%.2f)", ms[i].Said, r.Confidence)
		}
	}
	if !metatest.Recording() {
		if res[0].ProductID != "H760" || res[1].ProductID != "AP1" || res[2].Status == "ok" {
			t.Errorf("the recorded answers book harissa 760 g and the peach soda, and ask about harissa with no size")
		}
		if n := len(tape.Requests); n != 1 {
			t.Errorf("%d Spark calls, want one call for the whole message", n)
		}
	}
}

// Hand-written bad answers: an id that is not on the shortlist, an answer that is not JSON, and a pick
// under the confidence bar. None may book a line; each line goes to a person.
func TestHandwrittenBadPicks(t *testing.T) {
	for _, tape := range []string{"handwritten-pick-outside-shortlist", "handwritten-pick-not-json", "handwritten-pick-unsure"} {
		t.Run(tape, func(t *testing.T) {
			rv, tp := pickDesk(t, tape)
			res := rv.Resolve(context.Background(), []resolve.Mention{{Said: "avril harissa", Qty: 1}}, resolve.Context{}, "msg-1")
			if r := res[0]; r.Status == "ok" {
				t.Fatalf("a bad answer booked the line: %+v", r)
			}
			if len(tp.Requests) != 1 {
				t.Fatalf("the line never reached Muse Spark (%d calls)", len(tp.Requests))
			}
		})
	}
}
