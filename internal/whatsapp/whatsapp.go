// Package whatsapp is the WhatsApp channel, through Meta's WhatsApp Cloud API. It speaks to the
// desk in the same terms as Telegram (telegram.Messenger out, telegram.TgUpdate in), so the order
// and invoice flows do not know which channel a shop uses.
//
// Out: text messages; up to 3 choices as reply buttons, up to 10 as a list, more as a numbered
// message answered by its number. WhatsApp cannot edit a sent message: an edit is a new message.
// In: a webhook (GET verification, POST messages, signed with the app secret) for texts, voice
// notes, photos, documents and button replies.
package whatsapp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/telegram"
)

type Client struct {
	Token       string // permanent or temporary access token of the WhatsApp app
	PhoneID     string // the business phone number id (not the number itself)
	VerifyToken string // chosen by us, typed in the app's webhook settings
	AppSecret   string // to check X-Hub-Signature-256 on incoming calls (optional but recommended)
	Graph       string // https://graph.facebook.com/v23.0
	HTTP        *http.Client

	mu      sync.Mutex
	pending map[int64][]string   // chat → button data of a numbered choice
	last    map[int64]string     // chat → last incoming message id (read receipts, typing)
	seen    map[string]time.Time // incoming message ids already handled (Meta retries)
	queue   chan telegram.TgUpdate
	once    sync.Once
}

func New(token, phoneID, verifyToken, appSecret string) *Client {
	return &Client{Token: token, PhoneID: phoneID, VerifyToken: verifyToken, AppSecret: appSecret,
		Graph: "https://graph.facebook.com/v23.0", HTTP: &http.Client{Timeout: 30 * time.Second},
		pending: map[int64][]string{}, last: map[int64]string{}, seen: map[string]time.Time{}}
}

// ChatID is the desk's id for a WhatsApp number ("213555123456").
func ChatID(waID string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimPrefix(waID, "+"), 10, 64)
	if err != nil || n <= 0 || n >= telegram.WAOffset {
		return 0, fmt.Errorf("not a WhatsApp number: %q", waID)
	}
	return telegram.WAOffset + n, nil
}

func number(chatID int64) string { return strconv.FormatInt(chatID-telegram.WAOffset, 10) }

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.Graph+"/"+path, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		var e struct {
			Error struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		json.Unmarshal(raw, &e)
		return fmt.Errorf("whatsapp: HTTP %d: %s (%d)", res.StatusCode, e.Error.Message, e.Error.Code)
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// Me returns the business number's display number and verified name (for the panel).
func (c *Client) Me(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.Graph+"/"+c.PhoneID+"?fields=display_phone_number,verified_name", nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)
	var out struct {
		Display string `json:"display_phone_number"`
		Name    string `json:"verified_name"`
	}
	if err := c.do(req, &out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Display + " " + out.Name), nil
}

// ---- sending ---------------------------------------------------------------------------------

var (
	tagB    = regexp.MustCompile(`(?s)<b>(.*?)</b>`)
	tagI    = regexp.MustCompile(`(?s)<i>(.*?)</i>`)
	tagS    = regexp.MustCompile(`(?s)<s>(.*?)</s>`)
	tagCode = regexp.MustCompile(`(?s)<code>(.*?)</code>`)
	tagA    = regexp.MustCompile(`(?s)<a href="([^"]*)">(.*?)</a>`)
	tagAny  = regexp.MustCompile(`<[^>]+>`)
)

// Format turns the desk's Telegram HTML into WhatsApp's own marks (*bold*, _italic_, ~strike~).
func Format(s string) string {
	s = tagB.ReplaceAllString(s, "*$1*")
	s = tagI.ReplaceAllString(s, "_${1}_")
	s = tagS.ReplaceAllString(s, "~$1~")
	s = tagCode.ReplaceAllString(s, "`$1`")
	s = tagA.ReplaceAllString(s, "$2 ($1)")
	s = tagAny.ReplaceAllString(s, "")
	return html.UnescapeString(s)
}

