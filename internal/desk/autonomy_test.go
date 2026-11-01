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
		a.Record([]Line{right}, "S-A")
	}
	if a.Unlocked() {
		t.Fatalf("260 answers are not enough for 99 %% (bound %.4f)", a.Bound())
	}
	for i := 0; i < 15; i++ {
		a.Record([]Line{right}, "S-A")
	}
	if !a.Unlocked() || a.Supervised(cx) != "" {
		t.Fatalf("275 clean answers unlock (bound %.4f)", a.Bound())
	}
	wrong := Line{ProductID: "q", Level: resolve.LPerson, Checks: map[string]string{"det_product": "p"}}
	a.Record([]Line{wrong}, "S-A")
	if a.Supervised(cx) == "" {
		t.Fatal("a correction puts that supplier back under supervision")
	}
	if a.Supervised(resolve.Context{SupplierID: "S-B"}) != "" {
		t.Fatal("other suppliers stay autonomous")
	}
}
