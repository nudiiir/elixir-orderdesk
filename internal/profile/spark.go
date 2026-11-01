package profile

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/engineer"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/search"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

func libraryText() string {
	var b strings.Builder
	for _, bl := range Library {
		fmt.Fprintf(&b, "- %s: %s", bl.Name, bl.Description)
		for k, v := range bl.Params {
			fmt.Fprintf(&b, " [param %s: %s]", k, v)
		}
		b.WriteString("\n")
	}
	return b.String()
}

const howItWorks = `HOW THE DESK MATCHES A LINE (what you are configuring)
A supplier invoice line or a shop's message line must become one exact product of the distributor's catalogue.
Blocks you switch on run first, in a fixed order, on the lower-cased text of BOTH the lines and every catalogue name:
lexicon, split_glued, inch, strength, dimensions, units_glue, pack_count, shade. Then a fixed clean-up (accents,
punctuation, sizes such as 1,5 L → 1500ml, 1 kg → 1000g, mg/UI). A word that contains a digit then only ever matches
the very same word (a code, a dimension, a shade: "8x20" never matches "8x25"). Words without digits tolerate small
typos. The judge books a product only when exactly one product carries every word of the line and no stated
attribute contradicts it; otherwise a shortlist of 8 is built (by the search method) and an AI or a person picks.
Precision first: a wrong automatic match is far worse than a question.`

// ---- bootstrap -------------------------------------------------------------------------------

const bootstrapPrompt = `You are the pipeline engineer of Order Desk. A new distributor just connected its catalogue. Read a sample of
its product names and design the first version of its Matching Profile.

` + howItWorks + `

BLOCK LIBRARY (switch on only what this catalogue needs; each block must earn its place):
%s
Return JSON only:
{"industry": "one short label, e.g. hardware and electrical, supermarket / FMCG, cosmetics, auto parts",
 "reading": "3 to 5 sentences: how this catalogue writes its names (word order, sizes, units, packs, codes, abbreviations)
            and what tells two products of one brand apart",
 "blocks": [{"name": "dimensions", "params": {"bare": false}, "why": "..."}],
 "lexicon": [{"from": "disj", "to": "disjoncteur", "why": "suppliers of this industry write disj for the catalogue's disjoncteur"}],
 "attributes": [{"name": "colour", "words": ["bleu","rouge","noir"], "policy": "strict", "why": "..."}],
 "search": {"bm25": 1, "brand_boost": 2, "trigram": 0.5, "wordsim": 0, "why": "..."}}
Rules: lexicon "from" is ONE lower-case word as people write it, "to" is the word the catalogue uses (lower case,
no accents); only abbreviations you are confident this industry uses, at most 15. attributes: only words that tell
products of one brand apart and are written in the catalogue names (lower case, no accents), at most 4 attributes;
policy is "strict" (both say it and differ → different product) or "marker" (only a variant says it, e.g. zero, light).
search weights: bm25 favours rare words, trigram tolerates typos and glued words, wordsim is prefix/typo similarity.`

