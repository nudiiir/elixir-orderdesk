package erpnext

import (
	"context"
	"os"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter/adaptertest"
)

// ERPNEXT_URL=http://127.0.0.1:8090 ERPNEXT_USER=Administrator ERPNEXT_PASS=admin go test ./internal/adapter/erpnext
func TestERPNextLive(t *testing.T) {
	url := os.Getenv("ERPNEXT_URL")
	if url == "" {
		t.Skip("ERPNEXT_URL not set")
	}
	key, secret, name, err := Login(context.Background(), url, os.Getenv("ERPNEXT_USER"), os.Getenv("ERPNEXT_PASS"))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	t.Logf("logged in as %s, key %s…", name, key[:4])
	adaptertest.Run(t, New(url, key, secret))
}
