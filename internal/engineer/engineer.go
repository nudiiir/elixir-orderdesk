package engineer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

type Engineer struct {
	DB   *store.DB
	Cat  *catalog.Catalog
	Meta *meta.Client

	mu      sync.Mutex
	running bool
}

// Report is what one run did.
type Report struct {
	Trigger   string   `json:"trigger"`
	Residue   int      `json:"residue"`
	Proposed  []string `json:"proposed"`
	Accepted  []string `json:"accepted"`
	Rejected  []string `json:"rejected"`
	Cost      float64  `json:"cost"`
	Model     string   `json:"model"`              // the model of the first session
	Fallback  string   `json:"fallback,omitempty"` // the model of the second session, when the first found nothing
	Summary   string   `json:"summary"`
	Error     string   `json:"error,omitempty"`
}

func (e *Engineer) Running() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// MaybeRun starts a run when enough new residue has accumulated since the last one.
func (e *Engineer) MaybeRun(ctx context.Context) {
	if e.DB.Setting("engineer_auto", "1") != "1" {
		return
	}
	var n int
	e.DB.QueryRow(`SELECT COUNT(*) FROM ground_truth WHERE level IN ('ai','person') AND created_at > ?`,
		e.DB.Setting("engineer_last_run", "")).Scan(&n)
	if n >= e.DB.SettingInt("engineer_every", 50) {
		e.Run(ctx, "auto")
	}
}