// Bootstrap designs a desk's first profile from its catalogue alone. Every block Spark proposes is
// checked on the whole catalogue: it may not make two different products read the same.
func Bootstrap(ctx context.Context, mc *meta.Client, cat *catalog.Catalog, db *store.DB, eng *engineer.Engineer) (*Profile, float64, error) {
	var b strings.Builder
	prods := cat.Products()
	fmt.Fprintf(&b, "%d products, %d brands. Sample:\n", len(prods), len(cat.Brands()))
	for _, p := range sample(prods, 220) {
		fmt.Fprintf(&b, "%s | brand %s\n", p.Name, p.Brand)
	}
	var out struct {
		Industry string `json:"industry"`
		Reading  string `json:"reading"`
		Blocks   []struct {
			Name   string          `json:"name"`
			Params map[string]bool `json:"params"`
			Why    string          `json:"why"`
		} `json:"blocks"`
		Lexicon []struct {
			From, To, Why string
		} `json:"lexicon"`
		Attributes []struct {
			Name   string   `json:"name"`
			Words  []string `json:"words"`
			Policy string   `json:"policy"`
			Why    string   `json:"why"`
		} `json:"attributes"`
		Search struct {
			search.Config
			Why string `json:"why"`
		} `json:"search"`
	}
	cost, err := mc.JSON(ctx, meta.Spark, "profile", "bootstrap", fmt.Sprintf(bootstrapPrompt, libraryText()), b.String(), nil, 9000, &out)
	if err != nil {
		var c2 float64
		c2, err = mc.JSON(ctx, meta.Spark, "profile", "bootstrap", fmt.Sprintf(bootstrapPrompt, libraryText()), b.String(), nil, 12000, &out)
		cost += c2
	}
	if err != nil {
		return nil, cost, err
	}
	p := &Profile{Industry: out.Industry, Reading: out.Reading, Blocks: map[string]*BlockState{}}
	empty := &Profile{Blocks: map[string]*BlockState{}}
	empty.Install(cat)
	lab, err := NewLab(cat)
	if err != nil {
		return nil, cost, err
	}
	defer lab.Close()
	cur := empty
	for _, pb := range out.Blocks {
		if blockByName(pb.Name) == nil || pb.Name == "lexicon" {
			continue
		}
		cand := cur.Clone()
		cand.Blocks[pb.Name] = &BlockState{On: true, Params: pb.Params, Why: pb.Why}
		changed, tb, ta := lab.CatalogueEffect(cand, cur)
		ok := ta <= tb
		ev := fmt.Sprintf("reads %d catalogue names differently; products that read alike %d → %d", changed, tb, ta)
		cand.Blocks[pb.Name].Evidence = ev
		p.log("bootstrap", "block", pb.Name+paramText(pb.Params), "muse-spark", ev, ok, pb.Why)
		if ok {
			cur = cand
		}
	}
	for _, e := range out.Lexicon {
		from, to := strings.ToLower(strings.TrimSpace(e.From)), strings.ToLower(strings.TrimSpace(e.To))
		if from == "" || strings.ContainsAny(from, " ") || from == to {
			continue
		}
		if why := lexGuard(from, to, cat); why != "" {
			cur.Lexicon = append(cur.Lexicon, &LexEntry{From: from, To: to, Why: e.Why, Status: "rejected", Evidence: why})
			p.log("bootstrap", "lexicon", fmt.Sprintf("%q → %q", from, to), "muse-spark", why, false, e.Why)
			continue
		}
		// before any invoice, a word can only wait in shadow: it goes live when known lines prove it
		cand := cur.Clone()
		le := &LexEntry{From: from, To: to, Why: e.Why, Status: "live"}
		cand.Lexicon = append(cand.Lexicon, le)
		changed, tb, ta := lab.CatalogueEffect(cand, cur)
		le.Status = "shadow"
		le.Evidence = fmt.Sprintf("proposed before any invoice (reads %d catalogue names differently; products that read alike %d → %d): waits in shadow until known lines prove it", changed, tb, ta)
		if ta > tb {
			le.Status, le.Evidence = "rejected", fmt.Sprintf("would make %d more products read alike", ta-tb)
		}
		p.log("bootstrap", "lexicon", fmt.Sprintf("%q → %q", from, to), "muse-spark", le.Evidence, le.Status != "rejected", e.Why)
		cur.Lexicon = append(cur.Lexicon, le)
	}
	p.Blocks, p.Lexicon = cur.Blocks, cur.Lexicon
	p.Search = out.Search.Config
	if p.Search.BM25+p.Search.Trigram+p.Search.WordSim <= 0 {
		p.Search = search.Classic
	}
	p.Search.Name, p.Search.Note = "Spark's first guess", out.Search.Why
	p.log("bootstrap", "search", p.Search.Describe(), "muse-spark", "not measured yet: no line has a known answer", true, out.Search.Why)
	p.Install(cat)
	// attributes become vocabulary rules, through the rule gate (they start in shadow mode)
	for _, a := range out.Attributes {
		var words []string
		for _, w := range a.Words {
			if n := strings.TrimSpace(catalog.Normalize(w)); n != "" && !strings.Contains(n, " ") {
				words = append(words, n)
			}
		}
		if a.Name == "" || len(words) == 0 {
			continue
		}
		pol := rules.Strict
		if a.Policy == rules.Marker {
			pol = rules.Marker
		}
		r := rules.Rule{Type: rules.Attr, Scope: rules.Scope{Kind: "global"}, Pattern: strings.Join(words, "|"),
			Action: rules.Action{Attr: strings.ToLower(a.Name), Value: "{match}", Policy: pol}, Rationale: a.Why}
		got, bench, err := eng.Submit(r, "muse-spark-profile", false)
		if err != nil {
			p.log("bootstrap", "attribute", a.Name, "muse-spark", err.Error(), false, a.Why)
			continue
		}
		p.log("bootstrap", "attribute", got.Sentence, "muse-spark",
			fmt.Sprintf("reads %d catalogue names, %d pairs made alike → %s", bench.ProductsChanged, bench.Twins, got.Status), got.Status != "rejected", a.Why)
	}
	p.Save(db)
	return p, cost, nil
}

