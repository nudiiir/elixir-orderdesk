// Package resolve turns one mention ("fardou ifri", "SEL 2000ML X6") into one product and pack,
// cheapest level first: normalise → trusted alias → rules → shortlist → Muse Spark picks from the
// shortlist → one question to a person. Every resolution records how it was obtained.
package resolve

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
)

// Mention is one item as the customer said it.
type Mention struct {
	Said string  `json:"said"`           // the customer's own words for the product
	Hint string  `json:"hint,omitempty"` // the reader's best guess in plain words (voice: misheard brands)
	Qty  float64 `json:"qty"`
	Unit string  `json:"unit"` // carton | pack | unit | ""
}

// Levels.
const (
	LAlias   = "alias"
	LRules   = "rules"
	LHistory = "history"
	LAI      = "ai"
	LPerson  = "person"
	LNone    = "none"
)

type Resolution struct {
	Mention
	Norm       string            `json:"norm"`
	ProductID  string            `json:"product_id"`
	Level      string            `json:"level"`
	Used       []string          `json:"used"` // rule ids, "alias:<id>", "match"
	Confidence float64           `json:"confidence"`
	Options    []string          `json:"options"`
	Question   string            `json:"question"`
	Status     string            `json:"status"` // ok | ask | unknown
	Pack       *rules.PackSpec   `json:"pack"`
	Shadow     map[string]string `json:"shadow"` // shadow rule id → product it would give
	AICost     float64           `json:"ai_cost"`
	Why        string            `json:"why,omitempty"`    // what the code made of the line, in words
	Differ     []string          `json:"differ,omitempty"` // ambiguous: what the candidates differ on
	Top        []string          `json:"top,omitempty"`    // ambiguous: the tied candidates
	Unknown    []string          `json:"unknown,omitempty"` // line words no catalogue name has

	profile rules.Profile
	verdict Verdict
}

// Context is who is speaking.
type Context struct {
	CustomerID string
	SupplierID string
	Usual      map[string]int // product id → times ordered recently
}

type Resolver struct {
	Cat    *catalog.Catalog
	Live   *rules.Set
	Shadow []*rules.Rule
	Meta   *meta.Client
}

// Deterministic runs levels 0–2 only (no AI): it is also what the benchmark replays.
func Deterministic(cat *catalog.Catalog, set *rules.Set, m Mention, cx Context) Resolution {
	r := Resolution{Mention: m, Norm: catalog.Normalize(m.Said), Status: "unknown", Level: LNone, Shadow: map[string]string{}}
	if r.Norm == "" {
		return r
	}
	blocked := cat.Blocked(r.Norm, cx.CustomerID, cx.SupplierID)
	out := set.Apply(r.Norm, cx.CustomerID, cx.SupplierID)
	r.Pack = out.Pack
	r.profile = set.Profile(out.Text, cx.CustomerID, cx.SupplierID, false)
	// Level 1: a trusted wording settles it, unless a guard forbids that product.
	if a, ok := cat.Trusted(r.Norm, cx.CustomerID, cx.SupplierID); ok && !out.BlockIDs[a.ProductID] && !blockedByWord(cat.Get(a.ProductID), out.BlockWord) {
		r.ProductID, r.Level, r.Status, r.Confidence = a.ProductID, LAlias, "ok", 1
		r.Used = []string{fmt.Sprintf("alias:%d", a.ID)}
		if len(out.Used) > 0 && out.Pack != nil {
			r.Used = append(r.Used, out.Used...)
		}
		return r
	}
	// Level 2: rules rewrite the words and read the attributes, then the code decides identity
	// (see identity.go): exactly one product, or a precise reason why not.
	exclude := map[string]bool{}
	for id := range out.BlockIDs {
		exclude[id] = true
	}
	for id := range blocked {
		exclude[id] = true
	}
	used := append(append([]string{}, out.Used...), r.profile.Used...)
	if a, ok := cat.Trusted(out.Text, cx.CustomerID, cx.SupplierID); ok && out.Text != r.Norm && !exclude[a.ProductID] {
		r.ProductID, r.Level, r.Status, r.Confidence = a.ProductID, LRules, "ok", 1
		r.Used = append(out.Used, fmt.Sprintf("alias:%d", a.ID))
		return r
	}
	v := Judge(cat, set, r.profile, cx, exclude, out.BlockWord)
	r.verdict, r.Why, r.Differ, r.Unknown = v, v.Why, v.Differ, v.Unknown
	used = dedupe(append(used, v.Decisive...))
	switch v.Kind {
	case "match":
		r.ProductID, r.Level, r.Status, r.Confidence = v.Products[0].ID, LRules, "ok", 1
		r.Used = append(used, "match")
		return r
	case "ambiguous":
		for _, p := range v.Products {
			r.Top = append(r.Top, p.ID)
		}
		for _, d := range out.Defaults { // a default rule picks among the candidates
			for _, p := range v.Products {
				if p.ID == d {
					r.ProductID, r.Level, r.Status, r.Confidence = d, LRules, "ok", 1
					r.Used = used
					return r
				}
			}
		}
		// The shop's habit: of the candidates, it only ever buys one.
		var usual []string
		for _, p := range v.Products {
			if cx.Usual[p.ID] > 0 {
				usual = append(usual, p.ID)
			}
		}
		if len(usual) == 1 {
			r.ProductID, r.Level, r.Status, r.Confidence = usual[0], LRules, "ok", 0.95
			r.Used = append(used, "habit")
			r.Why = "the shop always buys this one of " + strconv.Itoa(len(v.Products))
			return r
		}
	}
	if len(used) > 0 {
		r.Used = used
	}
	return r
}

