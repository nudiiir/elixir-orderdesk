// Package meta is a small client for the Meta Model API (OpenAI-compatible chat completions,
// tool calling and Muse Voice Transcribe) with a cost meter: every call is written to ai_usage
// with its model, purpose, tokens and price, and a daily budget is enforced.
package meta

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

const (
	Spark      = "muse-spark-1.3"
	Wonder     = "muse-wonder-1.0"
	Transcribe = "muse-voice-transcribe-1.0"
)

// Prices in USD per million tokens (input, output). Transcription is priced per minute
// (an estimate until Meta publishes it; set "price_transcribe_min" to correct it).
var prices = map[string][2]float64{Spark: {1.25, 4.25}, Wonder: {4, 20}}

var ErrBudget = errors.New("daily AI budget reached")
var ErrNoKey = errors.New("no Meta API key configured")

type Client struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
	DB      *store.DB
}

func New(key string, db *store.DB) *Client {
	return &Client{BaseURL: "https://api.meta.ai/v1", Key: strings.TrimSpace(key), DB: db,
		HTTP: &http.Client{Timeout: 150 * time.Second}}
}

func (c *Client) Enabled() bool { return c != nil && c.Key != "" }

// SpentToday is the AI cost recorded since midnight.
func (c *Client) SpentToday() float64 {
	var v float64
	c.DB.QueryRow(`SELECT COALESCE(SUM(cost),0) FROM ai_usage WHERE at >= date('now','localtime')`).Scan(&v)
	return v
}

func (c *Client) check() error {
	if !c.Enabled() {
		return ErrNoKey
	}
	if budget := c.DB.SettingFloat("ai_budget_day", 2); budget > 0 && c.SpentToday() >= budget {
		return ErrBudget
	}
	return nil
}

func (c *Client) record(model, purpose, ref string, in, out int, audioSec float64) float64 {
	var cost float64
	if p, ok := prices[model]; ok {
		cost = float64(in)*p[0]/1e6 + float64(out)*p[1]/1e6
	} else if model == Transcribe {
		cost = audioSec / 60 * c.DB.SettingFloat("price_transcribe_min", 0.006)
	}
	c.DB.Exec(`INSERT INTO ai_usage (at, model, purpose, tokens_in, tokens_out, audio_sec, cost, ref) VALUES (?,?,?,?,?,?,?,?)`,
		store.Now(), model, purpose, in, out, audioSec, cost, ref)
	return cost
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Message is one chat turn (OpenAI format), including tool calls and tool results.
type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// JSON sends one user turn (text plus optional images) and decodes the model's JSON answer.
// It returns the cost of the call.
func (c *Client) JSON(ctx context.Context, model, purpose, ref, system, user string, images [][]byte, maxTokens int, out any) (float64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}
	content := []map[string]any{{"type": "text", "text": user}}
	for _, img := range images {
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]string{
			"url": "data:" + http.DetectContentType(img) + ";base64," + base64.StdEncoding.EncodeToString(img)}})
	}
	body := map[string]any{
		"model": model, "max_tokens": maxTokens, "reasoning_effort": "low",
		"response_format": map[string]string{"type": "json_object"},
		"messages":        []map[string]any{{"role": "system", "content": system}, {"role": "user", "content": content}},
	}
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage usage `json:"usage"`
	}
	if err := c.post(ctx, "/chat/completions", body, &resp); err != nil {
		return 0, err
	}
	cost := c.record(model, purpose, ref, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, 0)
	if len(resp.Choices) == 0 {
		return cost, fmt.Errorf("meta: empty answer")
	}
	return cost, decodeJSON(resp.Choices[0].Message.Content, out)
}

func decodeJSON(text string, out any) error {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(strings.TrimPrefix(text, "```json"), "```")
	text = strings.TrimSuffix(text, "```")
	if i, j := strings.Index(text, "{"), strings.LastIndex(text, "}"); i >= 0 && j > i {
		text = text[i : j+1]
	}
	if err := json.Unmarshal([]byte(text), out); err != nil {
		return fmt.Errorf("meta: bad JSON answer: %w", err)
	}
	return nil
}

// Chat runs one turn of a tool-using conversation and returns the assistant message.
func (c *Client) Chat(ctx context.Context, model, purpose, ref string, msgs []Message, tools []Tool, maxTokens int) (Message, float64, error) {
	if err := c.check(); err != nil {
		return Message{}, 0, err
	}
	ts := []map[string]any{}
	for _, t := range tools {
		ts = append(ts, map[string]any{"type": "function", "function": t})
	}
	body := map[string]any{"model": model, "max_tokens": maxTokens, "messages": msgs, "reasoning_effort": "low"}
	if len(ts) > 0 {
		body["tools"] = ts
	}
	var resp struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage usage `json:"usage"`
	}
	if err := c.post(ctx, "/chat/completions", body, &resp); err != nil {
		return Message{}, 0, err
	}
	cost := c.record(model, purpose, ref, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, 0)
	if len(resp.Choices) == 0 {
		return Message{}, cost, fmt.Errorf("meta: empty answer")
	}
	return resp.Choices[0].Message, cost, nil
}

// Text returns a message's text content.
func (m Message) Text() string {
	if s, ok := m.Content.(string); ok {
		return s
	}
	return ""
}

// DecodeJSON parses a JSON object out of a model's text answer.
func DecodeJSON(text string, out any) error { return decodeJSON(text, out) }

// Transcribe sends a 16 kHz mono WAV to Muse Voice Transcribe.
func (c *Client) Transcribe(ctx context.Context, wav []byte, ref string) (string, float64, error) {
	if err := c.check(); err != nil {
		return "", 0, err
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="request"`)
	h.Set("Content-Type", "application/json")
	part, _ := w.CreatePart(h)
	part.Write([]byte(`{"model":"` + Transcribe + `","audioEncoding":"WAV","mode":"PUSH_TO_TALK"}`))
	h = textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="audio"; filename="voice.wav"`)
	h.Set("Content-Type", "audio/wav")
	part, _ = w.CreatePart(h)
	part.Write(wav)
	w.Close()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/asr/transcribe", &buf)
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("transcribe: HTTP %d: %s", res.StatusCode, truncate(string(raw), 300))
	}
	var out struct {
		Transcript string `json:"transcript"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", 0, err
	}
	sec := float64(len(wav)-44) / 32000 // 16 kHz, 16-bit mono
	cost := c.record(Transcribe, "transcribe", ref, 0, 0, sec)
	return strings.TrimSpace(out.Transcript), cost, nil
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	payload, _ := json.Marshal(body)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
			continue
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
			lastErr = fmt.Errorf("meta: HTTP %d", res.StatusCode)
			time.Sleep(time.Duration(attempt+1) * 3 * time.Second)
			continue
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("meta: HTTP %d: %s", res.StatusCode, truncate(string(raw), 300))
		}
		return json.Unmarshal(raw, out)
	}
	return lastErr
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
