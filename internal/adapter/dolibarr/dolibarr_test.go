package dolibarr

import (
	"context"
	"os"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter/adaptertest"
)

// DOLIBARR_URL=http://127.0.0.1:8081 DOLIBARR_KEY=<admin's DOLAPIKEY> go test ./internal/adapter/dolibarr
// (or DOLIBARR_USER=admin DOLIBARR_PASS=admin instead of the key, to go through Login)
func TestDolibarrLive(t *testing.T) {
	url := os.Getenv("DOLIBARR_URL")
	if url == "" {
		t.Skip("DOLIBARR_URL not set")
	}
	key := os.Getenv("DOLIBARR_KEY")
	if key == "" {
		k, name, err := Login(context.Background(), url, os.Getenv("DOLIBARR_USER"), os.Getenv("DOLIBARR_PASS"))
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		t.Logf("logged in as %s, key %s…", name, k[:4])
		key = k
	}
	adaptertest.Run(t, New(url, key))
}

func TestParsePacks(t *testing.T) {
	for in, want := range map[string]int64{"Carton x6": 6000, "Pack de 12": 12000, "Palette = 480": 480000, "Fardeau (24)": 24000, "Carton": 0, "x1": 0} {
		got := int64(0)
		if p := parsePacks(in); len(p) > 0 {
			got = p[0].FactorMilli
		}
		if got != want {
			t.Errorf("%q: %d, want %d", in, got, want)
		}
	}
	if p := parsePacks("Carton x6, Palette x480"); len(p) != 2 || p[1].Name != "Palette x480" {
		t.Errorf("two packs: %+v", p)
	}
}
