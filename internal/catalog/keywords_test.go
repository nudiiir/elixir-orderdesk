package catalog

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

func TestKeywords(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ReplaceProducts(db, []Product{
		{ID: "a", Name: "CHOUFAN CITRON 150G", Brand: "CHOUFAN"},
		{ID: "b", Name: "CHOUFAN FRAISE 150G", Brand: "CHOUFAN"},
		{ID: "c", Name: "KOOL SACHET", Brand: "KOOL"},
		{ID: "d", Name: "ARWA GRENADINE 24CL", Brand: "ARWA"},
		{ID: "e", Name: "A VERY LONG BRAND NAME", Brand: "A BRAND NAME FAR TOO LONG FOR VOICE"},
		{ID: "f", Name: "NO BRAND"},
	})
	c := New(db)
	// most products first, then alphabetical; too long and empty brands are left out
	if got := c.Keywords(10, 30); !reflect.DeepEqual(got, []string{"CHOUFAN", "ARWA", "KOOL"}) {
		t.Errorf("Keywords = %v", got)
	}
	if got := c.Keywords(2, 30); len(got) != 2 {
		t.Errorf("Keywords(2) = %v", got)
	}
}
