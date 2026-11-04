// Package metatest records real Meta Model API answers once and replays them in tests, so the code around
// every model call (prompt building, answer parsing, the checks that decide what a model's answer may do)
// is tested offline, with no key and no network.
//
// A test asks for a client by tape name:
//
//	mc, tape := metatest.Client(t, db, "spark-pick")
//
// Replay (the default) serves the answers stored in testdata/meta/<name>.json. Each request is matched to a
// recorded call by a hash of its body; if the prompt has changed since the recording, the next unused call
// with the same endpoint and model is served instead and the test logs it.
//
// Record mode sends the same requests to the real API and rewrites the tape:
//
//	ORDERDESK_RECORD=1 META_MODEL_API_KEY=... go test ./internal/... -run Recorded
//
// (or ORDERDESK_KEYS=path/to/keys.env instead of META_MODEL_API_KEY). Tapes whose names start with
// "handwritten-" are written by hand to test what the code does with a bad answer (an id outside the
// shortlist, broken JSON, a rule that books a line wrong); they are never re-recorded.
package metatest

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// Call is one recorded request and the API's answer.
type Call struct {
	Path     string          `json:"path"`
	Model    string          `json:"model"`
	Ask      string          `json:"ask"`      // the request's text (prompt, tool results, keywords), for reading; images and audio left out
	SHA      string          `json:"sha256"`   // hash of the request body, to match a replayed request to its recording
	Status   int             `json:"status"`   // HTTP status the API answered
	Response json.RawMessage `json:"response"` // the API's answer, as received (SAM masks shortened: only boxes are used)
}

// Request is what the code under test sent, for assertions.
type Request struct {
	Path     string
	Model    string
	Keywords []string // Muse Voice Transcribe keywords
	Ask      string
}

// Tape serves (or records) one test's calls.
type Tape struct {
	Name     string
	Calls    []Call
	Requests []Request

	t      testing.TB
	file   string
	record bool
	key    string
	base   string
	used   []bool
	mu     sync.Mutex
}

// Recording reports whether this run talks to the real API.
func Recording() bool { return os.Getenv("ORDERDESK_RECORD") == "1" }

// Client returns a Meta client wired to a tape, with a generous daily budget so the tape decides.
func Client(t testing.TB, db *store.DB, name string) (*meta.Client, *Tape) {
	t.Helper()
	tp := &Tape{Name: name, t: t, file: filepath.Join("testdata", "meta", name+".json"),
		base: envOr("META_BASE_URL", "https://api.meta.ai/v1")}
	tp.record = Recording() && !strings.HasPrefix(name, "handwritten-")
	if tp.record {
		tp.key = apiKey()
		if tp.key == "" {
			t.Fatal("ORDERDESK_RECORD=1 needs META_MODEL_API_KEY or ORDERDESK_KEYS")
		}
	} else {
		raw, err := os.ReadFile(tp.file)
		if err != nil {
			t.Fatalf("no tape %s (record it with ORDERDESK_RECORD=1): %v", tp.file, err)
		}
		if err := json.Unmarshal(raw, &tp.Calls); err != nil {
			t.Fatalf("tape %s: %v", tp.file, err)
		}
		tp.used = make([]bool, len(tp.Calls))
	}
	srv := httptest.NewServer(http.HandlerFunc(tp.serve))
	t.Cleanup(func() {
		srv.Close()
		if tp.record {
			tp.save()
		}
	})
	db.SetSetting("ai_budget_day", "100")
	mc := meta.New("replay", db)
	mc.BaseURL = srv.URL
	return mc, tp
}

func (tp *Tape) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	req := describe(r.URL.Path, r.Header.Get("Content-Type"), body)
	sha := hashBody(r.Header.Get("Content-Type"), body)
	tp.mu.Lock()
	tp.Requests = append(tp.Requests, req)
	tp.mu.Unlock()

	if tp.record {
		status, resp := tp.forward(r, body)
		tp.mu.Lock()
		tp.Calls = append(tp.Calls, Call{Path: req.Path, Model: req.Model, Ask: req.Ask, SHA: sha, Status: status, Response: shorten(resp)})
		tp.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(resp)
		return
	}

	tp.mu.Lock()
	k := -1
	for i, c := range tp.Calls {
		if !tp.used[i] && c.SHA == sha {
			k = i
			break
		}
	}
	if k < 0 {
		for i, c := range tp.Calls {
			if !tp.used[i] && c.Path == req.Path && c.Model == req.Model {
				k = i
				if c.SHA == "" { // a hand-written answer: served in order
					break
				}
				tp.t.Logf("tape %s: request %d changed since the recording (%s %s); serving the next recorded answer", tp.Name, len(tp.Requests), req.Path, req.Model)
				break
			}
		}
	}
	if k >= 0 {
		tp.used[k] = true
	}
	tp.mu.Unlock()
	if k < 0 {
		tp.t.Errorf("tape %s: no recorded answer left for %s %s (re-record with ORDERDESK_RECORD=1)", tp.Name, req.Path, req.Model)
		http.Error(w, `{"error":"not on the tape"}`, http.StatusBadRequest) // 400: the client does not retry
		return
	}
	c := tp.Calls[k]
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(c.Status)
	w.Write(c.Response)
}

