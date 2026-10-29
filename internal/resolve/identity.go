package resolve

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
)

// Identity, precision first: a wrong match is worse than a question. A missing match costs one
// tap; a wrong one books the wrong product into the ERP without anyone noticing.
//
//  1. Candidates: the products that carry every name word of the line (typos and abbreviations
//     tolerated by catalog.WordSim).
//  2. Veto: each attribute both sides read decides by its policy. A strict attribute (a size)
//     excludes a product whose value differs; a marker (Zéro, Light) excludes a product when only
//     one side names it. A guessed value never excludes.
//  3. Verdict: a match only when one candidate is left, or when one is set apart by a stated
//     attribute the others lack or contradict. More matching words never decide. Otherwise the
//     line is ambiguous, and the verdict says on what (what the shop did not say).
//
// The vocabulary (which words are flavours, variants, containers, brands) is written by the rule
// engineer as rules; this file only applies it.

type Verdict struct {
	Kind     string             // match | ambiguous | none
	Products []*catalog.Product // match: the product; ambiguous: the tied best candidates
	Differ   []string           // ambiguous: what they differ on that the line did not say ("size", "name")
	Unknown  []string           // line words no product has
	Vetoed   string             // none: the attribute that excluded the closest candidates
	Decisive []string           // vocabulary rules that took part in the verdict
	Why      string
}

type cmp struct {
	veto     string // attribute that excludes the product, "" if none
	score    int    // stated agreements minus soft disagreements
	decisive []string
}

// productProfile reads a catalogue name with the vocabulary that holds for everyone. Cached per
// vocabulary: the benchmark builds one set per candidate rule.
var profiles struct {
	sync.Mutex
	byKey map[string]map[string]rules.Profile
	order []string
}

func productProfile(set *rules.Set, p *catalog.Product) rules.Profile {
	key := set.VocabKey()
	pk := p.ID + "\x00" + p.Name + "\x00" + p.Brand
	profiles.Lock()
	if profiles.byKey == nil {
		profiles.byKey = map[string]map[string]rules.Profile{}
	}
	m := profiles.byKey[key]
	if m == nil {
		m = map[string]rules.Profile{}
		profiles.byKey[key] = m
		profiles.order = append(profiles.order, key)
		if len(profiles.order) > 16 {
			delete(profiles.byKey, profiles.order[0])
			profiles.order = profiles.order[1:]
		}
	}
	if pr, ok := m[pk]; ok {
		profiles.Unlock()
		return pr
	}
	profiles.Unlock()
	pr := set.Profile(catalog.NormalizeName(p.Name+" "+p.Brand), "", "", true)
	for _, k := range p.Packs {
		if k.FactorMilli >= 2000 && k.FactorMilli%1000 == 0 {
			addVal(&pr, "count", strconv.FormatInt(k.FactorMilli/1000, 10))
		}
	}
	if bw, _ := catalog.Tokens(catalog.NormalizeName(p.Brand)); len(bw) > 0 {
		pr.Brands = append(pr.Brands, bw...)
	}
	profiles.Lock()
	m[pk] = pr
	profiles.Unlock()
	return pr
}

func addVal(p *rules.Profile, attr, v string) {
	for _, x := range p.Attrs[attr] {
		if x.V == v {
			return
		}
	}
	p.Attrs[attr] = append(p.Attrs[attr], rules.Val{V: v})
}

// ProductProfile exposes how the desk reads one catalogue name (for the panel and the engineer).
func ProductProfile(set *rules.Set, p *catalog.Product) rules.Profile { return productProfile(set, p) }

func hasWord(words []string, w string) bool {
	for _, x := range words {
		if catalog.WordSim(w, x) >= 0.85 {
			return true
		}
	}
	return false
}

func valEq(attr string, a, b rules.Val) bool {
	if a.V == b.V {
		return true
	}
	switch attr {
	case "size", "count", "num":
		return false
	}
	return catalog.WordSim(a.V, b.V) >= 0.85
}

func anyEq(attr string, as, bs []rules.Val) bool {
	for _, a := range as {
		for _, b := range bs {
			if valEq(attr, a, b) {
				return true
			}
		}
	}
	return false
}

