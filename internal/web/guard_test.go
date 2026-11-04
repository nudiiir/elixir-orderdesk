package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicReadOnlyGuard(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	call := func(public bool, method, remote string) int {
		r := httptest.NewRequest(method, "http://203.0.113.5:8610/api/rules", strings.NewReader("{}"))
		r.RemoteAddr, r.Host = remote, "203.0.113.5:8610"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		(&Server{PublicReadOnly: public}).guard(ok).ServeHTTP(w, r)
		return w.Code
	}
	if c := call(false, "GET", "198.51.100.7:4000"); c != 403 {
		t.Errorf("private desk: a visitor read the panel (%d)", c)
	}
	if c := call(true, "GET", "198.51.100.7:4000"); c != 200 {
		t.Errorf("public read-only desk: a visitor could not read (%d)", c)
	}
	for _, m := range []string{"POST", "PUT", "DELETE"} {
		if c := call(true, m, "198.51.100.7:4000"); c != 403 {
			t.Errorf("public read-only desk: a visitor's %s went through (%d)", m, c)
		}
	}
}
