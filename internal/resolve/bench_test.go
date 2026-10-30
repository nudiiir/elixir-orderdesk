package resolve

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// The identity benchmark: drinks, groceries and mass-market cosmetics, the way shops write them
// and suppliers print them. "want" is the product id, or "" when the words cannot prove one product
// (the desk must ask, never guess). No AI anywhere: this is levels 0-2 only.

var benchProducts = []catalog.Product{
	{ID: "ifri33", Name: "Ifri Eau Minérale 0.33L", Brand: "Ifri", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "ifri50", Name: "Ifri Eau Minérale 0.5L", Brand: "Ifri", Packs: []catalog.Pack{{Name: "Carton x12", FactorMilli: 12000}}},
	{ID: "ifri150", Name: "Ifri Eau Minérale 1.5L", Brand: "Ifri", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "ifri200", Name: "Ifri Eau Minérale 2L", Brand: "Ifri", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "ifrigaz", Name: "Ifri Eau Gazéifiée 1L", Brand: "Ifri", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "ifruitor", Name: "Ifri Ifruit Orange 1L", Brand: "Ifri", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "ifrisoda", Name: "Ifri Soda Citron 1L", Brand: "Ifri", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "saida150", Name: "Saïda Eau Minérale 1.5L", Brand: "Saïda", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "lk150", Name: "Lalla Khedidja Eau Minérale 1.5L", Brand: "Lalla Khedidja", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "coca1", Name: "Coca-Cola 1L", Brand: "Coca-Cola", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "coca15", Name: "Coca-Cola 1.5L", Brand: "Coca-Cola", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "coca33", Name: "Coca-Cola 33cl canette", Brand: "Coca-Cola", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "coca25", Name: "Coca-Cola 25cl verre", Brand: "Coca-Cola", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "cocaz1", Name: "Coca-Cola Zéro 1L", Brand: "Coca-Cola", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "cocaz33", Name: "Coca-Cola Zéro 33cl canette", Brand: "Coca-Cola", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "fantac1", Name: "Fanta Citron 1L", Brand: "Fanta", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "fantao1", Name: "Fanta Orange 1L", Brand: "Fanta", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "fantao2", Name: "Fanta Orange 2L", Brand: "Fanta", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "sel1", Name: "Hamoud Boualem Selecto 1L", Brand: "Hamoud Boualem", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "sel2", Name: "Hamoud Boualem Selecto 2L", Brand: "Hamoud Boualem", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "sel25", Name: "Hamoud Boualem Selecto 25cl verre", Brand: "Hamoud Boualem", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "sel33", Name: "Hamoud Boualem Selecto 33cl canette", Brand: "Hamoud Boualem", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "gaz2", Name: "Hamoud Boualem Hamoud Gazouz 2L", Brand: "Hamoud Boualem", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "slimc1", Name: "Hamoud Boualem Slim Citron 1L", Brand: "Hamoud Boualem", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "slimc2", Name: "Hamoud Boualem Slim Citron 2L", Brand: "Hamoud Boualem", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "slimo1", Name: "Hamoud Boualem Slim Orange 1L", Brand: "Hamoud Boualem", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "slimo2", Name: "Hamoud Boualem Slim Orange 2L", Brand: "Hamoud Boualem", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "hbpomme", Name: "Hamoud Boualem Pomme 1L", Brand: "Hamoud Boualem", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "rouor1", Name: "Rouiba Orange 1L", Brand: "Rouiba", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "rouor20", Name: "Rouiba Orange 20cl", Brand: "Rouiba", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "rouor2", Name: "Rouiba Orange 2L", Brand: "Rouiba", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "roupom1", Name: "Rouiba Pomme 1L", Brand: "Rouiba", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "candiad1", Name: "Candia Lait UHT Demi-écrémé 1L", Brand: "Candia", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "candiad50", Name: "Candia Lait UHT Demi-écrémé 50cl", Brand: "Candia", Packs: []catalog.Pack{{Name: "Carton x12", FactorMilli: 12000}}},
	{ID: "candiae1", Name: "Candia Lait UHT Entier 1L", Brand: "Candia", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "candiaec1", Name: "Candia Lait UHT Écrémé 1L", Brand: "Candia", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "elio1", Name: "Cevital Elio Huile 1L", Brand: "Cevital", Packs: []catalog.Pack{{Name: "Carton x12", FactorMilli: 12000}}},
	{ID: "elio2", Name: "Cevital Elio Huile 2L", Brand: "Cevital", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "elio5", Name: "Cevital Elio Huile 5L", Brand: "Cevital", Packs: []catalog.Pack{{Name: "Carton x6", FactorMilli: 6000}}},
	{ID: "skor", Name: "Cevital Skor Sucre 1kg", Brand: "Cevital", Packs: []catalog.Pack{{Name: "Carton x12", FactorMilli: 12000}}},
	{ID: "skorm", Name: "Cevital Skor Sucre Morceaux 1kg", Brand: "Cevital", Packs: []catalog.Pack{{Name: "Carton x12", FactorMilli: 12000}}},
	{ID: "tom400", Name: "Amor Benamor Tomate Concentrée 400g", Brand: "Amor Benamor", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "tom800", Name: "Amor Benamor Tomate Concentrée 800g", Brand: "Amor Benamor", Packs: []catalog.Pack{{Name: "Carton x12", FactorMilli: 12000}}},
	{ID: "amcf", Name: "Amor Benamor Couscous Fin 1kg", Brand: "Amor Benamor", Packs: []catalog.Pack{{Name: "Carton x12", FactorMilli: 12000}}},
	{ID: "amcm", Name: "Amor Benamor Couscous Moyen 1kg", Brand: "Amor Benamor", Packs: []catalog.Pack{{Name: "Carton x12", FactorMilli: 12000}}},
	{ID: "simspa1", Name: "Sim Spaghetti 1kg", Brand: "Sim", Packs: []catalog.Pack{{Name: "Carton x12", FactorMilli: 12000}}},
	{ID: "simspa5", Name: "Sim Spaghetti 500g", Brand: "Sim", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "amspa5", Name: "Amor Benamor Spaghetti 500g", Brand: "Amor Benamor", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "danyf", Name: "Danone Yaourt Fraise 100g", Brand: "Danone", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "danyn", Name: "Danone Yaourt Nature 100g", Brand: "Danone", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "soumyf", Name: "Soummam Yaourt Fraise 110g", Brand: "Soummam", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "soumyv", Name: "Soummam Yaourt Vanille 110g", Brand: "Soummam", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "harissa", Name: "Maïza Harissa 135g", Brand: "Maïza", Packs: []catalog.Pack{{Name: "Carton x24", FactorMilli: 24000}}},
	{ID: "dovesh", Name: "Dove Color Care Shampooing 250 ml", Brand: "Dove"},
	{ID: "dovedeo", Name: "Dove Déodorant stick Original 40 ml", Brand: "Dove"},
	{ID: "dovebar", Name: "Dove Pain de toilette Beauty Cream Bar 90 g", Brand: "Dove"},
	{ID: "hns", Name: "Head & Shoulders Supreme Shampooing 400 ml", Brand: "Head & Shoulders"},
	{ID: "niveasoft", Name: "Nivea Soft Crème hydratante 200 ml", Brand: "Nivea"},
	{ID: "niveabox", Name: "Nivea Crème boîte 150 ml", Brand: "Nivea"},
	{ID: "niveamen", Name: "Nivea Men Fresh Power roll-on 50 ml", Brand: "Nivea"},
	{ID: "sigtar", Name: "Signal Dentifrice Anti-tartre 75 ml", Brand: "Signal"},
	{ID: "sig8", Name: "Signal Integral 8 Complet Dentifrice 75 ml", Brand: "Signal"},
	{ID: "sigwhite", Name: "Signal White Now Détox Dentifrice 75 ml", Brand: "Signal"},
	{ID: "colgate", Name: "Colgate Triple Action Dentifrice 100 ml", Brand: "Colgate"},
	{ID: "pampact", Name: "Pampers Active Baby Pants T4 (9-15 kg)", Brand: "Pampers"},
	{ID: "pampprem", Name: "Pampers Premium Protection T4 (9-14 kg)", Brand: "Pampers"},
	{ID: "pampling", Name: "Pampers Lingettes Aqua Harmonie x48", Brand: "Pampers"},
	{ID: "garnsh", Name: "Garnier Ultra Doux Shampooing Avocat & Karité 400 ml", Brand: "Garnier"},
	{ID: "garnmic", Name: "Garnier SkinActive Eau micellaire tout-en-1 400 ml", Brand: "Garnier"},
	{ID: "velvet", Name: "Rouge à lèvres Velvet Mat", Brand: "Velvet"},
	{ID: "velvet1", Name: "Rouge à lèvres Velvet Mat — Teinte 01 Nude", Brand: "Velvet"},
	{ID: "velvet2", Name: "Rouge à lèvres Velvet Mat — Teinte 02 Rouge", Brand: "Velvet"},
	{ID: "modilac", Name: "Modilac Expert Anti-régurgitations 1 800 g", Brand: "Modilac"},
	{ID: "celia1", Name: "Célia 1 lait infantile dès la naissance 800 g", Brand: "Célia"},
	{ID: "celia2", Name: "Célia 2 lait 2e âge dès 6 mois 800 g", Brand: "Célia"},
}

