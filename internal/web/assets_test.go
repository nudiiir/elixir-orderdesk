package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestIndexStampsAssets(t *testing.T) {
	sub, _ := fs.Sub(static, "static")
	w := httptest.NewRecorder()
	index(sub)(w, httptest.NewRequest(http.MethodGet, "/", nil))
	body := w.Body.String()
	if !regexp.MustCompile(`src="app\.js\?v=[0-9a-f]{10}"`).MatchString(body) ||
		!regexp.MustCompile(`href="app\.css\?v=[0-9a-f]{10}"`).MatchString(body) {
		t.Fatalf("assets not stamped:\n%s", body)
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("the page itself must not be cached: %q", w.Header().Get("Cache-Control"))
	}
}
