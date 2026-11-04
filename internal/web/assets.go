package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
)

// index serves the panel's page with its script and style links stamped with a hash of their content, so a
// cache in front of the panel (Cloudflare keeps app.js for hours) never pairs an old app.js with a new server.
func index(sub fs.FS) http.HandlerFunc {
	page, _ := fs.ReadFile(sub, "index.html")
	h := sha256.New()
	for _, f := range []string{"app.js", "app.css"} {
		b, _ := fs.ReadFile(sub, f)
		h.Write(b)
	}
	v := hex.EncodeToString(h.Sum(nil))[:10]
	page = bytes.Replace(page, []byte(`href="app.css"`), []byte(`href="app.css?v=`+v+`"`), 1)
	page = bytes.Replace(page, []byte(`src="app.js"`), []byte(`src="app.js?v=`+v+`"`), 1)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(page)
	}
}
