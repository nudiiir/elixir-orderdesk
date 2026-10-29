// Package store opens Order Desk's SQLite database and holds the small helpers
// every other package uses: settings, events and ids.
package store

import (
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type DB struct{ *sql.DB }

// Open creates the database file if needed and applies the schema.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // one writer; SQLite serialises anyway
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	d := &DB{db}
	for _, c := range migrations {
		if err := d.addColumn(c[0], c[1], c[2]); err != nil {
			return nil, fmt.Errorf("migrate %s.%s: %w", c[0], c[1], err)
		}
	}
	return d, nil
}

// Columns added after the first schema (table, column, definition).
var migrations = [][3]string{
	{"drafts", "kind", "TEXT NOT NULL DEFAULT 'order'"}, // order | invoice
	{"drafts", "supplier_id", "TEXT NOT NULL DEFAULT ''"},
	{"drafts", "supplier_said", "TEXT NOT NULL DEFAULT ''"}, // supplier name as printed
	{"drafts", "doc_number", "TEXT NOT NULL DEFAULT ''"},
	{"drafts", "doc_date", "TEXT NOT NULL DEFAULT ''"},
	{"drafts", "doc_total", "INTEGER NOT NULL DEFAULT 0"}, // printed total HT, centimes
	{"drafts", "checks", "TEXT NOT NULL DEFAULT '{}'"},
	{"ground_truth", "kind", "TEXT NOT NULL DEFAULT 'order'"},
	{"ground_truth", "supplier_id", "TEXT NOT NULL DEFAULT ''"},
	{"products", "vat_rate", "INTEGER NOT NULL DEFAULT 0"},
	{"chats", "role", "TEXT NOT NULL DEFAULT 'shop'"}, // shop | owner
	{"pair_codes", "role", "TEXT NOT NULL DEFAULT 'shop'"},
}

func (db *DB) addColumn(table, col, def string) error {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return err
	}
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil && n == col {
			rows.Close()
			return nil
		}
	}
	rows.Close()
	_, err = db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + col + ` ` + def)
	return err
}

// Now is the timestamp format used everywhere (sortable, local time).
func Now() string { return time.Now().Format("2006-01-02 15:04:05") }

// NewID returns a short random id with a prefix, e.g. "ln_3f9a…".
func NewID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// Setting reads a setting, or def when it is not set.
func (db *DB) Setting(key, def string) string {
	var v string
	if db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v) != nil {
		return def
	}
	return v
}

func (db *DB) SettingInt(key string, def int) int {
	if n, err := strconv.Atoi(db.Setting(key, "")); err == nil {
		return n
	}
	return def
}

func (db *DB) SettingFloat(key string, def float64) float64 {
	if f, err := strconv.ParseFloat(db.Setting(key, ""), 64); err == nil {
		return f
	}
	return def
}

func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// Event records something that changed; the UI timelines are drawn from events.
func (db *DB) Event(kind, object, actor string, details any) {
	b, _ := json.Marshal(details)
	db.Exec(`INSERT INTO events (at, kind, object, actor, details) VALUES (?, ?, ?, ?, ?)`,
		Now(), kind, object, actor, string(b))
}

// JSON marshals v for a TEXT column.
func JSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
