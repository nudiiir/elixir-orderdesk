package web

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/desk"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// uploadInvoice: the owner drops a supplier invoice (photo or PDF) in the panel.
func (s *Server) uploadInvoice(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, 400, "send the invoice as the form field \"file\"")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil || len(data) == 0 {
		writeErr(w, 400, "empty file")
		return
	}
	ext := "jpg"
	if strings.HasSuffix(strings.ToLower(hdr.Filename), ".pdf") || http.DetectContentType(data) == "application/pdf" {
		ext = "pdf"
	} else if strings.HasSuffix(strings.ToLower(hdr.Filename), ".png") {
		ext = "png"
	}
	media := s.Desk.Keep("panel:owner", ext, data)
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	draft, err := s.Desk.IngestInvoice(ctx, "panel:owner", data, media)
	if err != nil {
		writeErr(w, 502, "could not read the invoice: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"draft": draft})
}

func (s *Server) setSupplier(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SupplierID string `json:"supplier_id"`
	}
	if decode(r, &in) != nil || catalog.SupplierName(s.DB.DB, in.SupplierID) == "" {
		writeErr(w, 400, "unknown supplier")
		return
	}
	if err := s.Desk.SetSupplier(r.Context(), chi.URLParam(r, "id"), in.SupplierID); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) suppliers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": catalog.Suppliers(s.DB.DB)})
}

func (s *Server) ownerPairCode(w http.ResponseWriter, r *http.Request) {
	code, err := desk.NewPairCode(s.DB, "", "owner")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"code": code, "bot": s.DB.Setting("telegram_bot", "")})
}

// unlinkChat forgets the link of one chat (shop or owner).
func (s *Server) unlinkChat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Chat string `json:"chat"`
	}
	if decode(r, &in) != nil || in.Chat == "" {
		writeErr(w, 400, "chat is required")
		return
	}
	s.Desk.Unlink(in.Chat)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) owner(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`SELECT name, chat_key, linked_at FROM chats WHERE role = 'owner' ORDER BY linked_at DESC`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var n, k, at string
		rows.Scan(&n, &k, &at)
		out = append(out, map[string]string{"name": n, "chat": k, "linked_at": at})
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": out, "bot": s.DB.Setting("telegram_bot", ""), "now": store.Now()})
}