func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// Send writes a message; buttons become WhatsApp reply buttons, a list, or numbered choices.
func (c *Client) Send(ctx context.Context, chatID int64, text string, rows [][]telegram.Button) (int, error) {
	to := number(chatID)
	body := Format(text)
	var btns []telegram.Button
	for _, r := range rows {
		btns = append(btns, r...)
	}
	c.mu.Lock()
	delete(c.pending, chatID)
	c.mu.Unlock()
	msg := map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": to}
	short := true
	for _, b := range btns {
		short = short && len([]rune(b.Text)) <= 20
	}
	switch {
	case len(btns) == 0:
		msg["type"] = "text"
		msg["text"] = map[string]any{"body": cut(body, 4096), "preview_url": false}
	case len(btns) <= 3 && short && len([]rune(body)) <= 1024:
		var bs []any
		for _, b := range btns {
			bs = append(bs, map[string]any{"type": "reply", "reply": map[string]any{"id": b.Data, "title": b.Text}})
		}
		msg["type"] = "interactive"
		msg["interactive"] = map[string]any{"type": "button", "body": map[string]any{"text": body}, "action": map[string]any{"buttons": bs}}
	case len(btns) <= 10 && len([]rune(body)) <= 1024:
		var rs []any
		for _, b := range btns {
			row := map[string]any{"id": b.Data, "title": cut(b.Text, 24)}
			if len([]rune(b.Text)) > 24 {
				row["description"] = cut(b.Text, 72)
			}
			rs = append(rs, row)
		}
		msg["type"] = "interactive"
		msg["interactive"] = map[string]any{"type": "list", "body": map[string]any{"text": body},
			"action": map[string]any{"button": "Choisir", "sections": []any{map[string]any{"title": "Choix", "rows": rs}}}}
	default:
		var b strings.Builder
		b.WriteString(body)
		b.WriteString("\n")
		data := make([]string, 0, len(btns))
		for i, x := range btns {
			fmt.Fprintf(&b, "\n*%d.* %s", i+1, x.Text)
			data = append(data, x.Data)
		}
		b.WriteString("\n\nRépondez par le numéro.")
		msg["type"] = "text"
		msg["text"] = map[string]any{"body": cut(b.String(), 4096), "preview_url": false}
		c.mu.Lock()
		c.pending[chatID] = data
		c.mu.Unlock()
	}
	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := c.post(ctx, c.PhoneID+"/messages", msg, &out); err != nil {
		log.Printf("[whatsapp] send to %s: %v", to, err)
		return 0, err
	}
	return 0, nil
}

// Edit sends the new version as a new message: WhatsApp cannot change a message once sent.
func (c *Client) Edit(ctx context.Context, chatID int64, messageID int, text string, rows [][]telegram.Button) error {
	_, err := c.Send(ctx, chatID, text, rows)
	return err
}

func (c *Client) Answer(ctx context.Context, callbackID, text string) {}

// Typing marks the shop's last message as read and shows the typing indicator.
func (c *Client) Typing(ctx context.Context, chatID int64) {
	c.mu.Lock()
	id := c.last[chatID]
	c.mu.Unlock()
	if id == "" {
		return
	}
	c.post(ctx, c.PhoneID+"/messages", map[string]any{"messaging_product": "whatsapp", "status": "read", "message_id": id,
		"typing_indicator": map[string]any{"type": "text"}}, nil)
}

// Download fetches a media file ("wa:<media id>"): its URL first, then the file, both with the token.
func (c *Client) Download(ctx context.Context, fileID string) ([]byte, error) {
	id := strings.TrimPrefix(fileID, "wa:")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.Graph+"/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)
	var meta struct {
		URL string `json:"url"`
	}
	if err := c.do(req, &meta); err != nil {
		return nil, err
	}
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, meta.URL, nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whatsapp media: HTTP %d", res.StatusCode)
	}
	return telegram.ReadMax(res.Body)
}

// ---- receiving -------------------------------------------------------------------------------

