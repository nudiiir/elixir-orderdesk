package desk

import "testing"

func TestParse(t *testing.T) {
	ms, ok := Parse("3 cartons dove shampooing 250ml\n2 fardou nivea creme, wahed carton hns")
	if !ok || len(ms) != 3 {
		t.Fatalf("parse: ok=%v %+v", ok, ms)
	}
	if ms[0].Qty != 3 || ms[0].Unit != "carton" || ms[0].Said != "dove shampooing 250ml" {
		t.Fatalf("line 1: %+v", ms[0])
	}
	if ms[1].Qty != 2 || ms[1].Unit != "pack" || ms[1].Said != "nivea creme" {
		t.Fatalf("line 2: %+v", ms[1])
	}
	if ms[2].Qty != 1 || ms[2].Unit != "carton" || ms[2].Said != "hns" {
		t.Fatalf("line 3: %+v", ms[2])
	}
	for _, chat := range []string{"comme la dernière fois", "combien le dove ?", "zid 2 cartons dove"} {
		if _, ok := Parse(chat); ok {
			t.Fatalf("%q must go to the AI", chat)
		}
	}
	if ms, ok := Parse("Ifri 1,5 x6"); !ok || ms[0].Said != "ifri 1.5 x6" {
		t.Fatalf("sizes stay in the words: %+v", ms)
	}
}
