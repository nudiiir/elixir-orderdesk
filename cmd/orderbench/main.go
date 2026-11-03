// orderbench measures shop orders end to end on a trained desk: every order goes through the same path as
// a live one (a voice note through Muse Voice Transcribe with the catalogue's brands, a screenshot through
// Muse Spark's list reader, a typed message straight in), then Ingest extracts and resolves its items.
// Nothing is answered or learned. Each line of the answer key is scored: booked right (and with the right
// quantity and unit), asked, booked wrong, or missed by the reader; lines the reader invented are counted.
//
//	orderbench -orders orders.json -industry pharmacy -desk ./trained/desk -voice ./voice -screens ./screens -out ./run -keys keys.env
//
// -desk is copied first: the trained desk itself is never changed.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/desk"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/profile"
	"github.com/nudiiir/elixir-orderdesk/internal/search"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

type keyLine struct {
	ProductID string  `json:"product_id"`
	Product   string  `json:"product"`
	Qty       float64 `json:"qty"`
	Unit      string  `json:"unit"`
	Say       string  `json:"say"`
}

type order struct {
	ID       string    `json:"id"`
	Industry string    `json:"industry"`
	Shop     string    `json:"shop"`
	Channel  string    `json:"channel"`
	Lines    []keyLine `json:"lines"`
	Text     string    `json:"text"`
	Audio    string    `json:"audio"`
	AudioSrc string    `json:"audio_source"`
	Image    string    `json:"image"`
}

type lineScore struct {
	Product string `json:"product"`
	Outcome string `json:"outcome"` // right, asked, wrong, missed
	QtyOK   bool   `json:"qty_ok"`
	Got     string `json:"got,omitempty"`
	GotQty  string `json:"got_qty,omitempty"`
	Level   string `json:"level,omitempty"`
}

type result struct {
	ID       string      `json:"id"`
	Channel  string      `json:"channel"`
	AudioSrc string      `json:"audio_source,omitempty"`
	Read     string      `json:"read"` // what the reader produced (transcript, photo lines or the message)
	Lines    []lineScore `json:"lines"`
	Extra    int         `json:"extra"` // items read that are in no line of the order
	Cost     float64     `json:"cost"`
	Error    string      `json:"error,omitempty"`
}

