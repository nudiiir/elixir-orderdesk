package twilio

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/telegram"
	"github.com/nudiiir/elixir-orderdesk/internal/whatsapp"
)

// Twilio signs the full URL followed by every POST parameter, sorted by name, as name+value
// (twilio-python RequestValidator.compute_signature), with HMAC-SHA1 and the auth token, base64.
func TestSignatureMatchesTwilio(t *testing.T) {
	form := url.Values{"To": {"+18005551212"}, "CallSid": {"CA1234567890ABCDE"}, "Digits": {"1234"},
		"From": {"+14158675309"}, "Caller": {"+12349013030"}}
	signed := "https://mycompany.com/myapp.php?foo=1&bar=2" +
		"CallSidCA1234567890ABCDE" + "Caller+12349013030" + "Digits1234" + "From+14158675309" + "To+18005551212"
	mac := hmac.New(sha1.New, []byte("12345"))
	mac.Write([]byte(signed))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if got := Signature("12345", "https://mycompany.com/myapp.php?foo=1&bar=2", form); got != want {
		t.Fatalf("signature %s, want %s", got, want)
	}
}

func fakeTwilio(t *testing.T) (*httptest.Server, func() []url.Values) {
	var mu sync.Mutex
	var sent []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "AC1" || p != "TOK" {
			http.Error(w, `{"message":"Authenticate","code":20003}`, 401)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/Messages.json"):
			r.ParseForm()
			mu.Lock()
			sent = append(sent, r.PostForm)
			mu.Unlock()
			io.WriteString(w, `{"sid":"SM1"}`)
		case strings.HasSuffix(r.URL.Path, "/Media/ME1"):
			io.WriteString(w, "OGG")
		default:
			io.WriteString(w, `{"friendly_name":"My first Twilio account"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []url.Values { mu.Lock(); defer mu.Unlock(); return append([]url.Values{}, sent...) }
}

func TestSendAndAnswer(t *testing.T) {
	srv, sent := fakeTwilio(t)
	c := New("AC1", "TOK", "whatsapp:+14155238886")
	c.API = srv.URL
	chat, _ := whatsapp.ChatID("213555123456")
	ctx := context.Background()
	if _, err := c.Send(ctx, chat, "« fanta 1l » : <b>lequel ?</b>", [][]telegram.Button{{{Text: "Fanta Citron 1L", Data: "c:ln1:0"}}, {{Text: "Fanta Orange 1L", Data: "c:ln1:1"}}}); err != nil {
		t.Fatal(err)
	}
	c.Send(ctx, chat, strings.Repeat("ligne de commande\n", 200), nil)
	s := sent()
	if len(s) != 4 {
		t.Fatalf("sent %d messages (1 + a 3,600-char text split in 3)", len(s))
	}
	if s[0].Get("From") != "whatsapp:+14155238886" || s[0].Get("To") != "whatsapp:+213555123456" ||
		!strings.Contains(s[0].Get("Body"), "*lequel ?*") || !strings.Contains(s[0].Get("Body"), "*2.* Fanta Orange 1L") {
		t.Fatalf("first message: %v", s[0])
	}
	for _, m := range s[1:] {
		if n := len([]rune(m.Get("Body"))); n > 1600 {
			t.Fatalf("a part of %d characters", n)
		}
	}
	// the choice made before the long text was replaced by it: send the question again, answer "2"
	c.Send(ctx, chat, "lequel ?", [][]telegram.Button{{{Text: "A", Data: "c:ln1:0"}, {Text: "B", Data: "c:ln1:1"}}})
	u := c.Update(url.Values{"From": {"whatsapp:+213555123456"}, "Body": {"2"}, "MessageSid": {"SM2"}, "ProfileName": {"Amina B"}})
	if u == nil || u.Callback == nil || u.Callback.Data != "c:ln1:1" || u.Callback.Message.Chat.ID != chat {
		t.Fatalf("numbered answer: %+v", u)
	}
	if again := c.Update(url.Values{"From": {"whatsapp:+213555123456"}, "Body": {"2"}, "MessageSid": {"SM2"}}); again != nil {
		t.Fatal("a repeated call must be ignored")
	}
	if q := c.Update(url.Values{"From": {"whatsapp:+213555123456"}, "Body": {"Confirmer"}, "ButtonPayload": {"ok:dr1"}, "MessageSid": {"SM3"}}); q == nil || q.Callback.Data != "ok:dr1" {
		t.Fatalf("quick reply: %+v", q)
	}
	v := c.Update(url.Values{"From": {"whatsapp:+213555123456"}, "NumMedia": {"1"}, "MediaUrl0": {srv.URL + "/Media/ME1"},
		"MediaContentType0": {"audio/ogg"}, "MessageSid": {"SM4"}})
	if v == nil || v.Message.Voice == nil {
		t.Fatalf("voice: %+v", v)
	}
	if b, err := c.Download(ctx, v.Message.Voice.FileID); err != nil || string(b) != "OGG" {
		t.Fatalf("download: %q %v", b, err)
	}
	p := c.Update(url.Values{"From": {"whatsapp:+213555123456"}, "NumMedia": {"1"}, "MediaUrl0": {"x"}, "MediaContentType0": {"application/pdf"}, "Body": {"facture"}, "MessageSid": {"SM5"}})
	if p == nil || p.Message.Document == nil || p.Message.Caption != "facture" {
		t.Fatalf("document: %+v", p)
	}
	if who, err := c.Me(ctx); err != nil || !strings.Contains(who, "+14155238886") {
		t.Fatalf("me: %q %v", who, err)
	}
}

func TestWebhookSignature(t *testing.T) {
	c := New("AC1", "TOK", "+14155238886")
	got := make(chan telegram.TgUpdate, 2)
	h := c.Webhook(func(_ context.Context, u telegram.TgUpdate) { got <- u })
	form := url.Values{"From": {"whatsapp:+213555123456"}, "Body": {"2 cartons ifri"}, "MessageSid": {"SM9"}}
	post := func(sig string) int {
		req := httptest.NewRequest(http.MethodPost, "https://abc.trycloudflare.com/whatsapp/webhook", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Host = "abc.trycloudflare.com"
		req.Header.Set("X-Twilio-Signature", sig)
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec.Code
	}
	if code := post("forged"); code != http.StatusForbidden {
		t.Fatalf("a forged call must be refused: %d", code)
	}
	if code := post(Signature("TOK", "https://abc.trycloudflare.com/whatsapp/webhook", form)); code != 200 {
		t.Fatalf("signed call: %d", code)
	}
	select {
	case u := <-got:
		if u.Message == nil || u.Message.Text != "2 cartons ifri" {
			t.Fatalf("update: %+v", u)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the message never reached the desk")
	}
}
