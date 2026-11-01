// Package profile is the desk's Matching Profile: how this distributor's industry writes its products,
// which first-pass blocks read them, which search method finds the right product, and the log of
// every decision with its evidence.
//
// Muse Spark designs the profile: first from the catalogue alone (Bootstrap), then from lines whose
// answer is known (Calibrate). Spark never writes code and never decides that its own proposal is
// good: it picks and tunes blocks from a fixed library, and the code measures every proposal in a
// scratch lab and keeps it only if it resolves more lines with zero wrong and zero lost, also on lines
// it was not chosen on.
package profile

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/search"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// Profile is one desk's matching pipeline.
type Profile struct {
	Version  int                    `json:"version"`
	Industry string                 `json:"industry"`
	Reading  string                 `json:"reading"` // Spark's description of how this catalogue writes names
	Blocks   map[string]*BlockState `json:"blocks"`
	Lexicon  []*LexEntry            `json:"lexicon"`
	Search   search.Config          `json:"search"`
	Log      []Decision             `json:"log"`
}

// BlockState is one library block as this desk uses it.
type BlockState struct {
	On       bool            `json:"on"`
	Params   map[string]bool `json:"params,omitempty"`
	Why      string          `json:"why"`      // Spark's reason
	Evidence string          `json:"evidence"` // what the code measured
}

// LexEntry is one word of the desk's own vocabulary: how its suppliers or shops abbreviate a word.
// An empty To drops the word (it never names a product: "ref", "art").
type LexEntry struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Why      string `json:"why"`
	Evidence string `json:"evidence"`
	Status   string `json:"status"` // live | rejected
}

// Decision is one entry of the experiment log.
type Decision struct {
	At       string `json:"at"`
	Stage    string `json:"stage"` // bootstrap | calibration N
	Kind     string `json:"kind"`  // block | lexicon | search | threshold | autonomy
	What     string `json:"what"`
	By       string `json:"by"` // muse-spark | library
	Numbers  string `json:"numbers"`
	Accepted bool   `json:"accepted"`
	Why      string `json:"why,omitempty"`
}

func (p *Profile) log(stage, kind, what, by, numbers string, ok bool, why string) {
	p.Log = append(p.Log, Decision{At: time.Now().Format("2006-01-02 15:04"), Stage: stage, Kind: kind, What: what, By: by,
		Numbers: numbers, Accepted: ok, Why: why})
}

// ---- the block library -----------------------------------------------------------------------
//
// Blocks run in this fixed order, on the lower-cased text, before punctuation is stripped. Each
// turns an industry's way of writing a number into one token both sides share; a token with a digit
// then only matches itself (catalog.ExactDigits).

type block struct {
	Name, Description string
	Params            map[string]string // param → what it does
	fn                func(s string, params map[string]bool, lex []*LexEntry) string
}

var tokenRe = regexp.MustCompile(`[\p{L}\p{N}]+`)

