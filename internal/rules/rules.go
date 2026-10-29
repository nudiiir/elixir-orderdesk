// Package rules is the rule language: a short list of rule types, a small pattern syntax,
// an interpreter and a sentence writer. Rules are data, not code: they can only rewrite words,
// set a pack, pick a default product or block a match, so a rule can never break the system,
// change a price or send a message.
package rules

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Rule types.
const (
	Rewrite = "rewrite" // a way of writing becomes the standard one ("2000ml" → "2l" is built in; "1l5" → "1.5l")
	Synonym = "synonym" // a word means a brand, a family or another word ("sel" → "selecto", "hns" → "head shoulders")
	PackT   = "pack"    // a pattern gives the pack ("x{n}$" → carton of n, "fardou" → pack)
	Default = "default" // fills a missing detail from a habit ("ifri" with no size → Ifri 1.5 L, for one shop)
	Guard   = "guard"   // forbids a match ("citron" never matches a product without "citron")
)

var Types = []string{Rewrite, Synonym, PackT, Default, Guard}

type Scope struct {
	Kind string `json:"kind"` // global | tenant | supplier | customer
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Action is the "then" part; only the field of the rule's type is used.
type Action struct {
	Replace   string `json:"replace,omitempty"`    // rewrite, synonym ({n} allowed)
	PackUnit  string `json:"pack_unit,omitempty"`  // pack: carton | pack | unit
	PackQty   string `json:"pack_qty,omitempty"`   // pack: "{n}" or a number; "" = the product's own pack
	Product   string `json:"product,omitempty"`    // default: product id
	BlockWord string `json:"block_word,omitempty"` // guard: products whose name lacks this word are excluded
	Block     string `json:"block,omitempty"`      // guard: this product id is excluded
}

type Rule struct {
	ID         string          `json:"id"`
	Version    int             `json:"version"`
	Type       string          `json:"type"`
	Scope      Scope           `json:"scope"`
	Pattern    string          `json:"pattern"`
	Action     Action          `json:"action"`
	Sentence   string          `json:"sentence"`
	Author     string          `json:"author"`
	Rationale  string          `json:"rationale"`
	InspiredBy []string        `json:"inspired_by"`
	Status     string          `json:"status"`
	Reason     string          `json:"reason"`
	Bench      json.RawMessage `json:"bench"`
	Strikes    int             `json:"strikes"`
	ShadowOK   int             `json:"shadow_agree"`
	Resolved   int             `json:"lines_resolved"`
	AICost     float64         `json:"ai_cost"`
	CreatedAt  string          `json:"created_at"`
	LiveAt     string          `json:"live_at"`

	re *regexp.Regexp
}

// ---- pattern syntax ----------------------------------------------------------------------
//
//   words        literal words, matched on word boundaries:   "fardou"
//   a|b          alternatives:                                 "fardou|fardeau"
//   {n}          a number (captured):                          "x{n}"
//   {size}       a size token such as 1500ml (captured):       "selecto {size}"
//   ^ and $      start and end of the line:                     "x{n}$"
//
// Patterns run on normalised text (see catalog.Normalize).

var tokenRe = regexp.MustCompile(`\{n\}|\{size\}|[^\s{}]+`)

func compile(p string) (*regexp.Regexp, error) {
	p = strings.TrimSpace(strings.ToLower(p))
	if p == "" {
		return nil, fmt.Errorf("empty pattern")
	}
	start, end := strings.HasPrefix(p, "^"), strings.HasSuffix(p, "$")
	p = strings.TrimSuffix(strings.TrimPrefix(p, "^"), "$")
	var parts []string
	for _, field := range strings.Fields(p) {
		var b strings.Builder
		for _, t := range tokenRe.FindAllString(field, -1) {
			switch t {
			case "{n}":
				b.WriteString(`(\d+(?:\.\d+)?)`)
			case "{size}":
				b.WriteString(`(\d+(?:\.\d+)?(?:ml|mg|g))`)
			default:
				alts := strings.Split(t, "|")
				for i := range alts {
					alts[i] = regexp.QuoteMeta(alts[i])
				}
				b.WriteString("(?:" + strings.Join(alts, "|") + ")")
			}
		}
		parts = append(parts, b.String())
	}
	expr := strings.Join(parts, `\s+`)
	if start {
		expr = "^" + expr
	} else {
		expr = `(?:^|[^\p{L}\p{N}])` + expr
	}
	if end {
		expr += "$"
	} else {
		expr += `(?:$|[^\p{L}\p{N}])`
	}
	return regexp.Compile(expr)
}

// Validate checks a rule's shape and compiles its pattern. An invalid rule never goes further.
func (r *Rule) Validate() error {
	ok := false
	for _, t := range Types {
		if r.Type == t {
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("unknown rule type %q", r.Type)
	}
	switch r.Scope.Kind {
	case "global", "tenant", "supplier", "customer":
	default:
		return fmt.Errorf("unknown scope %q", r.Scope.Kind)
	}
	if (r.Scope.Kind == "customer" || r.Scope.Kind == "supplier") && r.Scope.ID == "" {
		return fmt.Errorf("a %s rule needs the %s id", r.Scope.Kind, r.Scope.Kind)
	}
	re, err := compile(r.Pattern)
	if err != nil {
		return err
	}
	r.re = re
	a := r.Action
	switch r.Type {
	case Rewrite, Synonym:
		if strings.TrimSpace(a.Replace) == "" {
			return fmt.Errorf("%s needs action.replace", r.Type)
		}
	case PackT:
		if a.PackUnit != "carton" && a.PackUnit != "pack" && a.PackUnit != "unit" {
			return fmt.Errorf("pack needs action.pack_unit carton|pack|unit")
		}
		if a.PackQty != "" && a.PackQty != "{n}" {
			if _, err := strconv.Atoi(a.PackQty); err != nil {
				return fmt.Errorf("pack_qty must be {n} or a whole number")
			}
		}
	case Default:
		if a.Product == "" {
			return fmt.Errorf("default needs action.product")
		}
	case Guard:
		if a.Block == "" && a.BlockWord == "" {
			return fmt.Errorf("guard needs action.block or action.block_word")
		}
	}
	return nil
}

// Applies tells whether the rule's scope covers this context.
func (r *Rule) Applies(customerID, supplierID string) bool {
	switch r.Scope.Kind {
	case "customer":
		return r.Scope.ID == customerID
	case "supplier":
		return r.Scope.ID == supplierID
	}
	return true
}

func scopeRank(kind string) int {
	switch kind {
	case "customer":
		return 3
	case "supplier":
		return 2
	case "tenant":
		return 1
	}
	return 0
}

// ---- interpreter -----------------------------------------------------------------------

// PackSpec is what pack rules (or the shop's own words) say about the pack.
type PackSpec struct {
	Unit string `json:"unit"`          // carton | pack | unit
	Qty  int    `json:"qty,omitempty"` // units per pack when the text says it ("X6")
}

// Outcome is what the rules made of one line.
type Outcome struct {
	Text      string          // the line after rewrites and synonyms, pack words removed
	Pack      *PackSpec       // set by a pack rule
	Defaults  []string        // product ids proposed by default rules (most specific first)
	BlockIDs  map[string]bool // products excluded by guards
	BlockWord []string        // products lacking one of these words are excluded
	Used      []string        // ids of the rules that fired, in order
}

// Set is an ordered collection of rules ready to run (most specific scope first).
type Set struct{ rules []*Rule }

func NewSet(list []*Rule) *Set {
	s := &Set{}
	for _, r := range list {
		if r.re == nil && r.Validate() != nil {
			continue
		}
		s.rules = append(s.rules, r)
	}
	// Stable: rewrites before synonyms before the rest; within a type, the most specific scope first.
	order := map[string]int{Rewrite: 0, Synonym: 1, PackT: 2, Guard: 3, Default: 4}
	for i := 1; i < len(s.rules); i++ {
		for j := i; j > 0; j-- {
			a, b := s.rules[j-1], s.rules[j]
			if order[a.Type] > order[b.Type] || (order[a.Type] == order[b.Type] && scopeRank(a.Scope.Kind) < scopeRank(b.Scope.Kind)) {
				s.rules[j-1], s.rules[j] = b, a
			}
		}
	}
	return s
}

func (s *Set) Rules() []*Rule { return s.rules }

// With returns a copy of the set plus one more rule (used by the benchmark and shadow mode).
func (s *Set) With(extra *Rule) *Set {
	list := append([]*Rule{}, s.rules...)
	return NewSet(append(list, extra))
}

// Apply runs the rules on a normalised line, in order.
func (s *Set) Apply(norm, customerID, supplierID string) Outcome {
	out := Outcome{Text: norm, BlockIDs: map[string]bool{}}
	for _, r := range s.rules {
		if !r.Applies(customerID, supplierID) {
			continue
		}
		m := r.re.FindStringSubmatchIndex(out.Text)
		if m == nil {
			continue
		}
		groups := []string{}
		for g := 2; g+1 < len(m); g += 2 {
			if m[g] >= 0 {
				groups = append(groups, out.Text[m[g]:m[g+1]])
			}
		}
		fill := func(t string) string {
			if len(groups) > 0 {
				t = strings.ReplaceAll(t, "{n}", groups[0])
				t = strings.ReplaceAll(t, "{size}", groups[len(groups)-1])
			}
			return t
		}
		switch r.Type {
		case Rewrite, Synonym:
			out.Text = strings.Join(strings.Fields(r.re.ReplaceAllStringFunc(out.Text, func(seg string) string {
				// keep the separators the boundary groups swallowed
				lead, trail := "", ""
				if len(seg) > 0 && !isWordByte(seg[0]) {
					lead = seg[:1]
				}
				if len(seg) > 0 && !isWordByte(seg[len(seg)-1]) {
					trail = seg[len(seg)-1:]
				}
				repl := strings.ToLower(r.Action.Replace)
				if sm := r.re.FindStringSubmatch(seg); len(sm) > 1 {
					repl = strings.ReplaceAll(repl, "{n}", sm[1])
					repl = strings.ReplaceAll(repl, "{size}", sm[len(sm)-1])
				}
				return lead + " " + repl + " " + trail
			})), " ")
		case PackT:
			if out.Pack != nil {
				continue // the most specific pack rule already decided
			}
			ps := &PackSpec{Unit: r.Action.PackUnit}
			if q := fill(r.Action.PackQty); q != "" {
				ps.Qty, _ = strconv.Atoi(q)
			}
			out.Pack = ps
			// the pack words are not product words
			out.Text = strings.Join(strings.Fields(out.Text[:m[0]]+" "+out.Text[m[1]:]), " ")
		case Default:
			out.Defaults = append(out.Defaults, r.Action.Product)
		case Guard:
			if r.Action.Block != "" {
				out.BlockIDs[r.Action.Block] = true
			}
			if r.Action.BlockWord != "" {
				out.BlockWord = append(out.BlockWord, strings.ToLower(r.Action.BlockWord))
			}
		}
		out.Used = append(out.Used, r.ID)
	}
	return out
}

func isWordByte(b byte) bool {
	return b >= 0x80 || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// ---- sentences -------------------------------------------------------------------------

// Sentence writes the rule as a person would say it. Generated from the structured form by a
// template, never by the model, so the sentence always says exactly what the rule does.
func (r *Rule) MakeSentence(productName func(string) string) string {
	where := ""
	switch r.Scope.Kind {
	case "customer":
		where = "For " + nameOr(r.Scope.Name, "this customer") + ", "
	case "supplier":
		where = "On invoices from " + nameOr(r.Scope.Name, "this supplier") + ", "
	case "tenant":
		where = "Here, "
	}
	pat := human(r.Pattern)
	a := r.Action
	var s string
	switch r.Type {
	case Rewrite:
		s = fmt.Sprintf("%s is written as \"%s\".", pat, a.Replace)
	case Synonym:
		s = fmt.Sprintf("%s means \"%s\".", pat, a.Replace)
	case PackT:
		unit := map[string]string{"carton": "a carton", "pack": "a pack (fardeau)", "unit": "single units"}[a.PackUnit]
		switch a.PackQty {
		case "":
			s = fmt.Sprintf("%s means %s.", pat, unit)
		case "{n}":
			s = fmt.Sprintf("%s means %s of {n}.", pat, unit)
		default:
			s = fmt.Sprintf("%s means %s of %s.", pat, unit, a.PackQty)
		}
	case Default:
		s = fmt.Sprintf("%s with no size means %s.", pat, productName(a.Product))
	case Guard:
		if a.Block != "" {
			s = fmt.Sprintf("%s never means %s.", pat, productName(a.Block))
		} else {
			s = fmt.Sprintf("%s only matches products whose name contains \"%s\".", pat, a.BlockWord)
		}
	}
	if where != "" {
		return where + lowerFirst(s)
	}
	return upperFirst(s)
}

func human(p string) string {
	p = strings.TrimSpace(p)
	end := strings.HasSuffix(p, "$")
	start := strings.HasPrefix(p, "^")
	p = strings.TrimSuffix(strings.TrimPrefix(p, "^"), "$")
	p = strings.ReplaceAll(p, "|", "\" or \"")
	s := "\"" + p + "\""
	if end {
		s = "a trailing " + s
	} else if start {
		s = "a leading " + s
	}
	return s
}

func nameOr(n, def string) string {
	if strings.TrimSpace(n) == "" {
		return def
	}
	return n
}

func lowerFirst(s string) string {
	if strings.HasPrefix(s, "\"") || s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "\"") || strings.HasPrefix(s, "a ") {
		if strings.HasPrefix(s, "a ") {
			return "A " + s[2:]
		}
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
