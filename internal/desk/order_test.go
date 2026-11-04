package desk

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// testDesk is a desk on a small catalogue with no Meta key and no ERP.
func testDesk(t *testing.T) *Desk {
	db, err := store.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	catalog.ReplaceProducts(db, []catalog.Product{
		{ID: "1", Name: "Moutarde Amora 250g", Brand: "Amora", Packs: []catalog.Pack{{Name: "Carton", FactorMilli: 12000}}},
		{ID: "2", Name: "Nesquik Cacao 750g", Brand: "Nestle"},
		{ID: "3", Name: "Selecto 2L", Brand: "Hamoud"},
	})
	catalog.ReplaceCustomers(db, []catalog.Customer{{ID: "C1", Name: "Superette El Baraka"}})
	return &Desk{DB: db, Cat: catalog.New(db), Meta: meta.New("", db), DataDir: t.TempDir()}
}

// With no Meta key, a chatty Darja order is still read: greetings dropped, items booked or asked, never an error.
func TestIngestWithoutKey(t *testing.T) {
	d := testDesk(t)
	res, err := d.Ingest(context.Background(), "panel:C1", "C1", store.NewID("msg"), "text",
		"salam khouya, ab3athli 3 kartonat moutarde w zouj nesquik", "")
	if err != nil {
		t.Fatalf("the message failed: %v", err)
	}
	if res.Draft == "" || res.Ex.Method != "lenient" || len(res.Lines) != 2 {
		t.Fatalf("draft %q, method %q, lines %+v", res.Draft, res.Ex.Method, res.Lines)
	}
	if l := res.Lines[0]; l.Qty != 3 || l.Unit != "carton" || l.Said != "moutarde" {
		t.Errorf("line 1: %+v", l)
	}
	if l := res.Lines[1]; l.Qty != 2 || l.Said != "nesquik" {
		t.Errorf("line 2: %+v", l)
	}
	// what it cannot read is asked to a person, not lost
	res, err = d.Ingest(context.Background(), "panel:C1", "C1", store.NewID("msg"), "text", "0 selecto\nchhal el moutarde ?", "")
	if err != nil || len(res.Lines) != 2 {
		t.Fatalf("err %v, lines %+v", err, res.Lines)
	}
	for _, l := range res.Lines {
		if l.Status != "unknown" || l.Checks["unread"] == "" || !strings.Contains(l.Question, l.Said) {
			t.Errorf("not asked: %+v", l)
		}
	}
}

// fakeERP books orders in memory; slow makes CreateOrder create the order and still fail, like a timeout.
type fakeERP struct {
	adapter.ERP
	mu      sync.Mutex
	orders  map[string]adapter.OrderRef
	created int
	notes   []string
	slow    bool
	finder  bool
}

func (f *fakeERP) Name() string { return "fake" }

func (f *fakeERP) CreateOrder(ctx context.Context, o adapter.Order) (adapter.OrderRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created++
	ref := adapter.OrderRef{ID: o.ExternalRef, Number: "SO" + strings.Repeat("0", f.created), TotalTTC: 100}
	f.orders[o.ExternalRef] = ref
	f.notes = append(f.notes, o.Notes)
	if f.slow {
		f.slow = false
		return adapter.OrderRef{}, errors.New("ERP unreachable: timeout")
	}
	return ref, nil
}

type findingERP struct{ *fakeERP }

func (f findingERP) FindOrder(ctx context.Context, ref string) (adapter.OrderRef, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.orders[ref]
	return r, ok, nil
}

func bookable(t *testing.T, d *Desk) string {
	res, err := d.Ingest(context.Background(), "tg:42", "C1", store.NewID("msg"), "text", "3 cartons moutarde amora", "")
	if err != nil || len(res.Lines) != 1 || res.Lines[0].Status != "ok" {
		t.Fatalf("err %v, lines %+v", err, res.Lines)
	}
	d.DB.Exec(`INSERT INTO chats (chat_key, customer_id, name, role, linked_at) VALUES ('tg:42', 'C1', 'Telegram', 'shop', '')`)
	return res.Draft
}

