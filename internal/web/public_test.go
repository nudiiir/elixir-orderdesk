package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/desk"
	"github.com/nudiiir/elixir-orderdesk/internal/engineer"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// What must never reach a visitor of a public desk.
const (
	shopPhone  = "0555123456"
	chatID     = "tg:987654321"
	tgName     = "Karim Benali"
	ownerChat  = "tg:111222333"
	ownerName  = "Nadia Owner"
	pairCode   = "QX7PAIR"
	msgText    = "salam khoya ab3athli 3 cartons selecto"
	supPhone   = "0213777888"
	mediaBytes = "voice-note-bytes"
)

// testDesk is a desk with one shop, its linked Telegram chat, a pairing code, an order from a voice
// note, an owner chat and the events all of that leaves.
func testDesk(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "desk.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	now := store.Now()
	media := filepath.Join(dir, "media", "tg_987654321", "voice1.ogg")
	os.MkdirAll(filepath.Dir(media), 0o755)
	os.WriteFile(media, []byte(mediaBytes), 0o644)
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO products (id, name, brand, synced_at) VALUES ('P1', 'Selecto 2L', 'Hamoud', ?)`, []any{now}},
		{`INSERT INTO customers (id, name, phone) VALUES ('C1', 'Superette Ennour', ?)`, []any{shopPhone}},
		{`INSERT INTO suppliers (id, name, phone) VALUES ('S1', 'Hamoud Boualem', ?)`, []any{supPhone}},
		{`INSERT INTO chats (chat_key, customer_id, name, role, linked_at) VALUES (?, 'C1', ?, 'shop', ?)`, []any{chatID, tgName, now}},
		{`INSERT INTO chats (chat_key, customer_id, name, role, linked_at) VALUES (?, '', ?, 'owner', ?)`, []any{ownerChat, ownerName, now}},
		{`INSERT INTO pair_codes (code, customer_id, created_at, role) VALUES (?, 'C1', ?, 'shop')`, []any{pairCode, now}},
		{`INSERT INTO drafts (id, chat_key, customer_id, status, created_at, updated_at) VALUES ('D1', ?, 'C1', 'open', ?, ?)`, []any{chatID, now, now}},
		{`INSERT INTO messages (id, chat_key, customer_id, kind, text, media_path, extraction, created_at) VALUES ('M1', ?, 'C1', 'voice', ?, ?, 'ai', ?)`,
			[]any{chatID, msgText, media, now}},
		{`INSERT INTO lines (id, draft_id, message_id, position, said, norm, qty, product_id, level, status, created_at)
			VALUES ('L1', 'D1', 'M1', 1, '3 cartons selecto', 'selecto', 3, 'P1', 'ai', 'ok', ?)`, []any{now}},
	} {
		if _, err := db.Exec(q.sql, q.args...); err != nil {
			t.Fatal(q.sql, err)
		}
	}
	db.Event("chat.linked", "C1", tgName, map[string]string{"chat": chatID, "customer": "Superette Ennour"})
	db.Event("chat.linked", "owner", ownerName, map[string]string{"chat": ownerChat, "role": "owner"})
	db.Event("chat.unlinked", chatID, tgName, map[string]string{"chat": chatID})
	db.Event("order.created", "D1", "shop", map[string]any{"number": "S00001", "lines": 1, "erp": "Elixir"})
	db.Event("settings.changed", "settings", "clerk", map[string]string{"erp_url": "http://10.0.0.5"})
	cat := catalog.New(db)
	mc := meta.New("", db)
	return &Server{DB: db, Cat: cat, Meta: mc, DataDir: dir, PublicReadOnly: true,
		Desk: &desk.Desk{DB: db, Cat: cat, Meta: mc}, Eng: &engineer.Engineer{DB: db, Cat: cat, Meta: mc},
		ERP: func() adapter.ERP { return nil }}
}

// get calls a route as a visitor from another computer, or as the owner on this one.
func get(h http.Handler, path string, visitor bool) (int, string) {
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8610"+path, nil)
	r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:8610"
	if visitor {
		r.RemoteAddr, r.Host = "198.51.100.7:4000", "orderdesk.example.com"
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code, w.Body.String()
}

func TestPublicGetsCarryNoPersonalData(t *testing.T) {
	s := testDesk(t)
	h := s.Handler()
	secrets := []string{shopPhone, chatID, "987654321", tgName, ownerChat, "111222333", ownerName, pairCode, msgText, supPhone, mediaBytes, "/media?", "10.0.0.5"}
	routes := []string{"/api/overview", "/api/drafts", "/api/drafts?kind=order&status=open", "/api/drafts/D1", "/api/customers", "/api/events",
		"/api/events?limit=500", "/api/suppliers", "/api/products", "/api/products/P1", "/api/rules", "/api/engineer", "/api/learning",
		"/api/settings", "/api/owner", "/media?path=tg_987654321/voice1.ogg"}
	for _, p := range routes {
		code, body := get(h, p, true)
		for _, x := range secrets {
			if strings.Contains(body, x) {
				t.Errorf("public GET %s (%d) leaks %q: %s", p, code, x, body)
			}
		}
	}
	for _, p := range []string{"/api/owner", "/media?path=tg_987654321/voice1.ogg"} {
		if code, _ := get(h, p, true); code != 404 {
			t.Errorf("public GET %s: want 404, got %d", p, code)
		}
	}
	// the panel still shows how orders were resolved
	if _, body := get(h, "/api/drafts/D1", true); !strings.Contains(body, "3 cartons selecto") || !strings.Contains(body, "Superette Ennour") {
		t.Errorf("public draft lost its line words or its shop: %s", body)
	}
	if _, body := get(h, "/api/events", true); !strings.Contains(body, "S00001") || !strings.Contains(body, "Superette Ennour") {
		t.Errorf("public events lost the order or the shop: %s", body)
	}
	if _, body := get(h, "/api/overview", true); !strings.Contains(body, `"public":true`) {
		t.Errorf("the panel cannot tell it is public: %s", body)
	}
}

func TestLocalGetsKeepEverything(t *testing.T) {
	s := testDesk(t)
	h := s.Handler()
	for p, want := range map[string][]string{
		"/api/customers":                      {shopPhone, chatID, tgName, pairCode},
		"/api/drafts":                         {chatID},
		"/api/drafts/D1":                      {chatID, msgText, "/media?path=tg_987654321/voice1.ogg"},
		"/api/owner":                          {ownerChat, ownerName},
		"/api/events":                         {tgName, ownerChat, "settings.changed"},
		"/api/suppliers":                      {supPhone},
		"/media?path=tg_987654321/voice1.ogg": {mediaBytes},
		"/api/overview":                       {`"public":false`},
	} {
		code, body := get(h, p, false)
		for _, x := range want {
			if code != 200 || !strings.Contains(body, x) {
				t.Errorf("local GET %s (%d) lost %q: %s", p, code, x, body)
			}
		}
	}
	// a private desk never answers a visitor
	s.PublicReadOnly = false
	if code, _ := get(h, "/api/customers", true); code != 403 {
		t.Errorf("private desk answered a visitor: %d", code)
	}
}

func TestPublicAnswersAreCached(t *testing.T) {
	s := testDesk(t)
	h := s.Handler()
	_, a := get(h, "/api/engineer", true)
	s.DB.SetSetting("engineer_last_run", "2026-11-04 10:00:00")
	if _, b := get(h, "/api/engineer", true); a != b {
		t.Errorf("a visitor's /api/engineer was computed again: %s then %s", a, b)
	}
	if _, c := get(h, "/api/engineer", false); !strings.Contains(c, "2026-11-04 10:00:00") {
		t.Errorf("the owner must see the live state: %s", c)
	}
	_, a = get(h, "/api/overview", true)
	s.DB.SetSetting("telegram_bot", "otherbot")
	if _, b := get(h, "/api/overview", true); a != b {
		t.Errorf("a visitor's /api/overview was computed again")
	}
}

func TestDraftNotFound(t *testing.T) {
	h := testDesk(t).Handler()
	if code, _ := get(h, "/api/drafts/nope", false); code != 404 {
		t.Errorf("unknown draft: want 404, got %d", code)
	}
}

func TestSettingsValidation(t *testing.T) {
	s := testDesk(t)
	h := s.Handler()
	put := func(body string) int {
		r := httptest.NewRequest(http.MethodPut, "http://127.0.0.1:8610/api/settings", strings.NewReader(body))
		r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:8610"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	for _, b := range []string{`{"ai_budget_day":"NaN"}`, `{"ai_budget_day":"-1"}`, `{"ai_budget_day":"Inf"}`, `{"ai_budget_day":"1e400"}`,
		`{"ai_budget_day":"1000000"}`, `{"engineer_rounds":"2.5"}`, `{"engineer_model":"llama-free"}`, `{"engineer_model":""}`,
		`{"engineer_fallback_model":"gpt-x"}`, `{"shadow_agreements":"5","ai_budget_day":"nan"}`} {
		if c := put(b); c != 400 {
			t.Errorf("%s: want 400, got %d", b, c)
		}
	}
	if v := s.DB.Setting("shadow_agreements", "unset"); v != "unset" {
		t.Errorf("a refused body must change nothing, shadow_agreements = %q", v)
	}
	for _, b := range []string{`{"ai_budget_day":"0.5"}`, `{"engineer_model":"` + meta.Wonder + `"}`, `{"engineer_fallback_model":""}`, `{"engineer_rounds":"12"}`} {
		if c := put(b); c != 200 {
			t.Errorf("%s: want 200, got %d", b, c)
		}
	}
	if v := s.DB.Setting("ai_budget_day", ""); v != "0.5" {
		t.Errorf("budget not saved: %q", v)
	}
}

func TestERPConnectUnknownKind(t *testing.T) {
	s := testDesk(t)
	connected := false
	s.Connect = func(kind, url, db, login, token string) { connected = true }
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8610/api/erp/connect",
		strings.NewReader(`{"kind":"odooo","url":"http://127.0.0.1:1","username":"u","password":"p"}`))
	r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:8610"
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "unknown ERP") || connected {
		t.Errorf("unknown ERP kind: %d %s (connected %v)", w.Code, w.Body.String(), connected)
	}
}

func TestTunnelledRequestIsAVisitor(t *testing.T) {
	h := testDesk(t).Handler()
	// a tunnel on this computer: loopback address and host, but forwarding headers
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8610/api/customers", nil)
	r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:8610"
	r.Header.Set("Cf-Connecting-Ip", "198.51.100.7")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || strings.Contains(w.Body.String(), shopPhone) || strings.Contains(w.Body.String(), pairCode) {
		t.Errorf("a tunnelled request was served as the owner: %d %s", w.Code, w.Body.String())
	}
}
