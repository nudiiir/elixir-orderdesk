package desk

// Photos of the products themselves: a shop often photographs the bottles it wants (on the shelf, the counter,
// in its hand) instead of typing. SAM 3.1 finds every product, Muse Spark reads each one the way a shop would
// type it, identical products are counted, and the lines go through the same resolution as a typed order.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
)

// SeenGroup is one product seen on the photo, with how many units of it and where.
type SeenGroup struct {
	Said  string     `json:"said"`
	Count int        `json:"count"`
	Boxes []meta.Box `json:"-"`
}

// Seen is what SeeProducts found: the groups in order of first appearance, and the photo with every product
// boxed in its group's colour and number (JPEG), to send back to the shop.
type Seen struct {
	Groups []SeenGroup
	Marked []byte
}

var packagingWords = map[string]bool{"bottle": true, "can": true, "carton": true, "box": true, "pack": true,
	"jar": true, "bag": true, "tub": true, "sachet": true}

const (
	samMaxSide  = 1600 // the photo sent to SAM and drawn on
	cropMaxSide = 320  // each crop sent to Muse Spark
	wholeSide   = 1024 // the whole photo sent along for context
	maxProducts = 20
	minConf     = 0.4
	minArea     = 0.003 // of the photo
)

const readCropsPrompt = `A small shop in Algeria photographed products to order them from its distributor. The first
image is the whole photo; the next ones are crops of it, numbered from 1 in order, one product each.
For each crop, write the product the way a shop owner would type it in an order: brand, product or flavour, size,
for example "Selecto 2L", "Ifri gazeuse 1.5L", "Candia lait 1L", "Coca-Cola canette 33cl". Read only what is printed
or clearly visible, using the whole photo for context. Write "" when the crop is not a product or cannot be read.
Known brands: %s
Return JSON only: {"items":[{"n":1,"said":"Selecto 2L"}]}`

// PhotoOrder is a shop's photo turned into the text of an order.
type PhotoOrder struct {
	Kind  string // the message kind: photo (a written list) or products
	Text  string // the lines, as if the shop had typed them
	Media string // the stored photo: with the products boxed, for a photo of products
	Heard string // what the desk tells the shop it saw
	Seen  *Seen  // set for a photo of products
}

var (
	errNoLines    = errors.New("no line read on the photo")
	errNoProducts = errors.New("no product recognised on the photo")
)

// photoTrouble is what the shop is told when a photo could not be read.
func photoTrouble(err error) string {
	if errors.Is(err, errNoProducts) {
		return "je ne reconnais pas les produits sur cette photo : prenez-les de face, l'étiquette bien visible, ou écrivez la commande"
	}
	return "je n'arrive pas à lire cette photo, pouvez-vous en envoyer une plus nette ?"
}

// ReadShopPhoto reads a photo a shop sent to order: a written list is copied line by line; a photo of the
// products themselves goes to SAM 3.1 and Muse Spark. Channels and the panel's "try" both use it.
func (d *Desk) ReadShopPhoto(ctx context.Context, key string, img []byte, caption, ref string) (PhotoOrder, error) {
	rd, _, err := ReadPhoto(ctx, d.Meta, img, ref)
	if err != nil {
		return PhotoOrder{}, err
	}
	if rd.Kind == "products" {
		seen, _, err := SeeProducts(ctx, d.Meta, img, rd.Packaging, d.Cat.Brands(), ref)
		if err != nil {
			return PhotoOrder{}, err
		}
		if len(seen.Groups) == 0 {
			return PhotoOrder{}, errNoProducts
		}
		d.keep(key, "jpg", img)
		return PhotoOrder{Kind: "products", Text: strings.Join(PhotoLines(seen.Groups, caption), "\n"), Media: d.keep(key, "jpg", seen.Marked),
			Heard: PhotoLegend(seen.Groups), Seen: &seen}, nil
	}
	if len(rd.Lines) == 0 {
		return PhotoOrder{}, errNoLines
	}
	text := strings.Join(rd.Lines, "\n")
	if c := strings.TrimSpace(caption); c != "" {
		text = c + "\n" + text
	}
	return PhotoOrder{Kind: "photo", Text: text, Media: d.keep(key, "jpg", img),
		Heard: fmt.Sprintf("📷 <i>J'ai lu %d ligne(s) sur la photo.</i>", len(rd.Lines))}, nil
}

