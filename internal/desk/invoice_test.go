package desk

import (
	"reflect"
	"testing"
)

func TestAgree(t *testing.T) {
	spark := InvoiceDoc{Lines: []InvoiceLine{
		{Text: "GA5030R MEULEUSE GA5030 125 MM", Qty: 3, UnitPrice: 12887},
		{Text: "ART9383 RACCORD LAITON 15/21 F", Qty: 20, UnitPrice: 159.79},
		{Text: "FORET BETON 6MM INGCO", Qty: 10, UnitPrice: 152.6},
		{Text: "CHEVILLE A FRAPPER 8X60 B/100", Qty: 10, UnitPrice: 510.53},
	}}
	wonder := InvoiceDoc{Lines: []InvoiceLine{
		{Text: "GA5030R MEULEUSE GA5030 125 MM -", Qty: 3, UnitPrice: 12887}, // a stray mark: same words
		{Text: "ART9303 RACCORD LAITON 15/21 F", Qty: 20, UnitPrice: 159.79}, // another code
		{Text: "FORET BETON 6MM INGCO", Qty: 1, UnitPrice: 152.6},            // another quantity
		{Text: "CHEVILLE A FRAPPER 8X60 B/100", Qty: 10, UnitPrice: 510.53},
	}}
	if got, want := Agree(spark, wonder), []bool{true, false, false, true}; !reflect.DeepEqual(got, want) {
		t.Errorf("Agree = %v, want %v", got, want)
	}
	// one line of the second reading can back only one line of the first
	twice := InvoiceDoc{Lines: []InvoiceLine{spark.Lines[3], spark.Lines[3]}}
	if got := Agree(twice, InvoiceDoc{Lines: []InvoiceLine{spark.Lines[3]}}); !reflect.DeepEqual(got, []bool{true, false}) {
		t.Errorf("Agree twice = %v", got)
	}
}
