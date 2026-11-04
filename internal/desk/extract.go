package desk

import (
	"context"
	"log"
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
	Unread     []string          `json:"unread,omitempty"` // parts no reader could read: asked to a person
	Method     string            `json:"method"`           // parser | ai | lenient
	AICost     float64           `json:"ai_cost"`
}

var (
	splitRe = regexp.MustCompile(`\n|,|;|\s\+\s`)
	andRe   = regexp.MustCompile(`\s(?:et|w|wa|و)\s`)
	qtyRe   = regexp.MustCompile(`^\d+(?:[.,/]\d+)?x?$`)
	numRe   = regexp.MustCompile(`^\d+(?:\.\d+)?$`)
	// "3x nesquik": a count glued to its x before the name; "nesquik 750g x3": after the size
	countLead  = regexp.MustCompile(`^(\d+)x$`)
	countTrail = regexp.MustCompile(`^x(\d+)$`)
	// "1/2 carton ifri" at the start of a line or an item; "1/2 kg" is a size (500 g)
	fracRe = regexp.MustCompile(`(^|[,;]\s*)([1-3])\s*/\s*([2-4])\s+(\p{L}+)`)
	// Signs of a conversation rather than a list: the parser hands these to the AI.
	chatty = []string{"?", "comme", "habitude", "nafs", "kima", "meme", "dernier", "derniere", "la fois", "annul", "change",
		"au lieu", "plutot", "non ", "hier", "combien", "chhal", "prix", "kifach", "wach", "ajoute", "zid", "na9as", "enleve",
		"retire", "remplace", "same", "last", "نفس", "كيما", "بدل", "زيد", "نقص", "شحال", "واش", "؟",
		// greetings, fillers and self-corrections: a sentence, not a list
		"salam", "bonjour", "bonsoir", "allo", "allô", "merci", "saha", "khouya", "khoya", "smahli", "pardon", "la la",
		"...", "s'il", "svp", "stp", "mettez", "il me faut", "ab3ath", "3tini", "a3tini", "hatli", "7otli", "zidli"}
	arabicRe = regexp.MustCompile(`\p{Arabic}`) // Arabic script: number words and word order the parser does not read
)

// MaxQty is the largest quantity one line may ask for: more is a typing mistake, never an order.
const MaxQty = 100000

// fractions a shop writes before an item ("1/2 carton"), as words the parser reads as quantities
var fracWords = map[string]string{"1/2": "demi", "1/4": "unquart", "3/4": "troisquarts"}
var fracQty = map[string]float64{"demi": 0.5, "unquart": 0.25, "troisquarts": 0.75}

// sizeUnit: the word after "1/2" makes it a size ("1/2 kg", "1/2 l"), written as a decimal for Normalize
var sizeUnit = regexp.MustCompile(`^(?:litres?|liters?|ltrs?|lt|l|cl|ml|cc|kgs?|kilos?|grs?|g|mg)$`)

func fractions(line string) string {
	return fracRe.ReplaceAllStringFunc(line, func(m string) string {
		sm := fracRe.FindStringSubmatch(m)
		w, ok := fracWords[sm[2]+"/"+sm[3]]
		if !ok {
			return m
		}
		if sizeUnit.MatchString(strings.ToLower(sm[4])) {
			w = trimFloat(fracQty[w])
		}
		return sm[1] + w + " " + sm[4]
	})
}

// splitItems cuts a line into items: at commas and semicolons always, at "et", "w" or "و" only when a quantity
// follows ("2 selecto et 3 ifri"), never inside a product's name ("yaourt sucre et aromatise").
func splitItems(s string) []string {
	var out []string
	for _, seg := range splitRe.Split(s, -1) {
		start := 0
		for _, m := range andRe.FindAllStringIndex(seg, -1) {
			if next := strings.Fields(seg[m[1]:]); len(next) > 0 && startsQty(next[0]) {
				out = append(out, seg[start:m[0]])
				start = m[1]
			}
		}
		out = append(out, seg[start:])
	}
	return out
}

func startsQty(w string) bool {
	if _, ok := catalog.NumberWords[w]; ok {
		return true
	}
	return qtyRe.MatchString(w)
}

