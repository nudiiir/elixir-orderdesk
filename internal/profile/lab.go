package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/engineer"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/search"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// Lab is a scratch copy of the catalogue with no aliases and no learned memory: what a profile reads
// on its own. Every proposal is measured here before it touches the desk.
type Lab struct {
	db  *store.DB
	Cat *catalog.Catalog
	dir string
}

// NewLab copies the desk's catalogue into a scratch database.
func NewLab(cat *catalog.Catalog) (*Lab, error) {
	dir, err := os.MkdirTemp("", "orderdesk-lab-")
	if err != nil {
		return nil, err
	}
	db, err := store.Open(filepath.Join(dir, "lab.db"))
	if err != nil {
		return nil, err
	}
	var prods []catalog.Product
	for _, p := range cat.Products() {
		prods = append(prods, catalog.Product{ID: p.ID, Name: p.Name, Brand: p.Brand, Barcode: p.Barcode, Unit: p.Unit, Packs: p.Packs})
	}
	if err := catalog.ReplaceProducts(db, prods); err != nil {
		return nil, err
	}
	return &Lab{db: db, Cat: catalog.New(db), dir: dir}, nil
}

func (l *Lab) Close() {
	l.db.Close()
	os.RemoveAll(l.dir)
}

// Use installs a profile's first pass in the lab (and process-wide: the lab and the desk share it).
func (l *Lab) Use(p *Profile) {
	catalog.Pre, catalog.ExactDigits, rules.NumCap = p.Pre(), true, 1e7
	resolve.ResetProfiles()
	l.Cat.Reload()
}

// with installs a candidate profile's first pass for the duration of fn, then puts the current one back.
func (l *Lab) with(cand, current *Profile, fn func()) {
	catalog.Pre, catalog.ExactDigits = cand.Pre(), true
	resolve.ResetProfiles()
	l.Cat.Reload()
	fn()
	catalog.Pre = current.Pre()
	resolve.ResetProfiles()
	l.Cat.Reload()
}

// Outcome is how one profile does on a set of known lines, steps 0-2 only (no AI, no aliases).
type Outcome struct {
	Right, Wrong, Open int
	per                []string // per line: product id when resolved, "" otherwise
}

func (o Outcome) String() string {
	n := o.Right + o.Wrong + o.Open
	return fmt.Sprintf("%d/%d right with no AI (%s), %d wrong", o.Right, n, pct(o.Right, n), o.Wrong)
}

func pct(a, b int) string {
	if b == 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f %%", 100*float64(a)/float64(b))
}

// Score resolves every known line with the profile's reading and the given rules.
func (l *Lab) Score(set *rules.Set, truth []engineer.Truth) Outcome {
	var o Outcome
	for _, t := range truth {
		r := resolve.Deterministic(l.Cat, set, resolve.Mention{Said: t.Said, Unit: t.Unit, Qty: 1},
			resolve.Context{CustomerID: t.CustomerID, SupplierID: t.SupplierID})
		got := ""
		switch {
		case r.Status == "ok" && r.ProductID == t.ProductID:
			o.Right++
			got = r.ProductID
		case r.Status == "ok":
			o.Wrong++
			got = r.ProductID
		default:
			o.Open++
		}
		o.per = append(o.per, got)
	}
	return o
}

// Diff compares a candidate with the current profile line by line.
type Diff struct {
	Newly, Wrong, Lost int
	Distinct           int // newly resolved, counted once per (supplier, wording): repeats are not evidence
	Examples           []string
}

func (d Diff) String() string {
	return fmt.Sprintf("+%d newly resolved (%d distinct wordings), %d wrong, %d lost", d.Newly, d.Distinct, d.Wrong, d.Lost)
}

