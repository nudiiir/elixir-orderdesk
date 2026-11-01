package catalog

import (
	"path/filepath"
	"reflect"
	"strings"
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

func TestNumbersAreNotCodes(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ReplaceProducts(db, []Product{{ID: "a", Name: "IFRI EAU 1.5L", Brand: "IFRI"}, {ID: "b", Name: "SELECTO 2L", Brand: "HAMOUD"}})
	c := New(db)
	for _, line := range []string{"ifri 1500", "selecto 2027", "rouiba 1000"} {
		if got := c.ForeignCodes(Normalize(line)); len(got) != 0 {
			t.Errorf("%q: a bare number is not an article code, got %v", line, got)
		}
	}
	if got := c.ForeignCodes(Normalize("ref ab1234 selecto")); len(got) != 1 {
		t.Errorf("a word mixing letters and digits is a code, got %v", got)
	}
}

func TestPackIsNotThousands(t *testing.T) {
	for in, want := range map[string]string{"EAU PACK 6 500ML": "500ml", "LAIT 1 800 G": "1800g", "CANETTE 24 330ML": "330ml"} {
		if got := NormalizeName(in); !strings.Contains(got, want) {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}