// askable: the line's words all name catalogue products and several fit; only the shop knows
// which one (a size, a flavour it did not say). A model cannot know it either, so the question
// is asked directly, for free.
func askable(r Resolution) bool {
	return r.verdict.Kind == "ambiguous" && len(r.verdict.Unknown) == 0 && len(r.verdict.Products) >= 2 && len(r.verdict.Products) <= 6
}

func blockedByWord(p *catalog.Product, words []string) bool {
	if p == nil {
		return false
	}
	for _, w := range words {
		found := false
		for _, pw := range p.Words {
			if catalog.WordSim(w, pw) >= 0.85 {
				found = true
			}
		}
		if !found {
			return true
		}
	}
	return false
}

func filterWords(ps []*catalog.Product, words []string) []*catalog.Product {
	if len(words) == 0 {
		return ps
	}
	var out []*catalog.Product
	for _, p := range ps {
		if !blockedByWord(p, words) {
			out = append(out, p)
		}
	}
	return out
}

// Resolve resolves every mention of one message: levels 0–2 per line, then the reader's hint,
// then ONE Muse Spark call for all the lines still open, then questions.
func (rv *Resolver) Resolve(ctx context.Context, ms []Mention, cx Context, ref string) []Resolution {
	res := make([]Resolution, len(ms))
	var open []int
	for i, m := range ms {
		res[i] = Deterministic(rv.Cat, rv.Live, m, cx)
		for _, sr := range rv.Shadow { // shadow rules: what would they have said?
			if !sr.Applies(cx.CustomerID, cx.SupplierID) {
				continue
			}
			alt := Deterministic(rv.Cat, rv.Live.With(sr), m, cx)
			switch {
			case alt.Status == "ok" && (contains(alt.Used, sr.ID) || alt.ProductID != res[i].ProductID):
				res[i].Shadow[sr.ID] = alt.ProductID
			case res[i].Status == "ok" && alt.Status != "ok":
				res[i].Shadow[sr.ID] = "" // it would have stopped this line resolving
			}
		}
		if res[i].Status == "ok" {
			continue
		}
		// The reader (Spark, when it extracted the message) gave a plain-words hint: if the
		// catalogue agrees on one product from it, that line needed the AI, but no extra call.
		if h := strings.TrimSpace(m.Hint); h != "" && catalog.Normalize(h) != res[i].Norm {
			alt := Deterministic(rv.Cat, rv.Live, Mention{Said: h, Qty: m.Qty, Unit: m.Unit}, cx)
			if alt.Status == "ok" && Compatible(rv.Live, res[i].profile, rv.Cat.Get(alt.ProductID), cx) {
				res[i].ProductID, res[i].Level, res[i].Status, res[i].Confidence = alt.ProductID, LAI, "ok", 0.85
				res[i].Used = append([]string{"hint"}, alt.Used...)
				if res[i].Pack == nil {
					res[i].Pack = alt.Pack
				}
				continue
			}
		}
		if askable(res[i]) {
			r := &res[i]
			opts := append([]string{}, r.Top...)
			sort.SliceStable(opts, func(a, b int) bool { return cx.Usual[opts[a]] > cx.Usual[opts[b]] })
			r.Options, r.Status, r.Level = opts, "ask", LPerson
			r.Question = AskText(ms[i].Said, r.verdict, rv.Live)
			r.Used = append(r.Used, "ask")
			continue
		}
		open = append(open, i)
	}
	if len(open) == 0 {
		return res
	}
	// Level 3: a shortlist per open line.
	shorts := map[int][]*catalog.Product{}
	for _, i := range open {
		out := rv.Live.Apply(res[i].Norm, cx.CustomerID, cx.SupplierID)
		words, size := catalog.Tokens(out.Text)
		if hw, hs := catalog.Tokens(catalog.Normalize(ms[i].Hint)); len(hw) > 0 {
			words = append(words, hw...)
			if size == "" {
				size = hs
			}
		}
		exclude := rv.Cat.Blocked(res[i].Norm, cx.CustomerID, cx.SupplierID)
		for id := range out.BlockIDs {
			exclude[id] = true
		}
		// The model only ever picks among products the code would accept: same attributes, and the
		// brand the line names (a model picks a look-alike brand when the right one is missing).
		var short []*catalog.Product
		for _, p := range filterWords(rv.Cat.Shortlist(words, size, rv.Cat.Hints(res[i].Norm, cx.CustomerID, cx.SupplierID), cx.Usual, exclude, 16), out.BlockWord) {
			if Compatible(rv.Live, res[i].profile, p, cx) && brandOK(rv.Cat, rv.Live, res[i].profile, p) && len(short) < 8 {
				short = append(short, p)
			}
		}
		shorts[i] = short
	}
	// Level 4: Muse Spark picks among each shortlist, in one call for the whole message.
	picks, cost, err := rv.pick(ctx, ms, open, shorts, ref)
	share := 0.0
	if len(open) > 0 {
		share = cost / float64(len(open))
	}
	for _, i := range open {
		r := &res[i]
		r.AICost += share
		var ids []string
		for _, p := range shorts[i] {
			ids = append(ids, p.ID)
		}
		pk, ok := picks[i]
		if err == nil && ok && pk.ID != "" && contains(ids, pk.ID) && pk.Confidence >= 0.8 {
			r.ProductID, r.Level, r.Status, r.Confidence = pk.ID, LAI, "ok", pk.Confidence
			r.Used = append(r.Used, "spark")
			continue
		}
		// Level 5: ask. Options: the model's choices first, then the shortlist.
		var opts []string
		if ok {
			for _, o := range append([]string{pk.ID}, pk.Options...) {
				if o != "" && contains(ids, o) && !contains(opts, o) {
					opts = append(opts, o)
				}
			}
			r.Question = pk.Question
		}
		for _, id := range ids {
			if len(opts) >= 4 {
				break
			}
			if !contains(opts, id) {
				opts = append(opts, id)
			}
		}
		r.Options = opts
		if len(opts) > 0 {
			r.Status, r.Level = "ask", LPerson
		}
		if r.Question == "" {
			r.Question = fmt.Sprintf("« %s » : lequel ?", Readable(ms[i].Said))
		}
	}
	return res
}