// SeeProducts finds and reads every product on a photo: one SAM 3.1 call per kind of packaging (in parallel),
// then one Muse Spark call that reads all the crops at once.
func SeeProducts(ctx context.Context, m *meta.Client, img []byte, packaging, brands []string, ref string) (Seen, float64, error) {
	src, _, err := image.Decode(bytes.NewReader(img))
	if err != nil {
		return Seen{}, 0, fmt.Errorf("photo: %v", err)
	}
	photo := shrink(src, samMaxSide)
	sent := encodeJPEG(photo)

	var nouns []string
	for _, p := range packaging {
		if p = strings.ToLower(strings.TrimSpace(p)); packagingWords[p] && len(nouns) < 3 {
			nouns = append(nouns, p)
		}
	}
	if len(nouns) == 0 {
		nouns = []string{"bottle"}
	}
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		boxes []meta.Box
		cost  float64
		errs  []error
	)
	for _, n := range nouns {
		wg.Add(1)
		go func(noun string) {
			defer wg.Done()
			b, c, err := m.Segment(ctx, sent, noun, ref)
			mu.Lock()
			defer mu.Unlock()
			cost += c
			if err != nil {
				errs = append(errs, err)
			}
			boxes = append(boxes, b...)
		}(n)
	}
	wg.Wait()
	if len(boxes) == 0 && len(errs) > 0 {
		return Seen{}, cost, errs[0]
	}
	boxes = keepProducts(boxes, photo.Bounds())
	if len(boxes) == 0 {
		return Seen{}, cost, nil
	}

	images := [][]byte{encodeJPEG(shrink(photo, wholeSide))}
	for _, b := range boxes {
		images = append(images, encodeJPEG(shrink(crop(photo, b), cropMaxSide)))
	}
	var out struct {
		Items []struct {
			N    int    `json:"n"`
			Said string `json:"said"`
		} `json:"items"`
	}
	c, err := m.JSON(ctx, meta.Spark, "see", ref, fmt.Sprintf(readCropsPrompt, strings.Join(brands, ", ")),
		fmt.Sprintf("The whole photo, then crops 1 to %d.", len(boxes)), images, 2000, &out)
	cost += c
	if err != nil {
		return Seen{}, cost, err
	}
	said := make([]string, len(boxes))
	for _, it := range out.Items {
		if it.N >= 1 && it.N <= len(boxes) {
			said[it.N-1] = strings.TrimSpace(it.Said)
		}
	}
	var seen Seen
	index := map[string]int{}
	for i, s := range said {
		if s == "" {
			continue
		}
		k := catalog.Normalize(s)
		g, ok := index[k]
		if !ok {
			g = len(seen.Groups)
			index[k] = g
			seen.Groups = append(seen.Groups, SeenGroup{Said: s})
		}
		seen.Groups[g].Count++
		seen.Groups[g].Boxes = append(seen.Groups[g].Boxes, boxes[i])
	}
	if len(seen.Groups) > 0 {
		seen.Marked = mark(photo, seen.Groups)
	}
	return seen, cost, nil
}

