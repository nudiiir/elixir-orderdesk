package rules

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

const cols = `id, version, type, scope_kind, scope_id, scope_name, pattern, action, sentence, author, rationale, inspired_by,
	status, reason, bench, strikes, shadow_agree, lines_resolved, ai_cost, created_at, live_at`

func scan(rows interface{ Scan(...any) error }) (*Rule, error) {
	r := &Rule{}
	var action, inspired, bench string
	err := rows.Scan(&r.ID, &r.Version, &r.Type, &r.Scope.Kind, &r.Scope.ID, &r.Scope.Name, &r.Pattern, &action, &r.Sentence,
		&r.Author, &r.Rationale, &inspired, &r.Status, &r.Reason, &bench, &r.Strikes, &r.ShadowOK, &r.Resolved, &r.AICost,
		&r.CreatedAt, &r.LiveAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(action), &r.Action)
	json.Unmarshal([]byte(inspired), &r.InspiredBy)
	r.Bench = json.RawMessage(bench)
	return r, nil
}

// List returns the latest version of every rule (or every version when all is true).
func List(db *store.DB, all bool) ([]*Rule, error) {
	q := `SELECT ` + cols + ` FROM rules r`
	if !all {
		q += ` WHERE version = (SELECT MAX(version) FROM rules x WHERE x.id = r.id)`
	}
	rows, err := db.Query(q + ` ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Rule
	for rows.Next() {
		if r, err := scan(rows); err == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func Get(db *store.DB, id string) (*Rule, error) {
	return scan(db.QueryRow(`SELECT `+cols+` FROM rules WHERE id = ? ORDER BY version DESC LIMIT 1`, id))
}

func Versions(db *store.DB, id string) ([]*Rule, error) {
	rows, err := db.Query(`SELECT `+cols+` FROM rules WHERE id = ? ORDER BY version DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Rule
	for rows.Next() {
		if r, err := scan(rows); err == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

// WithStatus returns the latest versions in the given statuses, validated and ready to run.
func WithStatus(db *store.DB, statuses ...string) []*Rule {
	list, _ := List(db, false)
	var out []*Rule
	for _, r := range list {
		for _, s := range statuses {
			if r.Status == s && r.Validate() == nil {
				out = append(out, r)
			}
		}
	}
	return out
}

// NextID returns the next free rule id (R-0001, R-0002...).
func NextID(db *store.DB) string {
	var n int
	db.QueryRow(`SELECT COUNT(DISTINCT id) FROM rules`).Scan(&n)
	for {
		n++
		id := fmt.Sprintf("R-%04d", n)
		var x string
		if db.QueryRow(`SELECT id FROM rules WHERE id = ?`, id).Scan(&x) == sql.ErrNoRows {
			return id
		}
	}
}

// Insert stores a new rule or a new version of an existing one.
func Insert(db *store.DB, r *Rule) error {
	if r.CreatedAt == "" {
		r.CreatedAt = store.Now()
	}
	if len(r.Bench) == 0 {
		r.Bench = json.RawMessage("{}")
	}
	if r.InspiredBy == nil {
		r.InspiredBy = []string{}
	}
	_, err := db.Exec(`INSERT INTO rules (`+cols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Version, r.Type, r.Scope.Kind, r.Scope.ID, r.Scope.Name, r.Pattern, store.JSON(r.Action), r.Sentence,
		r.Author, r.Rationale, store.JSON(r.InspiredBy), r.Status, r.Reason, string(r.Bench), r.Strikes, r.ShadowOK,
		r.Resolved, r.AICost, r.CreatedAt, r.LiveAt)
	return err
}

// SetStatus changes the status of a rule's latest version.
func SetStatus(db *store.DB, id, status, reason string) error {
	live := ""
	if status == "live" {
		live = store.Now()
	}
	_, err := db.Exec(`UPDATE rules SET status = ?, reason = CASE WHEN ? = '' THEN reason ELSE ? END,
		live_at = CASE WHEN ? <> '' THEN ? ELSE live_at END
		WHERE id = ? AND version = (SELECT MAX(version) FROM rules WHERE id = ?)`, status, reason, reason, live, live, id, id)
	return err
}

// Bump adds to a rule's counters (lines resolved, shadow agreements, strikes).
func Bump(db *store.DB, id, field string, by int) {
	switch field {
	case "lines_resolved", "shadow_agree", "strikes":
		db.Exec(`UPDATE rules SET `+field+` = `+field+` + ? WHERE id = ? AND version = (SELECT MAX(version) FROM rules WHERE id = ?)`, by, id, id)
	}
}