func TestConfirmTwice(t *testing.T) {
	d := testDesk(t)
	f := &fakeERP{orders: map[string]adapter.OrderRef{}}
	draft := bookable(t, d) // read with no ERP connected: the fake only books
	d.ERP = func() adapter.ERP { return f }
	var wg sync.WaitGroup
	refs := make([]adapter.OrderRef, 3)
	for i := range refs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			if refs[i], err = d.Confirm(context.Background(), draft, "shop"); err != nil {
				t.Errorf("confirm %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if f.created != 1 || refs[0] != refs[1] || refs[1] != refs[2] {
		t.Fatalf("three taps booked %d orders: %+v", f.created, refs)
	}
	if n := f.notes[0]; strings.Contains(n, "Telegram") || !strings.Contains(n, "Superette El Baraka") || !strings.Contains(n, "(tg)") {
		t.Errorf("the ERP note names the shop and the channel only: %q", n)
	}
}

// The ERP created the order and the answer was lost: the next tap finds it by its reference.
func TestConfirmAfterTimeout(t *testing.T) {
	d := testDesk(t)
	f := &fakeERP{orders: map[string]adapter.OrderRef{}, slow: true}
	draft := bookable(t, d)
	d.ERP = func() adapter.ERP { return findingERP{f} }
	if _, err := d.Confirm(context.Background(), draft, "shop"); err == nil {
		t.Fatal("the first attempt timed out")
	}
	var status string
	d.DB.QueryRow(`SELECT status FROM drafts WHERE id = ?`, draft).Scan(&status)
	if status != "open" {
		t.Fatalf("after a failed booking the draft is %s, want open (the shop may tap again)", status)
	}
	ref, err := d.Confirm(context.Background(), draft, "shop")
	if err != nil || f.created != 1 || ref.Number != "SO0" {
		t.Fatalf("second tap: %+v %v, %d orders created", ref, err, f.created)
	}
}

// On an order, a line Muse Spark picked alone teaches nothing on its own; a person's choice does.
func TestLearnOnlyChecked(t *testing.T) {
	d := testDesk(t)
	ai := Line{ID: "l1", Said: "moutard", Norm: "moutard", ProductID: "1", Level: resolve.LAI, Status: "ok", Checks: map[string]string{}}
	person := Line{ID: "l2", Said: "cacao", Norm: "cacao", ProductID: "2", Level: resolve.LPerson, Status: "ok", Checks: map[string]string{}}
	d.DB.SetSetting("alias_trust_after", "1")
	d.learnLines([]Line{ai, person}, "order", "customer", "C1", "shop")
	var gt int
	d.DB.QueryRow(`SELECT COUNT(*) FROM ground_truth`).Scan(&gt)
	if gt != 1 {
		t.Fatalf("%d ground-truth lines, want only the person's", gt)
	}
	if a, ok := d.Cat.Trusted("moutard", "C1", ""); ok {
		t.Fatalf("an unchecked AI pick trusted a wording: %+v", a)
	}
	if hs := d.Cat.Hints("moutard", "C1", ""); len(hs) != 0 {
		t.Fatalf("an unchecked AI pick became a hint: %v", hs)
	}
	var state string
	var wit int
	d.DB.QueryRow(`SELECT state, witnesses FROM aliases WHERE text = 'moutard'`).Scan(&state, &wit)
	if state != "weak" || wit != 0 {
		t.Fatalf("weak evidence: state %q, %d witnesses", state, wit)
	}
	// a person later confirms the same wording: now it counts
	d.learnLines([]Line{{ID: "l3", Said: "moutard", Norm: "moutard", ProductID: "1", Level: resolve.LPerson, Status: "ok", Checks: map[string]string{}}},
		"order", "customer", "C1", "shop")
	if _, ok := d.Cat.Trusted("moutard", "C1", ""); !ok {
		t.Fatal("a person's confirmation trusts the wording")
	}
}

// An order with a line still asked is not booked without it: answer or remove it first.
func TestConfirmRefusesOpenLines(t *testing.T) {
	d := testDesk(t)
	f := &fakeERP{orders: map[string]adapter.OrderRef{}}
	res, err := d.Ingest(context.Background(), "tg:42", "C1", store.NewID("msg"), "text", "3 cartons moutarde amora\n2 zzz inconnu", "")
	if err != nil || len(res.Lines) != 2 || res.Lines[1].Status == "ok" {
		t.Fatalf("err %v, lines %+v", err, res.Lines)
	}
	d.ERP = func() adapter.ERP { return f }
	if _, err := d.Confirm(context.Background(), res.Draft, "clerk"); err == nil || f.created != 0 {
		t.Fatalf("booked with a line still asked: err %v, %d orders", err, f.created)
	}
}

// One common word that fits one product name by chance is not booked free.
func TestOneGenericWordIsNotBooked(t *testing.T) {
	d := testDesk(t)
	res, err := d.Ingest(context.Background(), "panel:C1", "C1", store.NewID("msg"), "text", "3 cacao", "")
	if err != nil || len(res.Lines) != 1 {
		t.Fatalf("err %v, lines %+v", err, res.Lines)
	}
	if l := res.Lines[0]; l.Status == "ok" {
		t.Errorf("one common word booked %s with no check", l.ProductID)
	}
}
