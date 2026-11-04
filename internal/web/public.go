package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// A public read-only desk shows visitors how orders were resolved: shop names, the words of each
// line, products, quantities, levels and costs. Pairing codes, phone numbers, chat ids, people's
// names, message text, transcripts, voice notes and photos stay on this computer.

// public tells whether a request is a visitor of a public read-only desk, not the owner on this computer.
func (s *Server) public(r *http.Request) bool {
	return s.PublicReadOnly && (!loopbackHost(r.Host) || !loopbackHost(r.RemoteAddr) || forwarded(r))
}

// forwarded tells whether a request came through a tunnel or a proxy.
func forwarded(r *http.Request) bool {
	return r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("Forwarded") != "" || r.Header.Get("Cf-Connecting-Ip") != ""
}

// notPublic hides a route from visitors: it only carries personal data.
func (s *Server) notPublic(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.public(r) {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

// publicEvents are the event kinds a visitor sees, each with the details it may keep ("*": all of
// them, for reports that only hold rule ids, line words and costs).
var publicEvents = map[string][]string{
	"order.created":     {"number", "lines", "erp"},
	"receipt.created":   {"number", "lines", "erp", "supplier"},
	"alias.created":     {"text", "witnesses", "alias"},
	"alias.trusted":     {"text", "witnesses", "alias"},
	"alias.candidate":   {"text", "witnesses", "alias"},
	"alias.blocked":     {"text", "witnesses", "alias"},
	"rule.proposed":     {"sentence", "rationale", "inspired_by"},
	"rule.benchmark":    {"*"},
	"rule.shadow":       {"gate", "agreements", "by"},
	"rule.live":         {"gate", "agreements", "by"},
	"rule.rejected":     {"gate", "said", "stage", "by"},
	"rule.rolled_back":  {"strikes", "by"},
	"rule.disabled":     {"by"},
	"rule.strike":       {"said", "wrong"},
	"line.corrected":    {"said", "wrong"},
	"chat.linked":       {"customer", "role"}, // the shop's name, never the chat or who linked it
	"engineer.started":  {"trigger", "residue", "benchmark"},
	"engineer.finished": {"*"},
	"engineer.fallback": {"from", "to", "residue"},
}

// publicEvent is an event as a visitor sees it: false when the visitor does not see it at all.
func publicEvent(kind, actor, details string) (string, json.RawMessage, bool) {
	keep, ok := publicEvents[kind]
	if !ok {
		return "", nil, false
	}
	switch {
	case actor == "clerk", actor == "shop", actor == "system", actor == "rule-engineer", strings.HasPrefix(actor, "muse-"):
	default:
		actor = "" // a person's name
	}
	if len(keep) == 1 && keep[0] == "*" {
		return actor, json.RawMessage(details), true
	}
	var all map[string]json.RawMessage
	json.Unmarshal([]byte(details), &all)
	out := map[string]json.RawMessage{}
	for _, k := range keep {
		if v, ok := all[k]; ok {
			out[k] = v
		}
	}
	b, _ := json.Marshal(out)
	return actor, b, true
}

type cacheEntry struct {
	mu sync.Mutex
	at time.Time
	v  any
}

// cached returns the answer built at most ttl ago, building it when it is older. Visitors of a public
// desk share it, so they cannot make the desk do expensive work on every call.
func (s *Server) cached(key string, ttl time.Duration, build func() any) any {
	s.cacheMu.Lock()
	if s.cache == nil {
		s.cache = map[string]*cacheEntry{}
	}
	e := s.cache[key]
	if e == nil {
		e = &cacheEntry{}
		s.cache[key] = e
	}
	s.cacheMu.Unlock()
	e.mu.Lock() // one build at a time: the others wait for it
	defer e.mu.Unlock()
	if e.v == nil || time.Since(e.at) > ttl {
		e.v, e.at = build(), time.Now()
	}
	return e.v
}
