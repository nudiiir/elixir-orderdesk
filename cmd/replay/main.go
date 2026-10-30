// replay runs past supplier invoices through Order Desk to measure how it learns.
//
//	replay -in data/replay.json -out ./replay-run -keys ../hackathon-sim/keys.env
//
// The input holds already-read invoices (lines with their right product) and the catalogue.
// Three passes, each through the real desk code:
//
//	before  the test invoices on an empty desk (no aliases, no rules), nothing learned
//	train   the training invoices in order on a second desk: a clerk who knows the answer answers
//	        every question and corrects every wrong line, the invoice is approved and learned
//	        from, and the rule engineer runs every -every invoices
//	after   the test invoices again, on the trained desk, nothing learned
//
// Each training invoice is dated one day after the previous one, so the panel's Learning page
// shows the curve (open the trained desk with orderdesk -data <out>/desk -no-telegram).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/desk"
	"github.com/nudiiir/elixir-orderdesk/internal/engineer"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

type input struct {
	Invoices []struct {
		ID       string            `json:"id"`
		Supplier string            `json:"supplier"`
		Month    string            `json:"month"`
		Split    string            `json:"split"`
		Lines    []desk.ReplayLine `json:"lines"`
	} `json:"invoices"`
	Products []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Brand string `json:"brand"`
	} `json:"products"`
}

type row struct {
	Phase    string `json:"phase"`
	Day      string `json:"day"`
	Invoice  string `json:"invoice"`
	Supplier string `json:"supplier"`
	desk.ReplayResult
	RulesLive int `json:"rules_live"`
	Aliases   int `json:"trusted_aliases"`
}

type engRun struct {
	After  string           `json:"after_invoice"`
	Report engineer.Report  `json:"report"`
	Rules  []map[string]any `json:"rules"`
}