type benchLine struct {
	said, want string
	supplier   string
}

var benchLines = []benchLine{
	// sizes written every way
	{said: "ifri 1.5", want: "ifri150"},
	{said: "ifri 1,5l", want: "ifri150"},
	{said: "Ifri 1,5 L", want: "ifri150"},
	{said: "ifri 150cl", want: "ifri150"},
	{said: "IFRI 1500ML X6", want: "ifri150"},
	{said: "ifri 6x1.5l", want: "ifri150"},
	{said: "ifri 1l5", want: "ifri150"},
	{said: "ifri 33cl", want: "ifri33"},
	{said: "ifri 0.5", want: "ifri50"},
	{said: "ifri 2l", want: "ifri200"},
	{said: "إفري 1.5", want: ""}, // Arabic script brand: no rule yet, must not guess
	// under-specified: the desk must ask
	{said: "ifri", want: ""},
	{said: "ifri eau", want: ""},
	{said: "rouiba orange", want: ""},
	{said: "fanta 1l", want: ""},
	{said: "selecto", want: ""},
	{said: "candia lait 1l", want: ""},
	{said: "elio", want: ""},
	{said: "signal dentifrice", want: ""},
	{said: "pampers t4", want: ""},
	{said: "nivea creme", want: ""},
	{said: "velvet rouge a levres", want: ""},
	{said: "yaourt fraise", want: ""},
	{said: "spaghetti 500g", want: ""},
	{said: "couscous amor", want: ""},
	// exact
	{said: "coca 1.5l", want: "coca15"},
	{said: "coca cola 33cl", want: "coca33"}, // with Zéro a marker, a line that does not say it is not Zéro
	{said: "coca zero 1l", want: "cocaz1"},
	{said: "coca zéro canette", want: "cocaz33"},
	{said: "fanta orange 2l", want: "fantao2"},
	{said: "fanta citron", want: "fantac1"},
	{said: "selecto 2l", want: "sel2"},
	{said: "selecto 25cl", want: "sel25"},
	{said: "slim citron 2l", want: "slimc2"},
	{said: "gazouz 2l", want: "gaz2"},
	{said: "rouiba orange 20cl", want: "rouor20"},
	{said: "rouiba pomme", want: "roupom1"},
	{said: "candia entier", want: "candiae1"},
	{said: "candia demi ecreme 50cl", want: "candiad50"},
	{said: "elio 5l", want: "elio5"},
	{said: "huile elio 2 litres", want: "elio2"},
	{said: "skor sucre morceaux", want: "skorm"},
	{said: "tomate amor 800g", want: "tom800"},
	{said: "tomate concentree 400 g", want: "tom400"},
	{said: "couscous moyen amor", want: "amcm"},
	{said: "sim spaghetti 1kg", want: "simspa1"},
	{said: "danone fraise", want: "danyf"},
	{said: "soummam vanille", want: "soumyv"},
	{said: "harissa maiza", want: "harissa"},
	{said: "dove shampooing", want: "dovesh"},
	{said: "dove 250ml", want: "dovesh"},
	{said: "head shoulders 400", want: "hns"},
	{said: "nivea soft 200ml", want: "niveasoft"},
	{said: "colgate", want: "colgate"},
	{said: "signal integral 8", want: "sig8"},
	{said: "pampers lingettes", want: "pampling"},
	{said: "garnier micellaire", want: "garnmic"},
	{said: "velvet teinte 02", want: "velvet2"},
	{said: "modilac 1800g", want: "modilac"},
	{said: "celia 2", want: "celia2"},
	{said: "celia 1 800g", want: "celia1"},
	// supplier invoices
	{said: "SELECTO 2000ML PET X6", want: "sel2", supplier: "S1"},
	{said: "SELECTO CAN 33CL X24", want: "sel33", supplier: "S1"},
	{said: "SELECTO 25CL VERRE X24", want: "sel25", supplier: "S1"},
	{said: "SLIM ORANGE 1L PET X6", want: "slimo1", supplier: "S1"},
	{said: "COCA COLA 1L PET X6", want: "coca1", supplier: "S2"},
	{said: "COCA COLA ZERO 33CL CAN X24", want: "cocaz33", supplier: "S2"},
	{said: "LAIT CANDIA ENTIER 1L X6", want: "candiae1", supplier: "S3"},
	// these need vocabulary (flavours, variants, containers): missed without it, never wrong
	{said: "coca 1l", want: "coca1"},
	{said: "coca canette", want: "coca33"},
	{said: "candia ecreme 1l", want: "candiaec1"},
	{said: "skor sucre 1kg", want: "skor"},
	{said: "ifri gazeuse", want: "ifrigaz"},
	{said: "HB SELECTO 2L", want: "sel2", supplier: "S1"},
	{said: "selecto canette", want: "sel33"},
	{said: "selecto verre", want: "sel25"},
	{said: "SLIM ORG 2L PET X6", want: "slimo2", supplier: "S1"},
	{said: "ifri citron", want: "ifrisoda"},
	{said: "rouiba 2l", want: "rouor2"},
	{said: "nivea men", want: "niveamen"},
	{said: "pampers premium", want: "pampprem"},
	{said: "slim 1l", want: ""},
	{said: "dove", want: ""},
	{said: "orange 1l", want: ""},
	{said: "eau 1.5", want: ""},
	{said: "pomme 1l", want: ""},
	{said: "lait 1l", want: ""},
	{said: "velvet mat", want: ""},
	{said: "selecto 33cl", want: "sel33"},
}