func main() {
	ordersPath := flag.String("orders", "orders.json", "orders with answer keys")
	industry := flag.String("industry", "", "only this industry")
	deskDir := flag.String("desk", "", "trained desk directory (with orderdesk.db), copied before use")
	voiceDir := flag.String("voice", "", "voice notes directory")
	screensDir := flag.String("screens", "", "screenshots directory")
	out := flag.String("out", "orderbench-run", "output directory (replaced)")
	keys := flag.String("keys", "", "keys file with META_MODEL_API_KEY")
	only := flag.String("only", "", "only these channels, comma-separated")
	n := flag.Int("n", 0, "at most this many orders (0 = all)")
	flag.Parse()
	if *keys != "" {
		loadKeys(*keys)
	}
	var orders []order
	b, err := os.ReadFile(*ordersPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(b, &orders); err != nil {
		log.Fatal(err)
	}
	chans := map[string]bool{}
	for _, c := range strings.Split(*only, ",") {
		if c = strings.TrimSpace(c); c != "" {
			chans[c] = true
		}
	}
	var todo []order
	for _, o := range orders {
		if (*industry == "" || o.Industry == *industry) && (len(chans) == 0 || chans[o.Channel]) {
			todo = append(todo, o)
		}
	}
	if *n > 0 && len(todo) > *n {
		todo = todo[:*n]
	}

	os.RemoveAll(*out)
	dd := filepath.Join(*out, "desk")
	os.MkdirAll(dd, 0o755)
	for _, f := range []string{"orderdesk.db", "orderdesk.db-wal", "orderdesk.db-shm"} {
		copyFile(filepath.Join(*deskDir, f), filepath.Join(dd, f))
	}
	db, err := store.Open(filepath.Join(dd, "orderdesk.db"))
	if err != nil {
		log.Fatal(err)
	}
	db.SetSetting("ai_budget_day", "25")
	var shops []catalog.Customer
	seen := map[string]bool{}
	for _, o := range todo {
		if !seen[o.Shop] {
			seen[o.Shop] = true
			shops = append(shops, catalog.Customer{ID: shopID(o.Shop), Name: o.Shop})
		}
	}
	catalog.ReplaceCustomers(db, shops)
	cat := catalog.New(db)
	mc := meta.New(os.Getenv("META_MODEL_API_KEY"), db)
	d := &desk.Desk{DB: db, Cat: cat, Meta: mc, DataDir: dd, Threshold: 0.9}
	if p := profile.Load(db); p != nil {
		p.Install(cat)
		d.Search = search.Build(cat, p.Search)
	}
	d.Autonomy = desk.NewAutonomy(0.99)
	json.Unmarshal([]byte(db.Setting("autonomy", "{}")), d.Autonomy)
	log.Printf("desk: %d products, autonomy unlocked: %v, %d orders to read", len(cat.Products()), d.Autonomy.Unlocked(), len(todo))

	ctx := context.Background()
	var results []result
	for _, o := range todo {
		r := run(ctx, d, cat, mc, db, o, *voiceDir, *screensDir)
		results = append(results, r)
		c := count(r)
		log.Printf("[%s] %s: right %d/%d (qty %d), asked %d, wrong %d, missed %d, extra %d, $%.4f %s",
			o.Channel, o.ID, c["right"], len(r.Lines), c["qty"], c["asked"], c["wrong"], c["missed"], r.Extra, r.Cost, r.Error)
	}
	rep := map[string]any{"results": results, "summary": summary(results)}
	os.WriteFile(filepath.Join(*out, "report.json"), []byte(store.JSON(rep)), 0o644)
	fmt.Print(markdown(results))
	db.Close()
}

func run(ctx context.Context, d *desk.Desk, cat *catalog.Catalog, mc *meta.Client, db *store.DB, o order, voiceDir, screensDir string) result {
	r := result{ID: o.ID, Channel: o.Channel, AudioSrc: o.AudioSrc}
	key, msgID := "bench:"+o.ID, store.NewID("msg")
	kind, text := "text", o.Text
	fail := func(err error) result {
		r.Error = err.Error()
		for _, k := range o.Lines {
			r.Lines = append(r.Lines, lineScore{Product: k.Product, Outcome: "missed"})
		}
		db.QueryRow(`SELECT COALESCE(SUM(cost),0) FROM ai_usage WHERE ref = ?`, msgID).Scan(&r.Cost)
		return r
	}
	switch o.Channel {
	case "voice":
		audio, err := os.ReadFile(filepath.Join(voiceDir, o.Audio))
		if err != nil {
			return fail(err)
		}
		wav, err := toWAV(ctx, audio)
		if err != nil {
			return fail(err)
		}
		// as the live desk does: the catalogue's brands as keywords
		text, _, err = mc.TranscribeWords(ctx, wav, msgID, cat.Keywords(db.SettingInt("voice_keywords", 100), meta.MaxKeyword))
		if err != nil {
			return fail(err)
		}
		kind = "voice"
	case "screenshot":
		img, err := os.ReadFile(filepath.Join(screensDir, o.Image))
		if err != nil {
			return fail(err)
		}
		po, err := d.ReadShopPhoto(ctx, key, img, "", msgID)
		if err != nil {
			return fail(err)
		}
		kind, text = po.Kind, po.Text
	}
	r.Read = text
	res, err := d.Ingest(ctx, key, shopID(o.Shop), msgID, kind, text, "")
	if err != nil && len(res.Lines) == 0 {
		return fail(err)
	}
	db.QueryRow(`SELECT COALESCE(SUM(cost),0) FROM ai_usage WHERE ref = ?`, msgID).Scan(&r.Cost)

	// score: booked lines matched to the key by product first, then what is left
	used := make([]bool, len(res.Lines))
	scores := make([]*lineScore, len(o.Lines))
	for i, k := range o.Lines {
		for j, l := range res.Lines {
			if !used[j] && l.Status == "ok" && l.ProductID == k.ProductID {
				used[j] = true
				scores[i] = &lineScore{Product: k.Product, Outcome: "right", Level: l.Level, GotQty: fmt.Sprintf("%g %s", l.Qty, l.Unit),
					QtyOK: l.Qty == k.Qty && unitOK(l.Unit, k.Unit)}
				break
			}
		}
	}
	for i, k := range o.Lines { // asked, with the right product among the options
		if scores[i] != nil {
			continue
		}
		for j, l := range res.Lines {
			if !used[j] && l.Status != "ok" && contains(l.Options, k.ProductID) {
				used[j] = true
				scores[i] = &lineScore{Product: k.Product, Outcome: "asked", Got: l.Said, GotQty: fmt.Sprintf("%g %s", l.Qty, l.Unit)}
				break
			}
		}
	}
	var leftOK, leftAsk []desk.Line
	for j, l := range res.Lines {
		if used[j] || l.Status == "removed" {
			continue
		}
		if l.Status == "ok" {
			leftOK = append(leftOK, l)
		} else {
			leftAsk = append(leftAsk, l)
		}
	}
	for i, k := range o.Lines { // what is left of the key: booked to another product, asked, or never read
		if scores[i] != nil {
			continue
		}
		switch {
		case len(leftOK) > 0:
			p := cat.Get(leftOK[0].ProductID)
			got := leftOK[0].ProductID
			if p != nil {
				got = p.Name
			}
			scores[i] = &lineScore{Product: k.Product, Outcome: "wrong", Got: got + " (said: " + leftOK[0].Said + ")", Level: leftOK[0].Level}
			leftOK = leftOK[1:]
		case len(leftAsk) > 0:
			scores[i] = &lineScore{Product: k.Product, Outcome: "asked", Got: leftAsk[0].Said}
			leftAsk = leftAsk[1:]
		default:
			scores[i] = &lineScore{Product: k.Product, Outcome: "missed"}
		}
	}
	for _, s := range scores {
		r.Lines = append(r.Lines, *s)
	}
	r.Extra = len(leftOK) + len(leftAsk)
	db.Exec(`UPDATE drafts SET status = 'cancelled' WHERE chat_key = ?`, key)
	return r
}

// unitOK: the unit the desk read against the unit the shop counted in. A count of units may be said with no unit.
func unitOK(got, want string) bool {
	got = strings.ToLower(got)
	switch want {
	case "carton":
		return got == "carton"
	case "pack":
		return got == "pack"
	}
	return got == "" || got == "unit"
}

func count(r result) map[string]int {
	c := map[string]int{}
	for _, l := range r.Lines {
		c[l.Outcome]++
		if l.Outcome == "right" && l.QtyOK {
			c["qty"]++
		}
	}
	return c
}

type agg struct {
	Orders, Lines, Right, QtyOK, Asked, Wrong, Missed, Extra, Errors int
	Cost                                                         float64
}

func summary(rs []result) map[string]*agg {
	m := map[string]*agg{}
	add := func(k string, r result) {
		a := m[k]
		if a == nil {
			a = &agg{}
			m[k] = a
		}
		c := count(r)
		a.Orders++
		a.Lines += len(r.Lines)
		a.Right += c["right"]
		a.QtyOK += c["qty"]
		a.Asked += c["asked"]
		a.Wrong += c["wrong"]
		a.Missed += c["missed"]
		a.Extra += r.Extra
		a.Cost += r.Cost
		if r.Error != "" {
			a.Errors++
		}
	}
	for _, r := range rs {
		add("all", r)
		add(r.Channel, r)
		if r.Channel == "voice" && r.AudioSrc != "" {
			add("voice: "+r.AudioSrc, r)
		}
	}
	return m
}

func markdown(rs []result) string {
	s := summary(rs)
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("| channel | orders | lines | booked right | right qty+unit | asked | **wrong** | missed by reader | extra | Meta cost |\n|---|---|---|---|---|---|---|---|---|---|\n")
	pct := func(a, t int) string {
		if t == 0 {
			return "—"
		}
		return fmt.Sprintf("%d (%.0f %%)", a, 100*float64(a)/float64(t))
	}
	for _, k := range keys {
		a := s[k]
		fmt.Fprintf(&b, "| %s | %d | %d | %s | %s | %s | **%d** | %s | %d | $%.3f |\n", k, a.Orders, a.Lines, pct(a.Right, a.Lines), pct(a.QtyOK, a.Right),
			pct(a.Asked, a.Lines), a.Wrong, pct(a.Missed, a.Lines), a.Extra, a.Cost)
	}
	b.WriteString("\n## Wrong bookings\n\n")
	for _, r := range rs {
		for _, l := range r.Lines {
			if l.Outcome == "wrong" {
				fmt.Fprintf(&b, "- %s (%s): %s → %s [%s]\n", r.ID, r.Channel, l.Product, l.Got, l.Level)
			}
		}
	}
	return b.String()
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func shopID(name string) string { return "shop-" + strings.ToLower(strings.ReplaceAll(name, " ", "-")) }

func toWAV(ctx context.Context, audio []byte) ([]byte, error) {
	bin := os.Getenv("FFMPEG")
	if bin == "" {
		bin = "ffmpeg"
	}
	cmd := exec.CommandContext(ctx, bin, "-hide_banner", "-loglevel", "error", "-i", "pipe:0", "-ac", "1", "-ar", "16000", "-f", "wav", "pipe:1")
	cmd.Stdin = bytes.NewReader(audio)
	var outb, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outb, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %v %s", err, errb.String())
	}
	return outb.Bytes(), nil
}

func copyFile(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	o, err := os.Create(dst)
	if err != nil {
		log.Fatal(err)
	}
	defer o.Close()
	io.Copy(o, in)
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