type inbound struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Contacts []struct {
					WaID    string `json:"wa_id"`
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
				} `json:"contacts"`
				Messages []struct {
					From  string                `json:"from"`
					ID    string                `json:"id"`
					Type  string                `json:"type"`
					Text  struct{ Body string } `json:"text"`
					Audio struct{ ID string }   `json:"audio"`
					Image struct {
						ID      string `json:"id"`
						Caption string `json:"caption"`
					} `json:"image"`
					Document struct {
						ID       string `json:"id"`
						Filename string `json:"filename"`
						MimeType string `json:"mime_type"`
						Caption  string `json:"caption"`
					} `json:"document"`
					Interactive struct {
						Type        string                     `json:"type"`
						ButtonReply struct{ ID, Title string } `json:"button_reply"`
						ListReply   struct{ ID, Title string } `json:"list_reply"`
					} `json:"interactive"`
					Button struct{ Payload, Text string } `json:"button"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// Updates turns one webhook delivery into desk updates (ignoring statuses and repeats).
func (c *Client) Updates(body []byte) []telegram.TgUpdate {
	var in inbound
	if json.Unmarshal(body, &in) != nil {
		return nil
	}
	var out []telegram.TgUpdate
	now := time.Now()
	for _, e := range in.Entry {
		for _, ch := range e.Changes {
			names := map[string]string{}
			for _, ct := range ch.Value.Contacts {
				names[ct.WaID] = ct.Profile.Name
			}
			for _, m := range ch.Value.Messages {
				chat, err := ChatID(m.From)
				if err != nil {
					continue
				}
				c.mu.Lock()
				_, dup := c.seen[m.ID]
				c.seen[m.ID] = now
				for id, at := range c.seen {
					if now.Sub(at) > time.Hour {
						delete(c.seen, id)
					}
				}
				c.last[chat] = m.ID
				pending := c.pending[chat]
				c.mu.Unlock()
				if dup {
					continue
				}
				first, last, _ := strings.Cut(names[m.From], " ")
				msg := &telegram.TgMessage{From: telegram.TgUser{ID: chat, FirstName: first, LastName: last},
					Chat: telegram.TgChat{ID: chat, Title: names[m.From]}}
				data := ""
				switch m.Type {
				case "text":
					msg.Text = m.Text.Body
					if n, err := strconv.Atoi(strings.TrimSpace(m.Text.Body)); err == nil && n >= 1 && n <= len(pending) {
						data = pending[n-1] // an answer to a numbered choice
					}
				case "audio":
					msg.Voice = &telegram.TgVoice{FileID: "wa:" + m.Audio.ID}
				case "image":
					msg.Photo = []telegram.TgPhoto{{FileID: "wa:" + m.Image.ID}}
					msg.Caption = m.Image.Caption
				case "document":
					msg.Document = &telegram.TgDoc{FileID: "wa:" + m.Document.ID, FileName: m.Document.Filename, MimeType: m.Document.MimeType}
					msg.Caption = m.Document.Caption
				case "interactive":
					data = m.Interactive.ButtonReply.ID
					if data == "" {
						data = m.Interactive.ListReply.ID
					}
				case "button":
					data = m.Button.Payload
				default:
					continue
				}
				if data != "" {
					c.mu.Lock()
					delete(c.pending, chat)
					c.mu.Unlock()
					out = append(out, telegram.TgUpdate{Callback: &telegram.TgCallback{ID: m.ID, From: msg.From, Message: msg, Data: data}})
					continue
				}
				out = append(out, telegram.TgUpdate{Message: msg})
			}
		}
	}
	return out
}

func (c *Client) signed(body []byte, header string) bool {
	if c.AppSecret == "" {
		return true
	}
	mac := hmac.New(sha256.New, []byte(c.AppSecret))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(header))
}

// Webhook serves Meta's webhook: the GET verification handshake, then signed POSTs of messages,
// acknowledged at once and handled in the background.
func (c *Client) Webhook(handle func(context.Context, telegram.TgUpdate)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			q := r.URL.Query()
			if q.Get("hub.mode") == "subscribe" && c.VerifyToken != "" && q.Get("hub.verify_token") == c.VerifyToken {
				io.WriteString(w, q.Get("hub.challenge"))
				return
			}
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if !c.signed(body, r.Header.Get("X-Hub-Signature-256")) {
			http.Error(w, "bad signature", http.StatusUnauthorized)
			return
		}
		ups := c.Updates(body)
		// one worker, in arrival order, like Telegram's single loop: a shop's messages never race
		c.once.Do(func() {
			c.queue = make(chan telegram.TgUpdate, 256)
			go func() {
				for u := range c.queue {
					handle(context.Background(), u)
				}
			}()
		})
		for _, u := range ups {
			select {
			case c.queue <- u:
			default:
				http.Error(w, "busy", http.StatusServiceUnavailable) // Meta retries
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

var _ telegram.Messenger = (*Client)(nil)
