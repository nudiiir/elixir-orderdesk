package desk

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/meta/metatest"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// Muse Spark and Muse Wonder each read the specimen supplier invoice (samples/invoice-specimen.png), from a
// recording. Every line keeps its printed quantity and price, and a line the two readers do not read the same
// would be asked, never booked (Agree; see IngestInvoice).
func TestRecordedInvoiceTwoReaders(t *testing.T) {
	img, err := os.ReadFile("../../samples/invoice-specimen.png")
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "i.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	mc, tape := metatest.Client(t, db, "invoice-two-readers")
	ctx := context.Background()
	spark, err := ReadInvoice(ctx, mc, meta.Spark, img, "inv-1")
	if err != nil {
		t.Fatal(err)
	}
	wonder, err := ReadInvoice(ctx, mc, meta.Wonder, img, "inv-1")
	if err != nil {
		t.Fatal(err)
	}
	agree := Agree(spark, wonder)
	same := 0
	sum := 0.0
	for i, l := range spark.Lines {
		if agree[i] {
			same++
		}
		sum += l.Qty * l.UnitPrice
		if l.Qty <= 0 || l.UnitPrice <= 0 || l.Text == "" {
			t.Errorf("line %d read without text, quantity or price: %+v", i+1, l)
		}
		if l.Total > 0 && math.Abs(l.Qty*l.UnitPrice-l.Total) > math.Max(1, 0.01*l.Total) {
			t.Logf("line %d: %g × %g ≠ %g as printed", i+1, l.Qty, l.UnitPrice, l.Total)
		}
	}
	t.Logf("%s %s: Spark %d lines, Wonder %d, the same on %d; Σ qty×price %.2f, printed total HT %.2f",
		spark.Supplier, spark.Number, len(spark.Lines), len(wonder.Lines), same, sum, spark.TotalHT)
	if spark.TotalHT > 0 && math.Abs(sum-spark.TotalHT) > 0.01*spark.TotalHT {
		t.Errorf("the lines add up to %.2f, the invoice prints %.2f", sum, spark.TotalHT)
	}
	if !metatest.Recording() {
		if len(spark.Lines) != 5 || same != 5 {
			t.Errorf("the recording reads 5 lines, all 5 the same by both readers")
		}
		if len(tape.Requests) != 2 || tape.Requests[1].Model != meta.Wonder {
			t.Errorf("want one Spark and one Wonder reading")
		}
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM ai_usage WHERE purpose = 'invoice' AND cost > 0`).Scan(&n)
	if n != 2 {
		t.Errorf("%d metered readings, want 2", n)
	}
}
