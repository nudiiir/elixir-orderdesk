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
	for _, chat := range []string{"comme la dernière fois", "combien le dove ?", "zid 2 cartons dove",
		"Salam khouya, ab3athli 12 Nivea crème, 30 ml", "12 Colgate Total... la la, smahli, Colgate Herbal",
		"حطلي أربعة أفان كلينانس جال"} {
		if _, ok := Parse(chat); ok {
			t.Fatalf("%q must go to the AI", chat)
		}
	}
	if ms, ok := Parse("Ifri 1,5 x6"); !ok || ms[0].Said != "ifri 1.5 x6" {
		t.Fatalf("sizes stay in the words: %+v", ms)
	}
}

func TestParseQuantities(t *testing.T) {
	type item struct {
		qty  float64
		unit string
		said string
	}
	for _, c := range []struct {
		text string
		want []item // nil: not a plain list, the AI reads it
	}{
		{"3x nesquik 750g", []item{{3, "", "nesquik 750g"}}},
		{"nesquik 750g x3", []item{{3, "", "nesquik 750g"}}},
		{"ifri 1.5 x6", []item{{1, "", "ifri 1.5 x6"}}},           // after a bare size, x6 is the pack
		{"2 nesquik 750g x3", []item{{2, "", "nesquik 750g x3"}}}, // a quantity already said: x3 is the pack
		{"1/2 carton ifri", []item{{0.5, "carton", "ifri"}}},
		{"3/4 carton ifri", []item{{0.75, "carton", "ifri"}}},
		{"1/2 kg sucre", []item{{1, "", "500g sucre"}}}, // a size, not a half
		{"2 selecto 2l 3 ifri", []item{{2, "", "selecto 2000ml"}, {3, "", "ifri"}}},
		{"2 selecto 3 cartons ifri", []item{{2, "", "selecto"}, {3, "carton", "ifri"}}},
		{"12 bt doliprane 1000", []item{{12, "unit", "doliprane 1000"}}}, // a number at the end stays a size
		{"2 pampers 4 maxi", []item{{2, "", "pampers 4 maxi"}}},          // and after a name word it is part of it
		{"0 selecto 2l", nil},
		{"selecto 0", nil},
		{"2 selecto 2l 0 ifri", nil},
		{"99999999999999999999 selecto", nil},
		{"100001 selecto", nil},
	} {
		ms, ok := Parse(c.text)
		if c.want == nil {
			if ok {
				t.Errorf("%q must not be booked as read: %+v", c.text, ms)
			}
			continue
		}
		if !ok || len(ms) != len(c.want) {
			t.Errorf("%q: ok=%v %+v, want %+v", c.text, ok, ms, c.want)
			continue
		}
		for i, w := range c.want {
			if ms[i].Qty != w.qty || ms[i].Unit != w.unit || ms[i].Said != w.said {
				t.Errorf("%q item %d: %+v, want %+v", c.text, i+1, ms[i], w)
			}
		}
	}
}

func TestParseLenient(t *testing.T) {
	ms, unread := ParseLenient("salam khouya, ab3athli 3 kartonat moutarde w zouj nesquik")
	if len(ms) != 2 || len(unread) != 0 {
		t.Fatalf("greetings and fillers are dropped: %+v, unread %q", ms, unread)
	}
	if ms[0].Qty != 3 || ms[0].Unit != "carton" || ms[0].Said != "moutarde" || ms[1].Qty != 2 || ms[1].Said != "nesquik" {
		t.Fatalf("items: %+v", ms)
	}
	ms, unread = ParseLenient("Bonjour, ça va ?\n2 cartons selecto\nannule le dove\n0 ifri")
	if len(ms) != 1 || ms[0].Said != "selecto" || len(unread) != 2 {
		t.Fatalf("a greeting is dropped, a change and a 0 go to a person: %+v, unread %q", ms, unread)
	}
	if ms, unread := ParseLenient("combien le dove ?"); len(ms) != 0 || len(unread) != 1 {
		t.Fatalf("a question goes to a person: %+v, %q", ms, unread)
	}
}