// vocabulary is the kind of rule set the rule engineer writes for this catalogue.
func vocabulary() []*rules.Rule {
	g := rules.Scope{Kind: "global"}
	n := 0
	attr := func(pattern, a, v, pol string) *rules.Rule {
		n++
		return &rules.Rule{ID: fmt.Sprintf("V%02d", n), Version: 1, Type: rules.Attr, Scope: g, Pattern: pattern,
			Action: rules.Action{Attr: a, Value: v, Policy: pol}}
	}
	syn := func(pattern, rep string) *rules.Rule {
		n++
		return &rules.Rule{ID: fmt.Sprintf("V%02d", n), Version: 1, Type: rules.Synonym, Scope: g, Pattern: pattern, Action: rules.Action{Replace: rep}}
	}
	return []*rules.Rule{
		attr("citron|lemon|limon", "flavour", "citron", ""),
		attr("orange|org", "flavour", "orange", ""),
		attr("pomme|pom", "flavour", "pomme", ""),
		attr("fraise", "flavour", "fraise", ""),
		attr("vanille", "flavour", "vanille", ""),
		attr("nature", "flavour", "nature", ""),
		attr("zero|light|sans sucre", "variant", "zero", rules.Marker),
		attr("canette|can", "container", "canette", ""),
		attr("verre", "container", "verre", ""),
		attr("pet|plastique", "container", "pet", ""),
		attr("demi ecreme", "fat", "demi", ""),
		attr("ecreme", "fat", "ecreme", ""),
		attr("entier", "fat", "entier", ""),
		attr("morceaux|morceau", "cut", "morceaux", rules.Marker),
		syn("gazeuse", "gazeifiee"),
		syn("hb", "hamoud boualem"),
	}
}