func paramText(p map[string]bool) string {
	var on []string
	for k, v := range p {
		if v {
			on = append(on, k)
		}
	}
	if len(on) == 0 {
		return ""
	}
	sort.Strings(on)
	return " (" + strings.Join(on, ", ") + ")"
}

// sample takes names across brands: a few per brand first, then the rest in catalogue order.
func sample(prods []*catalog.Product, n int) []*catalog.Product {
	byBrand := map[string][]*catalog.Product{}
	var brands []string
	for _, p := range prods {
		if _, ok := byBrand[p.Brand]; !ok {
			brands = append(brands, p.Brand)
		}
		byBrand[p.Brand] = append(byBrand[p.Brand], p)
	}
	var out []*catalog.Product
	for round := 0; len(out) < n && round < 50; round++ {
		added := false
		for _, br := range brands {
			if round < len(byBrand[br]) && len(out) < n {
				out = append(out, byBrand[br][round])
				added = true
			}
		}
		if !added {
			break
		}
	}
	return out
}

// ---- calibration -----------------------------------------------------------------------------

const calibratePrompt = `You are the pipeline engineer of Order Desk. This distributor's desk has been running; people confirmed the right
product of every line so far. Improve its Matching Profile from the evidence below.

` + howItWorks + `

BLOCK LIBRARY:
%s
You may: switch a block on or off or change its params; add lexicon words (how this desk's suppliers abbreviate a
catalogue word: ONE lower-case word → the catalogue's word, or "" to drop a word that never names a product, e.g. a
supplier's "ref" or "art"); propose up to 3 search configurations. The code measures every proposal on lines whose
answer is known and keeps it only if it resolves more lines with zero wrong and zero lost, also on lines you do not see.
Look at the unresolved lines: which way of writing does the catalogue not share? Look at the search misses: what
would rank the right product higher?
Return JSON only:
{"blocks": [{"name": "inch", "on": true, "params": {"bare": true}, "why": "..."}],
 "lexicon": [{"from": "disj", "to": "disjoncteur", "why": "..."}],
 "search": [{"name": "short name", "bm25": 1, "brand_boost": 2, "trigram": 0.5, "wordsim": 0, "why": "..."}],
 "note": "one sentence on what you saw"}`

// Calibration is what one calibration did.
type Calibration struct {
	Stage         string   `json:"stage"`
	Train         int      `json:"lines_seen"`
	Held          int      `json:"lines_unseen"`
	Before        string   `json:"before"`
	After         string   `json:"after"`
	Accepted      int      `json:"accepted"`
	Rejected      int      `json:"rejected"`
	SearchBefore  string   `json:"search_before,omitempty"`
	SearchAfter   string   `json:"search_after,omitempty"`
	Note          string   `json:"note,omitempty"`
	Cost          float64  `json:"cost"`
	EngineerRules []string `json:"engineer_rules,omitempty"`
}