func main() {
	in := flag.String("in", "replay.json", "invoices and catalogue")
	out := flag.String("out", "replay-run", "output directory (replaced)")
	keys := flag.String("keys", "", "keys file with META_MODEL_API_KEY")
	every := flag.Int("every", 10, "run the rule engineer after every N training invoices (0 = never)")
	engModel := flag.String("engineer-model", "", "model for the rule engineer (default: the desk's setting, Muse Wonder)")
	flag.Parse()
	if *keys != "" {
		loadKeys(*keys)
	}
	var data input
	b, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(b, &data); err != nil {
		log.Fatal(err)
	}
	os.RemoveAll(*out)
	ctx := context.Background()

	open := func(name string) (*store.DB, *catalog.Catalog, *desk.Desk, *meta.Client) {
		db, err := store.Open(filepath.Join(*out, name, "orderdesk.db"))
		if err != nil {
			log.Fatal(err)
		}
		var prods []catalog.Product
		for _, p := range data.Products {
			prods = append(prods, catalog.Product{ID: p.ID, Name: p.Name, Brand: p.Brand})
		}
		if err := catalog.ReplaceProducts(db, prods); err != nil {
			log.Fatal(err)
		}
		sup := map[string]bool{}
		var sups []catalog.Customer
		for _, inv := range data.Invoices {
			if !sup[inv.Supplier] {
				sup[inv.Supplier] = true
				sups = append(sups, catalog.Customer{ID: supplierID(inv.Supplier), Name: inv.Supplier})
			}
		}
		catalog.ReplaceSuppliers(db, sups)
		db.SetSetting("ai_budget_day", "25")
		db.SetSetting("engineer_auto", "0")
		db.SetSetting("tenant_name", "Replay — anonymised pharmacy invoices")
		cat := catalog.New(db)
		mc := meta.New(os.Getenv("META_MODEL_API_KEY"), db)
		return db, cat, &desk.Desk{DB: db, Cat: cat, Meta: mc, DataDir: filepath.Join(*out, name)}, mc
	}

	var rows []row
	var runs []engRun
	test := func(d *desk.Desk, db *store.DB, phase string, day0 time.Time) {
		k := 0
		for _, inv := range data.Invoices {
			if inv.Split != "test" {
				continue
			}
			stamp := day0.AddDate(0, 0, k)
			k++
			mark := store.Now()
			r := d.Replay(ctx, supplierID(inv.Supplier), inv.ID, inv.Month+"-01", inv.Lines, false)
			backdate(db, mark, stamp)
			rows = append(rows, row{Phase: phase, Day: stamp.Format("2006-01-02"), Invoice: inv.ID, Supplier: inv.Supplier, ReplayResult: r,
				RulesLive: count(db, `SELECT COUNT(*) FROM rules r WHERE status='live' AND version=(SELECT MAX(version) FROM rules x WHERE x.id=r.id)`),
				Aliases:   count(db, `SELECT COUNT(*) FROM aliases WHERE state='trusted'`)})
			log.Printf("[%s] %s %s: %d lines, %d alias, %d rules, %d ai, %d asked, %d wrong, $%.4f", phase, inv.ID, inv.Supplier,
				r.Lines, r.Alias, r.Rules, r.AI, r.Asked, r.Wrong, r.AICost)
		}
	}

	day0 := time.Date(2026, 9, 1, 10, 0, 0, 0, time.Local)

	// 1. before: the test invoices on an empty desk
	dbB, _, dB, _ := open("before")
	test(dB, dbB, "before", day0)
	dbB.Close()

	// 2. train
	db, cat, d, mc := open("desk")
	eng := &engineer.Engineer{DB: db, Cat: cat, Meta: mc}
	if *engModel != "" {
		db.SetSetting("engineer_model", *engModel)
	}
	n := 0
	for _, inv := range data.Invoices {
		if inv.Split != "train" {
			continue
		}
		stamp := day0.AddDate(0, 0, n)
		n++
		mark := store.Now()
		r := d.Replay(ctx, supplierID(inv.Supplier), inv.ID, inv.Month+"-01", inv.Lines, true)
		backdate(db, mark, stamp)
		rows = append(rows, row{Phase: "train", Day: stamp.Format("2006-01-02"), Invoice: inv.ID, Supplier: inv.Supplier, ReplayResult: r,
			RulesLive: count(db, `SELECT COUNT(*) FROM rules r WHERE status='live' AND version=(SELECT MAX(version) FROM rules x WHERE x.id=r.id)`),
			Aliases:   count(db, `SELECT COUNT(*) FROM aliases WHERE state='trusted'`)})
		log.Printf("[train %d] %s %s: %d lines, %d alias, %d rules, %d ai, %d asked, %d wrong, $%.4f", n, inv.ID, inv.Supplier,
			r.Lines, r.Alias, r.Rules, r.AI, r.Asked, r.Wrong, r.AICost)
		if *every > 0 && n%*every == 0 {
			mark := store.Now()
			rep := eng.Run(ctx, "replay")
			backdate(db, mark, stamp.Add(8*time.Hour))
			runs = append(runs, engRun{After: inv.ID, Report: rep, Rules: ruleList(db)})
			log.Printf("[engineer after %s] accepted %v rejected %v $%.4f %s", inv.ID, rep.Accepted, rep.Rejected, rep.Cost, rep.Error)
		}
	}

	// 3. after: the same test invoices on the trained desk
	test(d, db, "after", day0.AddDate(0, 0, n+1))

	rep := map[string]any{"rows": rows, "engineer": runs, "rules": ruleList(db), "summary": summary(rows)}
	os.WriteFile(filepath.Join(*out, "report.json"), []byte(store.JSON(rep)), 0o644)
	os.WriteFile(filepath.Join(*out, "report.md"), []byte(markdown(rows, runs, ruleList(db))), 0o644)
	fmt.Println(markdown(rows, runs, ruleList(db)))
	db.Close()
}

func supplierID(name string) string { return "S-" + strings.TrimSpace(strings.TrimPrefix(name, "Fournisseur")) }

// backdate moves every row written since mark to the simulated day.
func backdate(db *store.DB, mark string, day time.Time) {
	ts := day.Format("2006-01-02 15:04:05")
	for _, q := range []string{
		`UPDATE messages SET created_at = ? WHERE created_at >= ?`,
		`UPDATE drafts SET created_at = ?1, updated_at = ?1 WHERE updated_at >= ?2`,
		`UPDATE lines SET created_at = ? WHERE created_at >= ?`,
		`UPDATE ground_truth SET created_at = ? WHERE created_at >= ?`,
		`UPDATE events SET at = ? WHERE at >= ?`,
		`UPDATE ai_usage SET at = ? WHERE at >= ?`,
		`UPDATE rules SET created_at = ? WHERE created_at >= ?`,
		`UPDATE rules SET live_at = ? WHERE live_at >= ?`,
		`UPDATE aliases SET first_seen = ? WHERE first_seen >= ?`,
		`UPDATE aliases SET last_seen = ? WHERE last_seen >= ?`,
	} {
		if _, err := db.Exec(q, ts, mark); err != nil {
			log.Printf("backdate: %v", err)
		}
	}
	db.SetSetting("engineer_last_run", ts)
}

