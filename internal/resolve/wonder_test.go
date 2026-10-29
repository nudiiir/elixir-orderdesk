package resolve

import (
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/rules"
)

// The vocabulary Muse Wonder wrote for the demo catalogue on 2026-10-29 (R-0006…R-0013), plus the
// synonym it had written before (R-0005), scored on the identity benchmark.
func wonderVocabulary() []*rules.Rule {
	g := rules.Scope{Kind: "global"}
	a := func(id, pattern, attr, value, pol string) *rules.Rule {
		return &rules.Rule{ID: id, Version: 1, Type: rules.Attr, Scope: g, Pattern: pattern, Action: rules.Action{Attr: attr, Value: value, Policy: pol}}
	}
	return []*rules.Rule{
		{ID: "R-0005", Version: 1, Type: rules.Synonym, Scope: g, Pattern: "slkt|slct|select", Action: rules.Action{Replace: "selecto"}},
		a("R-0006", "demi ecreme", "fat", "demi", ""),
		a("R-0007", "citron|orange|pomme|fraise|banane|chocolat|vanille|abricot|peche|nature|multifruits", "flavour", "{match}", ""),
		a("R-0008", "zero|sans sucre", "variant", "zero", rules.Marker),
		a("R-0009", "canette|verre", "container", "{match}", rules.Marker),
		a("R-0010", "morceaux", "cut", "morceaux", rules.Marker),
		a("R-0011", "fin|moyen", "grain", "{match}", ""),
		a("R-0012", "slim", "variant", "slim", rules.Marker),
		a("R-0013", "ecreme", "fat", "ecreme", ""),
	}
}

func TestWonderVocabulary(t *testing.T) {
	s := runBench(t, benchFixture(t), rules.NewSet(wonderVocabulary()), true)
	t.Logf("Wonder's vocabulary: right %d, wrong %d, correctly asked %d, missed %d of %d", s.right, s.wrong, s.asked, s.missed, s.total)
	if s.wrong > 0 {
		t.Errorf("%d wrong", s.wrong)
	}
	// R-0009 as the gate now requires it: containers strict, not a marker
	v := wonderVocabulary()
	v[4].Action.Policy = ""
	s = runBench(t, benchFixture(t), rules.NewSet(v), false)
	t.Logf("with canette|verre strict: right %d, wrong %d, correctly asked %d, missed %d of %d", s.right, s.wrong, s.asked, s.missed, s.total)
	if s.wrong > 0 {
		t.Errorf("%d wrong", s.wrong)
	}
}
