package catalog

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Level 0: one comparable form for every way a line can be written or said.

// Text that went through the wrong encoding on the way (OCR, old ERPs, copy-paste from a PDF):
// UTF-8 read as Latin-1. Fixed before anything else, or "crÃ¨me" is a word no product has.
var mojibakeRepl = strings.NewReplacer("Ã©", "é", "Ã¨", "è", "Ãª", "ê", "Ã«", "ë", "Ã ", "à", "Ã¢", "â", "Ã®", "î",
	"Ã¯", "ï", "Ã´", "ô", "Ã»", "û", "Ã¹", "ù", "Ã§", "ç", "Ã‰", "É", "Â°", "°", "Âµ", "µ", "â€™", "'", "Â ", " ", " ", " ",
	" ", " ", " ", " ", "�", " ")

var accentRepl = strings.NewReplacer("é", "e", "è", "e", "ê", "e", "ë", "e", "à", "a", "â", "a", "ä", "a", "á", "a", "ã", "a",
	"î", "i", "ï", "i", "í", "i", "ô", "o", "ö", "o", "ó", "o", "ù", "u", "û", "u", "ü", "u", "ú", "u", "ç", "c", "ñ", "n",
	"œ", "oe", "æ", "ae", "’", "'", "‘", "'", "`", "'", "×", "x", "–", " ", "—", " ", "•", " ", "·", " ", "®", " ", "™", " ",
	"&", " ", "-", " ", "_", " ", "/", " ", "+", " + ", "(", " ", ")", " ", "[", " ", "]", " ", ":", " ", ";", " ", "!", " ", "?", " ",
	"\"", " ", "«", " ", "»", " ", "#", " ")

// Arabic-Indic and Persian digits → ASCII; Arabic letters to one spelling (hamza forms, alif maqsura,
// final ta marbuta, tatweel, short vowels), the way people type Arabic without care for spelling.
var digitRepl = strings.NewReplacer("٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4", "٥", "5", "٦", "6", "٧", "7",
	"٨", "8", "٩", "9", "۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4", "۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9",
	"أ", "ا", "إ", "ا", "آ", "ا", "ٱ", "ا", "ى", "ي", "ة", "ه", "ؤ", "و", "ئ", "ي", "ـ", "", "،", " ", "٫", ".",
	"ً", "", "ٌ", "", "ٍ", "", "َ", "", "ُ", "", "ِ", "", "ّ", "", "ْ", "")

