package catalog

import (
	"strings"
	"testing"
)

func TestModelCodeIsNotACount(t *testing.T) {
	for in, want := range map[string][]string{
		"CÂBLE HOCO X21 IPHONE": {"cable", "hoco", "x21", "iphone"},
		"CABLE HOCO X21 IP":     {"cable", "hoco", "x21", "ip"},
		"COCA COLA 1L X6":       {"coca", "cola"},
		"X6 IFRI 1,5L":          {"ifri"},
		"BISCUIT KOOL X12":      {"biscuit", "kool"},
	} {
		got, _ := Tokens(Normalize(in))
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("Tokens(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNormalizeSizeGluedToCount(t *testing.T) {
	for in, want := range map[string]string{
		"SPRITE LIMO 1LX12":        "sprite limo 1000ml x12",
		"JUS FRUIT MAX 2LX10":      "jus fruit max 2000ml x10",
		"PUREE VICTORIA 240GX12":   "puree victoria 240g x12",
		"FOURRAGE KRINCHY 500G*16": "fourrage krinchy 500g x16",
		"ARWA 33CLX24":             "arwa 330ml x24",
		"VIS TH M12X40 ZN":         "vis th m12x40 zn", // a dimension, not a size
		"IFRI 1L5":                 "ifri 1500ml",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
