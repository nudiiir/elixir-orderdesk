package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
)

// Val is one value of an attribute. A derived value was guessed by the parser (a bare "1.5" read
// as 1.5 L): it can make two texts agree, never disagree.
type Val struct {
	V       string `json:"v"`
	Derived bool   `json:"derived,omitempty"`
}

// Profile is how the desk reads one text, a shop's line or a catalogue name, with the same
// vocabulary on both sides: the words that name the product, and what the text says about it.
type Profile struct {
	Words  []string            `json:"words"`            // name words, attribute words taken out
	Brands []string            `json:"brands,omitempty"` // words a word rule calls a brand
	Attrs  map[string][]Val    `json:"attrs"`
	From   map[string][]string `json:"-"` // attribute → the vocabulary rules that gave it
	Used   []string            `json:"-"` // vocabulary rules that fired on this text
}

// NumCap is the largest bare number read as part of a name ("Integral 8"); a Matching Profile raises
// it so model numbers ("79505") count too.
var NumCap = 10000.0

// Built-in attributes, read by code on every text:
//
//	size   1500ml, 800g, however it was written; a bare "1.5" in a line is a derived 1.5 L
//	count  x6, x24: the units in the pack the text names (from the ERP packs for a product)
//	num    a bare whole number that is part of the name: "Integral 8", "Célia 2", "teinte 02"
var builtinPolicy = map[string]string{"size": Strict, "count": Soft, "num": Strict}

// PolicyFor is how an attribute decides identity in this context, and the rule that said so
// ("" when it is the built-in default). Attributes the vocabulary defines are strict unless a
// policy rule (or the attribute rule itself) says otherwise.
func (s *Set) PolicyFor(attr, customerID, supplierID string) (string, string) {
	for _, r := range s.rules {
		if r.Type == Policy && r.Action.Attr == attr && r.Applies(customerID, supplierID) {
			return r.Action.Policy, r.ID
		}
	}
	for _, r := range s.rules {
		if r.Type == Attr && r.Action.Attr == attr && r.Action.Policy != "" && r.Applies(customerID, supplierID) {
			return r.Action.Policy, r.ID
		}
	}
	if p, ok := builtinPolicy[attr]; ok {
		return p, ""
	}
	return Strict, ""
}

// VocabKey identifies the vocabulary that reads catalogue names: two sets with the same key
// read every product the same way (used to cache product profiles).
func (s *Set) VocabKey() string {
	var b strings.Builder
	for _, r := range s.rules {
		if (r.Type == Attr || r.Type == Word) && r.Scope.Kind != "customer" && r.Scope.Kind != "supplier" {
			// the content, not the id: every candidate on the benchmark is called "candidate" v1
			fmt.Fprintf(&b, "%s|%s|%s|%s|%s|%s|%s;", r.Type, r.Scope.Kind, r.Pattern, r.Action.Attr, r.Action.Value, r.Action.Policy, r.Action.Role)
		}
	}
	return b.String()
}

// HasVocabulary tells whether any attribute, word or policy rule is in the set.
func (s *Set) HasVocabulary() bool {
	for _, r := range s.rules {
		if r.Type == Attr || r.Type == Word || r.Type == Policy {
			return true
		}
	}
	return false
}

var wordRe = regexp.MustCompile(`[\p{L}\p{N}.]+`)

