// Package search is step 3's shortlist, built the way a desk measured best for its catalogue: BM25 over
// the name and brand words (rare words count more than generic ones), character trigrams (typos,
// glued and cut words), the classic word similarity, or a weighted mix of them. Plain code, no model.
//
// The method is part of the desk's Matching Profile: Muse Spark proposes configurations, the code
// measures each one (is the right product in the top 8?) on lines whose answer is known, and keeps
// the best only if it holds on lines it was not chosen on.
package search

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
)

// Config is one way of ranking products. Weights are relative; a zero weight turns a method off.
type Config struct {
	Name       string  `json:"name"`
	BM25       float64 `json:"bm25"`        // BM25 over name and brand words
	BrandBoost float64 `json:"brand_boost"` // how much a brand word counts in BM25 (1 = like a name word)
	Trigram    float64 `json:"trigram"`     // share of the line's character trigrams the product has
	WordSim    float64 `json:"wordsim"`     // the classic word similarity (prefixes, small typos)
	Note       string  `json:"note,omitempty"`
}

// Classic is today's ranking expressed as a Config (word similarity only).
var Classic = Config{Name: "classic word similarity", WordSim: 1}

// Library is the set of configurations always measured, next to Muse Spark's proposals.
var Library = []Config{
	Classic,
	{Name: "BM25", BM25: 1, BrandBoost: 1},
	{Name: "BM25, brand ×2", BM25: 1, BrandBoost: 2},
	{Name: "trigrams", Trigram: 1},
	{Name: "BM25 + trigrams", BM25: 1, BrandBoost: 1.5, Trigram: 1},
	{Name: "BM25 + trigrams + word similarity", BM25: 1, BrandBoost: 1.5, Trigram: 0.7, WordSim: 0.5},
}

type doc struct {
	p       *catalog.Product
	name    map[string]int // name word → count
	brand   map[string]bool
	length  int
	trigram map[string]bool
}

// Index is a catalogue indexed for one Config.
type Index struct {
	Cfg    Config
	docs   []doc
	df     map[string]int
	avg    float64
	byTri  map[string][]int
	byWord map[string][]int
}

const k1, b = 1.2, 0.75

// Build indexes the catalogue's current reading of every product name.
func Build(cat *catalog.Catalog, cfg Config) *Index {
	ix := &Index{Cfg: cfg, df: map[string]int{}, byTri: map[string][]int{}, byWord: map[string][]int{}}
	total := 0
	for _, p := range cat.Products() {
		d := doc{p: p, name: map[string]int{}, brand: map[string]bool{}, trigram: map[string]bool{}}
		bw, _ := catalog.Tokens(catalog.NormalizeName(p.Brand))
		for _, w := range bw {
			d.brand[w] = true
		}
		for _, w := range p.Words {
			d.name[w]++
			for _, t := range trigrams(w) {
				d.trigram[t] = true
			}
		}
		d.length = len(p.Words)
		total += d.length
		i := len(ix.docs)
		ix.docs = append(ix.docs, d)
		for w := range d.name {
			ix.df[w]++
			ix.byWord[w] = append(ix.byWord[w], i)
		}
		for t := range d.trigram {
			ix.byTri[t] = append(ix.byTri[t], i)
		}
	}
	if len(ix.docs) > 0 {
		ix.avg = float64(total) / float64(len(ix.docs))
	}
	return ix
}

func trigrams(w string) []string {
	r := []rune(" " + w + " ")
	if len(r) < 3 {
		return nil
	}
	out := make([]string, 0, len(r)-2)
	for i := 0; i+3 <= len(r); i++ {
		out = append(out, string(r[i:i+3]))
	}
	return out
}

