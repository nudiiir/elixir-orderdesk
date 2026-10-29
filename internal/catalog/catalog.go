// Package catalog is Order Desk's memory of the catalogue: the products mirrored from the ERP,
// their packs, and every wording people use for them (aliases).
package catalog

import (
	"database/sql"
	"sort"
	"strings"
	"sync"

	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

type Pack struct {
	Name        string `json:"name"`
	FactorMilli int64  `json:"factor_milli"`
}

type Product struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	NameAr  string `json:"name_ar"`
	Brand   string `json:"brand"`
	Barcode string `json:"barcode"`
	Unit    string `json:"unit"`
	Packs   []Pack `json:"packs"`
	Words   []string `json:"words"`
	Size    string   `json:"size"`
}

// Alias is a confirmed wording.
type Alias struct {
	ID        int64  `json:"id"`
	Text      string `json:"text"`
	ProductID string `json:"product_id"`
	ScopeKind string `json:"scope_kind"`
	ScopeID   string `json:"scope_id"`
	Source    string `json:"source"`
	Witnesses int    `json:"witnesses"`
	State     string `json:"state"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
}

// Catalog is an in-memory snapshot, reloaded after every sync or alias change.
type Catalog struct {
	mu       sync.RWMutex
	db       *store.DB
	products []*Product
	byID     map[string]*Product
	brands   []string
	aliases  map[string][]Alias // normalised text → aliases (all scopes and states)
}

func New(db *store.DB) *Catalog {
	c := &Catalog{db: db}
	c.Reload()
	return c
}

// Reload reads products, packs and aliases from the database.
func (c *Catalog) Reload() error {
	prods := []*Product{}
	byID := map[string]*Product{}
	rows, err := c.db.Query(`SELECT id, name, name_ar, brand, barcode, unit FROM products WHERE active = 1 ORDER BY name`)
	if err != nil {
		return err
	}
	for rows.Next() {
		p := &Product{}
		if rows.Scan(&p.ID, &p.Name, &p.NameAr, &p.Brand, &p.Barcode, &p.Unit) == nil {
			p.Words, p.Size = Tokens(Normalize(p.Name + " " + p.Brand))
			prods = append(prods, p)
			byID[p.ID] = p
		}
	}
	rows.Close()
	if pr, err := c.db.Query(`SELECT product_id, name, factor_milli FROM packs ORDER BY factor_milli`); err == nil {
		for pr.Next() {
			var id string
			var k Pack
			if pr.Scan(&id, &k.Name, &k.FactorMilli) == nil && byID[id] != nil {
				byID[id].Packs = append(byID[id].Packs, k)
			}
		}
		pr.Close()
	}
	brandSet := map[string]bool{}
	for _, p := range prods {
		if b := strings.TrimSpace(p.Brand); b != "" {
			brandSet[b] = true
		}
	}
	brands := make([]string, 0, len(brandSet))
	for b := range brandSet {
		brands = append(brands, b)
	}
	sort.Strings(brands)

	aliases := map[string][]Alias{}
	if ar, err := c.db.Query(`SELECT id, text, product_id, scope_kind, scope_id, source, witnesses, state, first_seen, last_seen
		FROM aliases`); err == nil {
		for ar.Next() {
			var a Alias
			if ar.Scan(&a.ID, &a.Text, &a.ProductID, &a.ScopeKind, &a.ScopeID, &a.Source, &a.Witnesses, &a.State,
				&a.FirstSeen, &a.LastSeen) == nil {
				aliases[a.Text] = append(aliases[a.Text], a)
			}
		}
		ar.Close()
	}
	c.mu.Lock()
	c.products, c.byID, c.brands, c.aliases = prods, byID, brands, aliases
	c.mu.Unlock()
	return nil
}

func (c *Catalog) Get(id string) *Product {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.byID[id]
}

func (c *Catalog) Products() []*Product {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.products
}

func (c *Catalog) Brands() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.brands
}

// scopeRank: the more specific scope wins.
func scopeRank(kind string) int {
	switch kind {
	case "customer":
		return 3
	case "supplier":
		return 2
	case "tenant":
		return 1
	}
	return 0
}

func applies(a Alias, customerID string) bool {
	return a.ScopeKind != "customer" || a.ScopeID == customerID
}

// Trusted returns the trusted alias for a wording, most specific scope first.
func (c *Catalog) Trusted(norm, customerID string) (Alias, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var best Alias
	found := false
	for _, a := range c.aliases[norm] {
		if a.State != "trusted" || !applies(a, customerID) || c.byID[a.ProductID] == nil {
			continue
		}
		if !found || scopeRank(a.ScopeKind) > scopeRank(best.ScopeKind) {
			best, found = a, true
		}
	}
	return best, found
}

// Blocked lists the products a wording must never resolve to (blocked aliases).
func (c *Catalog) Blocked(norm, customerID string) map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := map[string]bool{}
	for _, a := range c.aliases[norm] {
		if a.State == "blocked" && applies(a, customerID) {
			out[a.ProductID] = true
		}
	}
	return out
}

// Hints returns products that candidate aliases point to (they rank first in the shortlist).
func (c *Catalog) Hints(norm, customerID string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []string
	for _, a := range c.aliases[norm] {
		if a.State == "candidate" && applies(a, customerID) {
			out = append(out, a.ProductID)
		}
	}
	return out
}

// AliasesOf returns every wording known for a product.
func (c *Catalog) AliasesOf(productID string) []Alias {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []Alias
	for _, list := range c.aliases {
		for _, a := range list {
			if a.ProductID == productID {
				out = append(out, a)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Witnesses > out[j].Witnesses })
	return out
}

// Match is the deterministic part of level 2: the products whose words contain every
// product word of the line (exact or prefix) and, when a size is given, have that size.
func (c *Catalog) Match(words []string, size string, exclude map[string]bool) []*Product {
	if len(words) == 0 {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []*Product
	for _, p := range c.products {
		if exclude[p.ID] || (size != "" && p.Size != size) {
			continue
		}
		ok := true
		for _, w := range words {
			best := 0.0
			for _, pw := range p.Words {
				if s := WordSim(w, pw); s > best {
					best = s
				}
			}
			if best < 0.85 {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, p)
		}
	}
	return out
}

// Shortlist (level 3) ranks products against a line: word and size similarity,
// candidate aliases and what this customer usually buys.
func (c *Catalog) Shortlist(words []string, size string, hints []string, usual map[string]int, exclude map[string]bool, n int) []*Product {
	c.mu.RLock()
	defer c.mu.RUnlock()
	type scored struct {
		p *Product
		s float64
	}
	hinted := map[string]bool{}
	for _, h := range hints {
		hinted[h] = true
	}
	var out []scored
	for _, p := range c.products {
		if exclude[p.ID] {
			continue
		}
		s := 0.0
		for _, w := range words {
			best := 0.0
			for _, pw := range p.Words {
				if v := WordSim(w, pw); v > best {
					best = v
				}
			}
			s += best
		}
		if size != "" && p.Size == size {
			s += 1.2
		}
		if hinted[p.ID] {
			s += 3
		}
		if t := usual[p.ID]; t > 0 {
			s += 0.3 + 0.05*float64(min(t, 6))
		}
		if s > 0.6 {
			out = append(out, scored{p, s})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].s > out[j].s })
	res := []*Product{}
	for i := 0; i < len(out) && i < n; i++ {
		res = append(res, out[i].p)
	}
	return res
}

// ---- learning --------------------------------------------------------------------------

// Witness records one confirmation of a wording. It becomes trusted after `trustAfter`
// independent confirmations; a confirmation for another product blocks a trusted alias.
func (c *Catalog) Witness(norm, productID, scopeKind, scopeID, source string, trustAfter int) (Alias, string) {
	now := store.Now()
	change := ""
	// A trusted alias of the same scope that pointed elsewhere was wrong: block it.
	res, _ := c.db.Exec(`UPDATE aliases SET state = 'blocked', last_seen = ? WHERE text = ? AND scope_kind = ? AND scope_id = ?
		AND product_id <> ? AND state = 'trusted'`, now, norm, scopeKind, scopeID, productID)
	if n, _ := res.RowsAffected(); n > 0 {
		change = "blocked"
	}
	_, err := c.db.Exec(`INSERT INTO aliases (text, product_id, scope_kind, scope_id, source, witnesses, state, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?)
		ON CONFLICT(text, scope_kind, scope_id, product_id) DO UPDATE SET witnesses = witnesses + 1, last_seen = excluded.last_seen,
			state = CASE WHEN aliases.state = 'blocked' THEN 'candidate'
			             WHEN aliases.witnesses + 1 >= ? THEN 'trusted' ELSE aliases.state END`,
		norm, productID, scopeKind, scopeID, source, initialState(trustAfter), now, now, trustAfter)
	var a Alias
	if err == nil {
		c.db.QueryRow(`SELECT id, text, product_id, scope_kind, scope_id, source, witnesses, state, first_seen, last_seen
			FROM aliases WHERE text = ? AND scope_kind = ? AND scope_id = ? AND product_id = ?`, norm, scopeKind, scopeID, productID).
			Scan(&a.ID, &a.Text, &a.ProductID, &a.ScopeKind, &a.ScopeID, &a.Source, &a.Witnesses, &a.State, &a.FirstSeen, &a.LastSeen)
		if a.Witnesses == 1 {
			change = "created"
		} else if a.State == "trusted" && a.Witnesses == trustAfter {
			change = "trusted"
		}
	}
	c.Reload()
	return a, change
}

func initialState(trustAfter int) string {
	if trustAfter <= 1 {
		return "trusted"
	}
	return "candidate"
}

// Block marks a wording as never meaning a product (after a person's correction).
func (c *Catalog) Block(norm, productID, scopeKind, scopeID string) {
	c.db.Exec(`UPDATE aliases SET state = 'blocked', last_seen = ? WHERE text = ? AND product_id = ? AND scope_kind = ? AND scope_id = ?`,
		store.Now(), norm, productID, scopeKind, scopeID)
	c.Reload()
}

// SetAliasState lets a person trust or block a wording from the panel.
func (c *Catalog) SetAliasState(id int64, state string) error {
	_, err := c.db.Exec(`UPDATE aliases SET state = ?, last_seen = ? WHERE id = ?`, state, store.Now(), id)
	c.Reload()
	return err
}

// ---- sync from the ERP -----------------------------------------------------------------

// ReplaceProducts stores a fresh catalogue snapshot from the ERP adapter.
func ReplaceProducts(db *store.DB, prods []Product) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := store.Now()
	if _, err := tx.Exec(`UPDATE products SET active = 0`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM packs`); err != nil {
		return err
	}
	for _, p := range prods {
		if _, err := tx.Exec(`INSERT INTO products (id, name, name_ar, brand, barcode, unit, active, synced_at) VALUES (?, ?, ?, ?, ?, ?, 1, ?)
			ON CONFLICT(id) DO UPDATE SET name = excluded.name, name_ar = excluded.name_ar, brand = excluded.brand,
			barcode = excluded.barcode, unit = excluded.unit, active = 1, synced_at = excluded.synced_at`,
			p.ID, p.Name, p.NameAr, p.Brand, p.Barcode, p.Unit, now); err != nil {
			return err
		}
		for _, k := range p.Packs {
			if k.FactorMilli > 1000 {
				tx.Exec(`INSERT OR REPLACE INTO packs (product_id, name, factor_milli) VALUES (?, ?, ?)`, p.ID, k.Name, k.FactorMilli)
			}
		}
	}
	return tx.Commit()
}

type Customer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	RouteID string `json:"route_id"`
}

func ReplaceCustomers(db *store.DB, cs []Customer) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range cs {
		if _, err := tx.Exec(`INSERT INTO customers (id, name, phone, route_id) VALUES (?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET name = excluded.name, phone = excluded.phone, route_id = excluded.route_id`,
			c.ID, c.Name, c.Phone, c.RouteID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func CustomerName(db *sql.DB, id string) string {
	var n string
	db.QueryRow(`SELECT name FROM customers WHERE id = ?`, id).Scan(&n)
	return n
}