func diff(before, after Outcome, truth []engineer.Truth, name func(string) string) Diff {
	var d Diff
	seen := map[string]bool{}
	for i, t := range truth {
		b, a := before.per[i], after.per[i]
		switch {
		case b == t.ProductID && a != t.ProductID:
			d.Lost++
			if len(d.Examples) < 3 {
				d.Examples = append(d.Examples, fmt.Sprintf("lost « %s »", t.Said))
			}
		case a != "" && a != t.ProductID && b != a:
			d.Wrong++
			if len(d.Examples) < 3 {
				d.Examples = append(d.Examples, fmt.Sprintf("« %s » → %s instead of %s", t.Said, name(a), name(t.ProductID)))
			}
		case a == t.ProductID && b != t.ProductID:
			d.Newly++
			if k := t.SupplierID + "|" + t.CustomerID + "|" + strings.ToLower(t.Said); !seen[k] {
				seen[k] = true
				d.Distinct++
			}
			if len(d.Examples) < 3 {
				d.Examples = append(d.Examples, fmt.Sprintf("« %s » → %s", t.Said, name(a)))
			}
		}
	}
	return d
}

// Compare measures a candidate profile against the current one on known lines.
func (l *Lab) Compare(cand, current *Profile, set *rules.Set, truth []engineer.Truth) Diff {
	// "before" is always measured with the current profile installed: after an acceptance the lab
	// must not keep reading with the previous one (that inflated the next proposal's evidence).
	l.Use(current)
	before := l.Score(set, truth)
	var after Outcome
	l.with(cand, current, func() { after = l.Score(set, truth) })
	return diff(before, after, truth, l.name)
}

func (l *Lab) name(id string) string {
	if p := l.Cat.Get(id); p != nil {
		return p.Name
	}
	return id
}

// Twins counts catalogue products that read exactly like another product: words and size.
func (l *Lab) Twins() int {
	seen := map[string]string{}
	n := 0
	for _, p := range l.Cat.Products() {
		w := append([]string{}, p.Words...)
		sort.Strings(w)
		k := strings.Join(w, " ") + "|" + p.Size
		if o, ok := seen[k]; ok && o != p.ID {
			n++
			continue
		}
		seen[k] = p.ID
	}
	return n
}

// CatalogueEffect: how many names a candidate reads differently, and whether it makes new twins.
func (l *Lab) CatalogueEffect(cand, current *Profile) (changed, twinsBefore, twinsAfter int) {
	l.Use(current)
	before := map[string]string{}
	for _, p := range l.Cat.Products() {
		before[p.ID] = strings.Join(p.Words, " ") + "|" + p.Size
	}
	twinsBefore = l.Twins()
	l.with(cand, current, func() {
		for _, p := range l.Cat.Products() {
			if before[p.ID] != strings.Join(p.Words, " ")+"|"+p.Size {
				changed++
			}
		}
		twinsAfter = l.Twins()
	})
	return
}

// Recall measures a search method: how often the right product is in the shortlist step 3 would
// give (recall@8), and the mean reciprocal rank.
func (l *Lab) Recall(set *rules.Set, cfg search.Config, truth []engineer.Truth) (recall, mrr float64) {
	resolve.ResetProfiles()
	rv := &resolve.Resolver{Cat: l.Cat, Live: set, Search: search.Build(l.Cat, cfg)}
	if len(truth) == 0 {
		return 0, 0
	}
	hit := 0
	for _, t := range truth {
		cands := rv.Candidates(resolve.Mention{Said: t.Said, Unit: t.Unit, Qty: 1}, resolve.Context{CustomerID: t.CustomerID, SupplierID: t.SupplierID})
		for k, p := range cands {
			if p.ID == t.ProductID {
				hit++
				mrr += 1 / float64(k+1)
				break
			}
		}
	}
	return float64(hit) / float64(len(truth)), mrr / float64(len(truth))
}

// Split puts the newest quarter of known lines aside: proposals are chosen on the rest and must hold
// on these, which the proposer never saw.
func Split(truth []engineer.Truth) (train, held []engineer.Truth) {
	n := len(truth) * 3 / 4
	return truth[:n], truth[n:]
}

// Live is the desk's live rule set.
func Live(db *store.DB) *rules.Set { return rules.NewSet(rules.WithStatus(db, "live")) }