var (
	gluedRe     = regexp.MustCompile(`(\p{L}{4,})(\d)`)
	inchQuoted  = regexp.MustCompile(`(\d{1,2})\s*/\s*(\d{1,2})\s*(?:"|''|”|pouces?\b|po\b)`)
	inchWhole   = regexp.MustCompile(`(^|[^\d/])(\d)\s*(?:"|''|”|pouces?\b)`)
	plumbing    = regexp.MustCompile(`(^|[^\d])(15|20|26|33|40|50)\s*/\s*(21|27|34|42|49|60)($|[^\d])`)
	bareFrac    = regexp.MustCompile(`(^|[^\d/.])(1/2|3/4|1/4|3/8|1/8)($|[^\d/])`)
	strengthRe  = regexp.MustCompile(`(\d+(?:[.,]\d+)*)\s*(mg|g|mcg|µg|ug|mui|ui)\s*/\s*(\d+(?:[.,]\d+)*)\s*(mg|g|ml|mcg|µg|ug|mui|ui)?`)
	thousandsRe = regexp.MustCompile(`^\d{1,4}(?:\.\d{3})+$`)
	bareDose    = regexp.MustCompile(`(^|[^\p{L}\p{N}/.])(\d{1,4})\s*/\s*(\d{1,4})($|[^\p{L}\p{N}/.])`)
	dimRe       = regexp.MustCompile(`(^|[^\p{L}\p{N}.])m?\s?(\d{1,3}(?:[.,]\d)?)\s*[x*×]\s*(\d{1,4})(?:\s*[x*×]\s*(\d{1,4}))?`)
	afterUnit   = regexp.MustCompile(`^\s*(?:ml|l\b|lt|cl|g\b|gr|grs|kg|mg|cc|[.,]\d)`)
	unitGlue    = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*(mm²|mm2|mmq|mm|cm|kw|ah|mah|bar|nm|m|a|v|w)($|[^\p{L}\p{N}])`)
	packWord    = regexp.MustCompile(`(^|[^\p{L}\p{N}])(bte|bt|boites?|box|sachets?|crt|ctn|cartons?|cart|colis|fardeau|fd)\s*(?:/|de|x)?\s*(\d{1,4})($|[^\p{L}\p{N}.])`)
	packSlash   = regexp.MustCompile(`(^|[^\p{L}\p{N}])(b|s|c)\s*/\s*(\d{1,4})($|[^\p{L}\p{N}.])`)
	shadeRe     = regexp.MustCompile(`(?:n°|nº|n\s?°|no\.|teinte|#)\s*0*(\d{1,3})`)
)

var fracName = map[string]string{"1/2": "in12", "3/4": "in34", "1/4": "in14", "3/8": "in38", "1/8": "in18"}
var plumbName = map[string]string{"15/21": "in12", "20/27": "in34", "26/34": "in1", "33/42": "in114", "40/49": "in112", "50/60": "in2"}

func dec(s string) string { return strings.Replace(s, ",", ".", 1) }

// units reads a dosage value: "650.000" before UI is the French thousands dot (650000 UI).
func units(s, unit string) float64 {
	if (unit == "ui" || unit == "mui") && thousandsRe.MatchString(s) {
		s = strings.ReplaceAll(s, ".", "")
	}
	v, _ := strconv.ParseFloat(dec(s), 64)
	return v
}

func toMg(v float64, unit string) (float64, string) {
	switch unit {
	case "g":
		return v * 1000, "mg"
	case "mcg", "µg", "ug":
		return v / 1000, "mg"
	case "mui":
		return v * 1e6, "ui"
	}
	return v, unit
}

func fnum(v float64) string { return strconv.FormatFloat(math.Round(v*1e6)/1e6, 'f', -1, 64) }

// Library is every block a profile can use, in the order they run.
var Library = []block{
	{Name: "lexicon", Description: "the desk's own abbreviations, whole words only (\"disj\" → \"disjoncteur\", \"zn\" → \"zinguee\"); an empty target drops a word",
		fn: func(s string, _ map[string]bool, lex []*LexEntry) string {
			m := map[string]string{}
			for _, e := range lex {
				if e.Status == "live" {
					m[e.From] = e.To
				}
			}
			if len(m) == 0 {
				return s
			}
			return tokenRe.ReplaceAllStringFunc(s, func(w string) string {
				if to, ok := m[w]; ok {
					return to
				}
				return w
			})
		}},
	{Name: "split_glued", Description: "cuts a word glued to a number: \"disjoncteur16a\" → \"disjoncteur 16a\", \"venus5l\" → \"venus 5l\" (words of 4+ letters only, so codes like \"gsb13\" stay whole)",
		fn: func(s string, _ map[string]bool, _ []*LexEntry) string { return gluedRe.ReplaceAllString(s, "$1 $2") }},
	{Name: "inch", Description: "inch sizes and the plumbing names for them: 1/2\" and 15/21 → in12, 3/4\" and 20/27 → in34, 1\" and 26/34 → in1",
		Params: map[string]string{"bare": "also read a bare 1/2, 3/4, 1/4 with no inch sign as inches (plumbing catalogues)"},
		fn: func(s string, p map[string]bool, _ []*LexEntry) string {
			s = plumbing.ReplaceAllStringFunc(s, func(m string) string {
				sm := plumbing.FindStringSubmatch(m)
				return sm[1] + plumbName[sm[2]+"/"+sm[3]] + sm[4]
			})
			s = inchQuoted.ReplaceAllStringFunc(s, func(m string) string {
				sm := inchQuoted.FindStringSubmatch(m)
				if n, ok := fracName[sm[1]+"/"+sm[2]]; ok {
					return n + " "
				}
				return "in" + sm[1] + sm[2] + " "
			})
			s = inchWhole.ReplaceAllString(s, "${1}in$2 ")
			if p["bare"] {
				s = bareFrac.ReplaceAllStringFunc(s, func(m string) string {
					sm := bareFrac.FindStringSubmatch(m)
					return sm[1] + fracName[sm[2]] + sm[3]
				})
			}
			return s
		}},
	{Name: "strength", Description: "a dosage with two values becomes one token (values in mg): \"500mg/200mg\", \"0.5 g / 200 mg\" → dose500x200; \"250mg/5ml\" → dose250x5ml",
		Params: map[string]string{"bare": "also read a bare \"5/160\" with no unit as a two-value dosage in mg (catalogues that write dosages that way)"},
		fn: func(s string, p map[string]bool, _ []*LexEntry) string {
			s = strengthRe.ReplaceAllStringFunc(s, func(m string) string {
				sm := strengthRe.FindStringSubmatch(m)
				u2 := sm[4]
				if u2 == "" {
					u2 = sm[2]
				}
				a, c := units(sm[1], sm[2]), units(sm[3], u2)
				// "dose500x200" (mg / mg) and "dose250x5ml" (mg per ml): no unit right after the first
				// number, so the size clean-up that runs later cannot rewrite part of the token.
				a, _ = toMg(a, sm[2])
				c, u2 = toMg(c, u2)
				tail := ""
				if u2 != "mg" && !(u2 == "ui" && (sm[2] == "ui" || sm[2] == "mui")) {
					tail = u2
				}
				return " dose" + fnum(a) + "x" + fnum(c) + tail + " "
			})
			if p["bare"] {
				s = bareDose.ReplaceAllString(s, "$1 dose${2}x${3} $4")
			}
			return s
		}},
	{Name: "dimensions", Description: "a dimension pair or triple becomes one token: \"M8X20\", \"m8 x 20\", \"8x20\" → 8x20; never when a volume or mass follows (\"6x1.5l\" stays a pack)",
		fn: func(s string, _ map[string]bool, _ []*LexEntry) string {
			var b strings.Builder
			last := 0
			for _, ix := range dimRe.FindAllStringSubmatchIndex(s, -1) {
				if afterUnit.MatchString(s[ix[1]:]) {
					continue
				}
				b.WriteString(s[last:ix[0]])
				b.WriteString(s[ix[2]:ix[3]])
				tok := dec(s[ix[4]:ix[5]]) + "x" + s[ix[6]:ix[7]]
				if ix[8] >= 0 {
					tok += "x" + s[ix[8]:ix[9]]
				}
				b.WriteString(tok + " ")
				last = ix[1]
			}
			b.WriteString(s[last:])
			return b.String()
		}},
	{Name: "units_glue", Description: "technical units are glued to their number: \"2,5 mm²\" → 2.5mm2, \"16 A\" → 16a, \"18 V\" → 18v, \"100 m\" → 100m, \"8 mm\" → 8mm (hardware, electrical)",
		fn: func(s string, _ map[string]bool, _ []*LexEntry) string {
			return unitGlue.ReplaceAllStringFunc(s, func(m string) string {
				sm := unitGlue.FindStringSubmatch(m)
				u := sm[2]
				if u == "mm²" || u == "mmq" {
					u = "mm2"
				}
				return dec(sm[1]) + u + sm[3]
			})
		}},
	{Name: "pack_count", Description: "the count printed with a box or carton becomes one pack token: \"BTE 100\", \"BT100\", \"B/30\", \"S/30\", \"C/6\", \"CRT 12\" → x100, x30, x6, x12",
		fn: func(s string, _ map[string]bool, _ []*LexEntry) string {
			s = packWord.ReplaceAllString(s, "$1 x$3 $4")
			return packSlash.ReplaceAllString(s, "$1 x$3 $4")
		}},
	{Name: "shade", Description: "a shade or reference number becomes one token: \"N°02\", \"teinte 2\", \"#2\" → n2 (cosmetics)",
		fn: func(s string, _ map[string]bool, _ []*LexEntry) string { return shadeRe.ReplaceAllString(s, " n$1 ") }},
}

func blockByName(name string) *block {
	for i := range Library {
		if Library[i].Name == name {
			return &Library[i]
		}
	}
	return nil
}

// Pre is the first pass this profile asks for (nil when no block is on).
func (p *Profile) Pre() func(string) string {
	type step struct {
		b      *block
		params map[string]bool
	}
	var steps []step
	for i := range Library {
		bl := &Library[i]
		if bl.Name == "lexicon" {
			if p.liveLexicon() > 0 {
				steps = append(steps, step{bl, nil})
			}
			continue
		}
		if st := p.Blocks[bl.Name]; st != nil && st.On {
			steps = append(steps, step{bl, st.Params})
		}
	}
	if len(steps) == 0 {
		return nil
	}
	lex := p.Lexicon
	return func(s string) string {
		for _, st := range steps {
			s = st.b.fn(s, st.params, lex)
		}
		return s
	}
}

func (p *Profile) liveLexicon() int {
	n := 0
	for _, e := range p.Lexicon {
		if e.Status == "live" {
			n++
		}
	}
	return n
}

// Install makes the catalogue read with this profile: its first pass, exact digits, then a reload so
// every product name is read again.
func (p *Profile) Install(cat *catalog.Catalog) {
	catalog.Pre = p.Pre()
	catalog.ExactDigits = true
	rules.NumCap = 1e7
	resolve.ResetProfiles()
	cat.Reload()
}

// Clone is a deep copy (for experiments).
func (p *Profile) Clone() *Profile {
	var c Profile
	json.Unmarshal([]byte(store.JSON(p)), &c)
	if c.Blocks == nil {
		c.Blocks = map[string]*BlockState{}
	}
	return &c
}

// ---- storage -------------------------------------------------------------------------------

// Load reads the desk's profile (nil when it has none).
func Load(db *store.DB) *Profile {
	raw := db.Setting("matching_profile", "")
	if raw == "" {
		return nil
	}
	var p Profile
	if json.Unmarshal([]byte(raw), &p) != nil {
		return nil
	}
	if p.Blocks == nil {
		p.Blocks = map[string]*BlockState{}
	}
	return &p
}

// Save stores the profile as a new version.
func (p *Profile) Save(db *store.DB) {
	p.Version++
	db.SetSetting("matching_profile", store.JSON(p))
	db.Event("profile.saved", fmt.Sprint(p.Version), "system", map[string]any{"industry": p.Industry, "blocks": p.On(), "lexicon": p.liveLexicon(),
		"search": p.Search.Describe()})
}

// On lists the blocks that are on, in run order.
func (p *Profile) On() []string {
	var out []string
	for _, b := range Library {
		if st := p.Blocks[b.Name]; st != nil && st.On {
			out = append(out, b.Name)
		}
	}
	return out
}

// ---- re-keying ------------------------------------------------------------------------------

// Rekey reads every known wording again with the installed profile, and moves the aliases keyed on
// the old reading to the new one. It refuses (and changes nothing) when two aliases of one scope for
// different products would land on the same key: that change would make a trusted wording ambiguous.
func Rekey(db *store.DB) (moved int, err error) {
	rows, err := db.Query(`SELECT DISTINCT said, norm FROM ground_truth`)
	if err != nil {
		return 0, err
	}
	pairs := map[string]string{}
	for rows.Next() {
		var said, old string
		if rows.Scan(&said, &old) == nil {
			if n := catalog.Normalize(said); n != old && n != "" {
				pairs[old] = n
			}
		}
	}
	rows.Close()
	type key struct{ text, kind, id string }
	owner := map[key]string{}
	arows, err := db.Query(`SELECT text, scope_kind, scope_id, product_id FROM aliases WHERE state = 'trusted'`)
	if err != nil {
		return 0, err
	}
	for arows.Next() {
		var t, k, id, prod string
		if arows.Scan(&t, &k, &id, &prod) == nil {
			if n, ok := pairs[t]; ok {
				t = n
			}
			kk := key{t, k, id}
			if o, seen := owner[kk]; seen && o != prod {
				arows.Close()
				return 0, fmt.Errorf("two trusted wordings for different products would read the same: %q", t)
			}
			owner[kk] = prod
		}
	}
	arows.Close()
	olds := make([]string, 0, len(pairs))
	for o := range pairs {
		olds = append(olds, o)
	}
	sort.Strings(olds)
	for _, old := range olds {
		n := pairs[old]
		// an alias already at the new key (same scope and product) absorbs the old one
		db.Exec(`DELETE FROM aliases WHERE text = ? AND EXISTS (SELECT 1 FROM aliases b WHERE b.text = ? AND b.scope_kind = aliases.scope_kind
			AND b.scope_id = aliases.scope_id AND b.product_id = aliases.product_id)`, old, n)
		res, _ := db.Exec(`UPDATE aliases SET text = ? WHERE text = ?`, n, old)
		if k, _ := res.RowsAffected(); k > 0 {
			moved += int(k)
		}
		db.Exec(`UPDATE ground_truth SET norm = ? WHERE norm = ?`, n, old)
	}
	return moved, nil
}