const systemPrompt = `You are the rule engineer of Order Desk, a system that matches what shops write or say ("fardou ifri",
"sel 2000ml x6", "شمبوان دوف") and what suppliers print on invoices to one product of a distributor's catalogue.
Trusted aliases and rules resolve lines for free; only what they cannot resolve goes to an AI model or a person.
You write rules. Code applies them, the same way, to the shop's words AND to every catalogue name, and code decides.

HOW THE CODE DECIDES (precision first: a wrong match is worse than a question):
1. Normalise: lower case, no accents, Arabic letters unified, sizes canonical (1,5 L / 150cl / 1l5 -> 1500ml,
   1 kg -> 1000g), pack counts canonical (6x1.5l, X6 -> count 6). Built-in attributes: size, count (soft) and
   num (a bare whole number of the name: "Integral 8", "teinte 02").
2. Read: every attribute rule turns its words into an attribute value and takes them out of the name words.
3. Candidates: products that carry every remaining name word of the line (typos and prefixes tolerated).
4. Veto by policy: strict (default) = both state it and differ -> not this product; marker = stated on one side
   only, or different -> not this product; soft = only scores; ignore. A guessed value (a bare "1.5") never vetoes.
5. Verdict: match only if one candidate is left, or one is set apart by a STATED attribute the others lack or
   contradict. More matching words never decide. Otherwise the line is asked, with the reason ("not said: size").

THE RULE TYPES (patterns run on normalised text):
- attribute {pattern, action.attr, action.value, action.policy?}: words that give an attribute value, e.g.
  "citron|lemon|limon" -> flavour=citron; "zero|light|sans sucre" -> variant=zero with policy marker (a plain
  product does not say "zero"); "canette|can" -> container=canette; "demi ecreme" -> fat=demi. Use it for every
  word that tells two products of one brand apart: flavour, scent, variant, container, fat, colour or shade,
  audience (bebe, kids, men), cut (morceaux). Values must be the same on both sides: write the catalogue's word.
  One rule can cover a family: value "{match}" is the matched word itself, e.g. "citron|orange|pomme|fraise"
  -> flavour={match}. Spellings that differ from the catalogue's word need their own rule ("lemon" -> citron).
- policy {pattern: attribute name, action.policy}: change how an attribute decides, e.g. "variant" -> marker.
  Use marker ONLY for a word that a plain product of the same name and size lacks (Coca-Cola / Coca-Cola Zero);
  container, flavour, fat and the like stay strict, because shops often leave them out. The gate checks this.
- word {pattern, action.role: brand|descriptor|stop}: descriptor = printed or said but never tells products apart
  (e.g. "pet" when no catalogue name says it); stop = chatter, never a product word; brand = a brand name.
- synonym {pattern, action.replace}: an abbreviation or misspelling means a catalogue word: "slkt" -> "selecto",
  "hb" -> "hamoud boualem", "gazeuse" -> "gazeifiee".
- rewrite {pattern, action.replace}: a way of writing becomes the standard one.
- pack {pattern, action.pack_unit: carton|pack|unit, action.pack_qty: "{n}" or a number or ""}: the ordering unit.
- default {pattern, action.product}: when several products fit and the line does not say which, this one
  (scope customer: one shop's habit). Only for a real habit, never to guess a size for everyone.
- guard {pattern, action.block_word or action.block}: forbid a wrong match.
Pattern syntax: words; "a|b" separates whole alternatives ("zero|sans sucre"); "word?" optional; {n} a number;
{size} a size token; {word} any one word; ^ and $ anchors. "c{n}" also matches "c 24".
Scopes: {"kind":"global"} for spellings, vocabulary and abbreviations everyone uses; {"kind":"supplier","id":...}
for marks only one supplier prints; {"kind":"customer","id":...} only for a default (one shop's habit).

READ THE RESIDUE'S "why": "no product has «pet»" -> a word no catalogue name has: synonym (a misspelling),
descriptor (noise) or attribute (if it tells products apart). "N products fit; not said: name" and the right one is
the plain product -> the others carry a variant word: make it an attribute with policy marker. "not said: size" ->
the shop did not say it; nothing to learn for everyone (a customer default only if that shop always means one).
Use read_catalogue to see how the code reads names (words and attributes) before writing vocabulary: an attribute
pattern must not swallow a word that is part of a name ("rouge" in "rouge a levres"), and the gate refuses a rule that
makes two different products read the same.

Work like this: list_residue (or read_catalogue when there is no residue), write candidate rules, run_benchmark on
the ones you doubt, refine what is wrong, loses lines or reads nothing, then submit_rule. Every turn costs money:
pass several rules at once in "items" to run_benchmark and submit_rule, and call several tools in the same turn. The gate (code) accepts a rule only with 0
wrong, 0 conflicts, 0 lost lines and a minimum gain: lines newly resolved, or lines brought closer (progress: the
rule makes the code read more of the line's words, all of them words of the right product). A supplier line often has
several unread words ("comp enro", "pdre susp", "dci : pinaverium"): fix them one rule each, the progress counts;
vocabulary may also pass by reading at least two catalogue names without breaking anything. After run_benchmark,
SUBMIT what passes: a run that only benchmarks teaches nothing. Prefer few general rules. Never propose a rule already in list_rules, rejected included.
Stop after at most %d submitted rules, then answer with a one-line summary in English of what you did.`

func (e *Engineer) tools() []meta.Tool {
	str := map[string]any{"type": "string"}
	ruleSchema := map[string]any{"type": "object", "properties": map[string]any{
		"type":    map[string]any{"type": "string", "enum": rules.Types},
		"scope":   map[string]any{"type": "object", "properties": map[string]any{"kind": str, "id": str}},
		"pattern": str,
		"action": map[string]any{"type": "object", "properties": map[string]any{
			"replace": str, "pack_unit": str, "pack_qty": str, "product": str, "block_word": str, "block": str,
			"attr": str, "value": str, "policy": map[string]any{"type": "string", "enum": rules.Policies},
			"role": map[string]any{"type": "string", "enum": rules.Roles}}},
	}, "required": []string{"type", "scope", "pattern", "action"}}
	return []meta.Tool{
		{Name: "list_residue", Description: "Lines whose correct product is known but that rules and trusted aliases cannot resolve alone.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{"limit": map[string]any{"type": "integer"}}}},
		{Name: "search_catalogue", Description: "Catalogue products whose name or brand contains the query words, with their normalised words.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []string{"query"}}},
		{Name: "read_catalogue", Description: "How the code reads catalogue names today: name words and attributes per product, grouped by brand, plus products it cannot tell apart. Optional query filters by words.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}}},
		{Name: "list_rules", Description: "Existing rules (all statuses) with their sentence and, for rejected ones, the reason.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "run_benchmark", Description: "Test candidate rules on every known line and on the catalogue: newly resolved, wrong, conflicts, lost, names read, twins. Pass one rule, or several in items (each tested alone).",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{"rule": ruleSchema,
				"items": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"rule": ruleSchema}}}}}},
		{Name: "submit_rule", Description: "Send rules to the gate. Accepted rules start in shadow mode. Pass one rule, or several in items.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{"rule": ruleSchema,
				"rationale": map[string]any{"type": "string", "description": "one sentence: the pattern you saw"},
				"inspired_by": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "line ids from list_residue"},
				"items": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"rule": ruleSchema,
					"rationale": map[string]any{"type": "string"}, "inspired_by": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}}}}},
	}
}

