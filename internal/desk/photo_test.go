package desk

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

func TestPhotoLines(t *testing.T) {
	one := []SeenGroup{{Said: "Selecto 2L", Count: 2}}
	if got := PhotoLines(one, " 3 cartons "); len(got) != 1 || got[0] != "3 cartons Selecto 2L" {
		t.Fatalf("a quantity caption applies to the one product: %q", got)
	}
	two := []SeenGroup{{Said: "Selecto 2L", Count: 2}, {Said: "Ifri 1.5L", Count: 1}}
	if got := PhotoLines(two, "3 cartons"); len(got) != 3 || got[0] != "3 cartons" || got[1] != "2 Selecto 2L" || got[2] != "1 Ifri 1.5L" {
		t.Fatalf("with two products the caption stays its own line: %q", got)
	}
	if got := PhotoLines(one, ""); len(got) != 1 || got[0] != "2 Selecto 2L" {
		t.Fatalf("units seen: %q", got)
	}
}

func TestKeepProducts(t *testing.T) {
	r := image.Rect(0, 0, 1000, 800)
	got := keepProducts([]meta.Box{
		{X1: 500, Y1: 100, X2: 600, Y2: 400, Conf: 0.9},
		{X1: 505, Y1: 105, X2: 600, Y2: 400, Conf: 0.6}, // the same bottle, found as a second kind of packaging
		{X1: 100, Y1: 110, X2: 200, Y2: 400, Conf: 0.8},
		{X1: 100, Y1: 500, X2: 200, Y2: 780, Conf: 0.7}, // the shelf below
		{X1: 300, Y1: 100, X2: 400, Y2: 400, Conf: 0.2}, // too unsure
		{X1: 0, Y1: 0, X2: 20, Y2: 20, Conf: 0.9},       // a speck
	}, r)
	if len(got) != 3 {
		t.Fatalf("want 3 products, got %+v", got)
	}
	if got[0].X1 != 100 || got[0].Y1 != 110 || got[1].X1 != 500 || got[2].Y1 != 500 {
		t.Fatalf("reading order is top shelf left to right, then the next shelf: %+v", got)
	}
}

func TestMarkAndShrink(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3000, 2000))
	draw.Draw(src, src.Bounds(), &image.Uniform{color.RGBA{200, 200, 200, 255}}, image.Point{}, draw.Src)
	small := shrink(src, samMaxSide)
	if b := small.Bounds(); b.Dx() != 1600 || b.Dy() != 1066 {
		t.Fatalf("shrink: %v", b)
	}
	out := mark(small, []SeenGroup{{Said: "Selecto 2L", Count: 1, Boxes: []meta.Box{{X1: 100, Y1: 100, X2: 400, Y2: 900, Conf: 1}}},
		{Said: "Ifri 1.5L", Count: 1, Boxes: []meta.Box{{X1: 1500, Y1: 1000, X2: 1600, Y2: 1066, Conf: 1}}}})
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if r, g, _, _ := img.At(101, 500).RGBA(); r>>8 > 160 || g>>8 < 150 {
		t.Fatalf("the first box is drawn in the first colour (green), got %d %d", r>>8, g>>8)
	}
}

// TestSeeProductsLive runs SAM 3.1 and Muse Spark on a real photo:
//
//	META_MODEL_API_KEY=… ORDERDESK_PHOTO=shelf.jpg go test ./internal/desk -run Live -v
func TestSeeProductsLive(t *testing.T) {
	path, key := os.Getenv("ORDERDESK_PHOTO"), os.Getenv("META_MODEL_API_KEY")
	if path == "" || key == "" {
		t.Skip("ORDERDESK_PHOTO and META_MODEL_API_KEY not set")
	}
	img, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := meta.New(key, db)
	ctx := context.Background()
	rd, c1, err := ReadPhoto(ctx, m, img, "live")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("photo kind %q, packaging %v, lines %v ($%.4f)", rd.Kind, rd.Packaging, rd.Lines, c1)
	if rd.Kind != "products" {
		return
	}
	brands := []string{"Hamoud Boualem", "Selecto", "Ifri", "Coca-Cola", "Pepsi", "Rouiba", "Candia", "Soummam", "Ramy", "Fanta"}
	seen, c2, err := SeeProducts(ctx, m, img, rd.Packaging, brands, "live")
	if err != nil {
		t.Fatal(err)
	}
	for i, g := range seen.Groups {
		t.Logf("%s %d × %s", Squares[i%len(Squares)].Emoji, g.Count, g.Said)
	}
	t.Logf("lines %q, cost $%.4f", PhotoLines(seen.Groups, ""), c2)
	if out := os.Getenv("ORDERDESK_PHOTO_OUT"); out != "" && seen.Marked != nil {
		os.WriteFile(out, seen.Marked, 0o644)
	}
}
