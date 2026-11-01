// Package meta is a small client for the Meta Model API (OpenAI-compatible chat completions,
// tool calling, Muse Voice Transcribe and SAM 3.1) with a cost meter: every call is written to ai_usage
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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

const (
	Spark      = "muse-spark-1.3"
	Wonder     = "muse-wonder-1.0"
	Transcribe = "muse-voice-transcribe-1.0"
	SAM        = "sam-3.1"
)

// Prices in USD per million tokens (input, output). Transcription is priced per minute and SAM per image
// (estimates until Meta publishes them; set "price_transcribe_min" and "price_sam_image" to correct them).
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
	} else if model == SAM {
		cost = c.DB.SettingFloat("price_sam_image", 0.003)
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

// JSONFile is JSON with a document (a PDF invoice) attached as a file part.
func (c *Client) JSONFile(ctx context.Context, model, purpose, ref, system, user string, file []byte, mime string, maxTokens int, out any) (float64, error) {
	if err := c.check(); err != nil {
		return 0, err
	}
	content := []map[string]any{{"type": "text", "text": user}, {"type": "file", "file": map[string]string{
		"filename": "invoice.pdf", "file_data": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(file)}}}
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
	return c.TranscribeWords(ctx, wav, ref, nil)
}

// MaxKeyword is the longest keyword Muse Voice Transcribe accepts.
const MaxKeyword = 30

// TranscribeWords is Transcribe with keyword biasing: Muse Voice Transcribe is told which words it may
// hear (the distributor's brands), so "Choufan" is not heard as "chou fin".
func (c *Client) TranscribeWords(ctx context.Context, wav []byte, ref string, keywords []string) (string, float64, error) {
	if err := c.check(); err != nil {
		return "", 0, err
	}
	reqJSON := map[string]any{"model": Transcribe, "audioEncoding": "WAV", "mode": "PUSH_TO_TALK"}
	var kw []string
	for _, k := range keywords {
		if k = strings.TrimSpace(k); k != "" && len(k) <= MaxKeyword {
			kw = append(kw, k)
		}
	}
	if len(kw) > 0 {
		reqJSON["keywords"] = kw
	}
	reqBody, _ := json.Marshal(reqJSON)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="request"`)
	h.Set("Content-Type", "application/json")
	part, _ := w.CreatePart(h)
	part.Write(reqBody)
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

// Box is one object SAM found, in source pixels (X2 and Y2 exclusive), with its confidence (0 to 1).
type Box struct {
	X1, Y1, X2, Y2 int
	Conf           float64
}

// SAM answers in one output_text lane: "<0f>" then records like
// 0<|box;x1=440;y1=60;x2=560;y2=439;w=800;h=500;c=0.81|><|mask;…|>, comma-separated. Only the boxes are used.
var samBox = regexp.MustCompile(`<\|box;x1=(\d+);y1=(\d+);x2=(\d+);y2=(\d+);w=\d+;h=\d+(?:;c=([\d.]+))?\|>`)

// Segment asks SAM 3.1 for every instance of one noun phrase ("bottle", "can") in an image.
func (c *Client) Segment(ctx context.Context, img []byte, noun, ref string) ([]Box, float64, error) {
	if err := c.check(); err != nil {
		return nil, 0, err
	}
	body := map[string]any{
		"model":    SAM,
		"metadata": map[string]string{"include_confidence": "true"},
		"input": []map[string]any{{"role": "user", "content": []map[string]any{
			{"type": "input_image", "image_url": "data:" + http.DetectContentType(img) + ";base64," + base64.StdEncoding.EncodeToString(img)},
			{"type": "input_text", "text": noun},
		}}},
	}
	var resp struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := c.post(ctx, "/responses", body, &resp); err != nil {
		return nil, 0, err
	}
	cost := c.record(SAM, "segment", ref, 0, 0, 0)
	var boxes []Box
	for _, o := range resp.Output {
		for _, part := range o.Content {
			for _, m := range samBox.FindAllStringSubmatch(part.Text, -1) {
				b := Box{X1: atoi(m[1]), Y1: atoi(m[2]), X2: atoi(m[3]) + 1, Y2: atoi(m[4]) + 1, Conf: 1}
				if m[5] != "" {
					b.Conf, _ = strconv.ParseFloat(m[5], 64)
				}
				boxes = append(boxes, b)
			}
		}
	}
	return boxes, cost, nil
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

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