// Run executes one rule-engineer session (at most one at a time).
func (e *Engineer) Run(ctx context.Context, trigger string) Report {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return Report{Trigger: trigger, Error: "already running"}
	}
	e.running = true
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.running = false; e.mu.Unlock() }()

	rep := Report{Trigger: trigger}
	live := rules.NewSet(rules.WithStatus(e.DB, "live"))
	truth := GroundTruth(e.DB)
	residue := Residue(e.Cat, live, truth)
	rep.Residue = len(residue)
	e.DB.SetSetting("engineer_last_run", store.Now())
	runID := store.NewID("eng")
	e.DB.Event("engineer.started", runID, "rule-engineer", map[string]any{"trigger": trigger, "residue": len(residue), "benchmark": len(truth)})
	if len(residue) == 0 && live.HasVocabulary() {
		rep.Summary = "Nothing to learn: every known line is already resolved by rules or trusted aliases."
		e.DB.Event("engineer.finished", runID, "rule-engineer", rep)
		return rep
	}
	if !e.Meta.Enabled() {
		rep.Error = "no Meta API key"
		return rep
	}
	start := fmt.Sprintf("There are %d residue lines out of %d known lines. Start.", len(residue), len(truth))
	if !live.HasVocabulary() {
		start += " The catalogue has no vocabulary yet: begin with read_catalogue and write the attribute rules that tell its products apart."
	}
	// Muse Spark first: on the replays it wrote the same rules as Muse Wonder for a quarter of the price.
	// Wonder only when Spark admitted nothing and the residue has grown enough since Wonder last looked.
	rep.Model = e.DB.Setting("engineer_model", meta.Spark)
	first := e.session(ctx, rep.Model, start, residue, truth, live, runID, &rep)
	share(e.DB, rep.Proposed, first)
	if fb := e.DB.Setting("engineer_fallback_model", meta.Wonder); fb != "" && fb != rep.Model && len(rep.Accepted) == 0 && rep.Error == "" &&
		len(residue)-e.DB.SettingInt("engineer_fallback_residue_at", 0) >= e.DB.SettingInt("engineer_fallback_after", 100) {
		rep.Fallback = fb
		e.DB.SetSetting("engineer_fallback_residue_at", fmt.Sprint(len(residue)))
		e.DB.Event("engineer.fallback", runID, "rule-engineer", map[string]any{"from": rep.Model, "to": fb, "residue": len(residue)})
		before := len(rep.Proposed)
		cost := e.session(ctx, fb, start+" A cheaper model looked first and found nothing the gate admits.", residue, truth, live, runID, &rep)
		share(e.DB, rep.Proposed[before:], cost)
	}
	e.DB.Event("engineer.finished", runID, "rule-engineer", rep)
	log.Printf("[engineer] %s: %s%s, residue %d, accepted %v, rejected %v, $%.4f", trigger, rep.Model, map[bool]string{true: " → " + rep.Fallback}[rep.Fallback != ""],
		rep.Residue, rep.Accepted, rep.Rejected, rep.Cost)
	return rep
}

