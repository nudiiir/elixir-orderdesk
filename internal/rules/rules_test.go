package rules

import (
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
)

func TestPatternSyntax(t *testing.T) {
	cases := []struct {
		pattern, text string
		match         bool
	}{
		{"zero|sans sucre", "coca sans sucre 1000ml", true},
		{"zero|sans sucre", "coca sucre", false},
		{"demi? ecreme", "lait ecreme", true},
		{"demi? ecreme", "lait demi ecreme", true},
		{"c{n}", "sel c24", true},
		{"c{n}", "sel c 24", true},
		{"c{n}", "secret", false},
		{"parfum {word}", "gel parfum vanille", true},
		{"x{n}$", "ifri x6", true},
		{"x{n}$", "x6 ifri", false},
	}
	for _, c := range cases {
		re, err := compile(c.pattern)
		if err != nil {
			t.Fatalf("%s: %v", c.pattern, err)
		}
		if got := re.MatchString(c.text); got != c.match {
			t.Errorf("%q on %q: got %v", c.pattern, c.text, got)
		}
	}
	if _, err := compile("x{n}|pack"); err == nil {
		t.Error("alternatives with different placeholders must be refused")
	}
}

func TestProfileReadsBothSides(t *testing.T) {
	set := NewSet([]*Rule{
		{ID: "A", Type: Attr, Scope: Scope{Kind: "global"}, Pattern: "parfum {word}", Action: Action{Attr: "scent", Value: "{word}"}},
		{ID: "B", Type: Attr, Scope: Scope{Kind: "supplier", ID: "S1"}, Pattern: "c{n}", Action: Action{Attr: "count", Value: "{n}"}},
		{ID: "C", Type: Word, Scope: Scope{Kind: "global"}, Pattern: "pet", Action: Action{Role: RoleDescriptor}},
	})
	p := set.Profile(catalog.Normalize("Gel douche parfum Vanille 6x250ml"), "", "", false)
	if v := p.Attrs["scent"]; len(v) != 1 || v[0].V != "vanille" {
		t.Fatalf("scent: %+v", p)
	}
	if v := p.Attrs["count"]; len(v) != 1 || v[0].V != "6" {
		t.Fatalf("count: %+v", p)
	}
	if v := p.Attrs["size"]; len(v) != 1 || v[0].V != "250ml" {
		t.Fatalf("size: %+v", p)
	}
	inv := set.Profile(catalog.Normalize("SEL 2L PET C24"), "", "S1", false)
	if v := inv.Attrs["count"]; len(v) != 1 || v[0].V != "24" || len(inv.Words) != 1 {
		t.Fatalf("supplier marks: %+v", inv)
	}
	if other := set.Profile(catalog.Normalize("SEL 2L C24"), "", "S2", false); len(other.Attrs["count"]) != 0 {
		t.Fatalf("a supplier's marks are read on its invoices only: %+v", other)
	}
	if pol, _ := set.PolicyFor("count", "", ""); pol != Soft {
		t.Fatalf("count is soft by default, got %s", pol)
	}
}

func TestVocabularySentences(t *testing.T) {
	r := Rule{Type: Attr, Scope: Scope{Kind: "global"}, Pattern: "zero|light", Action: Action{Attr: "variant", Value: "zero", Policy: Marker}}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	want := `"zero" or "light" is the variant "zero", in orders, invoices and catalogue names alike; a variant stated on one side only, or a different one, is never the same product.`
	if s := r.MakeSentence(nil); s != want {
		t.Fatalf("sentence:\n%s\n%s", s, want)
	}
	p := Rule{Type: Policy, Scope: Scope{Kind: "global"}, Pattern: "flavour", Action: Action{Policy: Soft}}
	if err := p.Validate(); err != nil || p.MakeSentence(nil) != "A different flavour makes a match less likely, never impossible." {
		t.Fatalf("policy: %v %q", err, p.MakeSentence(nil))
	}
}

func TestMatchValue(t *testing.T) {
	set := NewSet([]*Rule{{ID: "F", Type: Attr, Scope: Scope{Kind: "global"}, Pattern: "citron|orange|fruits rouges", Action: Action{Attr: "flavour", Value: "{match}"}}})
	p := set.Profile("nectar fruits rouges 1000ml", "", "", false)
	if v := p.Attrs["flavour"]; len(v) != 1 || v[0].V != "fruits rouges" || len(p.Words) != 1 {
		t.Fatalf("{match}: %+v", p)
	}
}
