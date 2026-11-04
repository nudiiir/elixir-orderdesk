package catalog

import (
	"path/filepath"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

func TestMatchNeedsTheWholeWord(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ReplaceProducts(db, []Product{
		{ID: "sel", Name: "NASSAH ESPRIT DE SEL", Brand: "NASSAH"},
		{ID: "choc", Name: "TABLETTE CHOCOLAT NOIR", Brand: "AMOR"},
		{ID: "bisc", Name: "BISCUIT SABLE", Brand: "BIMO"},
	})
	c := New(db)
	words := func(s string) []string { w, _ := Tokens(Normalize(s)); return w }
	if got := c.Match(words("selecto"), "", nil); len(got) != 0 {
		t.Errorf("a product the catalogue does not carry must not match a short catalogue word: %v", got[0].Name)
	}
	if got := c.Match(words("choco noir"), "", nil); len(got) != 1 || got[0].ID != "choc" {
		t.Errorf("a shortened catalogue word still matches: %v", got)
	}
	if got := c.Match(words("biscuits sable"), "", nil); len(got) != 1 || got[0].ID != "bisc" {
		t.Errorf("a plural still matches: %v", got)
	}
}
