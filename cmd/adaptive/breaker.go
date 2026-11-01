package main

import (
	"fmt"
	"log"
	"os"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/engineer"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/profile"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// breakRules runs the breaker against every rule the rule engineer got through the gate in an earlier run,
// with the desk as it was at the end of that run (its profile, its live rules, its ground truth).
func breakRules(path string) {
	db, err := store.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	cat := catalog.New(db)
	if p := profile.Load(db); p != nil {
		p.Install(cat)
	}
	eng := &engineer.Engineer{DB: db, Cat: cat, Meta: meta.New(os.Getenv("META_MODEL_API_KEY"), db)}
	truth := engineer.GroundTruth(db)
	all, _ := rules.List(db, false)
	var cost float64
	tested, broken := 0, 0
	for _, r := range all {
		if r.Author != "rule-engineer" || (r.Status != "live" && r.Status != "shadow") {
			continue
		}
		// the desk without this rule, as the gate saw it
		var others []*rules.Rule
		for _, o := range all {
			if o.ID != r.ID && o.Status == "live" {
				others = append(others, o)
			}
		}
		live := rules.NewSet(others)
		b := engineer.RunBench(cat, live, r, truth)
		// later aliases resolve most of its lines by now: show the breaker the lines where the rule fires
		with := live.With(r)
		b.New = nil
		for _, t := range truth {
			res := resolve.Deterministic(cat, with, resolve.Mention{Said: t.Said, Unit: t.Unit, Qty: 1}, resolve.Context{SupplierID: t.SupplierID})
			fired := false
			for _, u := range res.Used {
				fired = fired || u == r.ID
			}
			if p := cat.Get(t.ProductID); fired && p != nil && len(b.New) < 5 {
				b.New = append(b.New, engineer.Example{Said: t.Said, Expected: p.Name})
			}
		}
		attacks, bad, c, err := eng.Break(live, r, b)
		cost += c
		if err != nil {
			log.Printf("%s: %v", r.ID, err)
			continue
		}
		tested++
		verdict := "held"
		if bad {
			broken++
			verdict = "BROKEN"
		}
		fmt.Printf("%s %s — %d attack lines: %s\n", r.ID, r.Sentence, len(attacks), verdict)
		for _, a := range attacks {
			mark := "  "
			if a.Broken {
				mark = "✗ "
			}
			got := a.Got
			if got == "" {
				got = "(asks)"
			}
			fmt.Printf("   %s%q means %s → %s\n", mark, a.Line, a.Means, got)
		}
	}
	fmt.Printf("\n%d rules attacked, %d broken; Meta spend $%.4f\n", tested, broken, cost)
}
