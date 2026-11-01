package engineer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
)

// The breaker: the benchmark only knows the lines the desk has seen. Before a rule that passed the gate
// goes to shadow, a second Muse Spark call tries to break it: it reads the rule and the products next to
// the ones the rule resolves, and writes lines a shop could send for those neighbours (another size,
// flavour, pack or brand). The code runs every attack line with and without the rule. If the rule books
// one of them to a product other than the one the attack means, the rule is refused.

// Attack is one line the breaker wrote, and what the rule did with it.
type Attack struct {
	Line   string `json:"line"`
	Means  string `json:"means"`  // the product the breaker says the line is
	Got    string `json:"got"`    // what the desk booked with the rule ("" when it would ask)
	Broken bool   `json:"broken"` // the rule booked it to another product
}

const breakerPrompt = `You test a matching rule before it goes live in a distributor's order desk.
The desk reads order and invoice lines and books each one to a product of the catalogue. The RULE below was
proposed and resolves the EXAMPLE lines correctly. Your job is to break it.
Write up to 8 lines a shop or a supplier could really write for one of the NEIGHBOUR products (not the
example products): another size, flavour, pack, colour or brand, abbreviated and spelled the way people
write them. Prefer lines that the rule could wrongly match. Each line must clearly mean one neighbour.
Return JSON only: {"attacks":[{"line":"…","means":"N3"}]}`

// Break attacks a candidate rule. It returns the attacks and whether one broke the rule.
func (e *Engineer) Break(live *rules.Set, cand *rules.Rule, b Bench) ([]Attack, bool, float64, error) {
	if !e.Meta.Enabled() || len(b.New) == 0 {
		return nil, false, 0, nil
	}
	with := live.With(cand)
	seen, neighbours := map[string]bool{}, []*catalog.Product{}
	var u strings.Builder
	fmt.Fprintf(&u, "RULE: %s\n\nEXAMPLES (line → product):\n", cand.MakeSentence(e.productName))
	for _, ex := range b.New {
		fmt.Fprintf(&u, "- \"%s\" → %s\n", ex.Said, ex.Expected)
		seen[ex.Expected] = true
		words, size := catalog.Tokens(catalog.Normalize(ex.Said))
		for _, p := range e.Cat.Shortlist(words, size, nil, nil, nil, 8) {
			if !seen[p.ID] && !seen[p.Name] && len(neighbours) < 24 {
				seen[p.ID] = true
				neighbours = append(neighbours, p)
			}
		}
	}
	if len(neighbours) == 0 {
		return nil, false, 0, nil
	}
	u.WriteString("\nNEIGHBOURS:\n")
	for i, p := range neighbours {
		fmt.Fprintf(&u, "N%d | %s\n", i+1, p.Name)
	}
	var resp struct {
		Attacks []struct {
			Line  string `json:"line"`
			Means string `json:"means"`
		} `json:"attacks"`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// Spark reasons before it answers: room first, then more room once if the answer came back empty
	cost, err := e.Meta.JSON(ctx, meta.Spark, "breaker", cand.ID, breakerPrompt, u.String(), nil, 8000, &resp)
	if err != nil || len(resp.Attacks) == 0 {
		c2, err2 := e.Meta.JSON(ctx, meta.Spark, "breaker", cand.ID, breakerPrompt, u.String(), nil, 14000, &resp)
		cost, err = cost+c2, err2
	}
	if err != nil {
		return nil, false, cost, err
	}
	var out []Attack
	broken := false
	for _, a := range resp.Attacks {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(a.Means), "N%d", &n); err != nil || n < 1 || n > len(neighbours) || strings.TrimSpace(a.Line) == "" {
			continue
		}
		want := neighbours[n-1]
		m := resolve.Mention{Said: a.Line, Qty: 1}
		after := resolve.Deterministic(e.Cat, with, m, resolve.Context{})
		at := Attack{Line: a.Line, Means: want.Name}
		if after.Status == "ok" {
			if p := e.Cat.Get(after.ProductID); p != nil {
				at.Got = p.Name
			}
			fired := false
			for _, id := range after.Used {
				fired = fired || id == cand.ID
			}
			// only the rule's own mistakes count: a line the desk already misread without it is not its doing
			before := resolve.Deterministic(e.Cat, live, m, resolve.Context{})
			if after.ProductID != want.ID && (fired || before.ProductID != after.ProductID) {
				at.Broken, broken = true, true
			}
		}
		out = append(out, at)
	}
	return out, broken, cost, nil
}