type pickOut struct {
	ID         string
	Confidence float64
	Options    []string
	Question   string
}

const pickPrompt = `You match items from a shop's order to a distributor's catalogue in Algeria. For each ITEM you get the
customer's words and a numbered list of CANDIDATES. Pick the candidate it means, using only the listed ids.
- If one candidate clearly matches, give its id and a confidence 0.0-1.0.
- If several fit (size, flavour or variant not said), give "" as id, 2 to 4 option ids, and one very short
  question in French naming what differs ("Ifri : 0,5 L ou 1,5 L ?").
- If none fits, give "" and no options.
Return JSON only: {"items":[{"item":1,"id":"C3","confidence":0.9,"options":[],"question":""}]}`

func (rv *Resolver) pick(ctx context.Context, ms []Mention, open []int, shorts map[int][]*catalog.Product, ref string) (map[int]pickOut, float64, error) {
	out := map[int]pickOut{}
	if !rv.Meta.Enabled() {
		return out, 0, meta.ErrNoKey
	}
	var b strings.Builder
	codes := map[int]map[string]string{}
	asked := []int{}
	for _, i := range open {
		if len(shorts[i]) == 0 {
			continue
		}
		asked = append(asked, i)
		codes[i] = map[string]string{}
		fmt.Fprintf(&b, "ITEM %d: \"%s\"", len(asked), ms[i].Said)
		if ms[i].Hint != "" {
			fmt.Fprintf(&b, " (probably: %s)", ms[i].Hint)
		}
		b.WriteString("\nCANDIDATES:\n")
		for k, p := range shorts[i] {
			code := fmt.Sprintf("C%d", k+1)
			codes[i][code] = p.ID
			fmt.Fprintf(&b, "%s | %s\n", code, p.Name)
		}
		b.WriteString("\n")
	}
	if len(asked) == 0 {
		return out, 0, nil
	}
	var resp struct {
		Items []struct {
			Item       int      `json:"item"`
			ID         string   `json:"id"`
			Confidence float64  `json:"confidence"`
			Options    []string `json:"options"`
			Question   string   `json:"question"`
		} `json:"items"`
	}
	cost, err := rv.Meta.JSON(ctx, meta.Spark, "resolve", ref, pickPrompt, b.String(), nil, 1500, &resp)
	if err != nil {
		return out, cost, err
	}
	for _, it := range resp.Items {
		if it.Item < 1 || it.Item > len(asked) {
			continue
		}
		i := asked[it.Item-1]
		po := pickOut{ID: codes[i][it.ID], Confidence: math.Min(math.Max(it.Confidence, 0), 1), Question: strings.TrimSpace(it.Question)}
		for _, o := range it.Options {
			if id := codes[i][o]; id != "" {
				po.Options = append(po.Options, id)
			}
		}
		out[i] = po
	}
	return out, cost, nil
}