func TestIdentityBenchWithVocabulary(t *testing.T) {
	cat := benchFixture(t)
	s := runBench(t, cat, rules.NewSet(vocabulary()), true)
	t.Logf("with vocabulary: right %d, wrong %d, correctly asked %d, missed %d of %d", s.right, s.wrong, s.asked, s.missed, s.total)
	if s.wrong > 0 {
		t.Errorf("%d wrong automatic matches", s.wrong)
	}
}

func TestAskText(t *testing.T) {
	cat := benchFixture(t)
	set := rules.NewSet(vocabulary())
	for said, want := range map[string]string{
		"ifri minerale": "« ifri minerale » : 33 cl, 50 cl, 1,5 L ou 2 L ?",
		"ifri eau":      "« ifri eau » : lequel ?",
		"fanta 1l":      "« fanta 1l » : citron ou orange ?",
		"rouiba orange": "« rouiba orange » : 20 cl, 1 L ou 2 L ?",
	} {
		r := Deterministic(cat, set, Mention{Said: said}, Context{})
		if !askable(r) {
			t.Fatalf("%s should be asked directly: %+v", said, r.verdict)
		}
		if got := AskText(said, r.verdict, set); got != want {
			t.Errorf("%s: got %q want %q", said, got, want)
		}
	}
}

