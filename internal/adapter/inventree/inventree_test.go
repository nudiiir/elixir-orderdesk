package inventree

import (
	"context"
	"os"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter/adaptertest"
)

// INVENTREE_URL=http://127.0.0.1:8083 INVENTREE_USER=admin INVENTREE_PASS=admin go test ./internal/adapter/inventree
func TestInvenTreeLive(t *testing.T) {
	url := os.Getenv("INVENTREE_URL")
	if url == "" {
		t.Skip("INVENTREE_URL not set")
	}
	token, name, err := Login(context.Background(), url, os.Getenv("INVENTREE_USER"), os.Getenv("INVENTREE_PASS"))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	t.Logf("logged in as %s, token %s…", name, token[:6])
	adaptertest.Run(t, New(url, token))
}
