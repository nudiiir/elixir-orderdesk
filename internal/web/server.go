// Package web serves the admin panel and its JSON API. It binds to localhost by default: the
// panel belongs to the distributor's own machine.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/elixir"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/erpnext"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/tryton"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/dolibarr"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/inventree"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/odoo"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/desk"
	"github.com/nudiiir/elixir-orderdesk/internal/engineer"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

//go:embed static
var static embed.FS

type Server struct {
	DB      *store.DB
	Cat     *catalog.Catalog
	Desk    *desk.Desk
	Eng     *engineer.Engineer
	Meta    *meta.Client
	DataDir string
	ERP     func() adapter.ERP
	Connect func(kind, url, db, login, token string) // installs a new ERP adapter
	Sync    func(ctx context.Context) error

	WhatsApp http.Handler // the WhatsApp Cloud API webhook, when configured
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	if s.WhatsApp != nil {
		r.Get("/whatsapp/webhook", s.WhatsApp.ServeHTTP)
		r.Post("/whatsapp/webhook", s.WhatsApp.ServeHTTP)
	}
	r.Route("/api", func(r chi.Router) {
		r.Get("/overview", s.overview)
		r.Get("/drafts", s.drafts)
		r.Get("/drafts/{id}", s.draft)
		r.Post("/drafts/{id}/confirm", s.confirm)
		r.Post("/lines/{id}/choose", s.choose)
		r.Post("/lines/{id}/remove", s.removeLine)
		r.Get("/products", s.products)
		r.Get("/products/{id}", s.product)
		r.Post("/aliases", s.addAlias)
		r.Post("/aliases/{id}/state", s.aliasState)
		r.Get("/rules", s.rulesList)
		r.Get("/rules/{id}", s.rule)
		r.Post("/rules", s.writeRule)
		r.Post("/rules/{id}/status", s.ruleStatus)
		r.Post("/rules/preview", s.rulePreview)
		r.Get("/engineer", s.engineerState)
		r.Post("/engineer/run", s.engineerRun)
		r.Get("/learning", s.learning)
		r.Get("/customers", s.customers)
		r.Post("/customers/{id}/pair-code", s.pairCode)
		r.Post("/owner/pair-code", s.ownerPairCode)
		r.Post("/chats/unlink", s.unlinkChat)
		r.Get("/owner", s.owner)
		r.Post("/invoices", s.uploadInvoice)
		r.Post("/drafts/{id}/supplier", s.setSupplier)
		r.Get("/suppliers", s.suppliers)
		r.Get("/settings", s.settings)
		r.Put("/settings", s.putSettings)
		r.Post("/erp/connect", s.erpConnect)
		r.Post("/erp/sync", s.erpSync)
		r.Post("/try", s.try)
		r.Get("/events", s.events)
	})
	r.Get("/media", s.media)
	sub, _ := fs.Sub(static, "static")
	r.Handle("/*", http.FileServer(http.FS(sub)))
	return r
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decode(r *http.Request, v any) error { return json.NewDecoder(r.Body).Decode(v) }

func (s *Server) productName(id string) string {
	if p := s.Cat.Get(id); p != nil {
		return p.Name
	}
	return ""
}

// ---- overview & learning ---------------------------------------------------------------

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{}
	var lines, free, ai, person, wrongAuto, auto int
	s.DB.QueryRow(`SELECT COUNT(*),
		COALESCE(SUM(level IN ('alias','rules','history')),0),
		COALESCE(SUM(level = 'ai'),0),
		COALESCE(SUM(level = 'person'),0),
		COALESCE(SUM(outcome = 'corrected' AND level IN ('alias','rules')),0),
		COALESCE(SUM(level IN ('alias','rules')),0)
		FROM lines WHERE status <> 'removed' AND said <> ''`).Scan(&lines, &free, &ai, &person, &wrongAuto, &auto)
	var spend, spendToday float64
	var calls int
	s.DB.QueryRow(`SELECT COALESCE(SUM(cost),0) FROM ai_usage`).Scan(&spend)
	s.DB.QueryRow(`SELECT COUNT(*) FROM ai_usage WHERE purpose = 'resolve'`).Scan(&calls)
	spendToday = s.Meta.SpentToday()
	out["lines"], out["free"], out["ai"], out["person"] = lines, free, ai, person
	out["auto"], out["wrong_auto"] = auto, wrongAuto
	out["spend"], out["spend_today"], out["budget_day"] = spend, spendToday, s.DB.SettingFloat("ai_budget_day", 2)
	out["resolve_calls"] = calls
	var orders, open, pending int
	s.DB.QueryRow(`SELECT COALESCE(SUM(status='confirmed'),0), COALESCE(SUM(status='open'),0) FROM drafts`).Scan(&orders, &open)
	s.DB.QueryRow(`SELECT COUNT(*) FROM lines l JOIN drafts d ON d.id = l.draft_id WHERE d.status = 'open' AND l.status IN ('ask','unknown')`).Scan(&pending)
	out["orders"], out["open_drafts"], out["pending_lines"] = orders, open, pending
	var nRules map[string]int = map[string]int{}
	if list, err := rules.List(s.DB, false); err == nil {
		for _, ru := range list {
			nRules[ru.Status]++
		}
	}
	out["rules"] = nRules
	var aliases, trusted int
	s.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(state='trusted'),0) FROM aliases`).Scan(&aliases, &trusted)
	out["aliases"], out["trusted_aliases"] = aliases, trusted
	out["products"] = len(s.Cat.Products())
	erp := map[string]any{"url": s.DB.Setting("erp_url", ""), "user": s.DB.Setting("erp_user", ""), "connected": false,
		"kind": s.DB.Setting("erp_kind", "elixir"), "db": s.DB.Setting("erp_db", ""), "login": s.DB.Setting("erp_login", ""),
		"synced_at": s.DB.Setting("erp_synced_at", ""), "error": s.DB.Setting("erp_error", "")}
	if e := s.ERP(); e != nil {
		erp["name"] = e.Name()
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := e.Ping(ctx); err == nil {
			erp["connected"] = true
		} else {
			erp["error"] = err.Error()
		}
	}
	out["erp"] = erp
	out["telegram"] = s.DB.Setting("telegram_bot", "")
	if s.WhatsApp != nil {
		out["whatsapp"] = s.DB.Setting("whatsapp_number", "configured")
	}
	out["meta"] = s.Meta.Enabled()
	out["engineer_running"] = s.Eng.Running()
	writeJSON(w, http.StatusOK, out)
}

// learning returns daily series: how lines were resolved, AI cost per 1,000 lines, spend by job.
func (s *Server) learning(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	kind := r.URL.Query().Get("kind") // "" | order | invoice
	if days <= 0 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days+1).Format("2006-01-02")
	type day struct {
		Day       string  `json:"day"`
		Alias     int     `json:"alias"`
		Rules     int     `json:"rules"`
		History   int     `json:"history"`
		AI        int     `json:"ai"`
		Person    int     `json:"person"`
		Lines     int     `json:"lines"`
		Cost      float64 `json:"cost"`
		Calls     int     `json:"resolve_calls"`
		RulesLive []string `json:"rules_live"`
		RulesBack []string `json:"rules_back"`
	}
	byDay := map[string]*day{}
	get := func(d string) *day {
		if byDay[d] == nil {
			byDay[d] = &day{Day: d, RulesLive: []string{}, RulesBack: []string{}}
		}
		return byDay[d]
	}
	rows, _ := s.DB.Query(`SELECT substr(created_at,1,10), level, COUNT(*) FROM lines WHERE status <> 'removed' AND said <> ''
		AND created_at >= ? AND (? = '' OR draft_id IN (SELECT id FROM drafts WHERE kind = ?)) GROUP BY 1, 2`, since, kind, kind)
	for rows != nil && rows.Next() {
		var d, lvl string
		var n int
		rows.Scan(&d, &lvl, &n)
		x := get(d)
		x.Lines += n
		switch lvl {
		case "alias":
			x.Alias += n
		case "rules":
			x.Rules += n
		case "ai":
			x.AI += n
		case "person", "none":
			x.Person += n
		}
	}
	if rows != nil {
		rows.Close()
	}
	rows, _ = s.DB.Query(`SELECT substr(at,1,10), COALESCE(SUM(cost),0), COALESCE(SUM(purpose='resolve'),0) FROM ai_usage
		WHERE at >= ? AND purpose <> 'engineer' AND (? = '' OR (CASE WHEN ref IN (SELECT id FROM messages WHERE kind = 'invoice') THEN 'invoice' ELSE 'order' END) = ?) GROUP BY 1`, since, kind, kind)
	for rows != nil && rows.Next() {
		var d string
		var c float64
		var n int
		rows.Scan(&d, &c, &n)
		x := get(d)
		x.Cost, x.Calls = c, n
	}
	if rows != nil {
		rows.Close()
	}
	rows, _ = s.DB.Query(`SELECT substr(at,1,10), object, kind FROM events WHERE kind IN ('rule.live','rule.rolled_back') AND at >= ?`, since)
	for rows != nil && rows.Next() {
		var d, id, k string
		rows.Scan(&d, &id, &k)
		if k == "rule.live" {
			get(d).RulesLive = append(get(d).RulesLive, id)
		} else {
			get(d).RulesBack = append(get(d).RulesBack, id)
		}
	}
	if rows != nil {
		rows.Close()
	}
	var series []*day
	for i := 0; i < days; i++ {
		d := time.Now().AddDate(0, 0, -days+1+i).Format("2006-01-02")
		if byDay[d] != nil {
			series = append(series, byDay[d])
		}
	}
	type job struct {
		Model   string  `json:"model"`
		Purpose string  `json:"purpose"`
		Calls   int     `json:"calls"`
		Cost    float64 `json:"cost"`
	}
	var jobs []job
	rows, _ = s.DB.Query(`SELECT model, purpose, COUNT(*), COALESCE(SUM(cost),0) FROM ai_usage WHERE at >= ? AND (? = '' OR (CASE WHEN ref IN (SELECT id FROM messages WHERE kind = 'invoice') THEN 'invoice' ELSE 'order' END) = ?)
		AND (? = '' OR purpose <> 'engineer') GROUP BY 1, 2 ORDER BY 4 DESC`, since, kind, kind, kind)
	for rows != nil && rows.Next() {
		var j job
		rows.Scan(&j.Model, &j.Purpose, &j.Calls, &j.Cost)
		jobs = append(jobs, j)
	}
	if rows != nil {
		rows.Close()
	}
	// Saved by rules: lines resolved for free × the average AI cost of a line that needed the AI.
	var aiCost float64
	var aiLines, freeLines int
	s.DB.QueryRow(`SELECT COALESCE(SUM(cost),0) FROM ai_usage WHERE purpose IN ('resolve','extract') AND at >= ? AND (? = '' OR (CASE WHEN ref IN (SELECT id FROM messages WHERE kind = 'invoice') THEN 'invoice' ELSE 'order' END) = ?)`,
		since, kind, kind).Scan(&aiCost)
	s.DB.QueryRow(`SELECT COALESCE(SUM(level='ai' OR level='person'),0), COALESCE(SUM(level IN ('alias','rules')),0) FROM lines
		WHERE created_at >= ? AND said <> '' AND (? = '' OR draft_id IN (SELECT id FROM drafts WHERE kind = ?))`, since, kind, kind).Scan(&aiLines, &freeLines)
	// totals for the headline figures of this view
	var tot struct {
		Lines     int `json:"lines"`
		Free      int `json:"free"`
		Auto      int `json:"auto"`
		WrongAuto int `json:"wrong_auto"`
		Calls     int `json:"resolve_calls"`
		Docs      int `json:"docs"`
	}
	s.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(level IN ('alias','rules','history')),0), COALESCE(SUM(level IN ('alias','rules')),0),
		COALESCE(SUM(outcome = 'corrected' AND level IN ('alias','rules')),0) FROM lines WHERE status <> 'removed' AND said <> '' AND (? = '' OR draft_id IN (SELECT id FROM drafts WHERE kind = ?))`,
		kind, kind).Scan(&tot.Lines, &tot.Free, &tot.Auto, &tot.WrongAuto)
	s.DB.QueryRow(`SELECT COUNT(*) FROM ai_usage WHERE purpose = 'resolve' AND (? = '' OR (CASE WHEN ref IN (SELECT id FROM messages WHERE kind = 'invoice') THEN 'invoice' ELSE 'order' END) = ?)`, kind, kind).Scan(&tot.Calls)
	s.DB.QueryRow(`SELECT COUNT(*) FROM drafts WHERE status = 'confirmed' AND (? = '' OR kind = ?)`, kind, kind).Scan(&tot.Docs)
	perLine := 0.0
	if aiLines > 0 {
		perLine = aiCost / float64(aiLines)
	}
	writeJSON(w, http.StatusOK, map[string]any{"days": series, "jobs": jobs, "saved": perLine * float64(freeLines),
		"cost_per_ai_line": perLine, "free_lines": freeLines, "totals": tot, "kind": kind})
}

