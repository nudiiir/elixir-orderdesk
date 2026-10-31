package tryton

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter/adaptertest"
)

// TRYTON_URL=http://127.0.0.1:8000 TRYTON_DB=orderdesk TRYTON_USER=admin TRYTON_PASS=admin go test ./internal/adapter/tryton
func TestTrytonLive(t *testing.T) {
	url := os.Getenv("TRYTON_URL")
	if url == "" {
		t.Skip("TRYTON_URL not set")
	}
	db, user := os.Getenv("TRYTON_DB"), os.Getenv("TRYTON_USER")
	token, name, err := Login(context.Background(), url, db, user, os.Getenv("TRYTON_PASS"))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	uid, _, _ := strings.Cut(token, ":")
	t.Logf("logged in as %s (user %s)", name, uid)
	adaptertest.Run(t, New(url, db, user, token))
}