// brandOK: when the line names a catalogue brand outright, the product must be of that brand.
func brandOK(cat *catalog.Catalog, set *rules.Set, line rules.Profile, p *catalog.Product) bool {
	brands := map[string]bool{}
	for _, b := range cat.Brands() {
		if bw, _ := catalog.Tokens(catalog.Normalize(b)); len(bw) > 0 && len([]rune(bw[0])) >= 3 {
			brands[bw[0]] = true
		}
	}
	for _, w := range line.Brands {
		brands[w] = true
	}
	pw := productProfile(set, p)
	for _, w := range line.Words {
		if brands[w] && !hasWord(pw.Words, w) {
			return false
		}
	}
	return true
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ChoosePack picks the ERP pack for a resolved line from what the customer said and the pack rules.
func ChoosePack(p *catalog.Product, unit string, spec *rules.PackSpec) (name string, factor int64) {
	if spec != nil {
		unit = spec.Unit
		if spec.Qty > 0 {
			for _, k := range p.Packs {
				if k.FactorMilli == int64(spec.Qty)*1000 {
					return k.Name, k.FactorMilli
				}
			}
		}
	}
	if unit == "unit" || len(p.Packs) == 0 {
		return "", 0
	}
	largest := p.Packs[len(p.Packs)-1]
	if unit == "pack" {
		for _, k := range p.Packs {
			n := strings.ToLower(k.Name)
			if strings.Contains(n, "fard") || strings.Contains(n, "pack") || strings.Contains(n, "paquet") {
				return k.Name, k.FactorMilli
			}
		}
		return p.Packs[0].Name, p.Packs[0].FactorMilli
	}
	if unit == "carton" {
		for _, k := range p.Packs {
			n := strings.ToLower(k.Name)
			if strings.Contains(n, "carton") || strings.Contains(n, "caisse") || strings.Contains(n, "colis") {
				return k.Name, k.FactorMilli
			}
		}
	}
	return largest.Name, largest.FactorMilli
}
