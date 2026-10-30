// Package twilio is the WhatsApp channel through Twilio (a Twilio WhatsApp sender, or the Twilio
// Sandbox for WhatsApp). It speaks to the desk like the other channels (telegram.Messenger out,
// telegram.TgUpdate in) and uses the same chat ids as the Meta Cloud API channel, so a shop is the
// same shop whichever provider carries its WhatsApp.
//
// Out: text messages (split at Twilio's 1,600 characters); choices are numbered and answered by
// their number, which works on every Twilio sender including the sandbox.
// In: Twilio's webhook (form POST, signed with X-Twilio-Signature) for texts, voice notes, photos,
// documents and quick-reply buttons; answered with an empty TwiML response.
package twilio

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/telegram"
	"github.com/nudiiir/elixir-orderdesk/internal/whatsapp"
)

type Client struct {
	SID       string // Account SID (AC…)
	Token     string // Auth Token: signs the webhook and authenticates API calls
	From      string // the WhatsApp sender, "+14155238886" (the sandbox) or your own number
	PublicURL string // the webhook's public URL as Twilio calls it; empty = rebuilt from the request
	API       string // https://api.twilio.com
	HTTP      *http.Client

	mu      sync.Mutex
	pending map[int64][]string
	seen    map[string]time.Time
	queue   chan telegram.TgUpdate
	once    sync.Once
}

func New(sid, token, from string) *Client {
	from = strings.TrimPrefix(strings.TrimSpace(from), "whatsapp:")
	if !strings.HasPrefix(from, "+") {
		from = "+" + from
	}
	return &Client{SID: sid, Token: token, From: from, API: "https://api.twilio.com", HTTP: &http.Client{Timeout: 30 * time.Second},
		pending: map[int64][]string{}, seen: map[string]time.Time{}}
}

func number(chatID int64) string { return strconv.FormatInt(chatID-telegram.WAOffset, 10) }

// Me checks the credentials and names the account (for the panel).
func (c *Client) Me(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.API+"/2010-04-01/Accounts/"+c.SID+".json", nil)
	req.SetBasicAuth(c.SID, c.Token)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("twilio: %w", err)
	}
	defer res.Body.Close()
	var out struct {
		FriendlyName string `json:"friendly_name"`
		Message      string `json:"message"`
	}
	json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("twilio: HTTP %d: %s", res.StatusCode, out.Message)
	}
	return c.From + " via Twilio (" + out.FriendlyName + ")", nil
}

func chunks(s string, n int) []string {
	r := []rune(s)
	var out []string
	for len(r) > n {
		cut := n
		for i := n; i > n/2; i-- { // break at a line end when one is close
			if r[i] == '\n' {
				cut = i
				break
			}
		}
		out = append(out, string(r[:cut]))
		r = r[cut:]
	}
	return append(out, string(r))
}

// Send writes a message; buttons become numbered choices answered by their number.
func (c *Client) Send(ctx context.Context, chatID int64, text string, rows [][]telegram.Button) (int, error) {
	body := whatsapp.Format(text)
	var data []string
	var b strings.Builder
	b.WriteString(body)
	n := 0
	for _, r := range rows {
		for _, x := range r {
			n++
			if n == 1 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "\n*%d.* %s", n, whatsapp.Format(x.Text))
			data = append(data, x.Data)
		}
	}
	if n > 0 {
		b.WriteString("\n\nRépondez par le numéro.")
	}
	c.mu.Lock()
	if len(data) > 0 {
		c.pending[chatID] = data
	} else {
		delete(c.pending, chatID)
	}
	c.mu.Unlock()
	for _, part := range chunks(b.String(), 1600) {
		form := url.Values{"From": {"whatsapp:" + c.From}, "To": {"whatsapp:+" + number(chatID)}, "Body": {part}}
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.API+"/2010-04-01/Accounts/"+c.SID+"/Messages.json", strings.NewReader(form.Encode()))
		req.SetBasicAuth(c.SID, c.Token)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := c.HTTP.Do(req)
		if err != nil {
			return 0, fmt.Errorf("twilio: %w", err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode >= 300 {
			var e struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			}
			json.Unmarshal(raw, &e)
			log.Printf("[twilio] send to %s: HTTP %d %s (%d)", number(chatID), res.StatusCode, e.Message, e.Code)
			return 0, fmt.Errorf("twilio: HTTP %d: %s (%d)", res.StatusCode, e.Message, e.Code)
		}
	}
	return 0, nil
}

// Edit sends the new version as a new message (WhatsApp cannot edit).
func (c *Client) Edit(ctx context.Context, chatID int64, messageID int, text string, rows [][]telegram.Button) error {
	_, err := c.Send(ctx, chatID, text, rows)
	return err
}

