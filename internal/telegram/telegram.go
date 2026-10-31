package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

// Telegram is a minimal Bot API client using long polling, so the shop PC
// needs no public address: it only makes outbound HTTPS calls.
type Telegram struct {
	Token string
	HTTP  *http.Client
}

func NewTelegram(token string) *Telegram {
	return &Telegram{Token: token, HTTP: &http.Client{Timeout: 70 * time.Second}}
}

type TgUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type TgChat struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type TgPhoto struct {
	FileID   string `json:"file_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	FileSize int    `json:"file_size"`
}

type TgVoice struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
}

type TgMessage struct {
	MessageID int       `json:"message_id"`
	From      TgUser    `json:"from"`
	Chat      TgChat    `json:"chat"`
	Text      string    `json:"text"`
	Caption   string    `json:"caption"`
	Photo     []TgPhoto `json:"photo"`
	Voice     *TgVoice  `json:"voice"`
	Audio     *TgVoice  `json:"audio"`
	Document  *TgDoc    `json:"document"`
}

type TgDoc struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
}

type TgCallback struct {
	ID      string     `json:"id"`
	From    TgUser     `json:"from"`
	Message *TgMessage `json:"message"`
	Data    string     `json:"data"`
}

type TgUpdate struct {
	UpdateID int         `json:"update_id"`
	Message  *TgMessage  `json:"message"`
	Callback *TgCallback `json:"callback_query"`
}

// Button is one inline keyboard button; Data comes back in a callback.
type Button struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

func (t *Telegram) call(ctx context.Context, method string, body any, out any) error {
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.telegram.org/bot"+t.Token+"/"+method, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	res, err := t.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var env struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
	}
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		return err
	}
	if !env.OK {
		return fmt.Errorf("telegram %s: %s", method, env.Description)
	}
	if out != nil {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

// Me returns the bot's own account (used to build the pairing link).
func (t *Telegram) Me(ctx context.Context) (TgUser, error) {
	var u TgUser
	err := t.call(ctx, "getMe", map[string]any{}, &u)
	return u, err
}

func (t *Telegram) Updates(ctx context.Context, offset int) ([]TgUpdate, error) {
	var ups []TgUpdate
	err := t.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 50,
		"allowed_updates": []string{"message", "callback_query"}}, &ups)
	return ups, err
}

// Send posts a message; rows of buttons are optional. Returns the message id.
func (t *Telegram) Send(ctx context.Context, chatID int64, text string, rows [][]Button) (int, error) {
	body := map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML",
		"disable_web_page_preview": true}
	if len(rows) > 0 {
		body["reply_markup"] = map[string]any{"inline_keyboard": rows}
	}
	var m TgMessage
	err := t.call(ctx, "sendMessage", body, &m)
	return m.MessageID, err
}

// SendPhoto posts a JPEG with an HTML caption. Returns the message id.
func (t *Telegram) SendPhoto(ctx context.Context, chatID int64, jpg []byte, caption string) (int, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("chat_id", strconv.FormatInt(chatID, 10))
	w.WriteField("caption", caption)
	w.WriteField("parse_mode", "HTML")
	part, _ := w.CreateFormFile("photo", "photo.jpg")
	part.Write(jpg)
	w.Close()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+t.Token+"/sendPhoto", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := t.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	var env struct {
		OK          bool      `json:"ok"`
		Result      TgMessage `json:"result"`
		Description string    `json:"description"`
	}
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		return 0, err
	}
	if !env.OK {
		return 0, fmt.Errorf("telegram sendPhoto: %s", env.Description)
	}
	return env.Result.MessageID, nil
}

// Edit replaces the text (and buttons) of a message the bot sent.
func (t *Telegram) Edit(ctx context.Context, chatID int64, messageID int, text string, rows [][]Button) error {
	body := map[string]any{"chat_id": chatID, "message_id": messageID, "text": text, "parse_mode": "HTML",
		"disable_web_page_preview": true}
	if rows == nil {
		rows = [][]Button{}
	}
	body["reply_markup"] = map[string]any{"inline_keyboard": rows}
	return t.call(ctx, "editMessageText", body, nil)
}

func (t *Telegram) Answer(ctx context.Context, callbackID, text string) {
	t.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": callbackID, "text": text}, nil)
}

func (t *Telegram) Typing(ctx context.Context, chatID int64) {
	t.call(ctx, "sendChatAction", map[string]any{"chat_id": chatID, "action": "typing"}, nil)
}

// Download fetches a file the shop sent (voice note, photo).
func (t *Telegram) Download(ctx context.Context, fileID string) ([]byte, error) {
	var f struct {
		FilePath string `json:"file_path"`
	}
	if err := t.call(ctx, "getFile", map[string]any{"file_id": fileID}, &f); err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.telegram.org/file/bot"+t.Token+"/"+f.FilePath, nil)
	res, err := t.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram download: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(res.Body)
}
