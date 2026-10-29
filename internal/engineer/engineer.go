package engineer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
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
	if n >= e.DB.SettingInt("engineer_every", 10) {
		e.Run(ctx, "auto")
	}
}

const systemPrompt = `You are the rule engineer of Order Desk, a system that matches what shops write or say ("fardou ifri",
"sel 2000ml x6", "شمبوان دوف") to a distributor's catalogue. Lines are resolved first by trusted aliases and rules,
for free; only what they cannot resolve goes to an AI model or a person. Your job: look at the RESIDUE (lines
whose correct product is known but that the rules still cannot resolve alone), find patterns, and write rules
that resolve them for free from now on.

The rule language (patterns run on normalised text: lower case, no accents, sizes like 1500ml / 800g):
- rewrite  {pattern, action.replace}: a way of writing becomes the standard one, e.g. "1l5" -> "1.5l".
- synonym  {pattern, action.replace}: a word means a brand or a catalogue word, e.g. "hns" -> "head shoulders",
           "shampoing|shampoo|chompo" -> "shampooing".
- pack     {pattern, action.pack_unit: carton|pack|unit, action.pack_qty: "{n}" or a number or ""}: e.g. "x{n}$".
- default  {pattern, action.product}: when the words fit several products and no size is said, pick this one
           (use scope customer for one shop's habit).
- guard    {pattern, action.block_word or action.block}: forbid wrong matches.
Pattern syntax: literal words, "a|b" alternatives, {n} a number, {size} a size token, ^ and $ anchors.
Scopes: {"kind":"global"} (true for everyone), {"kind":"tenant"}, {"kind":"customer","id":"<customer id>"}.
Lines come from shop orders ("fardou ifri") and from supplier invoices ("SEL 2000ML PET X6"). A pattern that only one
supplier prints (its codes, its pack marks such as X6 or C24) gets {"kind":"supplier","id":"<supplier id>"}.
Otherwise spelling, abbreviation, synonym and pack rules are almost always GLOBAL: the same misspelling from several shops is
one rule. Use a customer scope only for a default rule (one shop's habit). If the gate refuses a rule for a low
gain, broaden its scope or its pattern before giving up.

Work like this: call list_residue, look at groups of similar lines, use search_catalogue to see the catalogue
words, write ONE candidate rule, call run_benchmark on it, refine if it is wrong or resolves nothing, then
submit_rule. The gate (code) accepts a rule only with 0 wrong matches, 0 conflicts and a minimum gain. Prefer
general, simple rules (a synonym that fixes many lines beats a default for one). Never propose a rule that is
already in list_rules, rejected included. Stop after at most 3 submitted rules, then answer with a one-line
summary in English of what you did.`

func (e *Engineer) tools() []meta.Tool {
	ruleSchema := map[string]any{"type": "object", "properties": map[string]any{
		"type":    map[string]any{"type": "string", "enum": rules.Types},
		"scope":   map[string]any{"type": "object", "properties": map[string]any{"kind": map[string]any{"type": "string"}, "id": map[string]any{"type": "string"}}},
		"pattern": map[string]any{"type": "string"},
		"action": map[string]any{"type": "object", "properties": map[string]any{
			"replace": map[string]any{"type": "string"}, "pack_unit": map[string]any{"type": "string"}, "pack_qty": map[string]any{"type": "string"},
			"product": map[string]any{"type": "string"}, "block_word": map[string]any{"type": "string"}, "block": map[string]any{"type": "string"}}},
	}, "required": []string{"type", "scope", "pattern", "action"}}
	return []meta.Tool{
		{Name: "list_residue", Description: "Lines whose correct product is known but that rules and trusted aliases cannot resolve alone.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{"limit": map[string]any{"type": "integer"}}}},
		{Name: "search_catalogue", Description: "Catalogue products whose name or brand contains the query words, with their normalised words.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []string{"query"}}},
		{Name: "list_rules", Description: "Existing rules (all statuses) with their sentence and, for rejected ones, the reason.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "run_benchmark", Description: "Test a candidate rule on every known line: newly resolved, wrong, conflicts, examples.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{"rule": ruleSchema}, "required": []string{"rule"}}},
		{Name: "submit_rule", Description: "Send a tested rule to the gate. Accepted rules start in shadow mode.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{"rule": ruleSchema,
				"rationale": map[string]any{"type": "string", "description": "one sentence: the pattern you saw"},
				"inspired_by": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "line ids from list_residue"}},
				"required": []string{"rule", "rationale"}}},
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
	if len(residue) == 0 {
		rep.Summary = "Nothing to learn: every known line is already resolved by rules or trusted aliases."
		e.DB.Event("engineer.finished", runID, "rule-engineer", rep)
		return rep
	}
	if !e.Meta.Enabled() {
		rep.Error = "no Meta API key"
		return rep
	}
	model := e.DB.Setting("engineer_model", meta.Wonder)
	msgs := []meta.Message{{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("There are %d residue lines out of %d known lines. Start.", len(residue), len(truth))}}
	submitted, nudged := 0, false
	for round := 0; round < e.DB.SettingInt("engineer_rounds", 8); round++ {
		msg, cost, err := e.Meta.Chat(ctx, model, "engineer", runID, msgs, e.tools(), 4000)
		rep.Cost += cost
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
			out := e.call(tc.Function.Name, tc.Function.Arguments, residue, truth, live, runID, &rep, &submitted)
			msgs = append(msgs, meta.Message{Role: "tool", ToolCallID: tc.ID, Name: tc.Function.Name, Content: out})
		}
		if submitted >= 3 {
			break
		}
	}
	// the proposal cost is shared by the rules it produced
	if n := len(rep.Proposed); n > 0 {
		for _, id := range rep.Proposed {
			e.DB.Exec(`UPDATE rules SET ai_cost = ? WHERE id = ? AND version = (SELECT MAX(version) FROM rules WHERE id = ?)`, rep.Cost/float64(n), id, id)
		}
	}
	e.DB.Event("engineer.finished", runID, "rule-engineer", rep)
	log.Printf("[engineer] %s: residue %d, accepted %v, rejected %v, $%.4f", trigger, rep.Residue, rep.Accepted, rep.Rejected, rep.Cost)
	return rep
}

func (e *Engineer) call(name, args string, residue, truth []Truth, live *rules.Set, runID string, rep *Report, submitted *int) string {
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
		var items []map[string]string
		for i := len(residue) - 1; i >= 0 && len(items) < n; i-- { // most recent first
			t := residue[i]
			p := e.Cat.Get(t.ProductID)
			if p == nil {
				continue
			}
			items = append(items, map[string]string{"line": t.LineID, "source": map[bool]string{true: "supplier invoice", false: "shop order"}[t.Kind == "invoice"],
				"said": t.Said, "normalised": t.Norm, "unit": t.Unit,
				"customer_id": t.CustomerID, "customer": catalog.CustomerName(e.DB.DB, t.CustomerID),
				"supplier_id": t.SupplierID, "supplier": catalog.SupplierName(e.DB.DB, t.SupplierID),
				"correct_product_id": p.ID, "correct_product": p.Name, "product_words": strings.Join(p.Words, " "), "size": p.Size})
		}
		return store.JSON(map[string]any{"count": len(residue), "items": items})
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