// Shortlist ranks products for a line's words; same contract as catalog.Shortlist.
func (ix *Index) Shortlist(words []string, size string, hints []string, usual map[string]int, exclude map[string]bool, n int) []*catalog.Product {
	if len(ix.docs) == 0 {
		return nil
	}
	c := ix.Cfg
	bm := map[int]float64{}
	tri := map[int]float64{}
	sim := map[int]float64{}
	if c.BM25 > 0 {
		N := float64(len(ix.docs))
		for _, w := range words {
			df := float64(ix.df[w])
			if df == 0 {
				continue
			}
			idf := math.Log(1 + (N-df+0.5)/(df+0.5))
			for _, i := range ix.byWord[w] {
				d := ix.docs[i]
				tf := float64(d.name[w])
				if d.brand[w] {
					tf *= math.Max(c.BrandBoost, 0.1)
				}
				bm[i] += idf * tf * (k1 + 1) / (tf + k1*(1-b+b*float64(d.length)/ix.avg))
			}
		}
	}
	if c.Trigram > 0 {
		var q []string
		for _, w := range words {
			q = append(q, trigrams(w)...)
		}
		if len(q) > 0 {
			hits := map[int]int{}
			for _, t := range q {
				for _, i := range ix.byTri[t] {
					hits[i]++
				}
			}
			for i, h := range hits {
				if s := float64(h) / float64(len(q)); s >= 0.3 {
					tri[i] = s
				}
			}
		}
	}
	if c.WordSim > 0 {
		for i, d := range ix.docs {
			s := 0.0
			for _, w := range words {
				best := 0.0
				for pw := range d.name {
					if v := catalog.WordSim(w, pw); v > best {
						best = v
					}
				}
				s += best
			}
			if s > 0 {
				sim[i] = s
			}
		}
	}
	norm := func(m map[int]float64) func(int) float64 {
		top := 0.0
		for _, v := range m {
			top = math.Max(top, v)
		}
		return func(i int) float64 {
			if top == 0 {
				return 0
			}
			return m[i] / top
		}
	}
	nb, nt, ns := norm(bm), norm(tri), norm(sim)
	cand := map[int]bool{}
	for _, m := range []map[int]float64{bm, tri, sim} {
		for i := range m {
			cand[i] = true
		}
	}
	hinted := map[string]bool{}
	for _, h := range hints {
		hinted[h] = true
	}
	type scored struct {
		i int
		s float64
	}
	var out []scored
	wsum := c.BM25 + c.Trigram + c.WordSim
	for i := range cand {
		d := ix.docs[i]
		if exclude[d.p.ID] {
			continue
		}
		s := (c.BM25*nb(i) + c.Trigram*nt(i) + c.WordSim*ns(i)) / wsum
		if size != "" && d.p.Size == size {
			s += 0.25
		}
		if hinted[d.p.ID] {
			s += 1
		}
		if t := usual[d.p.ID]; t > 0 {
			s += 0.06 + 0.01*float64(min(t, 6))
		}
		if s > 0.15 {
			out = append(out, scored{i, s})
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].s != out[b].s {
			return out[a].s > out[b].s
		}
		return ix.docs[out[a].i].p.Name < ix.docs[out[b].i].p.Name
	})
	res := []*catalog.Product{}
	for k := 0; k < len(out) && k < n; k++ {
		res = append(res, ix.docs[out[k].i].p)
	}
	return res
}

// Describe writes a configuration as a short sentence for the panel and the report.
func (c Config) Describe() string {
	var parts []string
	if c.BM25 > 0 {
		s := "BM25"
		if c.BrandBoost != 1 && c.BrandBoost > 0 {
			s += " (brand ×" + ftoa(c.BrandBoost) + ")"
		}
		parts = append(parts, weighted(s, c.BM25))
	}
	if c.Trigram > 0 {
		parts = append(parts, weighted("trigrams", c.Trigram))
	}
	if c.WordSim > 0 {
		parts = append(parts, weighted("word similarity", c.WordSim))
	}
	return strings.Join(parts, " + ")
}

func weighted(s string, w float64) string {
	if w == 1 {
		return s
	}
	return ftoa(w) + "·" + s
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