// share spreads a session's cost over the rules it proposed.
func share(db *store.DB, ids []string, cost float64) {
	for _, id := range ids {
		db.Exec(`UPDATE rules SET ai_cost = ? WHERE id = ? AND version = (SELECT MAX(version) FROM rules WHERE id = ?)`, cost/float64(len(ids)), id, id)
	}
}

// session is one conversation of a model with the tools; it returns what it cost.
func (e *Engineer) session(ctx context.Context, model, start string, residue, truth []Truth, live *rules.Set, runID string, rep *Report) float64 {
	maxRules := e.DB.SettingInt("engineer_max_rules", 6)
	msgs := []meta.Message{{Role: "system", Content: fmt.Sprintf(systemPrompt, maxRules)}, {Role: "user", Content: start}}
	submitted, nudged, spent := 0, false, 0.0
	for round := 0; round < e.DB.SettingInt("engineer_rounds", 8); round++ {
		msg, cost, err := e.Meta.Chat(ctx, model, "engineer", runID, msgs, e.tools(), 4000)
		rep.Cost += cost
		spent += cost
		if err != nil {
			rep.Error = err.Error()
			break
		}
		if len(msg.ToolCalls) == 0 && strings.TrimSpace(msg.Text()) == "" && !nudged {
			// an empty turn (the answer budget went into thinking): ask once to act
			nudged = true
			msgs = append(msgs, meta.Message{Role: "user", Content: "Continue: call the next tool, or answer with your one-line summary."})
			continue
		}
		msgs = append(msgs, meta.Message{Role: "assistant", Content: msg.Content, ToolCalls: msg.ToolCalls})
		if len(msg.ToolCalls) == 0 {
			rep.Summary = strings.TrimSpace(msg.Text())
			break
		}
		for _, tc := range msg.ToolCalls {
			out := e.call(tc.Function.Name, tc.Function.Arguments, residue, truth, live, runID, rep, &submitted)
			msgs = append(msgs, meta.Message{Role: "tool", ToolCallID: tc.ID, Name: tc.Function.Name, Content: out})
		}
		if submitted >= maxRules {
			break
		}
	}
	return spent
}