// Calibrate lets Spark improve the profile from the confirmed lines, then runs the rule engineer.
func Calibrate(ctx context.Context, stage string, mc *meta.Client, cat *catalog.Catalog, db *store.DB, eng *engineer.Engineer) (*Profile, Calibration, error) {
	c := Calibration{Stage: stage}
	p := Load(db)
	if p == nil {
		return nil, c, fmt.Errorf("no profile")
	}
	truth := engineer.GroundTruth(db)
	train, held := Split(truth)
	c.Train, c.Held = len(train), len(held)
	if len(train) < 20 {
		return p, c, fmt.Errorf("not enough confirmed lines (%d)", len(train))
	}
	lab, err := NewLab(cat)
	if err != nil {
		return p, c, err
	}
	defer lab.Close()
	lab.Use(p)
	set := Live(db)
	before := lab.Score(set, train)
	c.Before = before.String()
	r8, mrr := lab.Recall(set, p.Search, train)
	c.SearchBefore = fmt.Sprintf("%s: right product in the top 8 for %.1f %% of lines (MRR %.2f)", p.Search.Describe(), 100*r8, mrr)

	// the evidence Spark sees: training lines only
	var b strings.Builder
	fmt.Fprintf(&b, "INDUSTRY: %s\nREADING: %s\nBLOCKS ON: %s\n", p.Industry, p.Reading, strings.Join(p.On(), ", "))
	b.WriteString("LEXICON: ")
	for _, e := range p.Lexicon {
		if e.Status == "live" {
			fmt.Fprintf(&b, "%s→%s; ", e.From, e.To)
		}
	}
	fmt.Fprintf(&b, "\nSEARCH NOW: %s\nRESOLVED WITH NO AI: %s\n\nUNRESOLVED OR WRONG LINES (line → right product, why):\n", c.SearchBefore, before)
	shown := 0
	for _, t := range train {
		if shown >= 45 {
			break
		}
		r := resolve.Deterministic(lab.Cat, set, resolve.Mention{Said: t.Said, Unit: t.Unit, Qty: 1}, resolve.Context{SupplierID: t.SupplierID, CustomerID: t.CustomerID})
		if r.Status == "ok" && r.ProductID == t.ProductID {
			continue
		}
		fmt.Fprintf(&b, "- %q → %s (%s)\n", t.Said, lab.name(t.ProductID), r.Why)
		shown++
	}
	b.WriteString("\nSEARCH MISSES (right product not in the top 8; what came first):\n")
	rv := &resolve.Resolver{Cat: lab.Cat, Live: set, Search: search.Build(lab.Cat, p.Search)}
	shown = 0
	for _, t := range train {
		if shown >= 20 {
			break
		}
		cands := rv.Candidates(resolve.Mention{Said: t.Said, Unit: t.Unit, Qty: 1}, resolve.Context{SupplierID: t.SupplierID, CustomerID: t.CustomerID})
		found := false
		for _, cp := range cands {
			found = found || cp.ID == t.ProductID
		}
		if found {
			continue
		}
		var top []string
		for k, cp := range cands {
			if k < 3 {
				top = append(top, cp.Name)
			}
		}
		fmt.Fprintf(&b, "- %q → %s; top: %s\n", t.Said, lab.name(t.ProductID), strings.Join(top, " / "))
		shown++
	}
	var out struct {
		Blocks []struct {
			Name   string          `json:"name"`
			On     bool            `json:"on"`
			Params map[string]bool `json:"params"`
			Why    string          `json:"why"`
		} `json:"blocks"`
		Lexicon []struct{ From, To, Why string } `json:"lexicon"`
		Search  []struct {
			search.Config
			Why string `json:"why"`
		} `json:"search"`
		Note string `json:"note"`
	}
	// Spark reasons before it answers: a long reasoning can eat the whole budget and leave an empty
	// answer, so the budget is large and an empty answer is asked once more.
	cost, err := mc.JSON(ctx, meta.Spark, "profile", stage, fmt.Sprintf(calibratePrompt, libraryText()), b.String(), nil, 9000, &out)
	if err != nil {
		var c2 float64
		c2, err = mc.JSON(ctx, meta.Spark, "profile", stage, fmt.Sprintf(calibratePrompt, libraryText()), b.String(), nil, 12000, &out)
		cost += c2
	}
	c.Cost += cost
	if err != nil {
		return p, c, err
	}
	c.Note = out.Note
	old := p.Clone()
	// accept measures a candidate: "live" when it resolves at least 2 distinct (supplier, wording) pairs
	// more with zero wrong, zero lost and no new look-alikes, including at least one on lines Spark never
	// saw; "shadow" when the gain is only on seen lines (kept, not used, tested again next time);
	// "rejected" otherwise.
	accept := func(cand *Profile, kind, what, why string) string {
		d := lab.Compare(cand, p, set, train)
		dh := lab.Compare(cand, p, set, held)
		_, tb, ta := lab.CatalogueEffect(cand, p)
		lab.Use(p)
		clean := d.Wrong == 0 && d.Lost == 0 && dh.Wrong == 0 && dh.Lost == 0 && ta <= tb
		verdict := "rejected"
		switch {
		case clean && d.Distinct+dh.Distinct >= 2 && dh.Distinct >= 1:
			verdict = "live"
		case clean && d.Distinct >= 2:
			verdict = "shadow"
		}
		nums := fmt.Sprintf("seen lines: %s; unseen lines: %s", d, dh)
		if ta > tb {
			nums += fmt.Sprintf("; would make %d more products read alike", ta-tb)
		}
		if verdict == "shadow" {
			nums += "; no gain yet on unseen lines: waits in shadow"
		}
		if len(d.Examples) > 0 {
			nums += "; e.g. " + strings.Join(d.Examples, "; ")
		}
		p.log(stage, kind, what, "muse-spark", nums, verdict == "live", why)
		if verdict == "live" {
			c.Accepted++
		} else {
			c.Rejected++
		}
		return verdict
	}
	for _, pb := range out.Blocks {
		if blockByName(pb.Name) == nil || pb.Name == "lexicon" {
			continue
		}
		cur := p.Blocks[pb.Name]
		if cur != nil && cur.On == pb.On && fmt.Sprint(cur.Params) == fmt.Sprint(pb.Params) {
			continue
		}
		cand := p.Clone()
		cand.Blocks[pb.Name] = &BlockState{On: pb.On, Params: pb.Params, Why: pb.Why}
		what := pb.Name + paramText(pb.Params)
		if !pb.On {
			what = pb.Name + " off"
		}
		if accept(cand, "block", what, pb.Why) == "live" {
			cand.Blocks[pb.Name].Evidence = p.Log[len(p.Log)-1].Numbers
			cand.Log = p.Log
			p = cand
		}
	}
	tryLex := func(from, to, why, by string) {
		cand := p.Clone()
		idx := cand.lexIndex(from)
		if idx < 0 {
			cand.Lexicon = append(cand.Lexicon, &LexEntry{From: from, To: to, Why: why})
			idx = len(cand.Lexicon) - 1
		}
		cand.Lexicon[idx].Status = "live"
		v := accept(cand, "lexicon", fmt.Sprintf("%q → %q%s", from, to, by), why)
		ev := p.Log[len(p.Log)-1].Numbers
		cand.Lexicon[idx].Status, cand.Lexicon[idx].Evidence = v, ev
		if v == "live" {
			cand.Log = p.Log
			p = cand
			return
		}
		// not live: record the verdict on the desk's profile without using the word
		if i := p.lexIndex(from); i >= 0 {
			p.Lexicon[i].Status, p.Lexicon[i].Evidence = v, ev
		} else {
			p.Lexicon = append(p.Lexicon, &LexEntry{From: from, To: to, Why: why, Status: v, Evidence: ev})
		}
	}
	for _, e := range out.Lexicon {
		from, to := strings.ToLower(strings.TrimSpace(e.From)), strings.ToLower(strings.TrimSpace(e.To))
		if from == "" || strings.Contains(from, " ") || from == to || p.hasLex(from) {
			continue
		}
		if why := lexGuard(from, to, cat); why != "" {
			p.Lexicon = append(p.Lexicon, &LexEntry{From: from, To: to, Why: e.Why, Status: "rejected", Evidence: why})
			p.log(stage, "lexicon", fmt.Sprintf("%q → %q", from, to), "muse-spark", why, false, e.Why)
			c.Rejected++
			continue
		}
		tryLex(from, to, e.Why, "")
	}
	// words waiting in shadow (proposed at bootstrap, or proven on seen lines only): tested again
	for _, e := range append([]*LexEntry{}, p.Lexicon...) {
		if e.Status == "shadow" {
			tryLex(e.From, e.To, e.Why, " (from shadow)")
		}
	}
	lab.Use(p)
	// search: the library and Spark's proposals, chosen on seen lines, confirmed on unseen ones
	cands := append([]search.Config{}, search.Library...)
	for _, s := range out.Search {
		if s.BM25+s.Trigram+s.WordSim > 0 {
			cfg := s.Config
			if cfg.Name == "" {
				cfg.Name = "Spark proposal"
			}
			cfg.Note = s.Why
			cfg.Name = "Spark: " + cfg.Name
			cands = append(cands, cfg)
		}
	}
	cur := p.Search
	cur.Name = "current: " + cur.Name
	cands = append(cands, cur)
	best, bestR, bestM := cur, -1.0, -1.0
	var table []string
	for _, cfg := range cands {
		r, m := lab.Recall(set, cfg, train)
		table = append(table, fmt.Sprintf("%s [%s]: %.1f %% in top 8, MRR %.2f", cfg.Name, cfg.Describe(), 100*r, m))
		if r > bestR+1e-9 || (math.Abs(r-bestR) < 1e-9 && m > bestM) {
			best, bestR, bestM = cfg, r, m
		}
	}
	hb, _ := lab.Recall(set, best, held)
	hc, _ := lab.Recall(set, p.Search, held)
	ok := hb+1e-9 >= hc
	by := "library"
	if strings.HasPrefix(best.Name, "Spark") {
		by = "muse-spark"
	}
	p.log(stage, "search", best.Name+" ["+best.Describe()+"]", by,
		fmt.Sprintf("%s. Chosen: %.1f %% in top 8 on seen lines; on unseen lines %.1f %% vs %.1f %% for the current method",
			strings.Join(table, "; "), 100*bestR, 100*hb, 100*hc), ok, best.Note)
	if ok {
		best.Name = strings.TrimPrefix(best.Name, "current: ")
		p.Search = best
	}
	// install the new reading; wordings already learned are re-keyed, or the change is undone
	p.Install(cat)
	if moved, err := Rekey(db); err != nil {
		p.log(stage, "rekey", "re-read every known wording", "code", err.Error(), false, "")
		old.Log = p.Log
		p = old
		p.Install(cat)
	} else if moved > 0 {
		p.log(stage, "rekey", "re-read every known wording", "code", fmt.Sprintf("%d alias(es) moved to the new reading, no collision", moved), true, "")
	}
	cat.Reload()
	after := lab.Score(Live(db), train)
	c.After = after.String()
	r8, mrr = lab.Recall(Live(db), p.Search, train)
	c.SearchAfter = fmt.Sprintf("%s: %.1f %% in the top 8 (MRR %.2f)", p.Search.Describe(), 100*r8, mrr)
	p.Save(db)
	// the rule engineer, on top of the new reading
	rep := eng.Run(ctx, stage)
	c.Cost += rep.Cost
	c.EngineerRules = rep.Accepted
	return p, c, nil
}