func benchFixture(t testing.TB) *catalog.Catalog {
	db, err := store.Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := catalog.ReplaceProducts(db, benchProducts); err != nil {
		t.Fatal(err)
	}
	return catalog.New(db)
}

type benchScore struct{ right, wrong, asked, missed, total int }

func runBench(t *testing.T, cat *catalog.Catalog, set *rules.Set, verbose bool) benchScore {
	var s benchScore
	for _, l := range benchLines {
		r := Deterministic(cat, set, Mention{Said: l.said}, Context{SupplierID: l.supplier})
		s.total++
		got := ""
		if r.Status == "ok" {
			got = r.ProductID
		}
		switch {
		case got != "" && got == l.want:
			s.right++
		case got != "":
			s.wrong++
			t.Logf("WRONG  %-28q → %s (want %q) %s", l.said, got, l.want, r.Why)
		case l.want == "":
			s.asked++
		default:
			s.missed++
			if verbose {
				t.Logf("missed %-28q (want %s) %s", l.said, l.want, r.Why)
			}
		}
	}
	return s
}

func TestIdentityBench(t *testing.T) {
	cat := benchFixture(t)
	s := runBench(t, cat, rules.NewSet(nil), true)
	t.Logf("no rules: right %d, wrong %d, correctly asked %d, missed %d of %d", s.right, s.wrong, s.asked, s.missed, s.total)
	if s.wrong > 0 {
		t.Errorf("%d wrong automatic matches", s.wrong)
	}
	_ = strings.TrimSpace
}

func TestStrengthsAndCombinations(t *testing.T) {
	set := rules.NewSet(nil)
	read := func(s string) rules.Profile { return set.Profile(catalog.Normalize(s), "", "", false) }
	if a, b := read("DOLIPRANE 1G CP/8").Attrs["size"], read("DOLIPRANE 1000MG B / 08").Attrs["size"]; len(a) != 1 || len(b) != 1 || a[0].V != b[0].V {
		t.Fatalf("1g and 1000mg must read the same: %v %v", a, b)
	}
	if v := read("LEVOTHYROX 25µg").Attrs["size"]; len(v) != 1 || v[0].V != "0.000025g" {
		t.Fatalf("µg: %v", v)
	}
	if c := compare(set, read("EXTEL 40MG/5MG COMP B/30"), read("EXTEL 80MG/5MG COMP B/30"), Context{}); c.veto != "size" {
		t.Fatalf("40mg/5mg is not 80mg/5mg: %+v", c)
	}
	if c := compare(set, read("EFFERALGAN VITAMINE C 500MG"), read("EFFERALGAN VITAMINE C 500MG / 200MG B / 16"), Context{}); c.veto != "" {
		t.Fatalf("a line stating one component of a combination is still that combination: %+v", c)
	}
}
