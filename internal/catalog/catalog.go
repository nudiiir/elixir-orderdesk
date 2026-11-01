// Package catalog is Order Desk's memory of the catalogue: the products mirrored from the ERP,
// their packs, and every wording people use for them (aliases).
package catalog

import (
	"database/sql"
	"regexp"
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
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	NameAr  string   `json:"name_ar"`
	Brand   string   `json:"brand"`
	Barcode string   `json:"barcode"`
	Unit    string   `json:"unit"`
	VatRate int64    `json:"vat_rate"`
	Packs   []Pack   `json:"packs"`
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
	codes    map[string]string  // code key (barcode, ERP or manufacturer reference) → product id; "" = shared by several
	vocab    map[string]bool    // every word and number that appears in a catalogue name
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
	rows, err := c.db.Query(`SELECT id, name, name_ar, brand, barcode, unit, vat_rate FROM products WHERE active = 1 ORDER BY name`)
	if err != nil {
		return err
	}
	for rows.Next() {
		p := &Product{}
		if rows.Scan(&p.ID, &p.Name, &p.NameAr, &p.Brand, &p.Barcode, &p.Unit, &p.VatRate) == nil {
			p.Words, p.Size = Tokens(NormalizeName(p.Name + " " + p.Brand))
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
	codes, vocab := map[string]string{}, map[string]bool{}
	for _, p := range prods {
		for _, k := range codeKeys(p.Barcode) {
			if o, seen := codes[k]; seen && o != p.ID {
				codes[k] = "" // two products share it: never a key
			} else if !seen {
				codes[k] = p.ID
			}
		}
		for _, w := range wordRe.FindAllString(NormalizeName(p.Name+" "+p.Brand), -1) {
			vocab[strings.Trim(w, ".")] = true
		}
	}
	c.mu.Lock()
	c.products, c.byID, c.brands, c.aliases, c.codes, c.vocab = prods, byID, brands, aliases, codes, vocab
	c.mu.Unlock()
	return nil
}

// ---- codes -----------------------------------------------------------------------------------
//
// A barcode, an ERP code or a manufacturer reference names exactly one product: when a line prints
// it, nothing else needs to be read. A supplier's own article code ("ART4881") names one product for
// that supplier only: it is learned from the first confirmed line (see the "code: " aliases).

var codeTok = regexp.MustCompile(`[\p{L}\p{N}]+`)

// codeKeys are the forms a printed code is compared in: lower case, no separators, and without
// leading zeros ("077011" is also printed "77011").
func codeKeys(raw string) []string {
	k := strings.ToLower(strings.Join(codeTok.FindAllString(raw, -1), ""))
	if len([]rune(k)) < 5 || !strings.ContainsAny(k, "0123456789") {
		return nil
	}
	out := []string{k}
	if t := strings.TrimLeft(k, "0"); t != k && len(t) >= 5 {
		out = append(out, t)
	}
	return out
}

// ByCode finds the product whose barcode or reference the text prints, alone or split in groups of
// digits ("2 608 597 145"). Only a code that one product alone carries counts.
func (c *Catalog) ByCode(text string) (*Product, string, bool) {
	toks := codeTok.FindAllString(strings.ToLower(text), -1)
	c.mu.RLock()
	defer c.mu.RUnlock()
	for n := 1; n <= 4; n++ {
		for i := 0; i+n <= len(toks); i++ {
			joined := strings.Join(toks[i:i+n], "")
			for _, k := range codeKeys(joined) {
				if id, ok := c.codes[k]; ok && id != "" {
					return c.byID[id], joined, true
				}
			}
		}
	}
	return nil, "", false
}

// canonicalTok is what the first pass and the size clean-up make of a number that belongs to the
// product (a size, a dimension, a dosage, an inch size, a pack count, a technical unit).
var canonicalTok = regexp.MustCompile(`^(\d+(\.\d+)?(ml|g|ui|pct|mm2|mm|cm|m|a|ma|v|w|kw|ah|mah|bar|nm|kg|l)|dose[\dx.]+(ml)?|[a-z]?\d+(\.\d+)?x\d+(x\d+)?|in\d+|x\d+|n\d+)$`)

// packTok is a pack, box or coil count printed glued to its word ("bt100", "b200", "r100", "crt12"):
// it counts units, it never names a product.
var packTok = regexp.MustCompile(`^(b|bt|bte|box|s|c|cart|crt|ctn|r|rl|x|lot|fd|pk|pq|sac)\d{1,4}$`)

// ForeignCodes are the code-like words of a normalised line that no catalogue name contains: a
// supplier's article code, a reference the catalogue does not carry. They identify a product for one
// supplier (once learned) but are never words of a product name, so the judge does not read them.
func (c *Catalog) ForeignCodes(norm string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []string
	for _, w := range strings.Fields(norm) {
		w = strings.Trim(w, ".")
		digits := 0
		for _, r := range w {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits < 3 || len(w) < 4 || c.vocab[w] || canonicalTok.MatchString(w) || packTok.MatchString(w) {
			continue
		}
		out = append(out, w)
	}
	return out
}

// CodeConflict tells whether a supplier code was ever confirmed for another product in this scope; if
// so, every alias of that code is blocked for good: a real article code names exactly one product.
func (c *Catalog) CodeConflict(code, productID, scopeKind, scopeID string) bool {
	var n int
	c.db.QueryRow(`SELECT COUNT(*) FROM aliases WHERE text = ? AND scope_kind = ? AND scope_id = ? AND product_id <> ?`,
		"code: "+code, scopeKind, scopeID, productID).Scan(&n)
	if n == 0 {
		return false
	}
	c.db.Exec(`UPDATE aliases SET state = 'blocked', last_seen = ? WHERE text = ? AND scope_kind = ? AND scope_id = ?`,
		store.Now(), "code: "+code, scopeKind, scopeID)
	c.Reload()
	return true
}

// HasWord tells whether a word appears in some catalogue name.
func (c *Catalog) HasWord(w string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.vocab[w]
}

// IsCanonical tells whether a word is a size, dimension, dosage, inch size, pack count or technical unit.
func IsCanonical(w string) bool { return canonicalTok.MatchString(w) }

// StripCodes removes a line's foreign codes (see ForeignCodes) before its words are read.
func (c *Catalog) StripCodes(norm string) string {
	codes := c.ForeignCodes(norm)
	if len(codes) == 0 {
		return norm
	}
	drop := map[string]bool{}
	for _, k := range codes {
		drop[k] = true
	}
	var keep []string
	for _, w := range strings.Fields(norm) {
		if !drop[strings.Trim(w, ".")] {
			keep = append(keep, w)
		}
	}
	return strings.Join(keep, " ")
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

func applies(a Alias, customerID, supplierID string) bool {
	switch a.ScopeKind {
	case "customer":
		return a.ScopeID == customerID
	case "supplier":
		return a.ScopeID == supplierID
	}
	return true
}

// Trusted returns the trusted alias for a wording, most specific scope first.
func (c *Catalog) Trusted(norm, customerID, supplierID string) (Alias, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var best Alias
	found := false
	for _, a := range c.aliases[norm] {
		if a.State != "trusted" || !applies(a, customerID, supplierID) || c.byID[a.ProductID] == nil {
			continue
		}
		if !found || scopeRank(a.ScopeKind) > scopeRank(best.ScopeKind) {
			best, found = a, true
		}
	}
	return best, found
}

// Blocked lists the products a wording must never resolve to (blocked aliases).
func (c *Catalog) Blocked(norm, customerID, supplierID string) map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := map[string]bool{}
	for _, a := range c.aliases[norm] {
		if a.State == "blocked" && applies(a, customerID, supplierID) {
			out[a.ProductID] = true
		}
	}
	return out
}

// Hints returns products that candidate aliases point to (they rank first in the shortlist).
func (c *Catalog) Hints(norm, customerID, supplierID string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []string
	for _, a := range c.aliases[norm] {
		if a.State == "candidate" && applies(a, customerID, supplierID) {
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
		if _, err := tx.Exec(`INSERT INTO products (id, name, name_ar, brand, barcode, unit, vat_rate, active, synced_at) VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)
			ON CONFLICT(id) DO UPDATE SET name = excluded.name, name_ar = excluded.name_ar, brand = excluded.brand,
			barcode = excluded.barcode, unit = excluded.unit, vat_rate = excluded.vat_rate, active = 1, synced_at = excluded.synced_at`,
			p.ID, p.Name, p.NameAr, p.Brand, p.Barcode, p.Unit, p.VatRate, now); err != nil {
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

// ReplaceSuppliers stores the ERP's suppliers.
func ReplaceSuppliers(db *store.DB, cs []Customer) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range cs {
		if _, err := tx.Exec(`INSERT INTO suppliers (id, name, phone) VALUES (?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET name = excluded.name, phone = excluded.phone`, c.ID, c.Name, c.Phone); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func SupplierName(db *sql.DB, id string) string {
	var n string
	db.QueryRow(`SELECT name FROM suppliers WHERE id = ?`, id).Scan(&n)
	return n
}

// Suppliers lists the suppliers known from the ERP.
func Suppliers(db *sql.DB) []Customer {
	rows, err := db.Query(`SELECT id, name, phone FROM suppliers ORDER BY name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Customer
	for rows.Next() {
		var c Customer
		if rows.Scan(&c.ID, &c.Name, &c.Phone) == nil {
			out = append(out, c)
		}
	}
	return out
}

// MatchSupplier finds the supplier a printed name refers to: a trusted supplier wording first, then
// the share of the ERP name's words found in the printed name.
func (c *Catalog) MatchSupplier(printed string) (string, float64) {
	norm := Normalize(printed)
	if norm == "" {
		return "", 0
	}
	if a, ok := c.Trusted("supplier: "+norm, "", ""); ok {
		return a.ProductID, 1
	}
	pw, _ := Tokens(norm)
	best, bestScore := "", 0.0
	for _, s := range Suppliers(c.db.DB) {
		sw, _ := Tokens(Normalize(s.Name))
		if len(sw) == 0 {
			continue
		}
		hit := 0.0
		for _, w := range sw {
			m := 0.0
			for _, q := range pw {
				if v := WordSim(w, q); v > m {
					m = v
				}
			}
			hit += m
		}
		if score := hit / float64(len(sw)); score > bestScore {
			best, bestScore = s.ID, score
		}
	}
	return best, bestScore
}