func (e *Engineer) call(name, args string, residue, truth []Truth, live *rules.Set, runID string, rep *Report, submitted *int) string {
	log.Printf("[engineer] %s %s", name, clip(args, 300))
	if name == "run_benchmark" || name == "submit_rule" {
		var batch struct {
			Items []json.RawMessage `json:"items"`
		}
		var quoted struct {
			Items string `json:"items"` // a model sometimes sends the list as a JSON string
		}
		if json.Unmarshal([]byte(args), &quoted) == nil && quoted.Items != "" {
			json.Unmarshal([]byte(quoted.Items), &batch.Items)
		} else {
			json.Unmarshal([]byte(args), &batch)
		}
		if len(batch.Items) > 0 {
			var outs []json.RawMessage
			for _, it := range batch.Items {
				if name == "submit_rule" && *submitted >= e.DB.SettingInt("engineer_max_rules", 6) {
					break
				}
				outs = append(outs, json.RawMessage(e.call(name, string(it), residue, truth, live, runID, rep, submitted)))
			}
			return store.JSON(map[string]any{"results": outs})
		}
	}
	var in struct {
		Limit      int        `json:"limit"`
		Query      string     `json:"query"`
		Rule       rules.Rule `json:"rule"`
		Rationale  string     `json:"rationale"`
		InspiredBy []string   `json:"inspired_by"`
	}
	json.Unmarshal([]byte(args), &in)
	switch name {
	case "list_residue":
		n := in.Limit
		if n <= 0 || n > 25 {
			n = 25
		}
		var items []map[string]any
		for i := len(residue) - 1; i >= 0 && len(items) < n; i-- { // most recent first
			t := residue[i]
			p := e.Cat.Get(t.ProductID)
			if p == nil {
				continue
			}
			cx := resolve.Context{CustomerID: t.CustomerID, SupplierID: t.SupplierID}
			r := resolve.Deterministic(e.Cat, live, resolve.Mention{Said: t.Said, Unit: t.Unit, Qty: 1}, cx)
			lp := live.Profile(live.Apply(catalog.Normalize(t.Said), t.CustomerID, t.SupplierID).Text, t.CustomerID, t.SupplierID, false)
			var fits []string
			for _, id := range r.Top {
				if q := e.Cat.Get(id); q != nil {
					fits = append(fits, q.Name)
				}
			}
			got := ""
			if r.Status == "ok" {
				got = e.productName(r.ProductID)
			}
			items = append(items, map[string]any{"line": t.LineID, "source": map[bool]string{true: "supplier invoice", false: "shop order"}[t.Kind == "invoice"],
				"said": t.Said, "read_as": readLine(lp), "why": r.Why, "candidates": fits, "code_gives": got,
				"customer_id": t.CustomerID, "customer": catalog.CustomerName(e.DB.DB, t.CustomerID),
				"supplier_id": t.SupplierID, "supplier": catalog.SupplierName(e.DB.DB, t.SupplierID),
				"correct_product_id": p.ID, "correct_product": p.Name, "correct_read_as": readLine(resolve.ProductProfile(live, p))})
		}
		return store.JSON(map[string]any{"count": len(residue), "items": items})
	case "read_catalogue":
		n := in.Limit
		if n <= 0 || n > 80 {
			n = 60
		}
		words, _ := catalog.Tokens(catalog.Normalize(in.Query))
		byBrand := map[string][]string{}
		seen := map[string][]string{}
		count := 0
		for _, p := range e.Cat.Products() {
			pr := resolve.ProductProfile(live, p)
			k := store.JSON(map[string]any{"w": pr.Words, "a": pr.Attrs})
			seen[k] = append(seen[k], p.Name)
			ok := true
			for _, w := range words {
				found := false
				for _, pw := range p.Words {
					if catalog.WordSim(w, pw) >= 0.85 {
						found = true
					}
				}
				ok = ok && found
			}
			if !ok || count >= n {
				continue
			}
			count++
			b := p.Brand
			if b == "" {
				b = "(no brand)"
			}
			byBrand[b] = append(byBrand[b], p.Name+" → "+readLine(resolve.ProductProfile(live, p)))
		}
		var twins [][]string
		for _, names := range seen {
			if len(names) > 1 {
				twins = append(twins, names)
			}
		}
		return store.JSON(map[string]any{"products": byBrand, "shown": count, "total": len(e.Cat.Products()), "read_the_same": twins})
	case "search_catalogue":
		words, size := catalog.Tokens(catalog.Normalize(in.Query))
		var items []map[string]any
		for _, p := range e.Cat.Shortlist(words, size, nil, nil, nil, 12) {
			items = append(items, map[string]any{"id": p.ID, "name": p.Name, "brand": p.Brand, "words": p.Words, "size": p.Size, "packs": p.Packs})
		}
		return store.JSON(map[string]any{"items": items})
	case "list_rules":
		list, _ := rules.List(e.DB, false)
		var items []map[string]string
		for _, r := range list {
			items = append(items, map[string]string{"id": r.ID, "type": r.Type, "pattern": r.Pattern, "sentence": r.Sentence,
				"status": r.Status, "reason": r.Reason})
		}
		return store.JSON(map[string]any{"items": items})
	case "run_benchmark", "submit_rule":
		r := in.Rule
		r.ID, r.Version = "candidate", 1
		e.fillScope(&r)
		if err := r.Validate(); err != nil {
			return store.JSON(map[string]string{"error": "invalid rule: " + err.Error()})
		}
		if r.Type == rules.Default && e.Cat.Get(r.Action.Product) == nil {
			return store.JSON(map[string]string{"error": "unknown product id " + r.Action.Product})
		}
		b := RunBench(e.Cat, live, &r, truth)
		if name == "run_benchmark" {
			return store.JSON(b)
		}
		*submitted++
		ok, reason := Gate(b, e.DB.SettingInt("gate_min_gain", 2))
		r.ID = rules.NextID(e.DB)
		r.Author, r.Rationale, r.InspiredBy = "rule-engineer", in.Rationale, in.InspiredBy
		r.Sentence = r.MakeSentence(e.productName)
		r.Bench = json.RawMessage(store.JSON(b))
		if ok {
			r.Status = "shadow"
			if e.DB.SettingInt("shadow_agreements", 5) <= 0 {
				r.Status, r.LiveAt = "live", store.Now()
			}
		} else {
			r.Status, r.Reason = "rejected", reason
		}
		if err := rules.Insert(e.DB, &r); err != nil {
			return store.JSON(map[string]string{"error": err.Error()})
		}
		rep.Proposed = append(rep.Proposed, r.ID)
		e.DB.Event("rule.proposed", r.ID, "rule-engineer", map[string]any{"run": runID, "sentence": r.Sentence, "rationale": in.Rationale,
			"inspired_by": len(in.InspiredBy)})
		e.DB.Event("rule.benchmark", r.ID, "system", b)
		if ok {
			rep.Accepted = append(rep.Accepted, r.ID)
			e.DB.Event("rule."+r.Status, r.ID, "system", map[string]string{"gate": reason})
		} else {
			rep.Rejected = append(rep.Rejected, r.ID)
			e.DB.Event("rule.rejected", r.ID, "system", map[string]string{"gate": reason})
		}
		return store.JSON(map[string]any{"id": r.ID, "accepted": ok, "verdict": reason, "status": r.Status})
	}
	return store.JSON(map[string]string{"error": "unknown tool " + name})
}