var (
	decimalComma = regexp.MustCompile(`(\d),(\d)`)
	sizeRe       = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(litres?|liters?|ltrs?|lt|l|cl|ml|cc|kgs?|grs?|g|mg)\b`)
	wordRe       = regexp.MustCompile(`[\p{L}\p{N}.]+`)
	// "1l5" is how people type 1.5 L.
	litreGlued = regexp.MustCompile(`\b(\d)l(\d{1,2})\b`)
	// "6x1.5l", "6 x 1500ml", "1500ml x 6", "x 6": the count of a pack, written "x6" once sizes are canonical.
	countBefore = regexp.MustCompile(`\b(\d{1,3})\s*[x*]\s*(\d+(?:\.\d+)?(?:ml|mg|g))\b`)
	countAfter  = regexp.MustCompile(`\b(\d+(?:\.\d+)?(?:ml|mg|g))\s*[x*]\s*(\d{1,3})\b`)
	countSpace  = regexp.MustCompile(`(^|\s)[x*]\s+(\d{1,3})\b`)
	// "1 800 g" in a catalogue name is 1800 g. Only in product names: in a shop's line, "2 500g" is
	// two packets of 500 g.
	thousands = regexp.MustCompile(`(^|[^\d.])(\d{1,3}) (\d{3})\s*(ml|g|kg|l)\b`)
)

// Normalize lowercases, strips accents and unifies digits, decimals, sizes and pack counts,
// so "Ifri 1,5 L", "ifri 1.5l", "ifri 1l5" and "IFRI 1500ML" all become "ifri 1500ml",
// and "6x1.5l" becomes "x6 1500ml".
func Normalize(s string) string {
	s = mojibakeRepl.Replace(s)
	s = digitRepl.Replace(accentRepl.Replace(strings.ToLower(s)))
	s = decimalComma.ReplaceAllString(s, "$1.$2")
	s = litreGlued.ReplaceAllString(s, "$1.${2}l")
	s = sizeRe.ReplaceAllStringFunc(s, canonicalSize)
	s = countBefore.ReplaceAllString(s, "x$1 $2")
	s = countAfter.ReplaceAllString(s, "$1 x$2")
	s = countSpace.ReplaceAllString(s, "${1}x$2")
	return strings.Join(strings.Fields(s), " ")
}

// NormalizeName is Normalize for a catalogue name, where a space inside a number is a thousands
// separator ("1 800 g").
func NormalizeName(s string) string {
	s = mojibakeRepl.Replace(strings.ToLower(s))
	s = thousands.ReplaceAllString(s, "$1$2$3$4")
	return Normalize(s)
}

// canonicalSize turns "1.5 l" / "150cl" / "1500 ml" into "1500ml", "1 kg" into "1000g".
func canonicalSize(m string) string {
	sm := sizeRe.FindStringSubmatch(m)
	if sm == nil {
		return m
	}
	v, _ := strconv.ParseFloat(sm[1], 64)
	switch sm[2] {
	case "l", "lt", "ltr", "ltrs", "litre", "litres", "liter", "liters":
		return fmt.Sprintf("%.0fml", v*1000)
	case "cl":
		return fmt.Sprintf("%.0fml", v*10)
	case "ml", "cc":
		return fmt.Sprintf("%.0fml", v)
	case "kg", "kgs":
		return fmt.Sprintf("%.0fg", v*1000)
	case "mg":
		return fmt.Sprintf("%gmg", v)
	default:
		return fmt.Sprintf("%.0fg", v)
	}
}

// SizeNumber is the number a size was written with in its own unit family ("1500ml" → 1500).
func SizeNumber(w string) float64 {
	for _, suf := range []string{"ml", "mg", "g"} {
		if strings.HasSuffix(w, suf) {
			f, _ := strconv.ParseFloat(strings.TrimSuffix(w, suf), 64)
			return f
		}
	}
	return 0
}

// SizeFamily is "ml" (volume) or "g" (mass): a volume and a mass never describe the same size.
func SizeFamily(w string) string {
	switch {
	case strings.HasSuffix(w, "ml"):
		return "ml"
	case strings.HasSuffix(w, "mg"):
		return "mg"
	case strings.HasSuffix(w, "g"):
		return "g"
	}
	return ""
}

// IsSize tells whether a normalised token is a size ("1500ml", "800g").
func IsSize(w string) bool {
	for _, suf := range []string{"ml", "mg", "g"} {
		if strings.HasSuffix(w, suf) {
			if _, err := strconv.ParseFloat(strings.TrimSuffix(w, suf), 64); err == nil {
				return true
			}
		}
	}
	return false
}

// IsCount tells whether a normalised token is a pack count ("x6", "x24").
func IsCount(w string) bool {
	if len(w) < 2 || len(w) > 4 || w[0] != 'x' {
		return false
	}
	_, err := strconv.Atoi(w[1:])
	return err == nil
}

// Numbers said in words: Darja (Latin and Arabic script), French, Arabic.
var NumberWords = map[string]float64{
	"wahed": 1, "wa7ed": 1, "zouj": 2, "zoudj": 2, "jouj": 2, "tlata": 3, "tleta": 3, "rebaa": 4, "arba": 4, "arba3": 4,
	"khamsa": 5, "setta": 6, "sebaa": 7, "seb3a": 7, "tmenya": 8, "tmania": 8, "tes3a": 9, "tesaa": 9, "achra": 10, "3achra": 10,
	"un": 1, "une": 1, "deux": 2, "trois": 3, "quatre": 4, "cinq": 5, "six": 6, "sept": 7, "huit": 8, "neuf": 9, "dix": 10,
	"douze": 12, "quinze": 15, "vingt": 20, "trente": 30, "demi": 0.5,
	"واحد": 1, "زوج": 2, "جوج": 2, "اثنين": 2, "ثلاثة": 3, "تلاتة": 3, "ثلاث": 3, "اربعة": 4, "أربعة": 4, "خمسة": 5,
	"ستة": 6, "سبعة": 7, "ثمانية": 8, "تسعة": 9, "عشرة": 10,
}

// Unit words: what the shop counts in. "" means it did not say.
var UnitWords = map[string]string{
	"carton": "carton", "cartons": "carton", "karton": "carton", "kartona": "carton", "cartonat": "carton", "caisse": "carton",
	"caisses": "carton", "colis": "carton", "كرطون": "carton", "كرتون": "carton", "كراطن": "carton",
	"fardeau": "pack", "fardeaux": "pack", "fardou": "pack", "fardo": "pack", "pack": "pack", "packs": "pack", "paquet": "pack",
	"paquets": "pack", "فاردو": "pack", "باك": "pack",
	"bouteille": "unit", "bouteilles": "unit", "piece": "unit", "pieces": "unit", "unite": "unit", "unites": "unit", "boite": "unit",
	"boites": "unit", "flacon": "unit", "flacons": "unit", "tube": "unit", "tubes": "unit", "sachet": "unit", "sachets": "unit",
	"u": "unit", "pcs": "unit", "حبة": "unit", "قرعة": "unit",
}

// Words that never name a product.
var StopWords = map[string]bool{"de": true, "du": true, "des": true, "la": true, "le": true, "les": true, "et": true,
	"x": true, "avec": true, "pour": true, "svp": true, "stp": true, "wa": true, "w": true, "fi": true, "ta3": true,
	"mta3": true, "a3tini": true, "atini": true, "jibli": true, "please": true, "commande": true, "bonjour": true,
	"salam": true, "salamou": true, "alikoum": true, "merci": true, "sahit": true, "aussi": true, "encore": true,
	"و": true, "عطيني": true, "جيبلي": true, "السلام": true, "عليكم": true, "من": true, "تاع": true}

// Tokens returns the words of a normalised text that can name a product, and its first size.
func Tokens(norm string) (words []string, size string) {
	for _, w := range wordRe.FindAllString(norm, -1) {
		w = strings.Trim(w, ".")
		if w == "" || StopWords[w] || UnitWords[w] != "" {
			continue
		}
		if IsSize(w) {
			if size == "" {
				size = w
			}
			continue
		}
		if IsCount(w) {
			continue
		}
		if f, err := strconv.ParseFloat(w, 64); err == nil {
			// "Ifri 1.5": a bare decimal is litres, which is how shops say it; a whole number is a quantity.
			if strings.Contains(w, ".") && f > 0 && f <= 10 && size == "" {
				size = fmt.Sprintf("%.0fml", f*1000)
			}
			continue
		}
		if _, ok := NumberWords[w]; ok {
			continue
		}
		if len([]rune(w)) < 2 {
			continue
		}
		words = append(words, w)
	}
	return words, size
}

// WordSim scores two words: exact 1, prefix 0.85, close spelling up to 0.8.
func WordSim(a, b string) float64 {
	if a == b {
		return 1
	}
	la, lb := len([]rune(a)), len([]rune(b))
	if la >= 3 && lb >= 3 && (strings.HasPrefix(b, a) || strings.HasPrefix(a, b)) {
		return 0.85
	}
	if la < 4 || lb < 4 {
		return 0
	}
	d := levenshtein(a, b)
	sim := 1 - float64(d)/math.Max(float64(la), float64(lb))
	if sim >= 0.7 {
		return sim * 0.85
	}
	return 0
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
