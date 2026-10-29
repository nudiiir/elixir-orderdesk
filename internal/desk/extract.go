package desk

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
)

// Extraction cuts a message into items. It is the step that gives a message the structure an
// invoice already has on paper. Identity (which product) is NOT decided here.
type Extraction struct {
	Intent     string            `json:"intent"` // order | other
	RepeatLast bool              `json:"repeat_last"`
	Reply      string            `json:"reply"`
	Mentions   []resolve.Mention `json:"mentions"`
	Method     string            `json:"method"` // parser | ai
	AICost     float64           `json:"ai_cost"`
}

var (
	splitRe = regexp.MustCompile(`\n|,|;|\s\+\s|\s(?:et|w|wa|و)\s`)
	numRe   = regexp.MustCompile(`^\d+(?:\.\d+)?$`)
	// Signs of a conversation rather than a list: the parser hands these to the AI.
	chatty = []string{"?", "comme", "habitude", "nafs", "kima", "meme", "dernier", "derniere", "la fois", "annul", "change",
		"au lieu", "plutot", "non ", "hier", "combien", "chhal", "prix", "kifach", "wach", "ajoute", "zid", "na9as", "enleve",
		"retire", "remplace", "same", "last", "نفس", "كيما", "بدل", "زيد", "نقص", "شحال", "واش", "؟"}
)

// Parse is the deterministic reader for text lists ("3 cartons selecto 2l\n2 fardou ifri").
// It returns ok=false whenever the message looks like a conversation; the AI reads those.
func Parse(text string) ([]resolve.Mention, bool) {
	low := " " + strings.ToLower(text) + " "
	for _, w := range chatty {
		if strings.Contains(low, w) {
			return nil, false
		}
	}
	var segs []string
	for _, raw := range strings.Split(text, "\n") { // lines first: normalising joins them
		segs = append(segs, splitRe.Split(catalog.Normalize(raw), -1)...)
	}
	var out []resolve.Mention
	for _, seg := range segs {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		m := resolve.Mention{}
		var said []string
		for _, w := range strings.Fields(seg) {
			if m.Qty == 0 && numRe.MatchString(w) && !strings.Contains(w, ".") {
				m.Qty, _ = strconv.ParseFloat(w, 64)
				continue
			}
			if m.Qty == 0 {
				if n, ok := catalog.NumberWords[w]; ok {
					m.Qty = n
					continue
				}
			}
			if u := catalog.UnitWords[w]; u != "" && m.Unit == "" {
				m.Unit = u
				continue
			}
			if catalog.StopWords[w] {
				continue
			}
			said = append(said, w)
		}
		if words, _ := catalog.Tokens(strings.Join(said, " ")); len(words) == 0 {
			if len(said) == 0 && m.Qty == 0 {
				continue // a greeting or a stray word
			}
			return nil, false
		}
		m.Said = strings.Join(said, " ")
		if m.Qty == 0 {
			m.Qty = 1
		}
		out = append(out, m)
	}
	return out, len(out) > 0
}

const extractPrompt = `You read orders that small shops in Algeria send to their distributor: Darja, French or Arabic,
often a voice note transcribed with mistakes. Cut the message into ordered items. Do NOT choose products.

For each item return:
- "said": the customer's own words for the product, copied from the message (no quantity, no unit word).
- "hint": the product in plain words if you can tell it, using the known brands below (transcription often
  mishears brands); "" if unsure.
- "qty": a number (wahed 1, zouj 2, tlata 3, rebaa 4, khamsa 5, setta 6, sebaa 7, tmenya 8, tes3a 9, achra 10, demi 0.5).
- "unit": "carton" (carton, caisse, كرطون), "pack" (fardeau, fardou, pack, paquet), "unit" (bouteille, pièce,
  unité, boîte, flacon) or "" when not said.
Corrections count: "two cartons, no wait three" is qty 3. Ignore greetings and small talk.
"Same as last time" (comme d'habitude, kima l'khatra li fatet, nafs l'commande, نفس الطلبية): set "repeat_last": true
and put in items ONLY what is added or changed.
If the message is not an order (a question, a greeting only), intent "other" and a one-sentence polite reply in French.
Known brands: %BRANDS%
Return JSON only: {"intent":"order","repeat_last":false,"reply":"","items":[{"said":"selecto","hint":"Selecto 2 L","qty":3,"unit":"carton"}]}`

// ExtractAI reads a conversation (voice transcript or chatty text) into items with Muse Spark.
// The prompt carries the brand names only, never the catalogue: that is what keeps it cheap.
func ExtractAI(ctx context.Context, m *meta.Client, brands []string, text, ref string) (Extraction, error) {
	var out struct {
		Intent     string            `json:"intent"`
		RepeatLast bool              `json:"repeat_last"`
		Reply      string            `json:"reply"`
		Items      []resolve.Mention `json:"items"`
	}
	prompt := strings.Replace(extractPrompt, "%BRANDS%", strings.Join(brands, ", "), 1)
	cost, err := m.JSON(ctx, meta.Spark, "extract", ref, prompt, text, nil, 1500, &out)
	ex := Extraction{Intent: out.Intent, RepeatLast: out.RepeatLast, Reply: out.Reply, Method: "ai", AICost: cost}
	for _, it := range out.Items {
		it.Said = strings.TrimSpace(it.Said)
		if it.Said == "" {
			it.Said = it.Hint
		}
		if it.Qty <= 0 {
			it.Qty = 1
		}
		if it.Said != "" {
			ex.Mentions = append(ex.Mentions, it)
		}
	}
	if len(ex.Mentions) > 0 || ex.RepeatLast {
		ex.Intent = "order"
	}
	return ex, err
}

const readListPrompt = `You read photos of handwritten or printed order lists sent by small shops in Algeria to their
distributor. Copy every ordered item exactly as written, one per line, keeping quantities and units.
Ignore headers, prices and signatures. Return JSON only: {"lines":["5 cartons Hamoud 2L","3 Ifri 1.5"]}`

// ReadPhoto copies the lines of a photographed list; the parser then cuts them into items.
func ReadPhoto(ctx context.Context, m *meta.Client, img []byte, ref string) ([]string, float64, error) {
	var out struct {
		Lines []string `json:"lines"`
	}
	cost, err := m.JSON(ctx, meta.Spark, "read", ref, readListPrompt, "Read this order list.", [][]byte{img}, 2000, &out)
	return out.Lines, cost, err
}

// Extract reads one message: the parser for plain lists, the AI for everything else.
func Extract(ctx context.Context, m *meta.Client, brands []string, text string, voice bool, ref string) (Extraction, error) {
	if !voice {
		if ms, ok := Parse(text); ok {
			return Extraction{Intent: "order", Mentions: ms, Method: "parser"}, nil
		}
	}
	return ExtractAI(ctx, m, brands, text, ref)
}
