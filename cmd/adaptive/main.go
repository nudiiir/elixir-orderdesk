// adaptive replays one distributor's invoices through the adaptive pipeline: Muse Spark designs the
// desk's Matching Profile from its catalogue, checks every line of the first invoices, calibrates the
// profile from the confirmed lines, and the trained desk is then measured on invoices it never saw.
//
//	adaptive -in data/replay-hardware.json -out ./run-hardware -keys keys.env -bar 0.99 -calib 15,30
//
// Training invoices: a clerk who knows the answer answers every question and corrects every wrong line
// (the same learning as an approved invoice). Test invoices: nothing is answered or learned.
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
	"strconv"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/desk"
	"github.com/nudiiir/elixir-orderdesk/internal/engineer"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/profile"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/search"
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
		Code  string `json:"code"`
	} `json:"products"`
}

type row struct {
	Phase    string `json:"phase"`
	Invoice  string `json:"invoice"`
	Supplier string `json:"supplier"`
	desk.ReplayResult
}

func main() {
	in := flag.String("in", "replay.json", "invoices and catalogue")
	out := flag.String("out", "adaptive-run", "output directory (replaced)")
	keys := flag.String("keys", "", "keys file with META_MODEL_API_KEY")
	bar := flag.Float64("bar", 0.99, "95 % lower bound the code's precision must clear before Spark stops checking every answer")
	calib := flag.String("calib", "15,30", "calibrate after these training invoices")
	creditDB := flag.String("credit", "", "only print who made the free answers possible, for the desk database of an earlier run")
	breakDB := flag.String("break", "", "only run the breaker against every rule the rule engineer got through the gate, in the desk database of an earlier run (needs -keys)")
	flag.Parse()
	if *keys != "" {
		loadKeys(*keys)
	}
	if *breakDB != "" {
		breakRules(*breakDB)
		return
	}
	if *creditDB != "" {
		db, err := store.Open(*creditDB)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(Attribute(db).Markdown())
		return
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
	calAt := map[int]bool{}
	for _, s := range strings.Split(*calib, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			calAt[n] = true
		}
	}

	db, err := store.Open(filepath.Join(*out, "desk", "orderdesk.db"))
	if err != nil {
		log.Fatal(err)
	}
	var prods []catalog.Product
	for _, p := range data.Products {
		prods = append(prods, catalog.Product{ID: p.ID, Name: p.Name, Brand: p.Brand, Barcode: p.Code})
	}
	if err := catalog.ReplaceProducts(db, prods); err != nil {
		log.Fatal(err)
	}
	seen := map[string]bool{}
	var sups []catalog.Customer
	for _, inv := range data.Invoices {
		if !seen[inv.Supplier] {
			seen[inv.Supplier] = true
			sups = append(sups, catalog.Customer{ID: supplierID(inv.Supplier), Name: inv.Supplier})
		}
	}
	catalog.ReplaceSuppliers(db, sups)
	db.SetSetting("ai_budget_day", "25")
	db.SetSetting("engineer_auto", "0")
	db.SetSetting("engineer_model", meta.Spark)
	db.SetSetting("tenant_name", "Adaptive replay")
	cat := catalog.New(db)
	mc := meta.New(os.Getenv("META_MODEL_API_KEY"), db)
	eng := &engineer.Engineer{DB: db, Cat: cat, Meta: mc}
	d := &desk.Desk{DB: db, Cat: cat, Meta: mc, DataDir: filepath.Join(*out, "desk")}

	// Phase 0: Spark reads the catalogue and designs the first profile.
	prof, cost, err := profile.Bootstrap(ctx, mc, cat, db, eng)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	log.Printf("[bootstrap] %s: blocks %v, lexicon %d, search %s ($%.4f)", prof.Industry, prof.On(), len(prof.Lexicon), prof.Search.Describe(), cost)
	d.Search, d.Autonomy, d.Threshold = search.Build(cat, prof.Search), desk.NewAutonomy(*bar), 0.9

	var rows []row
	var cals []profile.Calibration
	n, unlockedAt := 0, 0
	for _, inv := range data.Invoices {
		if inv.Split != "train" {
			continue
		}
		n++
		phase := "learning"
		if !d.Autonomy.Unlocked() {
			phase = "spark-checks-all"
		}
		r := d.Replay(ctx, supplierID(inv.Supplier), inv.ID, inv.Month+"-01", inv.Lines, true)
		rows = append(rows, row{Phase: phase, Invoice: inv.ID, Supplier: inv.Supplier, ReplayResult: r})
		log.Printf("[%s %d] %s: %d lines, alias %d, rules %d, ai %d, asked %d, wrong %d | agree %d/%d, disagree %d, spark alone %d/%d, $%.4f",
			phase, n, inv.ID, r.Lines, r.Alias, r.Rules, r.AI, r.Asked, r.Wrong, r.AgreeRight, r.Agree, r.Disagree, r.AloneRight, r.Alone, r.AICost)
		if phase == "spark-checks-all" && d.Autonomy.Unlocked() {
			a := sum(rows, "spark-checks-all")
			unlockedAt = n
			profile.Note(db, fmt.Sprintf("after %d invoices", n), "autonomy",
				"the desk's steps earned autonomy: Spark stops checking every answer",
				fmt.Sprintf("%d code answers confirmed, %d right (95 %% lower bound %.2f %% ≥ bar %.0f %%). While supervised: %d lines, Spark agreed with the code on %d (%d right), disagreed on %d; alone on %d, right on %d",
					d.Autonomy.Right+d.Autonomy.Wrong, d.Autonomy.Right, 100*d.Autonomy.Bound(), 100*d.Autonomy.Bar,
					a.Lines, a.Agree, a.AgreeRight, a.Disagree, a.Alone, a.AloneRight), true)
		}
		if calAt[n] {
			stage := fmt.Sprintf("calibration after %d invoices", n)
			p, c, err := profile.Calibrate(ctx, stage, mc, cat, db, eng)
			if err != nil {
				log.Printf("[%s] %v", stage, err)
			}
			if p != nil {
				d.Search = search.Build(cat, p.Search)
			}
			cals = append(cals, c)
			log.Printf("[%s] accepted %d, rejected %d; %s → %s; search %s; engineer %v ($%.4f)", stage, c.Accepted, c.Rejected, c.Before, c.After, c.SearchAfter, c.EngineerRules, c.Cost)
		}
	}
	// autonomy at the end of training
	if unlockedAt == 0 {
		profile.Note(db, "end of training", "autonomy", "the desk stays supervised: Spark keeps checking every code answer",
			fmt.Sprintf("%d code answers confirmed, %d wrong (95 %% lower bound %.2f %% < bar %.0f %%)", d.Autonomy.Right+d.Autonomy.Wrong, d.Autonomy.Wrong,
				100*d.Autonomy.Bound(), 100*d.Autonomy.Bar), false)
	}

	// Test: invoices the desk never saw; nothing learned.
	for _, inv := range data.Invoices {
		if inv.Split != "test" {
			continue
		}
		r := d.Replay(ctx, supplierID(inv.Supplier), inv.ID, inv.Month+"-01", inv.Lines, false)
		rows = append(rows, row{Phase: "test", Invoice: inv.ID, Supplier: inv.Supplier, ReplayResult: r})
		log.Printf("[test] %s: %d lines, alias %d, rules %d, ai %d, asked %d, wrong %d, $%.4f", inv.ID, r.Lines, r.Alias, r.Rules, r.AI, r.Asked, r.Wrong, r.AICost)
	}
	final := profile.Load(db)
	ruleList, _ := rules.List(db, false)
	var spent float64
	db.QueryRow(`SELECT COALESCE(SUM(cost),0) FROM ai_usage`).Scan(&spent)
	credit := Attribute(db)
	rep := map[string]any{"profile": final, "calibrations": cals, "rows": rows, "rules": ruleList, "spent": spent, "credit": credit,
		"phases": map[string]agg{"spark-checks-all": sum(rows, "spark-checks-all"), "learning": sum(rows, "learning"), "test": sum(rows, "test")}}
	os.WriteFile(filepath.Join(*out, "report.json"), []byte(store.JSON(rep)), 0o644)
	md := markdown(final, cals, rows, ruleList, spent) + credit.Markdown()
	os.WriteFile(filepath.Join(*out, "report.md"), []byte(md), 0o644)
	fmt.Println(md)
	db.Close()
}

