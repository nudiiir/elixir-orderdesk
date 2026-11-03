package resolve

import (
	"path/filepath"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// A brand the catalogue knows is never read as an abbreviation of a shorter catalogue word: "geltim" is
// GELTIM, not "gel"; "solian" is SOLIAN, not "sol". Abbreviations still work for words no product has.
func TestKnownWordNamesItself(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	catalog.ReplaceProducts(db, []catalog.Product{
		{ID: "geltim", Name: "GELTIM 1MG LP GEL OPHT"},
		{ID: "zydex", Name: "ZYDEX GEL OPHT 1% T/5G"},
		{ID: "solian", Name: "SOLIAN 200MG CP SEC B/30"},
		{ID: "nasabec", Name: "NASABEC 100MCG SOL NAS FL 200 DOSES"},
		{ID: "biscuit", Name: "KOOL BISCUIT FOURRE CHOCO"},
	})
	cat := catalog.New(db)
	set := rules.NewSet(nil)
	for said, never := range map[string]string{"geltim 1 gel ophtalmique": "zydex", "SOLIAN 200 SEC": "nasabec"} {
		if r := Deterministic(cat, set, Mention{Said: said}, Context{}); r.Status == "ok" && r.ProductID == never {
			t.Errorf("%q booked to %s", said, never)
		}
	}
	if r := Deterministic(cat, set, Mention{Said: "kool bisc fourre choco"}, Context{}); r.Status != "ok" || r.ProductID != "biscuit" {
		t.Errorf("an abbreviation of a word no product has must still match: %+v", r.ProductID)
	}
}