func (c *Client) Answer(ctx context.Context, callbackID, text string) {}
func (c *Client) Typing(ctx context.Context, chatID int64)              {}

// Download fetches a media file ("wa:<Twilio media URL>") with the account's credentials.
func (c *Client) Download(ctx context.Context, fileID string) ([]byte, error) {
	u := strings.TrimPrefix(fileID, "wa:")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.SetBasicAuth(c.SID, c.Token)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twilio media: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

// Signature is Twilio's X-Twilio-Signature for a webhook URL and its form parameters.
func Signature(token, fullURL string, form url.Values) string {
	keys := make([]string, 0, len(form))
	for k := range form {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(fullURL)
	for _, k := range keys {
		for _, v := range form[k] {
			b.WriteString(k)
			b.WriteString(v)
		}
	}
	mac := hmac.New(sha1.New, []byte(token))
	mac.Write([]byte(b.String()))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *Client) publicURL(r *http.Request) string {
	if c.PublicURL != "" {
		u := strings.TrimRight(c.PublicURL, "/")
		if !strings.HasSuffix(u, r.URL.Path) {
			u += r.URL.Path
		}
		return u
	}
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}
	return scheme + "://" + r.Host + r.URL.RequestURI()
}

// Update turns one Twilio webhook call into a desk update (nil for repeats and empty calls).
func (c *Client) Update(form url.Values) *telegram.TgUpdate {
	from := strings.TrimPrefix(form.Get("From"), "whatsapp:")
	chat, err := whatsapp.ChatID(from)
	if err != nil {
		return nil
	}
	sid := form.Get("MessageSid")
	now := time.Now()
	c.mu.Lock()
	_, dup := c.seen[sid]
	if sid != "" {
		c.seen[sid] = now
	}
	for id, at := range c.seen {
		if now.Sub(at) > time.Hour {
			delete(c.seen, id)
		}
	}
	pending := c.pending[chat]
	c.mu.Unlock()
	if dup {
		return nil
	}
	first, last, _ := strings.Cut(form.Get("ProfileName"), " ")
	msg := &telegram.TgMessage{From: telegram.TgUser{ID: chat, FirstName: first, LastName: last},
		Chat: telegram.TgChat{ID: chat, Title: form.Get("ProfileName")}, Text: strings.TrimSpace(form.Get("Body"))}
	data := form.Get("ButtonPayload")
	if n, err := strconv.Atoi(msg.Text); err == nil && n >= 1 && n <= len(pending) && data == "" {
		data = pending[n-1]
	}
	if data != "" {
		c.mu.Lock()
		delete(c.pending, chat)
		c.mu.Unlock()
		return &telegram.TgUpdate{Callback: &telegram.TgCallback{ID: "wamid." + sid, From: msg.From, Message: msg, Data: data}}
	}
	if n, _ := strconv.Atoi(form.Get("NumMedia")); n > 0 {
		media, kind := "wa:"+form.Get("MediaUrl0"), form.Get("MediaContentType0")
		caption := msg.Text
		switch {
		case strings.HasPrefix(kind, "audio/"):
			msg.Voice = &telegram.TgVoice{FileID: media}
			msg.Text = ""
		case strings.HasPrefix(kind, "image/"):
			msg.Photo = []telegram.TgPhoto{{FileID: media}}
			msg.Caption, msg.Text = caption, ""
		default:
			msg.Document = &telegram.TgDoc{FileID: media, MimeType: kind, FileName: "document"}
			msg.Caption, msg.Text = caption, ""
		}
	}
	return &telegram.TgUpdate{Message: msg}
}

// Webhook serves Twilio's "a message comes in" callback: checks the signature, answers an empty
// TwiML at once and hands the message to the desk in arrival order.
func (c *Client) Webhook(handle func(context.Context, telegram.TgUpdate)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			io.WriteString(w, "Order Desk: Twilio WhatsApp webhook")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if sig := Signature(c.Token, c.publicURL(r), r.PostForm); !hmac.Equal([]byte(sig), []byte(r.Header.Get("X-Twilio-Signature"))) {
			log.Printf("[twilio] refused a call with a bad signature for %s (set TWILIO_WEBHOOK_URL if the public address differs)", c.publicURL(r))
			http.Error(w, "bad signature", http.StatusForbidden)
			return
		}
		u := c.Update(r.PostForm)
		c.once.Do(func() {
			c.queue = make(chan telegram.TgUpdate, 256)
			go func() {
				for x := range c.queue {
					handle(context.Background(), x)
				}
			}()
		})
		w.Header().Set("Content-Type", "text/xml")
		io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Response></Response>`)
		if u != nil {
			c.queue <- *u
		}
	}
}

var _ telegram.Messenger = (*Client)(nil)