func (p *Profile) lexIndex(from string) int {
	for i, e := range p.Lexicon {
		if e.From == from {
			return i
		}
	}
	return -1
}

var digitRe = regexp.MustCompile(`\d`)

// lexGuard refuses vocabulary that would erase identity: a word whose digits it drops or changes (a
// code, a size, a grade: codes go to the code table, not the vocabulary), a word catalogue names use
// that it would drop, and a size or pack token. A rewrite that keeps the digits is allowed and measured
// like any other word.
func lexGuard(from, to string, cat *catalog.Catalog) string {
	df, dt := strings.Join(digitRe.FindAllString(from, -1), ""), strings.Join(digitRe.FindAllString(to, -1), "")
	switch {
	case df != "" && to == "":
		return "refused by the digit guard: it would drop a word with digits (a code or a size; codes are learned per supplier in the code table)"
	case df != dt:
		return "refused by the digit guard: it would change the digits of a word"
	case to == "" && cat.HasWord(from):
		return "refused: catalogue names use this word; dropping it would erase a difference between products"
	case catalog.IsCanonical(from):
		return "refused: a size or pack token belongs to the clean-up, not the vocabulary"
	}
	return ""
}

func (p *Profile) hasLex(from string) bool {
	for _, e := range p.Lexicon {
		if e.From == from {
			return true
		}
	}
	return false
}

// Note logs a measurement the caller made (Spark's precision, autonomy) into the profile's log.
func Note(db *store.DB, stage, kind, what, numbers string, ok bool) {
	p := Load(db)
	if p == nil {
		return
	}
	p.log(stage, kind, what, "code", numbers, ok, "")
	p.Version--
	p.Save(db)
}

// Wilson is the one-sided 95 % lower bound of a success rate.
func Wilson(ok, n int) float64 {
	if n == 0 {
		return 0
	}
	z := 1.645
	ph := float64(ok) / float64(n)
	nf := float64(n)
	return (ph + z*z/(2*nf) - z*math.Sqrt(ph*(1-ph)/nf+z*z/(4*nf*nf))) / (1 + z*z/nf)
}
