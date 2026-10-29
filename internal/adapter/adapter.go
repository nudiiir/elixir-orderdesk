// Package adapter is the contract between Order Desk and an ERP. The core never talks to an
// ERP any other way: Elixir Distribution, Odoo or a spreadsheet are all just adapters.
package adapter

import (
	"context"
	"errors"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
)

var ErrNotConnected = errors.New("ERP not connected")

// Quote is the ERP's price for one line, computed by the ERP's own pricing rules
// (contract prices, offers, free units). Amounts are in centimes.
type Quote struct {
	UnitPriceHT  int64  `json:"unit_price_ht"`
	LineTTC      int64  `json:"line_ttc"`
	FreeQtyMilli int64  `json:"free_qty_milli"`
	Source       string `json:"source"`
	Offer        string `json:"offer"`
}

type PastLine struct {
	ProductID string `json:"product_id"`
	QtyMilli  int64  `json:"qty_milli"`
}

type PastOrder struct {
	ID     string     `json:"id"`
	Number string     `json:"number"`
	Date   string     `json:"date"`
	Lines  []PastLine `json:"lines"`
}

type OrderLine struct {
	ProductID string
	QtyMilli  int64
	Notes     string
}

type Order struct {
	CustomerID string
	Lines      []OrderLine
	Notes      string // evidence link and channel
	ExternalRef string
}

type OrderRef struct {
	ID       string `json:"id"`
	Number   string `json:"number"`
	TotalTTC int64  `json:"total_ttc"`
}

type Delivery struct {
	Day   string `json:"day"`   // "mercredi 05/11"
	Route string `json:"route"` // "Boufarik"
}

// ERP is what every adapter fulfils. Optional capabilities return ErrUnsupported.
type ERP interface {
	Name() string
	Ping(ctx context.Context) error
	Products(ctx context.Context) ([]catalog.Product, error)
	Customers(ctx context.Context) ([]catalog.Customer, error)
	Price(ctx context.Context, customerID, productID string, qtyMilli int64) (Quote, error)
	Stock(ctx context.Context) (map[string]int64, error)
	History(ctx context.Context, customerID string, n int) ([]PastOrder, error)
	CreateOrder(ctx context.Context, o Order) (OrderRef, error)
	NextDelivery(ctx context.Context, customerID string) (Delivery, error)
	OrderURL(ref OrderRef) string
}

var ErrUnsupported = errors.New("not supported by this ERP")
