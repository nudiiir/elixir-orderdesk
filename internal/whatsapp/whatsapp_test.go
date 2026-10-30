package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/telegram"
)

// fakeGraph records what the client sends and serves media like the Graph API.
func fakeGraph(t *testing.T) (*httptest.Server, func() []map[string]any) {
	var mu sync.Mutex
	var sent []map[string]any
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer TOKEN" {
			http.Error(w, `{"error":{"message":"bad token","code":190}}`, 401)
			return
		}
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/PHONE/messages"):
			var m map[string]any
			json.NewDecoder(r.Body).Decode(&m)
			mu.Lock()
			sent = append(sent, m)
			mu.Unlock()
			io.WriteString(w, `{"messages":[{"id":"wamid.OUT"}]}`)
		case r.URL.Path == "/MEDIA1":
			io.WriteString(w, `{"url":"`+srv.URL+`/files/MEDIA1"}`)
		case r.URL.Path == "/files/MEDIA1":
			io.WriteString(w, "OGGDATA")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []map[string]any { mu.Lock(); defer mu.Unlock(); return append([]map[string]any{}, sent...) }
}

func client(url string) *Client {
	c := New("TOKEN", "PHONE", "VERIFY", "SECRET")
	c.Graph = url
	return c
}

func TestSendShapes(t *testing.T) {
	srv, sent := fakeGraph(t)
	c := client(srv.URL)
	ctx := context.Background()
	chat, _ := ChatID("213555123456")
	c.Send(ctx, chat, "Commande <b>CC-12</b> &amp; <i>livraison</i>", nil)
	c.Send(ctx, chat, "« ifri » : lequel ?", [][]telegram.Button{{{Text: "0,5 L", Data: "c:1"}, {Text: "1,5 L", Data: "c:2"}}})
	opts := [][]telegram.Button{}
	for i := 0; i < 5; i++ {
		opts = append(opts, []telegram.Button{{Text: "Hamoud Boualem Selecto 33cl canette", Data: "c:" + string(rune('a'+i))}})
	}
	c.Send(ctx, chat, "Lequel ?", opts)
	many := [][]telegram.Button{}
	for i := 0; i < 12; i++ {
		many = append(many, []telegram.Button{{Text: "Produit", Data: "p:" + string(rune('a'+i))}})
	}
	c.Send(ctx, chat, "Lequel ?", many)
	s := sent()
	if len(s) != 4 {
		t.Fatalf("sent %d messages", len(s))
	}
	if s[0]["to"] != "213555123456" || s[0]["type"] != "text" || s[0]["text"].(map[string]any)["body"] != "Commande *CC-12* & _livraison_" {
		t.Errorf("text: %+v", s[0])
	}
	if it := s[1]["interactive"].(map[string]any); it["type"] != "button" {
		t.Errorf("two choices must be reply buttons: %+v", s[1])
	}
	if it := s[2]["interactive"].(map[string]any); it["type"] != "list" {
		t.Errorf("long titles must be a list: %+v", s[2])
	}
	if s[3]["type"] != "text" || !strings.Contains(s[3]["text"].(map[string]any)["body"].(string), "*12.* Produit") {
		t.Errorf("twelve choices must be numbered: %+v", s[3])
	}
	// "12" answers the numbered choice: a button press for the desk
	ups := c.Updates(inboundText("wamid.1", "213555123456", "12"))
	if len(ups) != 1 || ups[0].Callback == nil || ups[0].Callback.Data != "p:l" || ups[0].Callback.Message.Chat.ID != chat {
		t.Fatalf("numbered answer: %+v", ups)
	}
}

func inboundText(id, from, body string) []byte {
	return []byte(`{"object":"whatsapp_business_account","entry":[{"changes":[{"field":"messages","value":{
		"contacts":[{"wa_id":"` + from + `","profile":{"name":"Amina Boudjema"}}],
		"messages":[{"from":"` + from + `","id":"` + id + `","type":"text","text":{"body":"` + body + `"}}]}}]}]}`)
}

