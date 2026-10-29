// Package engineer is the rule engineer: a Muse Wonder agent that studies the lines the rules
// could not resolve, writes new rules in the rule language and tests them on the benchmark.
// The decision to let a rule in is taken by code (the gate), never by the model.
package engineer

import (
	"fmt"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// Truth is one ground-truth line.
type Truth struct {
	LineID     string `json:"line_id"`
	Said       string `json:"said"`
	Norm       string `json:"norm"`
	Unit       string `json:"unit"`
	CustomerID string `json:"customer_id"`
	ProductID  string `json:"product_id"`
	Level      string `json:"level"`
	CreatedAt  string `json:"created_at"`
	Kind       string `json:"kind"`
	SupplierID string `json:"supplier_id"`
}

func GroundTruth(db *store.DB) []Truth {
	rows, err := db.Query(`SELECT line_id, said, norm, unit, customer_id, product_id, level, created_at, kind, supplier_id FROM ground_truth ORDER BY created_at`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Truth
	for rows.Next() {
		var t Truth
		if rows.Scan(&t.LineID, &t.Said, &t.Norm, &t.Unit, &t.CustomerID, &t.ProductID, &t.Level, &t.CreatedAt, &t.Kind, &t.SupplierID) == nil {
			out = append(out, t)
		}
	}
	return out
}

type Example struct {
	Said     string `json:"said"`
	Expected string `json:"expected"`
	Got      string `json:"got,omitempty"`
}

// Bench is the result of testing one candidate rule on every known line.
type Bench struct {
	Lines         int       `json:"lines"`
	NewlyResolved int       `json:"newly_resolved"`
	Wrong         int       `json:"wrong"`
	Conflicts     int       `json:"conflicts"` // lines it would turn from right to wrong
	Lost          int       `json:"lost"`      // lines right today that it would leave unresolved
	CoverBefore   float64   `json:"coverage_before"`
	CoverAfter    float64   `json:"coverage_after"`
	New           []Example `json:"examples_new"`
	Bad           []Example `json:"examples_wrong"`
	// vocabulary rules (attribute, word, policy) also read the catalogue:
	Vocabulary      bool      `json:"vocabulary,omitempty"`
	ProductsChanged int       `json:"products_changed,omitempty"` // catalogue names it reads differently
	Twins           int       `json:"twins,omitempty"`            // pairs of products it would make indistinguishable
	TwinExamples    []Example `json:"examples_twins,omitempty"`
	Marked          int       `json:"marked,omitempty"`  // marker rules: products carrying the marker
	Orphans         int       `json:"orphans,omitempty"` // of those, products with no plain product beside them
	OrphanExamples  []Example `json:"examples_orphans,omitempty"`
	ProductExamples []Example `json:"examples_products,omitempty"`
}

// RunBench applies the live rules with and without the candidate to every ground-truth line,
// using the deterministic levels only (no AI), exactly as the line would be resolved live.
func RunBench(cat *catalog.Catalog, live *rules.Set, cand *rules.Rule, truth []Truth) Bench {
	b := Bench{Lines: len(truth)}
	with := live.With(cand)
	name := func(id string) string {
		if p := cat.Get(id); p != nil {
			return p.Name
		}
		return id
	}
	okBefore, okAfter := 0, 0
	for _, t := range truth {
		m := resolve.Mention{Said: t.Said, Unit: t.Unit, Qty: 1}
		cx := resolve.Context{CustomerID: t.CustomerID, SupplierID: t.SupplierID}
		before := resolve.Deterministic(cat, live, m, cx)
		after := resolve.Deterministic(cat, with, m, cx)
		if before.Status == "ok" && before.ProductID == t.ProductID {
			okBefore++
		}
		if after.Status == "ok" && after.ProductID == t.ProductID {
			okAfter++
		}
		fired := before.Status != after.Status || before.ProductID != after.ProductID
		for _, u := range after.Used {
			if u == cand.ID {
				fired = true
			}
		}
		if !fired {
			continue
		}
		if before.Status == "ok" && before.ProductID == t.ProductID && after.Status != "ok" {
			b.Lost++
			if len(b.Bad) < 5 {
				b.Bad = append(b.Bad, Example{Said: t.Said, Expected: name(t.ProductID), Got: "(would ask: " + after.Why + ")"})
			}
			continue
		}
		switch {
		case after.Status == "ok" && after.ProductID == t.ProductID && !(before.Status == "ok" && before.ProductID == t.ProductID):
			b.NewlyResolved++
			if len(b.New) < 5 {
				b.New = append(b.New, Example{Said: t.Said, Expected: name(t.ProductID)})
			}
		case after.Status == "ok" && after.ProductID != t.ProductID:
			if before.Status == "ok" && before.ProductID == t.ProductID {
				b.Conflicts++
			} else {
				b.Wrong++
			}
			if len(b.Bad) < 5 {
				b.Bad = append(b.Bad, Example{Said: t.Said, Expected: name(t.ProductID), Got: name(after.ProductID)})
			}
		}
	}
	if len(truth) > 0 {
		b.CoverBefore = float64(okBefore) / float64(len(truth))
		b.CoverAfter = float64(okAfter) / float64(len(truth))
	}
	if cand.Type == rules.Attr || cand.Type == rules.Word || cand.Type == rules.Policy {
		b.Vocabulary = true
		catalogueEffect(cat, live, with, &b)
	}
	return b
}

// markerCheck: a marker says "a plain product exists, and this is the variant of it" (Coca-Cola /
// Coca-Cola Zéro). A product carrying the marker with no plain product beside it (same name words,
// same size, no marker) could then only be reached by naming the marker: the marker is wrong.
func markerCheck(cat *catalog.Catalog, with *rules.Set, b *Bench) {
	type pp struct {
		p  *catalog.Product
		pr rules.Profile
	}
	var all []pp
	for _, p := range cat.Products() {
		all = append(all, pp{p, resolve.ProductProfile(with, p)})
	}
	same := func(a, c []string) bool { return strings.Join(a, " ") == strings.Join(c, " ") }
	size := func(pr rules.Profile) string { return store.JSON(rules.Certain(pr.Attrs["size"])) }
	for _, x := range all {
		for a, vs := range x.pr.Attrs {
			if len(vs) == 0 {
				continue
			}
			if pol, _ := with.PolicyFor(a, "", ""); pol != rules.Marker {
				continue
			}
			b.Marked++
			plain := false
			for _, y := range all {
				if y.p.ID != x.p.ID && len(y.pr.Attrs[a]) == 0 && same(y.pr.Words, x.pr.Words) && size(y.pr) == size(x.pr) {
					plain = true
					break
				}
			}
			if !plain {
				b.Orphans++
				if len(b.OrphanExamples) < 4 {
					b.OrphanExamples = append(b.OrphanExamples, Example{Said: x.p.Name, Got: a})
				}
			}
		}
	}
}

// catalogueEffect: how a vocabulary rule changes the way catalogue names are read. A rule that
// makes two different products read the same erased a difference the catalogue has.
func catalogueEffect(cat *catalog.Catalog, live, with *rules.Set, b *Bench) {
	key := func(p rules.Profile) string { return store.JSON(map[string]any{"w": p.Words, "a": p.Attrs}) }
	before, after := map[string][]string{}, map[string][]string{}
	for _, p := range cat.Products() {
		kb, ka := key(resolve.ProductProfile(live, p)), key(resolve.ProductProfile(with, p))
		if kb != ka {
			b.ProductsChanged++
			if len(b.ProductExamples) < 4 {
				pr := resolve.ProductProfile(with, p)
				b.ProductExamples = append(b.ProductExamples, Example{Said: p.Name, Got: store.JSON(pr.Attrs)})
			}
		}
		before[kb] = append(before[kb], p.Name)
		after[ka] = append(after[ka], p.Name)
	}
	twins := func(m map[string][]string) map[string]bool {
		out := map[string]bool{}
		for _, names := range m {
			for i := range names {
				for j := i + 1; j < len(names); j++ {
					out[names[i]+" = "+names[j]] = true
				}
			}
		}
		return out
	}
	markerCheck(cat, with, b)
	tb := twins(before)
	for pair := range twins(after) {
		if !tb[pair] {
			b.Twins++
			if len(b.TwinExamples) < 4 {
				b.TwinExamples = append(b.TwinExamples, Example{Said: pair})
			}
		}
	}
}

// Gate is the code that decides. A rule passes only with zero wrong matches, no conflict and a
// minimum gain.
func Gate(b Bench, minGain int) (bool, string) {
	switch {
	case b.Wrong > 0:
		return false, fmt.Sprintf("%d wrong match(es) on the benchmark", b.Wrong)
	case b.Conflicts > 0:
		return false, fmt.Sprintf("would break %d line(s) that are right today", b.Conflicts)
	case b.Lost > 0:
		return false, fmt.Sprintf("would stop resolving %d line(s) that resolve today", b.Lost)
	case b.Twins > 0:
		return false, fmt.Sprintf("would make %d pair(s) of different products read the same", b.Twins)
	case b.Orphans > 0 && b.Orphans*5 > b.Marked:
		return false, fmt.Sprintf("a marker needs a plain product beside the variant: %d of %d products it marks have none (e.g. %s)",
			b.Orphans, b.Marked, b.OrphanExamples[0].Said)
	case b.Vocabulary && b.NewlyResolved < minGain && b.ProductsChanged >= 2:
		// vocabulary describes the catalogue: it may come in before any line needs it, as long as
		// it breaks nothing and reads at least two products
		return true, fmt.Sprintf("+%d resolved, reads %d catalogue names, 0 wrong, 0 conflicts", b.NewlyResolved, b.ProductsChanged)
	case b.NewlyResolved < minGain:
		return false, fmt.Sprintf("resolves only %d new line(s), the gate needs %d", b.NewlyResolved, minGain)
	}
	if b.Vocabulary {
		return true, fmt.Sprintf("+%d resolved, reads %d catalogue names, 0 wrong, 0 conflicts", b.NewlyResolved, b.ProductsChanged)
	}
	return true, fmt.Sprintf("+%d resolved, 0 wrong, 0 conflicts", b.NewlyResolved)
}

// Residue is the ground truth the live rules and trusted aliases still cannot resolve alone.
func Residue(cat *catalog.Catalog, live *rules.Set, truth []Truth) []Truth {
	var out []Truth
	for _, t := range truth {
		r := resolve.Deterministic(cat, live, resolve.Mention{Said: t.Said, Unit: t.Unit, Qty: 1}, resolve.Context{CustomerID: t.CustomerID, SupplierID: t.SupplierID})
		if r.Status != "ok" || r.ProductID != t.ProductID {
			out = append(out, t)
		}
	}
	return out
}
