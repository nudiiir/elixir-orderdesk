package desk

import (
	"math"
	"sync"

	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
)

// Autonomy decides when the desk's own steps (codes, aliases, rules, the judge) may answer without Muse
// Spark checking them. Every confirmed line whose answer the code gave is evidence: right when the
// person kept the code's product, wrong when they chose another. Spark checks every deterministic
// answer until the 95 % lower bound of the code's precision clears the bar (99 % by default: about
// 270 confirmed answers in a row). After that, a correction sends that supplier or shop back under
// supervision for Recover clean confirmations, and Spark keeps checking risky answers (see
// resolve.Resolver.Sentinel).
type Autonomy struct {
	Bar        float64        `json:"bar"`
	Recover    int            `json:"recover"`
	Right      int            `json:"right"`
	Wrong      int            `json:"wrong"`
	Demoted    map[string]int `json:"demoted"`
	UnlockedAt int            `json:"unlocked_after_answers"` // 0 while not unlocked
	mu         sync.Mutex
}

// NewAutonomy starts supervised.
func NewAutonomy(bar float64) *Autonomy {
	if bar <= 0 {
		bar = 0.99
	}
	return &Autonomy{Bar: bar, Recover: 30, Demoted: map[string]int{}}
}

// Bound is the 95 % one-sided lower bound of the code's precision so far.
func (a *Autonomy) Bound() float64 {
	n := a.Right + a.Wrong
	if n == 0 {
		return 0
	}
	z, ph, nf := 1.645, float64(a.Right)/float64(n), float64(n)
	return (ph + z*z/(2*nf) - z*math.Sqrt(ph*(1-ph)/nf+z*z/(4*nf*nf))) / (1 + z*z/nf)
}

// Unlocked tells whether the desk's steps have earned the right to answer alone.
func (a *Autonomy) Unlocked() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.UnlockedAt > 0
}

// Supervised is resolve.Resolver.Supervised.
func (a *Autonomy) Supervised(cx resolve.Context) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.UnlockedAt == 0 {
		return "the desk is still proving its steps"
	}
	if a.Demoted[cx.SupplierID+cx.CustomerID] > 0 {
		return "a person corrected this supplier or shop recently"
	}
	return ""
}

// Record learns from the lines of one confirmed order or invoice.
func (a *Autonomy) Record(lines []Line, scope string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, l := range lines {
		right, counted := false, false
		switch {
		case l.Checks["det_product"] != "":
			right, counted = l.ProductID == l.Checks["det_product"], true
		case l.Checks["corrected_from"] != "":
			right, counted = false, true
		case l.Level == resolve.LCode || l.Level == resolve.LAlias || l.Level == resolve.LRules:
			right, counted = true, true
		}
		if !counted {
			continue
		}
		if right {
			a.Right++
			if a.Demoted[scope] > 0 {
				a.Demoted[scope]--
			}
		} else {
			a.Wrong++
			if a.UnlockedAt > 0 {
				a.Demoted[scope] = a.Recover
			}
		}
	}
	if a.UnlockedAt == 0 && a.Bound() >= a.Bar {
		a.UnlockedAt = a.Right + a.Wrong
	}
}
