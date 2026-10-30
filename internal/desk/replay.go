package desk

import (
	"context"
	"fmt"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// Replay runs one already-read invoice through the desk exactly as a live one: the same resolver
// (aliases, rules, Spark picks), then a clerk who knows the right product answers every question
// and corrects every wrong line, then the same learning as an approved invoice. With learn=false
// nothing is answered or learned: the invoice only measures the desk as it is.

type ReplayLine struct {
	Text      string  `json:"text"`
	Qty       float64 `json:"qty"`
	UnitPrice float64 `json:"unit_price"`
	Truth     string   `json:"product"`
	Accept    []string `json:"accept,omitempty"` // the same presentation listed twice in the catalogue
}

type ReplayResult struct {
	Lines   int     `json:"lines"`
	Alias   int     `json:"alias"`  // right, from a trusted wording
	Rules   int     `json:"rules"`  // right, from rules and the catalogue (free)
	AI      int     `json:"ai"`     // right, picked by the model
	Asked   int     `json:"asked"`  // a person had to answer
	Wrong   int     `json:"wrong"`  // resolved automatically to the wrong product
	AICost  float64 `json:"ai_cost"`
	MsgID   string  `json:"msg_id"`
	DraftID string  `json:"draft_id"`
	Errors  []Miss  `json:"errors,omitempty"`
}

type Miss struct {
	Said  string `json:"said"`
	Got   string `json:"got"`
	Want  string `json:"want"`
	Level string `json:"level"`
}

func (d *Desk) Replay(ctx context.Context, supplierID, number, date string, in []ReplayLine, learn bool) ReplayResult {
	msgID := store.NewID("msg")
	var text strings.Builder
	for _, l := range in {
		fmt.Fprintf(&text, "%s | %g | %.2f\n", l.Text, l.Qty, l.UnitPrice)
	}
	d.DB.Exec(`INSERT INTO messages (id, chat_key, kind, text, media_path, extraction, created_at) VALUES (?,?,?,?,?,?,?)`,
		msgID, "replay", "invoice", number+"\n"+text.String(), "", "replay", store.Now())
	draft := store.NewID("inv")
	now := store.Now()
	d.DB.Exec(`INSERT INTO drafts (id, chat_key, customer_id, status, kind, supplier_id, supplier_said, doc_number, doc_date, doc_total,
		created_at, updated_at) VALUES (?, 'replay', '', 'open', 'invoice', ?, '', ?, ?, 0, ?, ?)`, draft, supplierID, number, date, now, now)
	var ms []resolve.Mention
	for _, l := range in {
		ms = append(ms, resolve.Mention{Said: l.Text, Qty: l.Qty})
	}
	lines := d.resolveInvoice(ctx, draft, msgID, supplierID, ms)
	res := ReplayResult{MsgID: msgID, DraftID: draft}
	d.DB.QueryRow(`SELECT COALESCE(SUM(cost),0) FROM ai_usage WHERE ref = ?`, msgID).Scan(&res.AICost)
	for i := range lines {
		l := &lines[i]
		l.UnitPrice = int64(in[i].UnitPrice * 100)
		saveLine(d.DB, l)
		want := in[i].Truth
		right := l.ProductID == want
		for _, a := range in[i].Accept {
			right = right || l.ProductID == a
		}
		res.Lines++
		switch {
		case l.Status == "ok" && right:
			switch l.Level {
			case resolve.LAlias:
				res.Alias++
			case resolve.LAI:
				res.AI++
			default:
				res.Rules++
			}
			continue
		case l.Status == "ok":
			res.Wrong++
			res.Errors = append(res.Errors, Miss{Said: l.Said, Got: l.ProductID, Want: want, Level: l.Level})
			if learn {
				d.Choose(ctx, l, want, true) // the clerk corrects it: strikes the rules that gave it
			}
		default:
			res.Asked++
			if learn {
				d.Choose(ctx, l, want, false)
			}
		}
	}
	if !learn {
		d.DB.Exec(`UPDATE drafts SET status = 'cancelled', updated_at = ? WHERE id = ?`, store.Now(), draft)
		return res
	}
	d.DB.Exec(`UPDATE drafts SET status = 'confirmed', erp_order_number = ?, updated_at = ? WHERE id = ?`, "replay "+number, store.Now(), draft)
	d.learnLines(DraftLines(d.DB, draft, false), "invoice", "supplier", supplierID, "replay")
	return res
}
