// Package engineer is the rule engineer: a Muse Wonder agent that studies the lines the rules
// could not resolve, writes new rules in the rule language and tests them on the benchmark.
// The decision to let a rule in is taken by code (the gate), never by the model.
package engineer

import (
	"fmt"

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
}

func GroundTruth(db *store.DB) []Truth {
	rows, err := db.Query(`SELECT line_id, said, norm, unit, customer_id, product_id, level, created_at FROM ground_truth ORDER BY created_at`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Truth
	for rows.Next() {
		var t Truth
		if rows.Scan(&t.LineID, &t.Said, &t.Norm, &t.Unit, &t.CustomerID, &t.ProductID, &t.Level, &t.CreatedAt) == nil {
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
	CoverBefore   float64   `json:"coverage_before"`
	CoverAfter    float64   `json:"coverage_after"`
	New           []Example `json:"examples_new"`
	Bad           []Example `json:"examples_wrong"`
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
		cx := resolve.Context{CustomerID: t.CustomerID}
		before := resolve.Deterministic(cat, live, m, cx)
		after := resolve.Deterministic(cat, with, m, cx)
		if before.Status == "ok" && before.ProductID == t.ProductID {
			okBefore++
		}
		if after.Status == "ok" && after.ProductID == t.ProductID {
			okAfter++
		}
		fired := false
		for _, u := range after.Used {
			if u == cand.ID {
				fired = true
			}
		}
		if !fired {
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
	return b
}

// Gate is the code that decides. A rule passes only with zero wrong matches, no conflict and a
// minimum gain.
func Gate(b Bench, minGain int) (bool, string) {
	switch {
	case b.Wrong > 0:
		return false, fmt.Sprintf("%d wrong match(es) on the benchmark", b.Wrong)
	case b.Conflicts > 0:
		return false, fmt.Sprintf("would break %d line(s) that are right today", b.Conflicts)
	case b.NewlyResolved < minGain:
		return false, fmt.Sprintf("resolves only %d new line(s), the gate needs %d", b.NewlyResolved, minGain)
	}
	return true, fmt.Sprintf("+%d resolved, 0 wrong, 0 conflicts", b.NewlyResolved)
}

// Residue is the ground truth the live rules and trusted aliases still cannot resolve alone.
func Residue(cat *catalog.Catalog, live *rules.Set, truth []Truth) []Truth {
	var out []Truth
	for _, t := range truth {
		r := resolve.Deterministic(cat, live, resolve.Mention{Said: t.Said, Unit: t.Unit, Qty: 1}, resolve.Context{CustomerID: t.CustomerID})
		if r.Status != "ok" || r.ProductID != t.ProductID {
			out = append(out, t)
		}
	}
	return out
}