// ---- drafts & lines --------------------------------------------------------------------

type draftRow struct {
	ID         string `json:"id"`
	ChatKey    string `json:"chat_key"`
	CustomerID string `json:"customer_id"`
	Customer   string `json:"customer"`
	Status     string `json:"status"`
	Number     string `json:"erp_order_number"`
	TotalTTC   int64  `json:"total_ttc"`
	Delivery   string `json:"delivery"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	Lines      int    `json:"lines"`
	Pending    int     `json:"pending"`
	Kinds      string  `json:"kinds"`
	AICost     float64 `json:"ai_cost"`
	Free       int     `json:"free"`
	Copied     int     `json:"copied"`
	Number2    string  `json:"doc_number"`
}

func (s *Server) drafts(w http.ResponseWriter, r *http.Request) {
	st, kind := r.URL.Query().Get("status"), r.URL.Query().Get("kind")
	q := `SELECT d.id, d.chat_key, d.customer_id,
		CASE WHEN d.kind = 'invoice' THEN COALESCE((SELECT name FROM suppliers s WHERE s.id = d.supplier_id), d.supplier_said || ' ?') ELSE COALESCE(c.name,'') END,
		d.status, d.erp_order_number, d.total_ttc, d.delivery,
		d.created_at, d.updated_at,
		(SELECT COUNT(*) FROM lines l WHERE l.draft_id = d.id AND l.status <> 'removed'),
		(SELECT COUNT(*) FROM lines l WHERE l.draft_id = d.id AND l.status IN ('ask','unknown')),
		COALESCE((SELECT GROUP_CONCAT(DISTINCT m.kind) FROM messages m JOIN lines l ON l.message_id = m.id WHERE l.draft_id = d.id),''),
		COALESCE((SELECT SUM(u.cost) FROM ai_usage u WHERE u.ref IN (SELECT DISTINCT message_id FROM lines l WHERE l.draft_id = d.id)),0),
		(SELECT COUNT(*) FROM lines l WHERE l.draft_id = d.id AND l.status = 'ok' AND l.level IN ('alias','rules')),
		(SELECT COUNT(*) FROM lines l WHERE l.draft_id = d.id AND l.status = 'ok' AND l.level = 'history'),
		d.doc_number
		FROM drafts d LEFT JOIN customers c ON c.id = d.customer_id WHERE 1=1`
	args := []any{}
	if st != "" {
		q += ` AND d.status = ?`
		args = append(args, st)
	}
	if kind != "" {
		q += ` AND d.kind = ?`
		args = append(args, kind)
	}
	rows, err := s.DB.Query(q+` ORDER BY d.updated_at DESC LIMIT 200`, args...)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []draftRow{}
	for rows.Next() {
		var d draftRow
		rows.Scan(&d.ID, &d.ChatKey, &d.CustomerID, &d.Customer, &d.Status, &d.Number, &d.TotalTTC, &d.Delivery, &d.CreatedAt,
			&d.UpdatedAt, &d.Lines, &d.Pending, &d.Kinds, &d.AICost, &d.Free, &d.Copied, &d.Number2)
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

type lineOut struct {
	desk.Line
	ProductName string            `json:"product_name"`
	OptionNames map[string]string `json:"option_names"`
	RuleNames   map[string]string `json:"rule_sentences"`
	Alts        []string          `json:"alts"`
}

func (s *Server) lineOut(l desk.Line) lineOut {
	o := lineOut{Line: l, ProductName: s.productName(l.ProductID), OptionNames: map[string]string{}, RuleNames: map[string]string{}}
	for _, id := range l.Options {
		o.OptionNames[id] = s.productName(id)
	}
	for _, u := range l.Used {
		if strings.HasPrefix(u, "R-") {
			if ru, err := rules.Get(s.DB, u); err == nil {
				o.RuleNames[u] = ru.Sentence
			}
		}
	}
	if id := l.Checks["corrected_from"]; id != "" {
		o.OptionNames[id] = s.productName(id)
	}
	if l.Status == "ok" { // what a person could correct it to
		words, size := catalog.Tokens(catalog.Normalize(l.Said + " " + l.Hint))
		for _, p := range s.Cat.Shortlist(words, size, nil, nil, map[string]bool{l.ProductID: true}, 4) {
			o.Alts = append(o.Alts, p.ID)
			o.OptionNames[p.ID] = p.Name
		}
	}
	return o
}

func (s *Server) draft(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var d draftRow
	var inv struct {
		Kind       string            `json:"kind"`
		SupplierID string            `json:"supplier_id"`
		Supplier   string            `json:"supplier"`
		Said       string            `json:"supplier_said"`
		Number     string            `json:"doc_number"`
		Date       string            `json:"doc_date"`
		Total      int64             `json:"doc_total"`
		Checks     map[string]string `json:"checks"`
	}
	var checks string
	err := s.DB.QueryRow(`SELECT d.id, d.chat_key, d.customer_id, COALESCE(c.name,''), d.status, d.erp_order_number, d.total_ttc,
		d.delivery, d.created_at, d.updated_at, d.kind, d.supplier_id, d.supplier_said, d.doc_number, d.doc_date, d.doc_total, d.checks
		FROM drafts d LEFT JOIN customers c ON c.id = d.customer_id WHERE d.id = ?`, id).
		Scan(&d.ID, &d.ChatKey, &d.CustomerID, &d.Customer, &d.Status, &d.Number, &d.TotalTTC, &d.Delivery, &d.CreatedAt, &d.UpdatedAt,
			&inv.Kind, &inv.SupplierID, &inv.Said, &inv.Number, &inv.Date, &inv.Total, &checks)
	json.Unmarshal([]byte(checks), &inv.Checks)
	inv.Supplier = catalog.SupplierName(s.DB.DB, inv.SupplierID)
	if err != nil {
		writeErr(w, 404, "draft not found")
		return
	}
	lines := []lineOut{}
	var total int64
	for _, l := range desk.DraftLines(s.DB, id, true) {
		lines = append(lines, s.lineOut(l))
		if l.Status == "ok" || inv.Kind == "invoice" {
			total += l.LineTTC
		}
	}
	type msg struct {
		ID         string  `json:"id"`
		Kind       string  `json:"kind"`
		Text       string  `json:"text"`
		Media      string  `json:"media"`
		Extraction string  `json:"extraction"`
		CreatedAt  string  `json:"created_at"`
		Cost       float64 `json:"ai_cost"`
	}
	msgs := []msg{}
	rows, _ := s.DB.Query(`SELECT m.id, m.kind, m.text, m.media_path, m.extraction, m.created_at,
		COALESCE((SELECT SUM(cost) FROM ai_usage u WHERE u.ref = m.id),0)
		FROM messages m WHERE m.id IN (SELECT DISTINCT message_id FROM lines WHERE draft_id = ?) ORDER BY m.created_at`, id)
	for rows != nil && rows.Next() {
		var m msg
		rows.Scan(&m.ID, &m.Kind, &m.Text, &m.Media, &m.Extraction, &m.CreatedAt, &m.Cost)
		if m.Media != "" {
			m.Media = "/media?path=" + filepath.Base(filepath.Dir(m.Media)) + "/" + filepath.Base(m.Media)
		}
		msgs = append(msgs, m)
	}
	if rows != nil {
		rows.Close()
	}
	writeJSON(w, http.StatusOK, map[string]any{"draft": d, "invoice": inv, "lines": lines, "messages": msgs, "total": total,
		"suppliers": catalog.Suppliers(s.DB.DB)})
}

func (s *Server) confirm(w http.ResponseWriter, r *http.Request) {
	ref, err := s.Desk.Confirm(r.Context(), chi.URLParam(r, "id"), "clerk")
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ref)
}

func (s *Server) choose(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProductID  string `json:"product_id"`
		Correction bool   `json:"correction"`
	}
	if decode(r, &in) != nil || s.Cat.Get(in.ProductID) == nil {
		writeErr(w, 400, "unknown product")
		return
	}
	l, err := desk.GetLine(s.DB, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, 404, "line not found")
		return
	}
	var status string
	s.DB.QueryRow(`SELECT status FROM drafts WHERE id = ?`, l.DraftID).Scan(&status)
	wrong := l.ProductID
	s.Desk.Choose(r.Context(), &l, in.ProductID, in.Correction)
	if status == "confirmed" {
		s.Desk.Relearn(l, wrong)
	}
	writeJSON(w, http.StatusOK, s.lineOut(l))
}

func (s *Server) removeLine(w http.ResponseWriter, r *http.Request) {
	s.DB.Exec(`UPDATE lines SET status = 'removed', outcome = 'removed' WHERE id = ?`, chi.URLParam(r, "id"))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// try runs a message through the whole pipeline from the panel, without a chat channel.
func (s *Server) try(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CustomerID string `json:"customer_id"`
		Text       string `json:"text"`
	}
	if decode(r, &in) != nil || in.CustomerID == "" || strings.TrimSpace(in.Text) == "" {
		writeErr(w, 400, "customer_id and text are required")
		return
	}
	res, err := s.Desk.Ingest(r.Context(), "panel:"+in.CustomerID, in.CustomerID, store.NewID("msg"), "text", in.Text, "")
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---- catalogue ---------------------------------------------------------------------------

func (s *Server) products(w http.ResponseWriter, r *http.Request) {
	q := catalog.Normalize(r.URL.Query().Get("q"))
	counts := map[string][2]int{}
	rows, _ := s.DB.Query(`SELECT product_id, COUNT(*), COALESCE(SUM(state='trusted'),0) FROM aliases GROUP BY product_id`)
	for rows != nil && rows.Next() {
		var id string
		var n, t int
		rows.Scan(&id, &n, &t)
		counts[id] = [2]int{n, t}
	}
	if rows != nil {
		rows.Close()
	}
	lineCounts := map[string][2]int{}
	rows, _ = s.DB.Query(`SELECT product_id, COUNT(*), COALESCE(SUM(level IN ('alias','rules')),0) FROM lines WHERE status = 'ok' AND said <> '' GROUP BY product_id`)
	for rows != nil && rows.Next() {
		var id string
		var n, f int
		rows.Scan(&id, &n, &f)
		lineCounts[id] = [2]int{n, f}
	}
	if rows != nil {
		rows.Close()
	}
	type item struct {
		*catalog.Product
		Aliases  int `json:"aliases"`
		Trusted  int `json:"trusted"`
		Lines    int `json:"lines"`
		FreeLine int `json:"free_lines"`
	}
	out := []item{}
	for _, p := range s.Cat.Products() {
		if q != "" && !strings.Contains(catalog.Normalize(p.Name+" "+p.Brand), q) {
			continue
		}
		c, lc := counts[p.ID], lineCounts[p.ID]
		out = append(out, item{Product: p, Aliases: c[0], Trusted: c[1], Lines: lc[0], FreeLine: lc[1]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) product(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p := s.Cat.Get(id)
	if p == nil {
		writeErr(w, 404, "product not found")
		return
	}
	aliases := s.Cat.AliasesOf(id)
	type al struct {
		catalog.Alias
		Scope string `json:"scope_name"`
	}
	outA := []al{}
	for _, a := range aliases {
		name := map[string]string{"global": "Global", "tenant": "Tenant"}[a.ScopeKind]
		if a.ScopeKind == "customer" {
			name = catalog.CustomerName(s.DB.DB, a.ScopeID)
		}
		outA = append(outA, al{Alias: a, Scope: name})
	}
	// rules that resolved lines of this product, or point at it
	ruleIDs := map[string]bool{}
	rows, _ := s.DB.Query(`SELECT used FROM lines WHERE product_id = ? AND status = 'ok'`, id)
	for rows != nil && rows.Next() {
		var u string
		rows.Scan(&u)
		var list []string
		json.Unmarshal([]byte(u), &list)
		for _, x := range list {
			if strings.HasPrefix(x, "R-") {
				ruleIDs[x] = true
			}
		}
	}
	if rows != nil {
		rows.Close()
	}
	var rs []*rules.Rule
	all, _ := rules.List(s.DB, false)
	for _, ru := range all {
		if ruleIDs[ru.ID] || ru.Action.Product == id || ru.Action.Block == id {
			rs = append(rs, ru)
		}
	}
	var n, free int
	s.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(level IN ('alias','rules')),0) FROM lines WHERE product_id = ? AND status = 'ok' AND said <> ''`, id).Scan(&n, &free)
	// how the code reads this name with the live vocabulary (the same reading a line gets)
	live := rules.NewSet(rules.WithStatus(s.DB, "live"))
	read := resolve.ProductProfile(live, p)
	from := map[string][]string{}
	for a, ids := range read.From {
		from[a] = ids
	}
	writeJSON(w, http.StatusOK, map[string]any{"product": p, "aliases": outA, "rules": rs, "lines": n, "free_lines": free,
		"read_as": map[string]any{"words": read.Words, "attrs": read.Attrs, "from": from}})
}

