// readbench measures the two invoice readers: Muse Spark and Muse Wonder each read every invoice image on
// their own, each reading is scored against the printed truth, and the desk's agreement check (desk.Agree)
// is scored on what it lets through: a line both readers agree on is booked without a second look, so the
// number that matters is how many wrong readings got through with both readers agreeing.
//
//	readbench -truth readbench/truth.json -keys keys.env
//
// truth.json: {"invoices":[{"image":"HW-43.jpg","lines":[{"text":"…","qty":3,"unit_price":12887}]}]}
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
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/desk"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

type result struct {
	Invoices, Lines int
	SparkRight      int // printed lines Spark read right (words, quantity and price)
	WonderRight     int
	BothAgree       int // Spark lines Wonder agrees with
	AgreeWrong      int // of those, lines Spark read wrong: the errors the check lets through
	Caught          int // Spark lines read wrong that Wonder disagreed with: sent to a person
	FalseAlarms     int // Spark lines read right that Wonder disagreed with
	SparkCost       float64
	WonderCost      float64
}

func main() {
	truthPath := flag.String("truth", "truth.json", "invoice images and what is printed on them")
	keys := flag.String("keys", "", "keys file with META_MODEL_API_KEY")
	out := flag.String("out", "readbench-run", "output directory (replaced)")
	flag.Parse()
	if *keys != "" {
		loadKeys(*keys)
	}
	var truth struct {
		Invoices []struct {
			Image string `json:"image"`
			Lines []struct {
				Text      string  `json:"text"`
				Qty       float64 `json:"qty"`
				UnitPrice float64 `json:"unit_price"`
			} `json:"lines"`
		} `json:"invoices"`
	}
	b, err := os.ReadFile(*truthPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(b, &truth); err != nil {
		log.Fatal(err)
	}
	os.RemoveAll(*out)
	db, err := store.Open(filepath.Join(*out, "bench.db"))
	if err != nil {
		log.Fatal(err)
	}
	db.SetSetting("ai_budget_day", "5")
	mc := meta.New(os.Getenv("META_MODEL_API_KEY"), db)
	cost := func(model, ref string) float64 {
		var c float64
		db.QueryRow(`SELECT COALESCE(SUM(cost),0) FROM ai_usage WHERE model = ? AND ref = ?`, model, ref).Scan(&c)
		return c
	}
	ctx := context.Background()
	var res result
	var log2 []map[string]any
	for _, inv := range truth.Invoices {
		img, err := os.ReadFile(filepath.Join(filepath.Dir(*truthPath), inv.Image))
		if err != nil {
			log.Fatal(err)
		}
		var want desk.InvoiceDoc
		for _, l := range inv.Lines {
			want.Lines = append(want.Lines, desk.InvoiceLine{Text: l.Text, Qty: l.Qty, UnitPrice: l.UnitPrice, Total: l.Qty * l.UnitPrice})
		}
		s, err := desk.ReadInvoice(ctx, mc, meta.Spark, img, inv.Image)
		if err != nil {
			log.Printf("[%s] spark: %v", inv.Image, err)
			continue
		}
		w, err := desk.ReadInvoice(ctx, mc, meta.Wonder, img, inv.Image)
		if err != nil {
			log.Printf("[%s] wonder: %v", inv.Image, err)
			continue
		}
		res.Invoices++
		res.Lines += len(want.Lines)
		sRight := desk.Agree(s, want) // per Spark line: matches a printed line
		wRight := desk.Agree(w, want)
		agree := desk.Agree(s, w)
		for _, ok := range wRight {
			if ok {
				res.WonderRight++
			}
		}
		var misses []string
		for i := range s.Lines {
			switch {
			case sRight[i]:
				res.SparkRight++
				if agree[i] {
					res.BothAgree++
				} else {
					res.FalseAlarms++
				}
			case agree[i]:
				res.BothAgree++
				res.AgreeWrong++
				misses = append(misses, "AGREED WRONG: "+s.Lines[i].Text)
			default:
				res.Caught++
				misses = append(misses, "caught: "+s.Lines[i].Text)
			}
		}
		res.SparkCost += cost(meta.Spark, inv.Image)
		res.WonderCost += cost(meta.Wonder, inv.Image)
		log.Printf("[%s] %d printed, Spark read %d (%d right), Wonder %d (%d right), agree %s", inv.Image, len(want.Lines), len(s.Lines),
			count(sRight), len(w.Lines), count(wRight), strings.Join(misses, "; "))
		log2 = append(log2, map[string]any{"image": inv.Image, "spark": s, "wonder": w, "misses": misses})
	}
	os.WriteFile(filepath.Join(*out, "report.json"), []byte(store.JSON(map[string]any{"result": res, "invoices": log2})), 0o644)
	fmt.Printf("%d invoices, %d printed lines\n", res.Invoices, res.Lines)
	fmt.Printf("  Muse Spark read right %d, Muse Wonder read right %d\n", res.SparkRight, res.WonderRight)
	fmt.Printf("  both agree on %d lines, of which %d were read wrong; %d wrong readings caught; %d false alarms\n",
		res.BothAgree, res.AgreeWrong, res.Caught, res.FalseAlarms)
	fmt.Printf("  cost: Spark $%.4f, Wonder $%.4f\n", res.SparkCost, res.WonderCost)
}

func count(bs []bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}
	return n
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
