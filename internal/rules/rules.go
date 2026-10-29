// Package rules is the rule language: a short list of rule types, a small pattern syntax,
// an interpreter and a sentence writer. Rules are data, not code: they can rewrite words, name
// what a word is (an attribute value, a brand, a word that says nothing), say how an attribute
// decides identity, set a pack, pick a default product or block a match. A rule can never break
// the system, change a price or send a message.
//
// The model writes the vocabulary; the code applies it, the same way, to the shop's words and to
// every catalogue name, then decides identity from it (see resolve).
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
	Attr    = "attribute" // a word gives an attribute value, on both sides ("zero|light" → variant = zero)
	Word    = "word"      // what a word is: a brand, or a word that never tells two products apart
	Policy  = "policy"    // how an attribute decides identity: strict, marker, soft or ignore
)

var Types = []string{Rewrite, Synonym, PackT, Default, Guard, Attr, Word, Policy}

// Policies: how one attribute decides whether two texts can be the same product.
const (
	Strict = "strict" // both state it and the values differ → never the same product (a size)
	Marker = "marker" // stated on one side only, or different → never the same product (Zéro, Light)
	Soft   = "soft"   // agreement helps, disagreement does not exclude (a pack count)
	Ignore = "ignore" // not used for identity
)

var Policies = []string{Strict, Marker, Soft, Ignore}

// Word roles.
const (
	RoleBrand      = "brand"      // the word names a brand: a line naming another brand is never this product
	RoleDescriptor = "descriptor" // the word describes a product but never tells two apart ("minerale", "pet")
	RoleStop       = "stop"       // the word is never part of a product name ("svp", "3afak")
)

var Roles = []string{RoleBrand, RoleDescriptor, RoleStop}

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
	Attr      string `json:"attr,omitempty"`       // attribute, policy: the attribute name (size, count, flavour…)
	Value     string `json:"value,omitempty"`      // attribute: the value ({n}, {size}, {word}, {match} allowed)
	Policy    string `json:"policy,omitempty"`     // policy: strict | marker | soft | ignore
	Role      string `json:"role,omitempty"`       // word: brand | descriptor | stop
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
//   word?        an optional word:                             "demi? ecreme"
//   {n}          a number (captured):                          "x{n}", "c{n}" (also "c 24")
//   {size}       a size token such as 1500ml (captured):       "selecto {size}"
//   {word}       any one word (captured):                      "parfum {word}"
//   ^ and $      start and end of the line:                     "x{n}$"
//
// Inside one word, a literal and a placeholder may be written glued or apart ("c{n}" matches "c24"
// and "c 24"). Patterns run on normalised text (see catalog.Normalize).

var tokenRe = regexp.MustCompile(`\{n\}|\{size\}|\{word\}|[^\s{}]+`)

func compile(p string) (*regexp.Regexp, error) {
	p = strings.TrimSpace(strings.ToLower(p))
	if p == "" {
		return nil, fmt.Errorf("empty pattern")
	}
	start, end := strings.HasPrefix(p, "^"), strings.HasSuffix(p, "$")
	p = strings.TrimSuffix(strings.TrimPrefix(p, "^"), "$")
	// "|" separates whole alternatives: "zero|sans sucre" is "zero" or "sans sucre". Every
	// alternative carries the same placeholders, in the same order.
	var cores []string
	var slots0 string
	for i, alt := range strings.Split(p, "|") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			return nil, fmt.Errorf("empty alternative in %q", p)
		}
		if s := strings.Join(tokenSlots(alt), ","); i == 0 {
			slots0 = s
		} else if s != slots0 {
			return nil, fmt.Errorf("every alternative needs the same placeholders ({n}, {size}, {word})")
		}
		cores = append(cores, phrase(alt))
	}
	expr := "(?:" + strings.Join(cores, "|") + ")"
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

