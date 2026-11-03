package desk

import "testing"

func TestParseShopAbbreviations(t *testing.T) {
	ms, ok := Parse("3 ctn napoli fromagere 220g\n5 fdx ifri 1.5\n12 bt doliprane 1000\n2 pcs sweety pistache")
	if !ok || len(ms) != 4 {
		t.Fatalf("parse: %v %+v", ok, ms)
	}
	for i, want := range []string{"carton", "pack", "unit", "unit"} {
		if ms[i].Unit != want {
			t.Errorf("line %d: unit %q, want %q (%+v)", i+1, ms[i].Unit, want, ms[i])
		}
	}
	if ms[0].Said != "napoli fromagere 220g" {
		t.Errorf("unit word left in the name: %q", ms[0].Said)
	}
}
