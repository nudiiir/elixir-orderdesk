package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/desk"
	"github.com/nudiiir/elixir-orderdesk/internal/meta/metatest"
	"github.com/nudiiir/elixir-orderdesk/internal/profile"
	"github.com/nudiiir/elixir-orderdesk/internal/search"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// One supermarket order per channel goes through the shipped trained desk (bench/trained-desks), exactly as
// in the benchmark, with Muse Spark and Muse Voice Transcribe answering from a recording: a human voice note,
// a screenshot, French, Darja in Latin letters, and Darja mixed with French in Arabic script.
// Offline, no key. Re-record with ORDERDESK_RECORD=1 (see internal/meta/metatest).
func TestRecordedOrdersEndToEnd(t *testing.T) {
	want := map[string]struct{ right, wrong int }{ // what the recorded answers give, line by line against the answer key
		"SUP-O01": {1, 0}, "SUP-O03": {6, 0}, "SUP-O04": {4, 0}, "SUP-O07": {8, 0}, "SUP-O14": {5, 0},
	}
	_, noFFmpeg := exec.LookPath("ffmpeg") // the voice note needs ffmpeg to become a 16 kHz WAV
	raw, err := os.ReadFile("../../bench/orders/orders.json")
	if err != nil {
		t.Fatal(err)
	}
	var all []order
	json.Unmarshal(raw, &all)

	dd := t.TempDir()
	copyFile("../../bench/trained-desks/supermarket/orderdesk.db", filepath.Join(dd, "orderdesk.db"))
	db, err := store.Open(filepath.Join(dd, "orderdesk.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var todo []order
	var shops []catalog.Customer
	for _, o := range all {
		if _, ok := want[o.ID]; ok {
			todo = append(todo, o)
			shops = append(shops, catalog.Customer{ID: shopID(o.Shop), Name: o.Shop})
		}
	}
	catalog.ReplaceCustomers(db, shops)
	cat := catalog.New(db)
	mc, tape := metatest.Client(t, db, "orders-supermarket")
	d := &desk.Desk{DB: db, Cat: cat, Meta: mc, DataDir: dd, Threshold: 0.9}
	if p := profile.Load(db); p != nil {
		p.Install(cat)
		d.Search = search.Build(cat, p.Search)
	}
	d.Autonomy = desk.NewAutonomy(0.99)
	json.Unmarshal([]byte(db.Setting("autonomy", "{}")), d.Autonomy)
	newMsgID = func(id string) string { return "msg-" + id }

	ctx := context.Background()
	skipped := 0
	for _, o := range todo {
		t.Run(o.ID, func(t *testing.T) {
			if o.Channel == "voice" && noFFmpeg != nil {
				skipped++
				t.Skipf("voice note not tested: ffmpeg is not on the PATH (%v)", noFFmpeg)
			}
			r := run(ctx, d, cat, mc, db, o, "../../bench/orders/voice", "../../bench/orders/screens")
			if r.Error != "" {
				t.Fatalf("%s: %s", o.ID, r.Error)
			}
			c := map[string]int{}
			for _, l := range r.Lines {
				c[l.Outcome]++
			}
			t.Logf("%s (%s): right %d, asked %d, wrong %d, missed %d of %d · $%.4f · read %q", o.ID, o.Channel, c["right"], c["asked"], c["wrong"], c["missed"], len(r.Lines), r.Cost, clipS(r.Read, 90))
			w := want[o.ID]
			if c["wrong"] != w.wrong {
				t.Errorf("%s: %d lines booked to the wrong product, want %d", o.ID, c["wrong"], w.wrong)
			}
			if w.right >= 0 && c["right"] != w.right {
				t.Errorf("%s: %d lines booked right, want %d", o.ID, c["right"], w.right)
			}
			if r.Cost <= 0 {
				t.Errorf("%s: no model cost metered", o.ID)
			}
		})
	}
	// the voice note went to Muse Voice Transcribe with the catalogue's brands as keywords
	for _, rq := range tape.Requests {
		if rq.Path == "/asr/transcribe" && len(rq.Keywords) < 20 {
			t.Errorf("transcription sent %d brand keywords, want the catalogue's brands", len(rq.Keywords))
		}
	}
	if !metatest.Recording() {
		if n := tape.Unused(); n != 0 && skipped == 0 {
			t.Errorf("%d recorded calls were never made", n)
		}
	}
}

func clipS(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return fmt.Sprint(s)
}
