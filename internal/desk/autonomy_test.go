package desk

import (
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
)

func TestAutonomy(t *testing.T) {
	a := NewAutonomy(0.99)
	cx := resolve.Context{SupplierID: "S-A"}
	if a.Supervised(cx) == "" {
		t.Fatal("a new desk is supervised")
	}
	right := Line{ProductID: "p", Level: resolve.LRules, Checks: map[string]string{"det_product": "p"}}
	for i := 0; i < 260; i++ {
		a.Record([]Line{right}, "S-A", false)
	}
	if a.Unlocked() {
		t.Fatalf("260 answers are not enough for 99 %% (bound %.4f)", a.Bound())
	}
	for i := 0; i < 15; i++ {
		a.Record([]Line{right}, "S-A", false)
	}
	if !a.Unlocked() || a.Supervised(cx) != "" {
		t.Fatalf("275 clean answers unlock (bound %.4f)", a.Bound())
	}
	wrong := Line{ProductID: "q", Level: resolve.LPerson, Checks: map[string]string{"det_product": "p"}}
	a.Record([]Line{wrong}, "S-A", false)
	if a.Supervised(cx) == "" {
		t.Fatal("a correction puts that supplier back under supervision")
	}
	if a.Supervised(resolve.Context{SupplierID: "S-B"}) != "" {
		t.Fatal("other suppliers stay autonomous")
	}
}

// An alias or rule answer nobody checked proves nothing on a live order; a replay's clerk checks every line.
func TestAutonomyCountsCheckedOnly(t *testing.T) {
	a := NewAutonomy(0.99)
	alone := Line{ProductID: "p", Level: resolve.LAlias, Checks: map[string]string{}}
	a.Record([]Line{alone, alone}, "C-1", false)
	if a.Right+a.Wrong != 0 {
		t.Fatalf("unchecked answers counted: right %d, wrong %d", a.Right, a.Wrong)
	}
	a.Record([]Line{alone}, "C-1", true)
	checkedLine := Line{ProductID: "p", Level: resolve.LRules, Checks: map[string]string{"det_product": "p", "spark": "agree"}}
	a.Record([]Line{checkedLine}, "C-1", false)
	if a.Right != 2 || a.Wrong != 0 {
		t.Fatalf("a reviewed answer and a Spark-checked one count: right %d, wrong %d", a.Right, a.Wrong)
	}
}
