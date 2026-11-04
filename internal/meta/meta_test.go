package meta

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	db, err := store.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New("test-key", db)
	c.BaseURL = srv.URL
	return c
}

// Calls that start together cannot all pass a budget only some of them fit in.
func TestBudgetReservedAtomically(t *testing.T) {
	var calls atomic.Int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(100 * time.Millisecond)
		// 4,706 output tokens of Spark: $0.02
		w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}],"usage":{"prompt_tokens":0,"completion_tokens":4706}}`))
	})
	c.DB.SetSetting("ai_budget_day", "0.03")
	c.DB.SetSetting("ai_reserve_call", "0.02")
	var wg sync.WaitGroup
	var refused atomic.Int32
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out map[string]any
			if _, err := c.JSON(context.Background(), Spark, "test", "", "s", "u", nil, 10, &out); errors.Is(err, ErrBudget) {
				refused.Add(1)
			}
		}()
	}
	wg.Wait()
	if n := calls.Load(); n > 2 || refused.Load() < 4 {
		t.Fatalf("%d calls reached the API on a budget of two, %d refused", n, refused.Load())
	}
	if c.reserved > 1e-9 || c.reserved < -1e-9 {
		t.Fatalf("%.4f still reserved after every call returned", c.reserved)
	}
}

// A retry does not keep a cancelled caller waiting.
func TestRetryHonoursContext(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	var out map[string]any
	_, err := c.JSON(ctx, Spark, "test", "", "s", "u", nil, 10, &out)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
		t.Fatalf("err %v after %v, want the context's error at once", err, time.Since(start))
	}
}