func TestInboundKinds(t *testing.T) {
	srv, _ := fakeGraph(t)
	c := client(srv.URL)
	body := []byte(`{"entry":[{"changes":[{"value":{"contacts":[{"wa_id":"213661000000","profile":{"name":"Epicerie Nour"}}],"messages":[
		{"from":"213661000000","id":"wamid.a","type":"audio","audio":{"id":"MEDIA1","mime_type":"audio/ogg; codecs=opus","voice":true}},
		{"from":"213661000000","id":"wamid.b","type":"image","image":{"id":"IMG1","caption":"ma liste"}},
		{"from":"213661000000","id":"wamid.c","type":"document","document":{"id":"DOC1","filename":"facture.pdf","mime_type":"application/pdf"}},
		{"from":"213661000000","id":"wamid.d","type":"interactive","interactive":{"type":"button_reply","button_reply":{"id":"ok:inv_1","title":"Valider"}}},
		{"from":"213661000000","id":"wamid.e","type":"interactive","interactive":{"type":"list_reply","list_reply":{"id":"c:ln_1:2","title":"1,5 L"}}}],
		"statuses":[{"id":"wamid.OUT","status":"read"}]}}]}]}`)
	ups := c.Updates(body)
	if len(ups) != 5 {
		t.Fatalf("updates: %d", len(ups))
	}
	if v := ups[0].Message.Voice; v == nil || v.FileID != "wa:MEDIA1" || ups[0].Message.From.FirstName != "Epicerie" {
		t.Errorf("voice: %+v", ups[0].Message)
	}
	if p := ups[1].Message.Photo; len(p) != 1 || p[0].FileID != "wa:IMG1" || ups[1].Message.Caption != "ma liste" {
		t.Errorf("photo: %+v", ups[1].Message)
	}
	if dc := ups[2].Message.Document; dc == nil || dc.MimeType != "application/pdf" {
		t.Errorf("document: %+v", ups[2].Message)
	}
	if ups[3].Callback == nil || ups[3].Callback.Data != "ok:inv_1" || ups[4].Callback.Data != "c:ln_1:2" {
		t.Errorf("buttons: %+v %+v", ups[3], ups[4])
	}
	if again := c.Updates(body); len(again) != 0 {
		t.Errorf("a repeated delivery must be ignored: %d", len(again))
	}
	audio, err := c.Download(context.Background(), "wa:MEDIA1")
	if err != nil || string(audio) != "OGGDATA" {
		t.Errorf("download: %q %v", audio, err)
	}
}

func TestWebhook(t *testing.T) {
	srv, _ := fakeGraph(t)
	c := client(srv.URL)
	got := make(chan telegram.TgUpdate, 4)
	h := c.Webhook(func(_ context.Context, u telegram.TgUpdate) { got <- u })

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/whatsapp/webhook?hub.mode=subscribe&hub.verify_token=VERIFY&hub.challenge=42", nil))
	if rec.Code != 200 || rec.Body.String() != "42" {
		t.Fatalf("verification: %d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/whatsapp/webhook?hub.mode=subscribe&hub.verify_token=nope&hub.challenge=42", nil))
	if rec.Code != 403 {
		t.Fatalf("a wrong verify token must be refused: %d", rec.Code)
	}
	body := inboundText("wamid.x", "213555123456", "2 cartons ifri")
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/whatsapp/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", "sha256=00")
	h(rec, req)
	if rec.Code != 401 {
		t.Fatalf("a bad signature must be refused: %d", rec.Code)
	}
	mac := hmac.New(sha256.New, []byte("SECRET"))
	mac.Write(body)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/whatsapp/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	h(rec, req)
	if rec.Code != 200 {
		t.Fatalf("signed delivery: %d", rec.Code)
	}
	select {
	case u := <-got:
		if u.Message == nil || u.Message.Text != "2 cartons ifri" {
			t.Fatalf("update: %+v", u)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the update never reached the desk")
	}
}
