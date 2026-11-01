package catalog

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

func TestCodes(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ReplaceProducts(db, []Product{
		{ID: "a", Name: "FORET BETON 8MM", Brand: "BOSCH", Barcode: "2608597145"},
		{ID: "b", Name: "PRISE 2P+T MOSAIC BLANC", Brand: "LEGRAND", Barcode: "077011"},
		{ID: "c", Name: "MEULEUSE 79505 125MM", Brand: "TOLSEN"},
	})
	c := New(db)
	for _, tc := range []struct{ line, want string }{
		{"REF 2608597145 FORET BETON 8MM", "a"},
		{"2 608 597 145 foret", "a"},
		{"REF 77011 PRISE 2P+T", "b"},
		{"PRISE MOSAIC BLANC", ""},
	} {
		p, _, ok := c.ByCode(tc.line)
		got := ""
		if ok {
			got = p.ID
		}
		if got != tc.want {
			t.Errorf("ByCode(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
	if got := c.ForeignCodes(Normalize("ART4881 COLLE PVC 500ML")); !reflect.DeepEqual(got, []string{"art4881"}) {
		t.Errorf("article code: %v", got)
	}
	if got := c.ForeignCodes(Normalize("MEUL 79505 125")); len(got) != 0 {
		t.Errorf("a number the catalogue names carry is not a foreign code: %v", got)
	}
	if got := c.ForeignCodes(Normalize("VIS TH M12X40 ZN BT100 R100 CRT12")); len(got) != 0 {
		t.Errorf("pack and coil counts are not codes: %v", got)
	}
	if got := c.StripCodes(Normalize("ART4881 COLLE PVC 500ML")); got != "colle pvc 500ml" {
		t.Errorf("strip: %q", got)
	}
}
