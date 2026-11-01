// sparkalone is the baseline for cmd/adaptive: Muse Spark alone, with no desk around it. For every test
// invoice of a replay file it gets the whole catalogue and the invoice lines, and picks one product per
// line (or says it is unsure). Nothing is learned, no shortlist, no rules, no gate: the same test lines
// the trained desk is measured on, read by the model on its own.
//
//	sparkalone -in data/replay-hardware.json -out ./alone-hardware -keys keys.env
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

	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

type line struct {
	Text   string   `json:"text"`
	Qty    float64  `json:"qty"`
	Truth  string   `json:"product"`
	Accept []string `json:"accept,omitempty"`
}

type input struct {
	Invoices []struct {
		ID    string `json:"id"`
		Split string `json:"split"`
		Lines []line `json:"lines"`
	} `json:"invoices"`
	Products []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Brand string `json:"brand"`
		Code  string `json:"code"`
	} `json:"products"`
}

const prompt = `You match the lines of a supplier invoice to a distributor's product catalogue.
For every numbered LINE, give the id (P…) of the catalogue product it is, and your confidence from 0.0 to 1.0.
If you cannot tell which product it is, give "" as id. Use only ids from the CATALOGUE.
Return JSON only: {"items":[{"line":1,"id":"P12","confidence":0.95}]}`

// Miss is one line Spark booked to the wrong product.
type Miss struct {
	Said string  `json:"said"`
	Got  string  `json:"got"`
	Want string  `json:"want"`
	Conf float64 `json:"confidence"`
}

type result struct {
	Invoices, Lines           int
	Right, Wrong, Unsure      int     // at Spark's own word: any id it gave counts as booked
	RightAt, WrongAt, AskedAt int     // with the desk's threshold: below 0.9 the line goes to a person
	Cost                      float64 `json:"cost"`
	Errors                    []Miss  `json:"errors"`
}

func main() {
	in := flag.String("in", "replay.json", "invoices and catalogue")
	out := flag.String("out", "spark-alone", "output directory (replaced)")
	keys := flag.String("keys", "", "keys file with META_MODEL_API_KEY")
	threshold := flag.Float64("threshold", 0.9, "confidence under which the line would go to a person")
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
	db, err := store.Open(filepath.Join(*out, "usage.db"))
	if err != nil {
		log.Fatal(err)
	}
	db.SetSetting("ai_budget_day", "10")
	mc := meta.New(os.Getenv("META_MODEL_API_KEY"), db)

	var cat strings.Builder
	ids := map[string]string{}
	for i, p := range data.Products {
		code := fmt.Sprintf("P%d", i+1)
		ids[code] = p.ID
		fmt.Fprintf(&cat, "%s | %s | %s", code, p.Name, p.Brand)
		if p.Code != "" {
			fmt.Fprintf(&cat, " | ref %s", p.Code)
		}
		cat.WriteString("\n")
	}
	ctx := context.Background()
	var res result
	for _, inv := range data.Invoices {
		if inv.Split != "test" {
			continue
		}
		var u strings.Builder
		u.WriteString("CATALOGUE:\n" + cat.String() + "\nINVOICE LINES:\n")
		for i, l := range inv.Lines {
			fmt.Fprintf(&u, "LINE %d: %s | qty %g\n", i+1, l.Text, l.Qty)
		}
		var resp struct {
			Items []struct {
				Line       int     `json:"line"`
				ID         string  `json:"id"`
				Confidence float64 `json:"confidence"`
			} `json:"items"`
		}
		// Spark reasons before it answers: give it room, then more room once if the answer came back empty
		cost, err := mc.JSON(ctx, meta.Spark, "spark-alone", inv.ID, prompt, u.String(), nil, 4000+120*len(inv.Lines), &resp)
		if err != nil || len(resp.Items) == 0 {
			c2, err2 := mc.JSON(ctx, meta.Spark, "spark-alone", inv.ID, prompt, u.String(), nil, 12000, &resp)
			cost, err = cost+c2, err2
		}
		if err != nil {
			log.Printf("[%s] %v", inv.ID, err)
		}
		got := map[int]struct {
			id   string
			conf float64
		}{}
		for _, it := range resp.Items {
			got[it.Line] = struct {
				id   string
				conf float64
			}{ids[strings.TrimSpace(it.ID)], it.Confidence}
		}
		res.Invoices++
		res.Cost += cost
		var r, w, s int
		for i, l := range inv.Lines {
			res.Lines++
			g := got[i+1]
			right := g.id == l.Truth
			for _, a := range l.Accept {
				right = right || g.id == a
			}
			switch {
			case g.id == "":
				res.Unsure++
				res.AskedAt++
				s++
			case right:
				res.Right++
				r++
			default:
				res.Wrong++
				w++
				res.Errors = append(res.Errors, Miss{Said: l.Text, Got: g.id, Want: l.Truth, Conf: g.conf})
			}
			switch {
			case g.id == "":
			case g.conf < *threshold:
				res.AskedAt++
			case right:
				res.RightAt++
			default:
				res.WrongAt++
			}
		}
		log.Printf("[%s] %d lines: right %d, wrong %d, unsure %d ($%.4f)", inv.ID, len(inv.Lines), r, w, s, cost)
	}
	os.WriteFile(filepath.Join(*out, "report.json"), []byte(store.JSON(res)), 0o644)
	fmt.Printf("Spark alone on %d test invoices, %d lines\n", res.Invoices, res.Lines)
	fmt.Printf("  at its own word: right %d, wrong %d, unsure %d\n", res.Right, res.Wrong, res.Unsure)
	fmt.Printf("  with the %.2f threshold: booked right %d, booked wrong %d, sent to a person %d\n", *threshold, res.RightAt, res.WrongAt, res.AskedAt)
	fmt.Printf("  Meta spend: $%.4f\n", res.Cost)
	db.Close()
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