func count(db *store.DB, q string) int {
	var n int
	db.QueryRow(q).Scan(&n)
	return n
}

func ruleList(db *store.DB) []map[string]any {
	list, _ := rules.List(db, false)
	var out []map[string]any
	for _, r := range list {
		out = append(out, map[string]any{"id": r.ID, "type": r.Type, "status": r.Status, "sentence": r.Sentence, "reason": r.Reason,
			"lines_resolved": r.Resolved, "strikes": r.Strikes, "shadow_agree": r.ShadowOK})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["id"].(string) < out[j]["id"].(string) })
	return out
}

type agg struct {
	Invoices, Lines, Alias, Rules, AI, Asked, Wrong int
	Cost                                           float64
}

func (a *agg) add(r row) {
	a.Invoices++
	a.Lines += r.Lines
	a.Alias += r.Alias
	a.Rules += r.Rules
	a.AI += r.AI
	a.Asked += r.Asked
	a.Wrong += r.Wrong
	a.Cost += r.AICost
}

func pct(n, d int) string {
	if d == 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f %%", 100*float64(n)/float64(d))
}

func summary(rows []row) map[string]*agg {
	out := map[string]*agg{}
	for _, r := range rows {
		k := r.Phase
		if k == "train" {
			k = fmt.Sprintf("train %02d-%02d", (indexOf(rows, r)/10)*10+1, (indexOf(rows, r)/10)*10+10)
		}
		if out[k] == nil {
			out[k] = &agg{}
		}
		out[k].add(r)
	}
	return out
}

func indexOf(rows []row, r row) int {
	k := 0
	for _, x := range rows {
		if x.Phase != "train" {
			continue
		}
		if x.Invoice == r.Invoice {
			return k
		}
		k++
	}
	return k
}

func markdown(rows []row, runs []engRun, rl []map[string]any) string {
	var b strings.Builder
	s := summary(rows)
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		o := func(k string) string {
			switch {
			case k == "before":
				return "0"
			case k == "after":
				return "9"
			}
			return "5" + k
		}
		return o(keys[i]) < o(keys[j])
	})
	b.WriteString("| phase | invoices | lines | free (alias + rules) | AI pick | asked a person | wrong auto | AI cost | AI cost / 100 lines |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, k := range keys {
		a := s[k]
		per := 0.0
		if a.Lines > 0 {
			per = 100 * a.Cost / float64(a.Lines)
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d (%s) | %d (%s) | %d (%s) | %d | $%.4f | $%.4f |\n", k, a.Invoices, a.Lines,
			a.Alias+a.Rules, pct(a.Alias+a.Rules, a.Lines), a.AI, pct(a.AI, a.Lines), a.Asked, pct(a.Asked, a.Lines), a.Wrong, a.Cost, per)
	}
	b.WriteString("\nRule engineer runs:\n")
	for _, r := range runs {
		fmt.Fprintf(&b, "- after %s: accepted %v, refused %v, $%.4f %s\n", r.After, r.Report.Accepted, r.Report.Rejected, r.Report.Cost, r.Report.Summary)
	}
	b.WriteString("\nRules at the end:\n")
	for _, r := range rl {
		fmt.Fprintf(&b, "- %s [%s] %s (resolved %v, strikes %v)", r["id"], r["status"], r["sentence"], r["lines_resolved"], r["strikes"])
		if r["reason"] != "" {
			fmt.Fprintf(&b, " — %s", r["reason"])
		}
		b.WriteString("\n")
	}
	return b.String()
}

func loadKeys(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && os.Getenv(strings.TrimSpace(k)) == "" {
			os.Setenv(strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`))
		}
	}
}