// readLine writes a reading compactly: "words: ifri eau minerale | size=1500ml count=6".
func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func readLine(pr rules.Profile) string {
	var attrs []string
	for a, vs := range pr.Attrs {
		for _, v := range vs {
			x := a + "=" + v.V
			if v.Derived {
				x += "(guess)"
			}
			attrs = append(attrs, x)
		}
	}
	sort.Strings(attrs)
	return "words: " + strings.Join(pr.Words, " ") + " | " + strings.Join(attrs, " ")
}

func (e *Engineer) productName(id string) string {
	if p := e.Cat.Get(id); p != nil {
		return p.Name
	}
	return id
}

func (e *Engineer) fillScope(r *rules.Rule) {
	if r.Scope.Kind == "" {
		r.Scope.Kind = "global"
	}
	if r.Scope.Kind == "customer" && r.Scope.Name == "" {
		r.Scope.Name = catalog.CustomerName(e.DB.DB, r.Scope.ID)
	}
	if r.Scope.Kind == "supplier" && r.Scope.Name == "" {
		r.Scope.Name = catalog.SupplierName(e.DB.DB, r.Scope.ID)
	}
}

// Submit sends a rule written by a person through the same gate.
func (e *Engineer) Submit(r rules.Rule, author string, skipShadow bool) (*rules.Rule, Bench, error) {
	e.fillScope(&r)
	r.ID, r.Version = "candidate", 1
	if err := r.Validate(); err != nil {
		return nil, Bench{}, err
	}
	live := rules.NewSet(rules.WithStatus(e.DB, "live"))
	b := RunBench(e.Cat, live, &r, GroundTruth(e.DB))
	ok, reason := Gate(b, 0) // a person's rule needs no minimum gain, but never a wrong match
	r.ID = rules.NextID(e.DB)
	r.Author = author
	r.Sentence = r.MakeSentence(e.productName)
	r.Bench = json.RawMessage(store.JSON(b))
	switch {
	case !ok:
		r.Status, r.Reason = "rejected", reason
	case skipShadow:
		r.Status, r.LiveAt = "live", store.Now()
	default:
		r.Status = "shadow"
	}
	if err := rules.Insert(e.DB, &r); err != nil {
		return nil, b, err
	}
	e.DB.Event("rule.proposed", r.ID, author, map[string]any{"sentence": r.Sentence})
	e.DB.Event("rule.benchmark", r.ID, "system", b)
	e.DB.Event("rule."+r.Status, r.ID, "system", map[string]string{"gate": reason})
	return &r, b, nil
}