// keepProducts drops weak and tiny boxes and the same object found twice (by two kinds of packaging, or
// twice by one), and keeps the most confident ones, in reading order (top to bottom, left to right).
func keepProducts(boxes []meta.Box, r image.Rectangle) []meta.Box {
	area := float64(r.Dx() * r.Dy())
	sort.Slice(boxes, func(i, j int) bool { return boxes[i].Conf > boxes[j].Conf })
	var kept []meta.Box
	for _, b := range boxes {
		if b.Conf < minConf || float64((b.X2-b.X1)*(b.Y2-b.Y1)) < minArea*area {
			continue
		}
		dup := false
		for _, k := range kept {
			if iou(b, k) > 0.5 {
				dup = true
				break
			}
		}
		if !dup {
			kept = append(kept, b)
		}
		if len(kept) == maxProducts {
			break
		}
	}
	sort.Slice(kept, func(i, j int) bool {
		a, b := kept[i], kept[j]
		if rowA, rowB := (a.Y1+a.Y2)/2, (b.Y1+b.Y2)/2; abs(rowA-rowB) > (a.Y2-a.Y1)/2 {
			return rowA < rowB
		}
		return a.X1 < b.X1
	})
	return kept
}

func iou(a, b meta.Box) float64 {
	x1, y1, x2, y2 := max(a.X1, b.X1), max(a.Y1, b.Y1), min(a.X2, b.X2), min(a.Y2, b.Y2)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	inter := float64((x2 - x1) * (y2 - y1))
	return inter / (float64((a.X2-a.X1)*(a.Y2-a.Y1)) + float64((b.X2-b.X1)*(b.Y2-b.Y1)) - inter)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// captionQty is a caption that only gives a quantity ("3 cartons"), for a photo of one product.
var captionQty = regexp.MustCompile(`(?i)^\s*(\d+)\s*(cartons?|packs?|fardeaux?|caisses?|bouteilles?|unit[ée]s?|pi[eè]ces?|x)?\s*$`)

// PhotoLines turns the products seen into order lines. A caption that only gives a quantity applies to the
// product when there is one; any other caption is kept as its own line, as for a list.
func PhotoLines(groups []SeenGroup, caption string) []string {
	caption = strings.TrimSpace(caption)
	if m := captionQty.FindStringSubmatch(caption); m != nil && len(groups) == 1 {
		return []string{strings.TrimSpace(m[1] + " " + m[2] + " " + groups[0].Said)}
	}
	var lines []string
	if caption != "" {
		lines = append(lines, caption)
	}
	for _, g := range groups {
		lines = append(lines, fmt.Sprintf("%d %s", g.Count, g.Said))
	}
	return lines
}

// Squares are the colours of Telegram's square emoji, so the reply's legend matches the boxes on the photo.
var Squares = []struct {
	Emoji string
	C     color.RGBA
}{
	{"🟩", color.RGBA{120, 177, 89, 255}}, {"🟦", color.RGBA{85, 172, 238, 255}}, {"🟥", color.RGBA{221, 46, 68, 255}},
	{"🟧", color.RGBA{244, 144, 12, 255}}, {"🟪", color.RGBA{170, 142, 214, 255}}, {"🟨", color.RGBA{253, 203, 88, 255}},
	{"🟫", color.RGBA{193, 105, 79, 255}},
}

// PhotoLegend is the reply that goes with the marked photo: one line per product, in its box colour.
func PhotoLegend(groups []SeenGroup) string {
	var b strings.Builder
	fmt.Fprintf(&b, "📷 <b>J'ai reconnu %d produit(s) sur la photo :</b>", len(groups))
	for i, g := range groups {
		fmt.Fprintf(&b, "\n%s %d × %s", Squares[i%len(Squares)].Emoji, g.Count, esc(g.Said))
	}
	return b.String()
}

// mark draws every product's box in its group's colour, with the group's number in the corner.
func mark(photo image.Image, groups []SeenGroup) []byte {
	r := photo.Bounds()
	dst := image.NewRGBA(r)
	draw.Draw(dst, r, photo, r.Min, draw.Src)
	t := max(3, max(r.Dx(), r.Dy())/250)
	for gi, g := range groups {
		col := Squares[gi%len(Squares)].C
		for _, b := range g.Boxes {
			for _, e := range []image.Rectangle{
				image.Rect(b.X1, b.Y1, b.X2, b.Y1+t), image.Rect(b.X1, b.Y2-t, b.X2, b.Y2),
				image.Rect(b.X1, b.Y1, b.X1+t, b.Y2), image.Rect(b.X2-t, b.Y1, b.X2, b.Y2),
			} {
				draw.Draw(dst, e.Add(r.Min), &image.Uniform{col}, image.Point{}, draw.Src)
			}
			label(dst, r.Min.X+b.X1, r.Min.Y+b.Y1, gi+1, col, t*2)
		}
	}
	return encodeJPEG(dst)
}

// digits is a 3×5 font for the box numbers.
var digits = [10][5]string{
	{"###", "#.#", "#.#", "#.#", "###"}, {".#.", "##.", ".#.", ".#.", "###"}, {"###", "..#", "###", "#..", "###"},
	{"###", "..#", "###", "..#", "###"}, {"#.#", "#.#", "###", "..#", "..#"}, {"###", "#..", "###", "..#", "###"},
	{"###", "#..", "###", "#.#", "###"}, {"###", "..#", "..#", "..#", "..#"}, {"###", "#.#", "###", "#.#", "###"},
	{"###", "#.#", "###", "..#", "###"},
}

// label draws n in white on a filled square of colour col, at the top-left corner of a box.
func label(dst *image.RGBA, x, y, n int, col color.RGBA, px int) {
	s := fmt.Sprint(n)
	w, h := (len(s)*4+1)*px, 7*px
	bg := image.Rect(x, y, x+w, y+h).Intersect(dst.Bounds())
	draw.Draw(dst, bg, &image.Uniform{col}, image.Point{}, draw.Src)
	for i, ch := range s {
		for row, line := range digits[ch-'0'] {
			for c, on := range line {
				if on == '#' {
					cell := image.Rect(x+(1+i*4+c)*px, y+(1+row)*px, x+(2+i*4+c)*px, y+(2+row)*px)
					draw.Draw(dst, cell.Intersect(dst.Bounds()), image.White, image.Point{}, draw.Src)
				}
			}
		}
	}
}

// crop cuts a box out of the photo with a margin, so the label and the cap stay in the picture.
func crop(photo image.Image, b meta.Box) image.Image {
	r := photo.Bounds()
	mx, my := (b.X2-b.X1)/10, (b.Y2-b.Y1)/10
	c := image.Rect(b.X1-mx, b.Y1-my, b.X2+mx, b.Y2+my).Add(r.Min).Intersect(r)
	dst := image.NewRGBA(image.Rect(0, 0, c.Dx(), c.Dy()))
	draw.Draw(dst, dst.Bounds(), photo, c.Min, draw.Src)
	return dst
}

// shrink scales an image down (box filter) so its longest side is at most side pixels.
func shrink(src image.Image, side int) image.Image {
	r := src.Bounds()
	w, h := r.Dx(), r.Dy()
	if w <= side && h <= side {
		return src
	}
	nw, nh := side, h*side/w
	if h > w {
		nw, nh = w*side/h, side
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		y0, y1 := r.Min.Y+y*h/nh, r.Min.Y+max((y+1)*h/nh, y*h/nh+1)
		for x := 0; x < nw; x++ {
			x0, x1 := r.Min.X+x*w/nw, r.Min.X+max((x+1)*w/nw, x*w/nw+1)
			var sr, sg, sb, n uint32
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					cr, cg, cb, _ := src.At(xx, yy).RGBA()
					sr, sg, sb, n = sr+cr, sg+cg, sb+cb, n+1
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(sr / n >> 8), uint8(sg / n >> 8), uint8(sb / n >> 8), 255})
		}
	}
	return dst
}

func encodeJPEG(img image.Image) []byte {
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}
