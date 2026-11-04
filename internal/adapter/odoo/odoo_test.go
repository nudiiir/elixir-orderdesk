package odoo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
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

// FindOrder looks the order up by the reference CreateOrder writes, and confirms a quotation left behind.
func TestFindOrder(t *testing.T) {
	var confirmed []any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Method string `json:"method"`
				Args   []any  `json:"args"`
			} `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		res := any(nil)
		switch {
		case req.Params.Method == "authenticate":
			res = 2
		case req.Params.Args[4] == "search_read":
			domain := fmt.Sprint(req.Params.Args[5])
			if strings.Contains(domain, "orderdesk:dr_1") {
				res = []map[string]any{{"id": 7, "state": "draft"}}
			} else {
				res = []map[string]any{}
			}
		case req.Params.Args[4] == "action_confirm":
			confirmed = append(confirmed, req.Params.Args[5])
			res = true
		case req.Params.Args[4] == "read":
			res = []map[string]any{{"id": 7, "name": "S00007", "amount_total": 12.5}}
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": res})
	}))
	defer srv.Close()
	c := New(srv.URL, "db", "admin", "key")
	var _ adapter.OrderFinder = c
	ref, ok, err := c.FindOrder(context.Background(), "orderdesk:dr_1")
	if err != nil || !ok || ref.ID != "7" || ref.Number != "S00007" || ref.TotalTTC != 1250 || len(confirmed) != 1 {
		t.Fatalf("found %+v %v %v, confirmed %v", ref, ok, err, confirmed)
	}
	if _, ok, err := c.FindOrder(context.Background(), "orderdesk:dr_2"); ok || err != nil {
		t.Fatalf("an order never created: %v %v", ok, err)
	}
}