type agg struct {
	Invoices, Lines, Alias, Rules, AI, Asked, Wrong int
	Agree, AgreeRight, Disagree, Alone, AloneRight  int
	Cost                                            float64
}

func sum(rows []row, phase string) agg {
	var a agg
	for _, r := range rows {
		if r.Phase != phase {
			continue
		}
		a.Invoices++
		a.Lines += r.Lines
		a.Alias += r.Alias
		a.Rules += r.Rules
		a.AI += r.AI
		a.Asked += r.Asked
		a.Wrong += r.Wrong
		a.Agree += r.Agree
		a.AgreeRight += r.AgreeRight
		a.Disagree += r.Disagree
		a.Alone += r.Alone
		a.AloneRight += r.AloneRight
		a.Cost += r.AICost
	}
	return a
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func pct(a, b int) string {
	if b == 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f %%", 100*float64(a)/float64(b))
}

func markdown(p *profile.Profile, cals []profile.Calibration, rows []row, rl []*rules.Rule, spent float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# What Muse Spark identified: %s\n\n%s\n\n", p.Industry, p.Reading)
	b.WriteString("## First-pass blocks\n\n")
	for _, bl := range profile.Library {
		st := p.Blocks[bl.Name]
		if st == nil || bl.Name == "lexicon" {
			continue
		}
		state := "off"
		if st.On {
			state = "ON"
		}
		fmt.Fprintf(&b, "- **%s** %s — %s\n  - evidence: %s\n", bl.Name, state, st.Why, st.Evidence)
	}
	b.WriteString("\n## The desk's own vocabulary (lexicon)\n\n")
	for _, e := range p.Lexicon {
		fmt.Fprintf(&b, "- `%s` → `%s` [%s] — %s\n  - evidence: %s\n", e.From, e.To, e.Status, e.Why, e.Evidence)
	}
	fmt.Fprintf(&b, "\n## Search method\n\n**%s** (%s)\n\n", p.Search.Describe(), p.Search.Name)
	b.WriteString("## Rules (rule engineer and attributes)\n\n")
	for _, r := range rl {
		fmt.Fprintf(&b, "- %s [%s] %s — by %s", r.ID, r.Status, r.Sentence, r.Author)
		if r.Reason != "" {
			fmt.Fprintf(&b, " (%s)", r.Reason)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## Phases\n\n| phase | invoices | lines | free (alias + rules) | AI | asked | wrong auto | Spark agreed with code | Spark alone right | AI cost |\n|---|---|---|---|---|---|---|---|---|---|\n")
	for _, ph := range []string{"spark-checks-all", "learning", "test"} {
		a := sum(rows, ph)
		fmt.Fprintf(&b, "| %s | %d | %d | %d (%s) | %d (%s) | %d (%s) | %d | %d/%d | %d/%d | $%.4f |\n", ph, a.Invoices, a.Lines,
			a.Alias+a.Rules, pct(a.Alias+a.Rules, a.Lines), a.AI, pct(a.AI, a.Lines), a.Asked, pct(a.Asked, a.Lines), a.Wrong,
			a.AgreeRight, a.Agree, a.AloneRight, a.Alone, a.Cost)
	}
	b.WriteString("\n## Calibrations\n\n")
	for _, c := range cals {
		fmt.Fprintf(&b, "- **%s** (%d seen + %d unseen lines): %s → %s; accepted %d, rejected %d; search: %s → %s; engineer rules %v; $%.4f. Spark: %s\n",
			c.Stage, c.Train, c.Held, c.Before, c.After, c.Accepted, c.Rejected, c.SearchBefore, c.SearchAfter, c.EngineerRules, c.Cost, c.Note)
	}
	b.WriteString("\n## Experiment log\n\n")
	for _, x := range p.Log {
		v := "rejected"
		if x.Accepted {
			v = "accepted"
		}
		fmt.Fprintf(&b, "- [%s] %s · %s: %s (%s, by %s) — %s\n", x.Stage, x.Kind, v, x.What, x.Numbers, x.By, x.Why)
	}
	fmt.Fprintf(&b, "\nMeta model spend for the whole run: $%.4f\n", spent)
	return b.String()
}

func supplierID(name string) string {
	return "S-" + strings.TrimSpace(strings.TrimPrefix(name, "Fournisseur"))
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