func (s *Server) addAlias(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text      string `json:"text"`
		ProductID string `json:"product_id"`
		ScopeKind string `json:"scope_kind"`
		ScopeID   string `json:"scope_id"`
	}
	if decode(r, &in) != nil || s.Cat.Get(in.ProductID) == nil || strings.TrimSpace(in.Text) == "" {
		writeErr(w, 400, "text and product_id are required")
		return
	}
	if in.ScopeKind == "" {
		in.ScopeKind = "global"
	}
	a, _ := s.Cat.Witness(catalog.Normalize(in.Text), in.ProductID, in.ScopeKind, in.ScopeID, "clerk", 1)
	s.Cat.SetAliasState(a.ID, "trusted")
	s.DB.Event("alias.created", in.ProductID, "clerk", map[string]any{"text": a.Text, "alias": a.ID})
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) aliasState(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var in struct {
		State string `json:"state"`
	}
	if decode(r, &in) != nil || (in.State != "trusted" && in.State != "blocked" && in.State != "candidate") {
		writeErr(w, 400, "state must be trusted, candidate or blocked")
		return
	}
	s.Cat.SetAliasState(id, in.State)
	s.DB.Event("alias."+in.State, chi.URLParam(r, "id"), "clerk", map[string]any{"alias": id})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- rules ---------------------------------------------------------------------------

func (s *Server) rulesList(w http.ResponseWriter, r *http.Request) {
	list, err := rules.List(s.DB, false)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if list == nil {
		list = []*rules.Rule{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) rule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ru, err := rules.Get(s.DB, id)
	if err != nil {
		writeErr(w, 404, "rule not found")
		return
	}
	versions, _ := rules.Versions(s.DB, id)
	type ev struct {
		At      string          `json:"at"`
		Kind    string          `json:"kind"`
		Actor   string          `json:"actor"`
		Details json.RawMessage `json:"details"`
	}
	events := []ev{}
	rows, _ := s.DB.Query(`SELECT at, kind, actor, details FROM events WHERE object = ? ORDER BY id`, id)
	for rows != nil && rows.Next() {
		var e ev
		var d string
		rows.Scan(&e.At, &e.Kind, &e.Actor, &d)
		e.Details = json.RawMessage(d)
		events = append(events, e)
	}
	if rows != nil {
		rows.Close()
	}
	// the lines that inspired it, and the lines it resolved since
	inspired := []map[string]string{}
	for _, lid := range ru.InspiredBy {
		var said, pid, at, level string
		if s.DB.QueryRow(`SELECT said, product_id, created_at, level FROM ground_truth WHERE line_id = ?`, lid).Scan(&said, &pid, &at, &level) == nil {
			inspired = append(inspired, map[string]string{"said": said, "product": s.productName(pid), "at": at, "level": level})
		}
	}
	resolved := []map[string]string{}
	rows, _ = s.DB.Query(`SELECT l.said, l.product_id, l.created_at, l.used, l.level,
		CASE WHEN d.kind = 'invoice' THEN COALESCE((SELECT name FROM suppliers x WHERE x.id = d.supplier_id),'') ELSE COALESCE((SELECT name FROM customers c WHERE c.id = d.customer_id),'') END
		FROM lines l JOIN drafts d ON d.id = l.draft_id WHERE l.used LIKE ? AND l.status = 'ok' ORDER BY l.created_at DESC LIMIT 12`,
		`%"`+id+`"%`)
	for rows != nil && rows.Next() {
		var said, pid, at, used, level, who string
		rows.Scan(&said, &pid, &at, &used, &level, &who)
		resolved = append(resolved, map[string]string{"said": said, "product": s.productName(pid), "at": at, "used": used, "level": level, "who": who})
	}
	if rows != nil {
		rows.Close()
	}
	var cost float64
	s.DB.QueryRow(`SELECT COALESCE(AVG(cost),0) FROM ai_usage WHERE purpose IN ('resolve','extract')`).Scan(&cost)
	var perLine float64
	var aiLines int
	s.DB.QueryRow(`SELECT COUNT(*) FROM lines WHERE level IN ('ai','person') AND said <> ''`).Scan(&aiLines)
	var aiCost float64
	s.DB.QueryRow(`SELECT COALESCE(SUM(cost),0) FROM ai_usage WHERE purpose IN ('resolve','extract')`).Scan(&aiCost)
	if aiLines > 0 {
		perLine = aiCost / float64(aiLines)
	}
	writeJSON(w, http.StatusOK, map[string]any{"rule": ru, "versions": versions, "events": events, "inspired": inspired,
		"resolved": resolved, "saved": perLine * float64(ru.Resolved), "cost_per_ai_line": perLine,
		"product_name": s.productName(ru.Action.Product), "shadow_needed": s.DB.SettingInt("shadow_agreements", 5),
		"rollback_strikes": s.DB.SettingInt("rollback_strikes", 2), "gate_min_gain": s.DB.SettingInt("gate_min_gain", 2)})
}

type ruleIn struct {
	Type       string       `json:"type"`
	Scope      rules.Scope  `json:"scope"`
	Pattern    string       `json:"pattern"`
	Action     rules.Action `json:"action"`
	SkipShadow bool         `json:"skip_shadow"`
	Rationale  string       `json:"rationale"`
}

func (in ruleIn) rule() rules.Rule {
	return rules.Rule{Type: in.Type, Scope: in.Scope, Pattern: in.Pattern, Action: in.Action, Rationale: in.Rationale}
}

func (s *Server) writeRule(w http.ResponseWriter, r *http.Request) {
	var in ruleIn
	if err := decode(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	ru, b, err := s.Eng.Submit(in.rule(), "clerk", in.SkipShadow)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rule": ru, "bench": b})
}

// rulePreview benchmarks a rule without saving it (the "Test" button of the rule form).
func (s *Server) rulePreview(w http.ResponseWriter, r *http.Request) {
	var in ruleIn
	if err := decode(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	ru := in.rule()
	ru.ID, ru.Version = "candidate", 1
	if ru.Scope.Kind == "" {
		ru.Scope.Kind = "global"
	}
	if err := ru.Validate(); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	live := rules.NewSet(rules.WithStatus(s.DB, "live"))
	b := engineer.RunBench(s.Cat, live, &ru, engineer.GroundTruth(s.DB))
	ok, verdict := engineer.Gate(b, 0)
	writeJSON(w, http.StatusOK, map[string]any{"bench": b, "passes": ok, "verdict": verdict, "sentence": ru.MakeSentence(s.productName)})
}

func (s *Server) ruleStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var in struct {
		Status string `json:"status"`
	}
	if decode(r, &in) != nil {
		writeErr(w, 400, "bad body")
		return
	}
	switch in.Status {
	case "live", "disabled", "rolled_back", "shadow":
	default:
		writeErr(w, 400, "status must be live, shadow, disabled or rolled_back")
		return
	}
	if err := rules.SetStatus(s.DB, id, in.Status, "set by a person"); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.DB.Event("rule."+in.Status, id, "clerk", map[string]string{"by": "panel"})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) engineerState(w http.ResponseWriter, r *http.Request) {
	var last string
	s.DB.QueryRow(`SELECT details FROM events WHERE kind = 'engineer.finished' ORDER BY id DESC LIMIT 1`).Scan(&last)
	var rep any
	json.Unmarshal([]byte(last), &rep)
	truth := engineer.GroundTruth(s.DB)
	live := rules.NewSet(rules.WithStatus(s.DB, "live"))
	writeJSON(w, http.StatusOK, map[string]any{"running": s.Eng.Running(), "last": rep, "last_run": s.DB.Setting("engineer_last_run", ""),
		"benchmark": len(truth), "residue": len(engineer.Residue(s.Cat, live, truth))})
}

func (s *Server) engineerRun(w http.ResponseWriter, r *http.Request) {
	if s.Eng.Running() {
		writeErr(w, 409, "the rule engineer is already running")
		return
	}
	go s.Eng.Run(context.Background(), "manual")
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

// ---- customers, settings, ERP ---------------------------------------------------------------

func (s *Server) customers(w http.ResponseWriter, r *http.Request) {
	type cu struct {
		catalog.Customer
		Chats  []string `json:"chats"`
		Code   string   `json:"pair_code"`
		Orders int      `json:"orders"`
	}
	rows, err := s.DB.Query(`SELECT c.id, c.name, c.phone, c.route_id,
		COALESCE((SELECT GROUP_CONCAT(chat_key || '|' || name, ',,') FROM chats h WHERE h.customer_id = c.id AND h.role = 'shop'),''),
		COALESCE((SELECT code FROM pair_codes p WHERE p.customer_id = c.id AND p.used_at = '' ORDER BY created_at DESC LIMIT 1),''),
		(SELECT COUNT(*) FROM drafts d WHERE d.customer_id = c.id AND d.status = 'confirmed')
		FROM customers c ORDER BY c.name`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []cu{}
	for rows.Next() {
		var c cu
		var chats string
		rows.Scan(&c.ID, &c.Name, &c.Phone, &c.RouteID, &chats, &c.Code, &c.Orders)
		c.Chats = []string{}
		if chats != "" {
			c.Chats = strings.Split(chats, ",,")
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "bot": s.DB.Setting("telegram_bot", "")})
}

func (s *Server) pairCode(w http.ResponseWriter, r *http.Request) {
	code, err := desk.NewPairCode(s.DB, chi.URLParam(r, "id"), "shop")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"code": code, "bot": s.DB.Setting("telegram_bot", "")})
}

// Editable settings and their defaults.
var settingKeys = map[string]string{
	"alias_trust_after": "2", "shadow_agreements": "5", "rollback_strikes": "2", "gate_min_gain": "2",
	"engineer_auto": "1", "engineer_every": "50", "engineer_model": meta.Spark, "engineer_rounds": "8",
	"engineer_fallback_model": meta.Wonder, "engineer_fallback_after": "100",
	"ai_budget_day": "2", "price_transcribe_min": "0.006", "receipt_post": "1",
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	out := map[string]string{}
	for k, def := range settingKeys {
		out[k] = s.DB.Setting(k, def)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var in map[string]string
	if decode(r, &in) != nil {
		writeErr(w, 400, "bad body")
		return
	}
	for k, v := range in {
		if _, ok := settingKeys[k]; ok {
			s.DB.SetSetting(k, strings.TrimSpace(v))
		}
	}
	s.DB.Event("settings.changed", "settings", "clerk", in)
	s.settings(w, r)
}

// erpConnect opens a session on the ERP with the person's credentials. Only the session token is
// kept; the password is used once and never stored.
func (s *Server) erpConnect(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind     string `json:"kind"` // elixir | odoo | erpnext | tryton | dolibarr | inventree
		URL      string `json:"url"`
		Database string `json:"database"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decode(r, &in) != nil || in.URL == "" || in.Username == "" {
		writeErr(w, 400, "url and username are required")
		return
	}
	in.URL = strings.TrimRight(strings.TrimSpace(in.URL), "/")
	var tok, name string
	var err error
	switch in.Kind {
	case "odoo":
		// Odoo's API takes the login with an API key (or the password) on every call: it is kept,
		// so an API key that can be revoked in Odoo is the right thing to give here.
		if in.Database == "" {
			writeErr(w, 400, "the Odoo database name is required")
			return
		}
		name, err = odoo.New(in.URL, in.Database, in.Username, in.Password).Authenticate(r.Context())
		tok = in.Password
	case "erpnext":
		// the password is used once to generate an API key and secret; only those are kept
		var key, secret string
		key, secret, name, err = erpnext.Login(r.Context(), in.URL, in.Username, in.Password)
		tok = key + ":" + secret
	case "tryton":
		// a user application key is created with the password once; the key is kept and can be revoked in Tryton
		if in.Database == "" {
			writeErr(w, 400, "the Tryton database name is required")
			return
		}
		tok, name, err = tryton.Login(r.Context(), in.URL, in.Database, in.Username, in.Password)
	case "dolibarr":
		// the password is used once to fetch the user's API key; only the key is kept
		tok, name, err = dolibarr.Login(r.Context(), in.URL, in.Username, in.Password)
	case "inventree":
		// the password is used once to create a revocable API token; only the token is kept
		tok, name, err = inventree.Login(r.Context(), in.URL, in.Username, in.Password)
	default:
		in.Kind = "elixir"
		tok, name, err = elixir.Login(r.Context(), in.URL, in.Username, in.Password)
	}
	if err != nil {
		writeErr(w, 400, "login failed: "+err.Error())
		return
	}
	s.DB.SetSetting("erp_kind", in.Kind)
	s.DB.SetSetting("erp_url", in.URL)
	s.DB.SetSetting("erp_db", in.Database)
	s.DB.SetSetting("erp_login", in.Username)
	s.DB.SetSetting("erp_user", name)
	s.DB.SetSetting("erp_token", tok)
	s.Connect(in.Kind, in.URL, in.Database, in.Username, tok)
	s.DB.Event("erp.connected", "erp", "clerk", map[string]string{"kind": in.Kind, "url": in.URL, "user": name})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := s.Sync(ctx); err != nil {
		writeErr(w, 502, "connected, but the first sync failed: "+err.Error())
		return
	}
	s.overview(w, r)
}

func (s *Server) erpSync(w http.ResponseWriter, r *http.Request) {
	if err := s.Sync(r.Context()); err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": len(s.Cat.Products()), "synced_at": s.DB.Setting("erp_synced_at", "")})
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 60
	}
	rows, err := s.DB.Query(`SELECT id, at, kind, object, actor, details FROM events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var at, kind, obj, actor, det string
		rows.Scan(&id, &at, &kind, &obj, &actor, &det)
		out = append(out, map[string]any{"id": id, "at": at, "kind": kind, "object": obj, "actor": actor, "details": json.RawMessage(det)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// media serves a stored voice note or photo (evidence), only from the data directory.
func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	rel := filepath.Clean(r.URL.Query().Get("path"))
	if strings.Contains(rel, "..") || filepath.IsAbs(rel) {
		http.NotFound(w, r)
		return
	}
	p := filepath.Join(s.DataDir, "media", rel)
	if _, err := os.Stat(p); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, p)
}
