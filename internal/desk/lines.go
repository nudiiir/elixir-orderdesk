package desk

import (
	"database/sql"
	"encoding/json"

	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// Line is one order line as stored in the lines table.
type Line struct {
	ID         string            `json:"id"`
	DraftID    string            `json:"draft_id"`
	MessageID  string            `json:"message_id"`
	Position   int               `json:"position"`
	Said       string            `json:"said"`
	Norm       string            `json:"norm"`
	Hint       string            `json:"hint"`
	Qty        float64           `json:"qty"`
	Unit       string            `json:"unit"`
	ProductID  string            `json:"product_id"`
	PackName   string            `json:"pack_name"`
	PackFactor int64             `json:"pack_factor"`
	QtyMilli   int64             `json:"qty_milli"`
	Level      string            `json:"level"`
	Used       []string          `json:"used"`
	Confidence float64           `json:"confidence"`
	Options    []string          `json:"options"`
	Question   string            `json:"question"`
	Status     string            `json:"status"`
	Checks     map[string]string `json:"checks"`
	UnitPrice  int64             `json:"unit_price"`
	LineTTC    int64             `json:"line_ttc"`
	Shadow     map[string]string `json:"shadow"`
	Outcome    string            `json:"outcome"`
	CreatedAt  string            `json:"created_at"`
	AICost     float64           `json:"ai_cost"`
}

const lineCols = `id, draft_id, message_id, position, said, norm, hint, qty, unit, product_id, pack_name, pack_factor, qty_milli,
	level, used, confidence, options, question, status, checks, unit_price, line_ttc, shadow, outcome, created_at`

func scanLine(rows interface{ Scan(...any) error }) (Line, error) {
	var l Line
	var used, opts, checks, shadow string
	err := rows.Scan(&l.ID, &l.DraftID, &l.MessageID, &l.Position, &l.Said, &l.Norm, &l.Hint, &l.Qty, &l.Unit, &l.ProductID,
		&l.PackName, &l.PackFactor, &l.QtyMilli, &l.Level, &used, &l.Confidence, &opts, &l.Question, &l.Status, &checks,
		&l.UnitPrice, &l.LineTTC, &shadow, &l.Outcome, &l.CreatedAt)
	json.Unmarshal([]byte(used), &l.Used)
	json.Unmarshal([]byte(opts), &l.Options)
	json.Unmarshal([]byte(checks), &l.Checks)
	json.Unmarshal([]byte(shadow), &l.Shadow)
	if l.Checks == nil {
		l.Checks = map[string]string{}
	}
	if l.Shadow == nil {
		l.Shadow = map[string]string{}
	}
	return l, err
}

// DraftLines returns the lines of a draft in order (removed lines excluded unless all).
func DraftLines(db *store.DB, draftID string, all bool) []Line {
	q := `SELECT ` + lineCols + ` FROM lines WHERE draft_id = ?`
	if !all {
		q += ` AND status <> 'removed'`
	}
	rows, err := db.Query(q+` ORDER BY position`, draftID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Line
	for rows.Next() {
		if l, err := scanLine(rows); err == nil {
			out = append(out, l)
		}
	}
	return out
}

func GetLine(db *store.DB, id string) (Line, error) {
	return scanLine(db.QueryRow(`SELECT `+lineCols+` FROM lines WHERE id = ?`, id))
}

func saveLine(db *store.DB, l *Line) error {
	if l.ID == "" {
		l.ID = store.NewID("ln")
		l.CreatedAt = store.Now()
		if l.Used == nil {
			l.Used = []string{}
		}
		if l.Options == nil {
			l.Options = []string{}
		}
		_, err := db.Exec(`INSERT INTO lines (`+lineCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			l.ID, l.DraftID, l.MessageID, l.Position, l.Said, l.Norm, l.Hint, l.Qty, l.Unit, l.ProductID, l.PackName, l.PackFactor,
			l.QtyMilli, l.Level, store.JSON(l.Used), l.Confidence, store.JSON(l.Options), l.Question, l.Status, store.JSON(l.Checks),
			l.UnitPrice, l.LineTTC, store.JSON(l.Shadow), l.Outcome, l.CreatedAt)
		return err
	}
	_, err := db.Exec(`UPDATE lines SET qty=?, unit=?, product_id=?, pack_name=?, pack_factor=?, qty_milli=?, level=?, used=?,
		confidence=?, options=?, question=?, status=?, checks=?, unit_price=?, line_ttc=?, shadow=?, outcome=? WHERE id=?`,
		l.Qty, l.Unit, l.ProductID, l.PackName, l.PackFactor, l.QtyMilli, l.Level, store.JSON(l.Used), l.Confidence,
		store.JSON(l.Options), l.Question, l.Status, store.JSON(l.Checks), l.UnitPrice, l.LineTTC, store.JSON(l.Shadow), l.Outcome, l.ID)
	return err
}

func nextPosition(db *store.DB, draftID string) int {
	var n sql.NullInt64
	db.QueryRow(`SELECT MAX(position) FROM lines WHERE draft_id = ?`, draftID).Scan(&n)
	return int(n.Int64) + 1
}