// Parse is the deterministic reader for text lists ("3 cartons selecto 2l\n2 fardou ifri").
// It returns ok=false whenever the message looks like a conversation; the AI reads those.
func Parse(text string) ([]resolve.Mention, bool) {
	low := " " + strings.ToLower(text) + " "
	if arabicRe.MatchString(text) {
		return nil, false
	}
	for _, w := range chatty {
		if strings.Contains(low, w) {
			return nil, false
		}
	}
	var out []resolve.Mention
	for _, raw := range strings.Split(text, "\n") { // lines first: normalising joins them
		for _, seg := range splitItems(catalog.Normalize(fractions(raw))) {
			ms, ok := parseItems(strings.Fields(seg))
			if !ok {
				return nil, false // a quantity of 0 or beyond MaxQty, or a quantity naming nothing: the AI reads it
			}
			out = append(out, ms...)
		}
	}
	return out, len(out) > 0
}

// parseItems reads the items of one segment of a list: usually one, two when a new quantity starts after a
// size or before a unit word ("2 selecto 2l 3 ifri"). ok=false when a quantity is 0, beyond MaxQty, or
// names nothing.
func parseItems(ws []string) ([]resolve.Mention, bool) {
	var out []resolve.Mention
	m := resolve.Mention{}
	var said []string
	said0 := false // a quantity of 0 was written: never read as 1
	flush := func() bool {
		if words, _ := catalog.Tokens(strings.Join(said, " ")); len(words) == 0 {
			return len(said) == 0 && m.Qty == 0 && !said0 // a greeting or a stray word; a lone quantity is not
		}
		if said0 || m.Qty > MaxQty {
			return false
		}
		m.Said = strings.Join(said, " ")
		if m.Qty == 0 {
			m.Qty = 1
		}
		out = append(out, m)
		return true
	}
	for i, w := range ws {
		bare := numRe.MatchString(w) && !strings.Contains(w, ".")
		if m.Qty == 0 && !said0 {
			if bare {
				m.Qty, _ = strconv.ParseFloat(w, 64)
				said0 = m.Qty == 0
				continue
			}
			if c := countLead.FindStringSubmatch(w); c != nil && len(said) == 0 {
				m.Qty, _ = strconv.ParseFloat(c[1], 64)
				said0 = m.Qty == 0
				continue
			}
			if c := countTrail.FindStringSubmatch(w); c != nil && i == len(ws)-1 && i > 0 && catalog.IsSize(ws[i-1]) {
				// "nesquik 750g x3": three of them. After a bare number ("ifri 1.5 x6") it is the pack.
				m.Qty, _ = strconv.ParseFloat(c[1], 64)
				said0 = m.Qty == 0
				continue
			}
			if n, ok := catalog.NumberWords[w]; ok {
				m.Qty = n
				continue
			}
			if n, ok := fracQty[w]; ok && len(said) == 0 {
				m.Qty = n
				continue
			}
		} else if bare && i > 0 && i < len(ws)-1 && len(said) > 0 &&
			(catalog.IsSize(ws[i-1]) || strings.Contains(ws[i-1], ".") && numRe.MatchString(ws[i-1]) || catalog.UnitWords[ws[i+1]] != "") {
			// a second item on the same line: its quantity follows the first item's size, or comes before a unit
			if !flush() {
				return nil, false
			}
			m, said, said0 = resolve.Mention{}, nil, false
			m.Qty, _ = strconv.ParseFloat(w, 64)
			said0 = m.Qty == 0
			continue
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
	if !flush() {
		return nil, false
	}
	return out, true
}

// Words that only carry a greeting or politeness ("salam khouya", "ab3athli", "s'il vous plait"): the lenient
// reader drops them, where Parse hands the whole message to the AI.
var fillers = map[string]bool{"salam": true, "salamou": true, "slm": true, "alikoum": true, "aleykoum": true, "alaykoum": true,
	"bonjour": true, "bonsoir": true, "bjr": true, "allo": true, "hello": true, "merci": true, "saha": true, "sahit": true,
	"khouya": true, "khoya": true, "khou": true, "sahbi": true, "akhi": true, "chef": true, "smahli": true, "pardon": true,
	"svp": true, "stp": true, "s'il": true, "sil": true, "vous": true, "plait": true, "please": true, "mettez": true, "il": true,
	"me": true, "faut": true, "ca": true, "va": true, "cava": true, "labas": true, "hamdoullah": true, "wesh": true, "ya": true,
	"rani": true, "nheb": true, "nhab": true, "bghit": true, "ana": true, "lyoum": true, "ghodwa": true, "inchallah": true,
	"zid": true, "zidli": true, "ajoute": true, "ajouter": true, "jibli": true, "hatli": true, "a3tini": true, "atini": true,
	"السلام": true, "عليكم": true, "خويا": true, "شكرا": true, "صحة": true, "عطيني": true, "جيبلي": true, "حطلي": true,
	"ابعثلي": true, "بعثلي": true, "فضلك": true, "زيد": true, "زيدلي": true,
	// openers and closings shops write around the list ("sbah lkhir khouya, hadarli: …", "commande pour demain svp")
	"sbah": true, "sba7": true, "lkhir": true, "lkheir": true, "hadarli": true, "hadirli": true, "hadrli": true,
	"aatini": true, "3atini": true, "commande": true, "pour": true, "demain": true, "rabi": true, "yhafdek": true,
	"yhafdak": true, "yhfdek": true, "baraka": true, "barak": true, "allah": true, "fik": true, "صباح": true, "الخير": true}

var fillerPrefixes = []string{"ab3ath", "b3ath", "abaath", "3tini", "7ot", "zidl"}

// Words of a conversation the lenient reader does not act on alone (a change, a cancellation, a question,
// "same as last time"): what carries them goes to a person.
var talk = []string{"comme", "habitude", "nafs", "kima", "meme", "dernier", "derniere", "la fois", "annul", "change",
	"au lieu", "plutot", " non ", "hier", "combien", "chhal", "prix", "kifach", "wach", "na9as", "enleve", "retire",
	"remplace", "same", "last", " la la ", "نفس", "كيما", "بدل", "نقص", "شحال", "واش"}

func filler(w string) bool {
	if fillers[w] {
		return true
	}
	for _, p := range fillerPrefixes {
		if strings.HasPrefix(w, p) {
			return true
		}
	}
	return false
}

// ParseLenient reads what it can of a message Parse handed to the AI, for when the AI cannot answer (no key,
// budget reached, the model failing): greetings and fillers are dropped, every item it reads as surely as
// Parse is returned, and every part it cannot read (a question, a change, a quantity of 0) is returned in
// unread, for a person to answer. It never fails.
func ParseLenient(text string) (ms []resolve.Mention, unread []string) {
	for _, raw := range strings.Split(text, "\n") {
		if strings.ContainsAny(raw, "?؟") || strings.Contains(raw, "...") {
			// a question or a sentence that trails off: a person reads the whole line, unless it is only a greeting
			var rest []string
			for _, w := range strings.Fields(catalog.Normalize(raw)) {
				if !filler(w) {
					rest = append(rest, w)
				}
			}
			if words, _ := catalog.Tokens(strings.Join(rest, " ")); len(words) > 0 {
				unread = append(unread, strings.TrimSpace(raw))
			}
			continue
		}
		for _, seg := range splitItems(catalog.Normalize(fractions(raw))) {
			var ws []string
			for _, w := range strings.Fields(seg) {
				if !filler(w) {
					ws = append(ws, w)
				}
			}
			if len(ws) == 0 {
				continue
			}
			said := strings.Join(ws, " ")
			isTalk := false
			for _, t := range talk {
				isTalk = isTalk || strings.Contains(" "+said+" ", t)
			}
			items, ok := parseItems(ws)
			if isTalk || !ok {
				unread = append(unread, said)
				continue
			}
			ms = append(ms, items...)
		}
	}
	return ms, unread
}

const extractPrompt = `You read orders that small shops in Algeria send to their distributor: Darja, French or Arabic,
often a voice note transcribed with mistakes. Cut the message into ordered items. Do NOT choose products.

For each item return:
- "said": the customer's own words for the product, copied from the message (no quantity, no unit word).
- "hint": the product in plain words if you can tell it, written with the known brand and product names below in
  Latin script, with the size in digits ("Rouiba Orange 1 L" for روييبة برتقال لتر): transcription often mishears
  these names and a voice note may be in Arabic script; "" if unsure.
- "qty": a number (wahed 1, zouj 2, tlata 3, rebaa 4, khamsa 5, setta 6, sebaa 7, tmenya 8, tes3a 9, achra 10, demi 0.5).
- "unit": "carton" (carton, caisse, كرطون), "pack" (fardeau, fardou, pack, paquet), "unit" (bouteille, pièce,
  unité, boîte, flacon) or "" when not said.
Corrections count: "two cartons, no wait three" is qty 3. Ignore greetings and small talk.
"Same as last time" (comme d'habitude, kima l'khatra li fatet, nafs l'commande, نفس الطلبية): set "repeat_last": true
and put in items ONLY what is added or changed.
If the message is not an order (a question, a greeting only), intent "other" and a one-sentence polite reply in French.
Known brands and product names: %BRANDS%
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
	// 1,500 tokens (reasoning included) cut long orders mid-JSON and lost every item; 9 items with hints need more
	cost, err := m.JSON(ctx, meta.Spark, "extract", ref, prompt, text, nil, 4000, &out)
	ex := Extraction{Intent: out.Intent, RepeatLast: out.RepeatLast, Reply: out.Reply, Method: "ai", AICost: cost}
	for _, it := range out.Items {
		it.Said = strings.TrimSpace(it.Said)
		if it.Said == "" {
			it.Said = it.Hint
		}
		if it.Qty <= 0 {
			it.Qty = 1
		}
		if it.Qty > MaxQty {
			ex.Unread = append(ex.Unread, strings.TrimSpace(trimFloat(it.Qty)+" "+it.Said))
			continue
		}
		if it.Said != "" {
			ex.Mentions = append(ex.Mentions, it)
		}
	}
	if len(ex.Mentions) > 0 || len(ex.Unread) > 0 || ex.RepeatLast {
		ex.Intent = "order"
	}
	return ex, err
}

const readPhotoPrompt = `You look at a photo a small shop in Algeria sent to its distributor to order. It is either a
written order list (handwritten or printed), or it shows the products themselves (bottles, cans, cartons or packs
on a shelf, a counter, in a crate or in a hand).
For a list: copy every ordered item exactly as written, one per line, keeping quantities and units. Ignore headers,
prices and signatures.
For products: name the kinds of packaging you see, using only these words: bottle, can, carton, box, pack, jar,
bag, tub, sachet. Most common first, at most three.
Return JSON only, either {"kind":"list","lines":["5 cartons Hamoud 2L","3 Ifri 1.5"]}
or {"kind":"products","packaging":["bottle","can"]}`

// PhotoReading is what one look at a shop's photo found: a written list, or products to find with SAM.
type PhotoReading struct {
	Kind      string   `json:"kind"` // list | products
	Lines     []string `json:"lines"`
	Packaging []string `json:"packaging"`
}

// ReadPhoto copies the lines of a photographed list (the parser then cuts them into items), or says which
// kinds of packaging a photo of products shows, so SAM can find each product.
func ReadPhoto(ctx context.Context, m *meta.Client, img []byte, ref string) (PhotoReading, float64, error) {
	var out PhotoReading
	cost, err := m.JSON(ctx, meta.Spark, "read", ref, readPhotoPrompt, "Read this photo.", [][]byte{img}, 2000, &out)
	if out.Kind != "products" {
		out.Kind = "list"
	}
	return out, cost, err
}

// Extract reads one message: the parser for plain lists, the AI for everything else. When the AI cannot
// answer (no key, the day's budget spent, the model failing), the lenient reader takes what it can and a
// person is asked the rest: a message is never lost to a missing model.
func Extract(ctx context.Context, m *meta.Client, brands []string, text string, voice bool, ref string) (Extraction, error) {
	if !voice {
		if ms, ok := Parse(text); ok {
			return Extraction{Intent: "order", Mentions: ms, Method: "parser"}, nil
		}
	}
	ex, err := ExtractAI(ctx, m, brands, text, ref)
	if err == nil || len(ex.Mentions) > 0 || ex.RepeatLast {
		return ex, err
	}
	log.Printf("[desk] %s: the AI could not read the message (%v), reading it leniently", ref, err)
	ms, unread := ParseLenient(text)
	lx := Extraction{Intent: "other", Mentions: ms, Unread: unread, Method: "lenient", AICost: ex.AICost}
	if len(ms) > 0 || len(unread) > 0 {
		lx.Intent = "order"
	}
	return lx, nil
}