// Unused is the number of recorded calls the test never made: a path the code no longer takes.
func (tp *Tape) Unused() int {
	n := 0
	for _, u := range tp.used {
		if !u {
			n++
		}
	}
	return n
}

func (tp *Tape) forward(r *http.Request, body []byte) (int, []byte) {
	req, _ := http.NewRequest(http.MethodPost, tp.base+r.URL.Path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tp.key)
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	if a := r.Header.Get("Accept"); a != "" {
		req.Header.Set("Accept", a)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		tp.t.Errorf("recording %s: %v", tp.Name, err)
		return http.StatusBadGateway, []byte(`{"error":"upstream"}`)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}

func (tp *Tape) save() {
	os.MkdirAll(filepath.Dir(tp.file), 0o755)
	raw, _ := json.MarshalIndent(tp.Calls, "", " ")
	if err := os.WriteFile(tp.file, append(raw, '\n'), 0o644); err != nil {
		tp.t.Errorf("save tape: %v", err)
	}
}

var dataURL = regexp.MustCompile(`data:[a-z/+.-]+;base64,[A-Za-z0-9+/=]+`)
var mask = regexp.MustCompile(`<\|mask;[^|]*\|>`)

// shorten drops SAM's mask encodings (often hundreds of kilobytes; the desk reads only the boxes).
func shorten(resp []byte) json.RawMessage {
	out := mask.ReplaceAll(resp, []byte("<|mask;…|>"))
	var v any
	if json.Unmarshal(out, &v) != nil {
		b, _ := json.Marshal(string(out))
		return b
	}
	b, _ := json.Marshal(v)
	return b
}

// hashBody hashes what a request says: for a multipart upload, its parts without the random boundary.
func hashBody(ctype string, body []byte) string {
	h := sha256.New()
	if mt, params, _ := mime.ParseMediaType(ctype); strings.HasPrefix(mt, "multipart/") {
		mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			h.Write([]byte(p.FormName() + "\x00"))
			io.Copy(h, p)
		}
	} else {
		h.Write(body)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// describe reads the model, the text and the keywords out of a request.
func describe(path, ctype string, body []byte) Request {
	req := Request{Path: path}
	if mt, params, _ := mime.ParseMediaType(ctype); strings.HasPrefix(mt, "multipart/") {
		mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			if p.FormName() == "request" {
				var j struct {
					Model    string   `json:"model"`
					Keywords []string `json:"keywords"`
				}
				json.NewDecoder(p).Decode(&j)
				req.Model, req.Keywords = j.Model, j.Keywords
				req.Ask = "keywords: " + strings.Join(j.Keywords, ", ")
			}
		}
		return req
	}
	var j map[string]any
	json.Unmarshal(body, &j)
	req.Model, _ = j["model"].(string)
	var texts []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			if s := strings.TrimSpace(dataURL.ReplaceAllString(x, "[file]")); s != "" && s != "[file]" {
				texts = append(texts, s)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for _, k := range []string{"role", "content", "text", "arguments", "name"} {
				if e, ok := x[k]; ok {
					if k == "role" || k == "name" {
						if s, _ := e.(string); s != "" {
							texts = append(texts, "["+s+"]")
						}
						continue
					}
					walk(e)
				}
			}
			if f, ok := x["function"]; ok {
				walk(f)
			}
			if tc, ok := x["tool_calls"]; ok {
				walk(tc)
			}
		}
	}
	walk(j["messages"])
	walk(j["input"])
	req.Ask = strings.Join(texts, "\n")
	return req
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func apiKey() string {
	if k := strings.TrimSpace(os.Getenv("META_MODEL_API_KEY")); k != "" {
		return k
	}
	f, err := os.Open(os.Getenv("ORDERDESK_KEYS"))
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "="); ok && strings.TrimSpace(k) == "META_MODEL_API_KEY" {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}