// compare decides whether a line can be this product, attribute by attribute.
func compare(set *rules.Set, line, prod rules.Profile, cx Context) cmp {
	var c cmp
	seen := map[string]bool{}
	attrs := []string{}
	for a := range line.Attrs {
		attrs = append(attrs, a)
	}
	for a := range prod.Attrs {
		attrs = append(attrs, a)
	}
	sort.Strings(attrs)
	for _, a := range attrs {
		if seen[a] {
			continue
		}
		seen[a] = true
		lv, pv := line.Attrs[a], prod.Attrs[a]
		if a == "num" && len(lv) > 0 {
			// a bare number in the line may be the size written without its unit ("head shoulders 400")
			for _, s := range prod.Attrs["size"] {
				pv = append(pv, rules.Val{V: strconv.Itoa(int(catalog.SizeNumber(s.V))), Derived: true})
			}
		}
		pol, polRule := set.PolicyFor(a, cx.CustomerID, cx.SupplierID)
		lc, pc := rules.Certain(lv), rules.Certain(pv)
		from := append(append([]string{}, line.From[a]...), prod.From[a]...)
		if polRule != "" {
			from = append(from, polRule)
		}
		agree := len(lv) > 0 && len(pv) > 0 && anyEq(a, lv, pv)
		switch pol {
		case rules.Ignore:
			continue
		case rules.Soft:
			if len(lv) > 0 && len(pv) > 0 {
				if agree {
					c.score++
				} else {
					c.score--
				}
				c.decisive = append(c.decisive, from...)
			}
		case rules.Marker:
			if (len(lc) > 0) != (len(pc) > 0) || (len(lc) > 0 && !agree) {
				c.veto, c.decisive = a, from
				return c
			}
			if agree {
				c.score++
				c.decisive = append(c.decisive, from...)
			}
		default: // strict
			switch {
			case len(lc) > 0 && len(pc) > 0 && !agree:
				c.veto, c.decisive = a, from
				return c
			case agree && len(lc)+len(pc) > 0:
				c.score++
				c.decisive = append(c.decisive, from...)
			case len(lv) > 0 && len(pv) > 0:
				c.score-- // a guess that does not fit: never a veto
			}
		}
	}
	return c
}

// Judge finds the product a line names, or says why it cannot.
func Judge(cat *catalog.Catalog, set *rules.Set, line rules.Profile, cx Context, exclude map[string]bool, blockWords []string) Verdict {
	v := Verdict{Kind: "none"}
	line = liftValues(cat, set, line)
	if len(line.Words) == 0 && !hasVocabAttr(line) {
		v.Why = "no product word"
		return v
	}
	type cand struct {
		p    *catalog.Product
		prof rules.Profile
		c    cmp
	}
	var cands []cand
	found := map[string]bool{}
	for _, p := range cat.Products() {
		pr := productProfile(set, p)
		ok := true
		for _, w := range line.Words {
			if hasWord(pr.Words, w) {
				found[w] = true
			} else {
				ok = false
			}
		}
		if !ok || exclude[p.ID] || blockedByWord(p, blockWords) {
			continue
		}
		cands = append(cands, cand{p: p, prof: pr})
	}
	for _, w := range line.Words {
		if !found[w] {
			v.Unknown = append(v.Unknown, w)
		}
	}
	if len(cands) == 0 {
		if len(v.Unknown) > 0 {
			v.Why = "no product has «" + strings.Join(v.Unknown, " ") + "»"
		} else {
			v.Why = "no product has all these words"
		}
		return v
	}
	var survivors []cand
	vetoes := map[string]int{}
	var vetoRules []string
	for _, c := range cands {
		c.c = compare(set, line, c.prof, cx)
		if c.c.veto != "" {
			vetoes[c.c.veto]++
			vetoRules = append(vetoRules, c.c.decisive...)
			continue
		}
		survivors = append(survivors, c)
	}
	if len(survivors) == 0 {
		best := ""
		for a, n := range vetoes {
			if best == "" || n > vetoes[best] || (n == vetoes[best] && a < best) {
				best = a
			}
		}
		v.Vetoed = best
		v.Decisive = dedupe(vetoRules)
		v.Why = fmt.Sprintf("every product with these words has another %s", best)
		return v
	}
	sort.SliceStable(survivors, func(i, j int) bool { return survivors[i].c.score > survivors[j].c.score })
	best := survivors[0].c.score
	var top []cand
	for _, s := range survivors {
		if s.c.score == best {
			top = append(top, s)
		}
	}
	if len(top) == 1 && (len(survivors) == 1 || best > 0) {
		v.Kind, v.Products = "match", []*catalog.Product{top[0].p}
		v.Decisive = dedupe(append(top[0].c.decisive, vetoRules...))
		v.Why = "one product fits"
		if len(survivors) > 1 {
			v.Why = "one product fits what was said"
		}
		return v
	}
	v.Kind = "ambiguous"
	for _, t := range top {
		v.Products = append(v.Products, t.p)
	}
	v.Decisive = dedupe(vetoRules)
	// What the tied candidates differ on.
	attrs := map[string]bool{}
	for _, t := range top {
		for a := range t.prof.Attrs {
			attrs[a] = true
		}
	}
	keys := make([]string, 0, len(attrs))
	for a := range attrs {
		keys = append(keys, a)
	}
	sort.Strings(keys)
	for _, a := range keys {
		if a == "count" {
			continue // the pack, not the product
		}
		first := valKey(top[0].prof.Attrs[a])
		for _, t := range top[1:] {
			if valKey(t.prof.Attrs[a]) != first {
				v.Differ = append(v.Differ, a)
				break
			}
		}
	}
	extra := func(pr rules.Profile) string {
		var out []string
		for _, w := range pr.Words {
			if !hasWord(line.Words, w) {
				out = append(out, w)
			}
		}
		return strings.Join(out, " ")
	}
	e0 := extra(top[0].prof)
	for _, t := range top[1:] {
		if extra(t.prof) != e0 {
			v.Differ = append(v.Differ, "name")
			break
		}
	}
	if len(v.Differ) > 0 {
		v.Why = fmt.Sprintf("%d products fit; not said: %s", len(top), strings.Join(v.Differ, ", "))
	} else {
		v.Why = fmt.Sprintf("%d products fit and the catalogue names do not tell them apart", len(top))
	}
	return v
}

