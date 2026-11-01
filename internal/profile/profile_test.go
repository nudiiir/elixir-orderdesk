package profile

import (
	"strings"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
)

func TestBlocks(t *testing.T) {
	all := &Profile{Blocks: map[string]*BlockState{}, Lexicon: []*LexEntry{{From: "disj", To: "disjoncteur", Status: "live"}}}
	for _, b := range Library {
		all.Blocks[b.Name] = &BlockState{On: true}
	}
	all.Blocks["inch"].Params = map[string]bool{"bare": true}
	catalog.Pre, catalog.ExactDigits = all.Pre(), true
	defer func() { catalog.Pre, catalog.ExactDigits = nil, false }()
	cases := []struct{ in, want, not string }{
		{"VIS TH M8X20 ZN BTE 100", "8x20", " x20 "},
		{"vis m8 x 25", "8x25", ""},
		{"boulon 8x20 zingue", "8x20", ""},
		{"EAU IFRI 6X1.5L", "1500ml", "6x1"},
		{"CAB RIG 2,5 MM² BLEU", "2.5mm2", ""},
		{"disj 16 A 1P+N", "disjoncteur 16a", ""},
		{"disjoncteur16A", "disjoncteur 16a", ""},
		{"robinet 15/21", "in12", ""},
		{"RACCORD LAITON 1/2\" F", "in12", ""},
		{"mamelon 20/27", "in34", ""},
		{"RACCORD 1 1/2\" F", "in112", " in12 "},
		{"manchon 1 1/4", "in114", " in14 "},
		{"tuyau 40/42", "40", ""},
		{"lait nan 2", "nan", ""},
		{"PRODUIT A 1G/125MG SACH", "dose1000x125", ""},
		{"PRODUIT B 500MG/200MG", "dose500x200", ""},
		{"PRODUIT C 250MG/5ML", "dose250x5ml", ""},
		{"PRODUIT D 650.000UI/1000.000UI", "dose650000x1000000", ""},
		{"PRODUIT E 1,5MUI/250MG", "dose1500000x250", ""},
		{"PRODUIT F 10MG B/30", "x30", ""},
		{"BOISSON B12 PET", "b12", "x12"},
		{"ROUGE A LEVRES N°02", "n2", ""},
		{"PEINTURE VENUS 5L BLANC", "5000ml", ""},
	}
	for _, c := range cases {
		got := catalog.Normalize(c.in)
		if !strings.Contains(" "+got+" ", c.want) {
			t.Errorf("%q → %q, want it to contain %q", c.in, got, c.want)
		}
		if c.not != "" && strings.Contains(" "+got+" ", c.not) {
			t.Errorf("%q → %q, must not contain %q", c.in, got, c.not)
		}
	}
	if catalog.WordSim("8x20", "8x25") != 0 || catalog.WordSim("5w30", "5w40") != 0 {
		t.Error("words with digits must only match themselves")
	}
	if catalog.WordSim("disjoncteur", "disjoncter") == 0 {
		t.Error("words without digits still tolerate typos")
	}
}

func TestWilson(t *testing.T) {
	if w := Wilson(270, 270); w < 0.99 || w > 0.995 {
		t.Errorf("270/270 → %.4f, want just above 0.99", w)
	}
}