// Profile reads a normalised text. catalogue=true reads a product name: only the vocabulary that
// holds for everyone applies (a supplier's marks are read on that supplier's invoices only).
func (s *Set) Profile(text, customerID, supplierID string, catalogue bool) Profile {
	p := Profile{Attrs: map[string][]Val{}, From: map[string][]string{}}
	t := text
	for _, r := range s.rules {
		if r.Type != Attr && r.Type != Word {
			continue
		}
		if catalogue {
			if r.Scope.Kind == "customer" || r.Scope.Kind == "supplier" {
				continue
			}
		} else if !r.Applies(customerID, supplierID) {
			continue
		}
		locs := r.re.FindAllStringSubmatchIndex(t, -1)
		if locs == nil {
			continue
		}
		slots := tokenSlots(r.Pattern)
		var b strings.Builder
		last := 0
		for _, m := range locs {
			// only the groups of the alternative that matched take part, in placeholder order
			groups := map[string]string{}
			k := 0
			for g := 2; g+1 < len(m) && k < len(slots); g += 2 {
				if m[g] >= 0 {
					groups[slots[k]] = t[m[g]:m[g+1]]
					k++
				}
			}
			switch r.Type {
			case Attr:
				v := r.Action.Value
				for k, g := range groups {
					v = strings.ReplaceAll(v, k, g)
				}
				if strings.Contains(v, "{match}") { // the matched words themselves: one rule for a family
					v = strings.ReplaceAll(v, "{match}", strings.Join(wordRe.FindAllString(t[m[0]:m[1]], -1), " "))
				}
				if r.Action.Attr == "size" {
					v = catalog.Normalize(v)
				}
				p.add(r.Action.Attr, Val{V: v}, r.ID)
			case Word:
				if r.Action.Role == RoleBrand {
					p.Brands = append(p.Brands, wordRe.FindAllString(t[m[0]:m[1]], -1)...)
					continue // a brand stays a name word
				}
			}
			b.WriteString(t[last:m[0]])
			b.WriteString(" ")
			last = m[1]
		}
		b.WriteString(t[last:])
		t = b.String()
		p.Used = append(p.Used, r.ID)
	}
	for _, w := range wordRe.FindAllString(t, -1) {
		w = strings.Trim(w, ".")
		if w == "" || catalog.StopWords[w] || catalog.UnitWords[w] != "" || catalog.UnitNames[w] {
			continue
		}
		if catalog.IsSize(w) {
			p.add("size", Val{V: w}, "")
			continue
		}
		if catalog.IsCount(w) {
			n, _ := strconv.Atoi(w[1:])
			p.add("count", Val{V: strconv.Itoa(n)}, "")
			continue
		}
		if f, err := strconv.ParseFloat(w, 64); err == nil && catalog.IsNumber(w) {
			switch {
			case strings.Contains(w, "."):
				// "Ifri 1.5": a bare decimal is litres, which is how shops say it. A guess.
				if !catalogue && f > 0 && f <= 10 {
					p.add("size", Val{V: fmt.Sprintf("%.0fml", f*1000), Derived: true}, "")
				}
			case f > 0 && f < NumCap:
				p.add("num", Val{V: strconv.Itoa(int(f))}, "")
			}
			continue
		}
		if _, ok := catalog.NumberWords[w]; ok {
			continue
		}
		if len([]rune(w)) < 2 || in(w, p.Words) {
			continue
		}
		p.Words = append(p.Words, w)
	}
	return p
}

func (p *Profile) add(attr string, v Val, ruleID string) {
	for _, x := range p.Attrs[attr] {
		if x.V == v.V {
			if x.Derived && !v.Derived {
				break // a certain value replaces a guess, below
			}
			return
		}
	}
	list := p.Attrs[attr][:0:0]
	for _, x := range p.Attrs[attr] {
		if x.V != v.V {
			list = append(list, x)
		}
	}
	p.Attrs[attr] = append(list, v)
	if ruleID != "" {
		p.From[attr] = append(p.From[attr], ruleID)
	}
}

// tokenSlots lists the placeholders of a pattern in order: they are the regexp's capture groups.
func tokenSlots(pattern string) []string {
	var out []string
	for _, t := range tokenRe.FindAllString(strings.ToLower(pattern), -1) {
		if t == "{n}" || t == "{size}" || t == "{word}" {
			out = append(out, t)
		}
	}
	return out
}

// Certain is the values of an attribute that were stated, not guessed.
func Certain(vs []Val) []Val {
	var out []Val
	for _, v := range vs {
		if !v.Derived {
			out = append(out, v)
		}
	}
	return out
}