// liftValues: a line word no catalogue name has, but that abbreviates or misspells an attribute
// value the catalogue has ("can" for canette), is read as that value.
func liftValues(cat *catalog.Catalog, set *rules.Set, line rules.Profile) rules.Profile {
	type av struct{ attr, val string }
	var values []av
	seen := map[av]bool{}
	var profs []rules.Profile
	for _, p := range cat.Products() {
		pr := productProfile(set, p)
		profs = append(profs, pr)
		for a, vs := range pr.Attrs {
			if a == "size" || a == "count" || a == "num" {
				continue
			}
			for _, x := range vs {
				if k := (av{a, x.V}); !seen[k] && !strings.Contains(x.V, " ") {
					seen[k] = true
					values = append(values, k)
				}
			}
		}
	}
	if len(values) == 0 {
		return line
	}
	out := line
	out.Words = nil
	out.Attrs = map[string][]rules.Val{}
	for a, vs := range line.Attrs {
		out.Attrs[a] = vs
	}
	for _, w := range line.Words {
		named := false
		for _, pr := range profs {
			if hasWord(pr.Words, w) {
				named = true
				break
			}
		}
		var hit []av
		if !named {
			for _, k := range values {
				if len(line.Attrs[k.attr]) == 0 && catalog.WordSim(w, k.val) >= 0.85 {
					hit = append(hit, k)
				}
			}
		}
		if len(hit) == 1 {
			out.Attrs[hit[0].attr] = append(out.Attrs[hit[0].attr], rules.Val{V: hit[0].val})
			continue
		}
		out.Words = append(out.Words, w)
	}
	return out
}

func hasVocabAttr(p rules.Profile) bool {
	for a, vs := range p.Attrs {
		if a != "size" && a != "count" && a != "num" && len(vs) > 0 {
			return true
		}
	}
	return false
}

func valKey(vs []rules.Val) string {
	var out []string
	for _, x := range rules.Certain(vs) {
		out = append(out, x.V)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func dedupe(list []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, x := range list {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// Compatible tells whether a product can be what the line says, attributes only (used to keep the
// AI's shortlist honest: the model may only pick among products the code would accept).
func Compatible(set *rules.Set, line rules.Profile, p *catalog.Product, cx Context) bool {
	return compare(set, line, productProfile(set, p), cx).veto == ""
}

// AskText writes the question for an ambiguous line from what its candidates differ on.
func AskText(said string, v Verdict, set *rules.Set) string {
	if len(v.Differ) == 1 && v.Differ[0] != "name" {
		a := v.Differ[0]
		var vals []string
		seen := map[string]bool{}
		ps := append([]*catalog.Product{}, v.Products...)
		if a == "size" {
			sort.SliceStable(ps, func(i, j int) bool {
				return catalog.SizeNumber(valKey(productProfile(set, ps[i]).Attrs[a])) < catalog.SizeNumber(valKey(productProfile(set, ps[j]).Attrs[a]))
			})
		}
		for _, p := range ps {
			x := valKey(productProfile(set, p).Attrs[a])
			if x == "" {
				x = "sans"
			} else if a == "size" {
				x = showSize(x)
			}
			if !seen[x] {
				seen[x] = true
				vals = append(vals, x)
			}
		}
		if len(vals) >= 2 && len(vals) <= 6 {
			return fmt.Sprintf("« %s » : %s ou %s ?", said, strings.Join(vals[:len(vals)-1], ", "), vals[len(vals)-1])
		}
	}
	return fmt.Sprintf("« %s » : lequel ?", said)
}

// showSize writes a canonical size the way it is printed on a shelf: 1500ml → 1,5 L, 330ml → 33 cl.
func showSize(s string) string {
	n := catalog.SizeNumber(s)
	comma := func(f float64) string { return strings.Replace(strconv.FormatFloat(f, 'f', -1, 64), ".", ",", 1) }
	switch catalog.SizeFamily(s) {
	case "ml":
		if n >= 1000 {
			return comma(n/1000) + " L"
		}
		if int(n)%10 == 0 {
			return comma(n/10) + " cl"
		}
		return comma(n) + " ml"
	case "g":
		if n >= 1000 {
			return comma(n/1000) + " kg"
		}
		return comma(n) + " g"
	}
	return s
}