// phrase compiles one alternative: its words in order, separated by spaces.
func phrase(p string) string {
	expr, sep := "", ""
	for _, field := range strings.Fields(p) {
		optional := len(field) > 1 && strings.HasSuffix(field, "?")
		field = strings.TrimSuffix(field, "?")
		var b strings.Builder
		for k, t := range tokenRe.FindAllString(field, -1) {
			if k > 0 {
				b.WriteString(`\s?`)
			}
			switch t {
			case "{n}":
				b.WriteString(`(\d+(?:\.\d+)?)`)
			case "{size}":
				b.WriteString(`(\d+(?:\.\d+)?(?:ml|mg|g))`)
			case "{word}":
				b.WriteString(`(\p{L}[\p{L}\p{N}]*)`)
			default:
				b.WriteString(regexp.QuoteMeta(t))
			}
		}
		switch {
		case optional && sep == "":
			// an optional first word carries its own trailing space
			expr += "(?:" + b.String() + `\s+)?`
			continue
		case optional:
			expr += "(?:" + sep + b.String() + ")?"
		default:
			expr += sep + b.String()
		}
		sep = `\s+`
	}
	return expr
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
	a := r.Action
	if r.Type == Policy {
		// a policy names an attribute, not words
		r.Pattern = strings.TrimSpace(strings.ToLower(r.Pattern))
		if r.Pattern == "" {
			r.Pattern = strings.ToLower(a.Attr)
		}
		if !attrName.MatchString(r.Pattern) {
			return fmt.Errorf("a policy's pattern is an attribute name such as flavour")
		}
		if !in(a.Policy, Policies) {
			return fmt.Errorf("policy needs action.policy strict|marker|soft|ignore")
		}
		r.Action.Attr = r.Pattern
		return nil
	}
	re, err := compile(r.Pattern)
	if err != nil {
		return err
	}
	r.re = re
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
	case Attr:
		r.Action.Attr = strings.ToLower(strings.TrimSpace(a.Attr))
		r.Action.Value = strings.ToLower(strings.TrimSpace(a.Value))
		if !attrName.MatchString(r.Action.Attr) {
			return fmt.Errorf("attribute needs action.attr, a lower-case name such as flavour")
		}
		if r.Action.Value == "" {
			return fmt.Errorf("attribute needs action.value")
		}
		if r.Action.Attr == "size" && !strings.Contains(r.Action.Value, "{size}") && !sizeValue.MatchString(r.Action.Value) {
			return fmt.Errorf("a size value is {size} or a size such as 1500ml")
		}
		if r.Action.Policy != "" && !in(r.Action.Policy, Policies) {
			return fmt.Errorf("unknown policy %q", r.Action.Policy)
		}
	case Word:
		if !in(a.Role, Roles) {
			return fmt.Errorf("word needs action.role brand|descriptor|stop")
		}
	}
	return nil
}

var (
	attrName  = regexp.MustCompile(`^[a-z][a-z_]{1,23}$`)
	sizeValue = regexp.MustCompile(`^\d+(?:\.\d+)?(?:ml|mg|g)$`)
)

func in(s string, list []string) bool {
	for _, x := range list {
		if s == x {
			return true
		}
	}
	return false
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
	// Stable: rewrites before synonyms before the rest; within a type, the most specific scope
	// first; vocabulary rules with the longest pattern first ("demi ecreme" before "ecreme").
	order := map[string]int{Rewrite: 0, Synonym: 1, PackT: 2, Guard: 3, Default: 4, Word: 5, Attr: 6, Policy: 7}
	before := func(a, b *Rule) bool {
		if order[a.Type] != order[b.Type] {
			return order[a.Type] < order[b.Type]
		}
		if scopeRank(a.Scope.Kind) != scopeRank(b.Scope.Kind) {
			return scopeRank(a.Scope.Kind) > scopeRank(b.Scope.Kind)
		}
		if a.Type == Attr || a.Type == Word {
			return len(a.Pattern) > len(b.Pattern)
		}
		return false
	}
	for i := 1; i < len(s.rules); i++ {
		for j := i; j > 0 && before(s.rules[j], s.rules[j-1]); j-- {
			s.rules[j-1], s.rules[j] = s.rules[j], s.rules[j-1]
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
		if !r.Applies(customerID, supplierID) || r.Type == Attr || r.Type == Word || r.Type == Policy {
			continue // vocabulary: read by Profile
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
				var sm []string
				for _, g := range r.re.FindStringSubmatch(seg)[1:] {
					if g != "" {
						sm = append(sm, g)
					}
				}
				if len(sm) > 0 {
					repl = strings.ReplaceAll(repl, "{n}", sm[0])
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
	case Attr:
		s = fmt.Sprintf("%s is the %s \"%s\", in orders, invoices and catalogue names alike.", pat, a.Attr, a.Value)
		if a.Value == "{match}" {
			s = fmt.Sprintf("%s each name a %s, in orders, invoices and catalogue names alike.", strings.ReplaceAll(pat, "\" or \"", "\", \""), a.Attr)
		}
		if a.Policy != "" {
			s = strings.TrimSuffix(s, ".") + "; " + policyClause(a.Attr, a.Policy)
		}
	case Word:
		switch a.Role {
		case RoleBrand:
			s = fmt.Sprintf("%s is a brand: a line naming another brand never means its products.", pat)
		case RoleDescriptor:
			s = fmt.Sprintf("%s describes a product but never tells two products apart.", pat)
		default:
			s = fmt.Sprintf("%s is never part of a product name.", pat)
		}
	case Policy:
		s = upperFirst(policyClause(a.Attr, a.Policy))
	}
	if where != "" {
		return where + lowerFirst(s)
	}
	return upperFirst(s)
}

func policyClause(attr, policy string) string {
	switch policy {
	case Strict:
		return fmt.Sprintf("two texts whose %s differs are never the same product.", attr)
	case Marker:
		return fmt.Sprintf("a %s stated on one side only, or a different one, is never the same product.", attr)
	case Soft:
		return fmt.Sprintf("a different %s makes a match less likely, never impossible.", attr)
	}
	return fmt.Sprintf("the %s is not used to tell products apart.", attr)
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
