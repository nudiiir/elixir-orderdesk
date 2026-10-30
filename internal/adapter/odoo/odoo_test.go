package odoo

import (
	"os"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter/adaptertest"
)

// ODOO_URL=http://127.0.0.1:8069 ODOO_DB=orderdesk ODOO_LOGIN=admin ODOO_KEY=admin go test ./internal/adapter/odoo
func TestOdooLive(t *testing.T) {
	url := os.Getenv("ODOO_URL")
	if url == "" {
		t.Skip("ODOO_URL not set")
	}
	adaptertest.Run(t, New(url, os.Getenv("ODOO_DB"), os.Getenv("ODOO_LOGIN"), os.Getenv("ODOO_KEY")))
}
