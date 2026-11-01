// voicebench measures keyword biasing in Muse Voice Transcribe: every clip is transcribed twice, once
// plain and once with the catalogue's brands as keywords, and each run is scored on how many of the
// brands the speaker said come back in the transcript, and on the words of the product names.
//
//	voicebench -clips clips.json -catalog data/replay-siir.json -keys keys.env -n 100
//
// clips.json: {"clips":[{"wav":"wav/00.wav","text":"…","brands":["CHOUFAN"],"names":["choufan au citron 150g"]}]}
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

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

type clip struct {
	WAV    string   `json:"wav"`
	Text   string   `json:"text"`
	Brands []string `json:"brands"`
	Names  []string `json:"names"`
}

type score struct {
	Brands, BrandsHeard int
	Words, WordsHeard   int
	Cost                float64
}

type sample struct {
	Text   string `json:"said"`
	Plain  string `json:"plain"`
	Primed string `json:"primed"`
}

func main() {
	clipsPath := flag.String("clips", "clips.json", "clips to transcribe")
	catPath := flag.String("catalog", "replay.json", "replay file whose products give the keywords")
	keys := flag.String("keys", "", "keys file with META_MODEL_API_KEY")
	n := flag.Int("n", 100, "number of keywords")
	out := flag.String("out", "voicebench-run", "output directory (replaced)")
	flag.Parse()
	if *keys != "" {
		loadKeys(*keys)
	}
	var clips struct {
		Clips []clip `json:"clips"`
	}
	readJSON(*clipsPath, &clips)
	var cat struct {
		Products []struct {
			ID, Name, Brand string
		} `json:"products"`
	}
	readJSON(*catPath, &cat)
	os.RemoveAll(*out)
	db, err := store.Open(filepath.Join(*out, "bench.db"))
	if err != nil {
		log.Fatal(err)
	}
	var prods []catalog.Product
	for _, p := range cat.Products {
		prods = append(prods, catalog.Product{ID: p.ID, Name: p.Name, Brand: p.Brand})
	}
	catalog.ReplaceProducts(db, prods)
	c := catalog.New(db)
	kw := c.Keywords(*n, meta.MaxKeyword)
	db.SetSetting("ai_budget_day", "5")
	mc := meta.New(os.Getenv("META_MODEL_API_KEY"), db)
	ctx := context.Background()
	base := filepath.Dir(*clipsPath)
	var plain, primed score
	var samples []sample
	for i, cl := range clips.Clips {
		wav, err := os.ReadFile(filepath.Join(base, cl.WAV))
		if err != nil {
			log.Fatal(err)
		}
		p, c1, err := mc.TranscribeWords(ctx, wav, fmt.Sprint("plain-", i), nil)
		if err != nil {
			log.Fatalf("clip %d plain: %v", i, err)
		}
		k, c2, err := mc.TranscribeWords(ctx, wav, fmt.Sprint("primed-", i), kw)
		if err != nil {
			log.Fatalf("clip %d primed: %v", i, err)
		}
		plain.add(cl, p, c1)
		primed.add(cl, k, c2)
		samples = append(samples, sample{Text: cl.Text, Plain: p, Primed: k})
		log.Printf("[%02d] plain: %s\n      primed: %s", i, p, k)
	}
	rep := map[string]any{"keywords": kw, "plain": plain, "primed": primed, "samples": samples}
	os.WriteFile(filepath.Join(*out, "report.json"), []byte(store.JSON(rep)), 0o644)
	fmt.Printf("%d clips, %d keywords\n", len(clips.Clips), len(kw))
	fmt.Printf("  plain : brands heard %d/%d, name words heard %d/%d ($%.4f)\n", plain.BrandsHeard, plain.Brands, plain.WordsHeard, plain.Words, plain.Cost)
	fmt.Printf("  primed: brands heard %d/%d, name words heard %d/%d ($%.4f)\n", primed.BrandsHeard, primed.Brands, primed.WordsHeard, primed.Words, primed.Cost)
}

func (s *score) add(c clip, heard string, cost float64) {
	words := map[string]bool{}
	for _, w := range toks(heard) {
		words[w] = true
	}
	for _, b := range c.Brands {
		s.Brands++
		if allIn(toks(b), words) {
			s.BrandsHeard++
		}
	}
	for _, n := range c.Names {
		for _, w := range toks(n) {
			if len(w) < 3 || strings.ContainsAny(w, "0123456789") {
				continue // sizes are read as numbers, spelled or not: not a hearing test
			}
			s.Words++
			if words[w] {
				s.WordsHeard++
			}
		}
	}
	s.Cost += cost
}

// toks are the words of a text as the desk's normaliser cuts them.
func toks(s string) []string {
	w, _ := catalog.Tokens(catalog.Normalize(s))
	return w
}

func allIn(ts []string, set map[string]bool) bool {
	for _, t := range ts {
		if !set[t] {
			return false
		}
	}
	return len(ts) > 0
}

func readJSON(path string, v any) {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		log.Fatal(err)
	}
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
