// Package desk is the order agent: it reads a shop's messages (text, Darja voice note, photo of
// a list), resolves every item against the catalogue, asks only what it must, and books a
// confirmed order in the ERP through the adapter. Every confirmed line teaches the catalogue.
package desk

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/resolve"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
	"github.com/nudiiir/elixir-orderdesk/internal/telegram"
)

type Desk struct {
	DB      *store.DB
	Cat     *catalog.Catalog
	Meta    *meta.Client
	TG      *telegram.Telegram
	ERP     func() adapter.ERP // nil until the ERP is connected
	DataDir string
	OnLearn func() // called after every confirmed order (the rule engineer decides whether to run)

	hist sync.Map // customer id → *history
}

type history struct {
	at    time.Time
	usual map[string]int
	avg   map[string]int64
	last  *adapter.PastOrder
}

func (d *Desk) erp() adapter.ERP {
	if d.ERP == nil {
		return nil
	}
	return d.ERP()
}

// ruleSets returns the live rules (ready to run) and the shadow rules.
func (d *Desk) ruleSets() (*rules.Set, []*rules.Rule) {
	return rules.NewSet(rules.WithStatus(d.DB, "live")), rules.WithStatus(d.DB, "shadow")
}

func (d *Desk) history(ctx context.Context, customerID string) *history {
	if v, ok := d.hist.Load(customerID); ok && time.Since(v.(*history).at) < 10*time.Minute {
		return v.(*history)
	}
	h := &history{at: time.Now(), usual: map[string]int{}, avg: map[string]int64{}}
	if e := d.erp(); e != nil {
		if orders, err := e.History(ctx, customerID, 10); err == nil {
			sums := map[string]int64{}
			for i := range orders {
				if i == 0 {
					h.last = &orders[0]
				}
				for _, l := range orders[i].Lines {
					h.usual[l.ProductID]++
					sums[l.ProductID] += l.QtyMilli
				}
			}
			for id, n := range h.usual {
				h.avg[id] = sums[id] / int64(n)
			}
		}
	}
	d.hist.Store(customerID, h)
	return h
}

// ---- Telegram loop ---------------------------------------------------------------------

func (d *Desk) Run(ctx context.Context) {
	if d.TG == nil {
		return
	}
	me, err := d.TG.Me(ctx)
	if err != nil {
		log.Printf("[desk] telegram: %v", err)
		return
	}
	d.DB.SetSetting("telegram_bot", me.Username)
	log.Printf("[desk] listening on Telegram as @%s", me.Username)
	offset := 0
	for ctx.Err() == nil {
		ups, err := d.TG.Updates(ctx, offset)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		for _, u := range ups {
			offset = u.UpdateID + 1
			d.handle(ctx, u)
		}
	}
}

func (d *Desk) handle(ctx context.Context, u telegram.TgUpdate) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[desk] panic: %v", r)
		}
	}()
	switch {
	case u.Message != nil:
		d.onMessage(ctx, u.Message)
	case u.Callback != nil:
		d.onCallback(ctx, u.Callback)
	}
}

func chatKey(chat int64) string { return fmt.Sprintf("tg:%d", chat) }

func (d *Desk) linked(key string) (customerID, name string) {
	d.DB.QueryRow(`SELECT c.customer_id, COALESCE(u.name, '') FROM chats c LEFT JOIN customers u ON u.id = c.customer_id
		WHERE c.chat_key = ? AND c.role = 'shop'`, key).Scan(&customerID, &name)
	return
}

func (d *Desk) isOwner(key string) bool {
	var n int
	d.DB.QueryRow(`SELECT COUNT(*) FROM chats WHERE chat_key = ? AND role = 'owner'`, key).Scan(&n)
	return n > 0
}

func (d *Desk) onMessage(ctx context.Context, msg *telegram.TgMessage) {
	chat := msg.Chat.ID
	key := chatKey(chat)
	text := strings.TrimSpace(msg.Text)
	customerID, shop := d.linked(key)

	if strings.HasPrefix(text, "/start") {
		code := strings.TrimSpace(strings.TrimPrefix(text, "/start"))
		if code == "" && d.isOwner(key) {
			d.say(ctx, chat, ownerHelp)
			return
		}
		if code == "" && customerID != "" {
			d.say(ctx, chat, fmt.Sprintf("Bonjour %s 👋\n%s", esc(shop), helpText))
			return
		}
		d.pair(ctx, msg, code)
		return
	}
	if looksLikeCode(text) && d.codeExists(text) {
		d.pair(ctx, msg, text)
		return
	}
	if low := strings.ToLower(text); low == "/deconnecter" || low == "/unlink" || low == "/reset" {
		d.Unlink(key)
		d.say(ctx, chat, "🔓 Ce compte Telegram n'est plus lié. Envoyez un nouveau code pour le lier à un client ou comme propriétaire.")
		return
	}
	if d.isOwner(key) {
		d.onOwnerMessage(ctx, msg)
		return
	}
	if customerID == "" {
		if looksLikeCode(text) {
			d.pair(ctx, msg, text)
			return
		}
		d.say(ctx, chat, "Bonjour 👋 Pour commander, envoyez d'abord le <b>code client</b> que votre distributeur vous a donné.")
		return
	}
	switch strings.ToLower(text) {
	case "/annuler", "annuler":
		if dr := d.openDraft(key); dr != "" {
			d.cancel(ctx, chat, dr)
		}
		return
	case "/commande":
		if dr := d.openDraft(key); dr != "" {
			d.next(ctx, chat, dr)
		} else {
			d.say(ctx, chat, "Aucune commande en cours.")
		}
		return
	case "/aide", "/help":
		d.say(ctx, chat, helpText)
		return
	}

	d.TG.Typing(ctx, chat)
	msgID := store.NewID("msg")
	kind, media, heard := "text", "", ""
	switch {
	case msg.Voice != nil || msg.Audio != nil:
		v := msg.Voice
		if v == nil {
			v = msg.Audio
		}
		kind = "voice"
		audio, err := d.TG.Download(ctx, v.FileID)
		if err != nil {
			d.fail(ctx, chat, "je n'ai pas pu récupérer la note vocale", err)
			return
		}
		media = d.keep(key, "ogg", audio)
		wav, err := toWAV(ctx, audio)
		if err != nil {
			d.fail(ctx, chat, "je n'ai pas pu lire la note vocale", err)
			return
		}
		text, _, err = d.Meta.Transcribe(ctx, wav, msgID)
		if err != nil || text == "" {
			d.fail(ctx, chat, "je n'ai pas compris la note vocale, pouvez-vous l'écrire ?", err)
			return
		}
		heard = "🎤 <i>« " + esc(text) + " »</i>"
	case len(msg.Photo) > 0:
		kind = "photo"
		img, err := d.TG.Download(ctx, msg.Photo[len(msg.Photo)-1].FileID)
		if err != nil {
			d.fail(ctx, chat, "je n'ai pas pu récupérer la photo", err)
			return
		}
		media = d.keep(key, "jpg", img)
		lines, _, err := ReadPhoto(ctx, d.Meta, img, msgID)
		if err != nil || len(lines) == 0 {
			d.fail(ctx, chat, "je n'arrive pas à lire cette photo, pouvez-vous en envoyer une plus nette ?", err)
			return
		}
		text = strings.Join(lines, "\n")
		if c := strings.TrimSpace(msg.Caption); c != "" {
			text = c + "\n" + text
		}
		heard = fmt.Sprintf("📷 <i>J'ai lu %d ligne(s) sur la photo.</i>", len(lines))
	case text == "":
		d.say(ctx, chat, "Envoyez votre commande en texte, en note vocale ou en photo de la liste.")
		return
	}
	res, err := d.Ingest(ctx, key, customerID, msgID, kind, text, media)
	if err != nil {
		d.fail(ctx, chat, "je n'ai pas pu analyser la commande, réessayez dans un instant", err)
		return
	}
	if res.Draft == "" {
		reply := res.Ex.Reply
		if reply == "" {
			reply = "Je n'ai pas trouvé de produit dans ce message. Envoyez par exemple : « 3 cartons Dove shampooing 250 ml »."
		}
		if heard != "" {
			reply = heard + "\n\n" + esc(reply)
		}
		d.say(ctx, chat, reply)
		return
	}
	if heard != "" {
		d.say(ctx, chat, heard)
	}
	d.next(ctx, chat, res.Draft)
}

// IngestResult is what one message turned into.
type IngestResult struct {
	Draft string     `json:"draft"`
	Ex    Extraction `json:"extraction"`
	Lines []Line     `json:"lines"`
}

// Ingest reads one message (text, transcript or lines read from a photo), resolves its items and
// adds them to the customer's open draft. Channels and the panel's "try a message" both use it.
func (d *Desk) Ingest(ctx context.Context, key, customerID, msgID, kind, text, media string) (IngestResult, error) {
	d.DB.Exec(`INSERT INTO messages (id, chat_key, customer_id, kind, text, media_path, created_at) VALUES (?,?,?,?,?,?,?)`,
		msgID, key, customerID, kind, text, media, store.Now())
	ex, err := Extract(ctx, d.Meta, d.Cat.Brands(), text, kind == "voice", msgID)
	d.DB.Exec(`UPDATE messages SET extraction = ? WHERE id = ?`, ex.Method, msgID)
	if err != nil && len(ex.Mentions) == 0 {
		return IngestResult{Ex: ex}, err
	}
	if ex.Intent != "order" && !ex.RepeatLast {
		return IngestResult{Ex: ex}, nil
	}
	h := d.history(ctx, customerID)
	draft := d.ensureDraft(key, customerID)
	pos := nextPosition(d.DB, draft)
	var added []Line

	if ex.RepeatLast && h.last != nil {
		// "Same as last time" is copied by code from the ERP, never re-guessed.
		for _, pl := range h.last.Lines {
			p := d.Cat.Get(pl.ProductID)
			if p == nil {
				continue
			}
			l := Line{DraftID: draft, MessageID: msgID, Position: pos, Said: "", Norm: "", ProductID: p.ID, Level: resolve.LHistory,
				Used: []string{"order:" + h.last.Number}, Confidence: 1, Status: "ok", Checks: map[string]string{}, Shadow: map[string]string{}}
			l.Qty, l.Unit = 0, ""
			l.QtyMilli = pl.QtyMilli
			for _, k := range p.Packs {
				if pl.QtyMilli%k.FactorMilli == 0 {
					l.PackName, l.PackFactor = k.Name, k.FactorMilli
				}
			}
			if l.PackFactor > 0 {
				l.Qty = float64(pl.QtyMilli / l.PackFactor)
			} else {
				l.Qty = float64(pl.QtyMilli) / 1000
			}
			l.Checks["history"] = "comme la commande " + h.last.Number + " du " + h.last.Date
			pos++
			added = append(added, l)
		}
	}
	live, shadow := d.ruleSets()
	rv := &resolve.Resolver{Cat: d.Cat, Live: live, Shadow: shadow, Meta: d.Meta}
	res := rv.Resolve(ctx, ex.Mentions, resolve.Context{CustomerID: customerID, Usual: h.usual}, msgID)
	for _, r := range res {
		l := Line{DraftID: draft, MessageID: msgID, Position: pos, Said: r.Said, Norm: r.Norm, Hint: r.Hint, Qty: r.Qty, Unit: r.Unit,
			ProductID: r.ProductID, Level: r.Level, Used: r.Used, Confidence: r.Confidence, Options: r.Options, Question: r.Question,
			Status: r.Status, Checks: map[string]string{}, Shadow: r.Shadow}
		if r.Pack != nil {
			l.Checks["pack_rule"] = r.Pack.Unit
			if r.Pack.Qty > 0 {
				l.Checks["pack_rule"] = fmt.Sprintf("%s ×%d", r.Pack.Unit, r.Pack.Qty)
			}
		}
		if l.Status == "ok" {
			d.setPack(&l, r.Pack)
		}
		// a repeat_last line naming a product already copied replaces it
		for i := range added {
			if l.Status == "ok" && added[i].ProductID == l.ProductID && added[i].Level == resolve.LHistory {
				added[i].Status = "removed"
			}
		}
		pos++
		added = append(added, l)
	}
	stock := d.stock(ctx)
	for i := range added {
		if added[i].Status == "ok" {
			d.price(ctx, customerID, &added[i], h, stock)
		}
		saveLine(d.DB, &added[i])
	}
	d.DB.Exec(`UPDATE drafts SET updated_at = ? WHERE id = ?`, store.Now(), draft)
	return IngestResult{Draft: draft, Ex: ex, Lines: added}, nil
}

// setPack converts the said quantity into ERP base units using the product's real packs.
func (d *Desk) setPack(l *Line, spec *rules.PackSpec) {
	p := d.Cat.Get(l.ProductID)
	if p == nil {
		return
	}
	l.PackName, l.PackFactor = resolve.ChoosePack(p, l.Unit, spec)
	if l.PackFactor > 0 {
		l.QtyMilli = int64(l.Qty*float64(l.PackFactor) + 0.5)
	} else {
		l.QtyMilli = int64(l.Qty*1000 + 0.5)
	}
	if spec != nil && spec.Qty > 0 && l.PackFactor != int64(spec.Qty)*1000 {
		l.Checks["pack"] = fmt.Sprintf("le message dit ×%d, le catalogue n'a pas ce conditionnement", spec.Qty)
	}
	if l.Unit == "" && spec == nil && l.PackFactor > 0 {
		l.Checks["unit"] = "unité non précisée : " + strings.ToLower(l.PackName)
	}
}

// price asks the ERP for the price and checks stock and the usual quantity.
func (d *Desk) stock(ctx context.Context) map[string]int64 {
	if e := d.erp(); e != nil {
		if st, err := e.Stock(ctx); err == nil {
			return st
		}
	}
	return nil
}

func (d *Desk) price(ctx context.Context, customerID string, l *Line, h *history, stock map[string]int64) {
	e := d.erp()
	if e == nil {
		l.Checks["price"] = "ERP non connecté"
		return
	}
	q, err := e.Price(ctx, customerID, l.ProductID, l.QtyMilli)
	if err != nil {
		l.Checks["price"] = "prix introuvable"
	} else {
		l.UnitPrice, l.LineTTC = q.UnitPriceHT, q.LineTTC
		if q.FreeQtyMilli > 0 {
			l.Checks["offer"] = fmt.Sprintf("offre : +%s gratuit", trimQty(q.FreeQtyMilli))
		}
		delete(l.Checks, "price")
	}
	if stock != nil {
		if have := stock[l.ProductID]; have < l.QtyMilli {
			l.Checks["stock"] = fmt.Sprintf("stock insuffisant (%s disponibles)", trimQty(have))
		}
	}
	if avg := h.avg[l.ProductID]; h.usual[l.ProductID] >= 2 && avg > 0 && l.QtyMilli > 3*avg {
		l.Checks["usual"] = fmt.Sprintf("quantité inhabituelle (d'habitude %s)", trimQty(avg))
	}
}

// ---- drafts ----------------------------------------------------------------------------

func (d *Desk) openDraft(key string) string {
	var id string
	d.DB.QueryRow(`SELECT id FROM drafts WHERE chat_key = ? AND status = 'open' ORDER BY created_at DESC LIMIT 1`, key).Scan(&id)
	return id
}

func (d *Desk) ensureDraft(key, customerID string) string {
	if id := d.openDraft(key); id != "" {
		return id
	}
	id := store.NewID("dr")
	now := store.Now()
	d.DB.Exec(`INSERT INTO drafts (id, chat_key, customer_id, status, created_at, updated_at) VALUES (?, ?, ?, 'open', ?, ?)`,
		id, key, customerID, now, now)
	return id
}

// ---- pairing ---------------------------------------------------------------------------

const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func looksLikeCode(s string) bool {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) != 6 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(codeAlphabet, r) {
			return false
		}
	}
	return true
}

// NewPairCode creates a one-time code a customer (role shop) or the owner (role owner, no
// customer) sends to the bot to link its chat.
func NewPairCode(db *store.DB, customerID, role string) (string, error) {
	b := make([]byte, 6)
	rand.Read(b)
	code := make([]byte, 6)
	for i := range b {
		code[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	_, err := db.Exec(`INSERT INTO pair_codes (code, customer_id, role, created_at) VALUES (?, ?, ?, ?)`, string(code), customerID, role, store.Now())
	return string(code), err
}

func (d *Desk) codeExists(code string) bool {
	var n int
	d.DB.QueryRow(`SELECT COUNT(*) FROM pair_codes WHERE code = ? AND used_at = '' AND created_at >= datetime('now', 'localtime', '-7 day')`,
		strings.ToUpper(strings.TrimSpace(code))).Scan(&n)
	return n > 0
}

// Unlink forgets which customer (or the owner) a chat belongs to; its open drafts are cancelled.
func (d *Desk) Unlink(key string) {
	var who string
	d.DB.QueryRow(`SELECT name FROM chats WHERE chat_key = ?`, key).Scan(&who)
	d.DB.Exec(`DELETE FROM chats WHERE chat_key = ?`, key)
	d.DB.Exec(`UPDATE drafts SET status = 'cancelled', updated_at = ? WHERE chat_key = ? AND status = 'open'`, store.Now(), key)
	d.DB.Event("chat.unlinked", key, who, map[string]string{"chat": key})
}

func (d *Desk) pair(ctx context.Context, msg *telegram.TgMessage, code string) {
	chat := msg.Chat.ID
	code = strings.ToUpper(strings.TrimSpace(code))
	var customerID, name, role string
	err := d.DB.QueryRow(`SELECT p.customer_id, COALESCE(c.name, ''), p.role FROM pair_codes p LEFT JOIN customers c ON c.id = p.customer_id
		WHERE p.code = ? AND p.used_at = '' AND p.created_at >= datetime('now', 'localtime', '-7 day')`, code).Scan(&customerID, &name, &role)
	if err != nil {
		d.say(ctx, chat, "Ce code n'est pas valide ou a expiré. Demandez un nouveau code à votre distributeur.")
		return
	}
	who := strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName)
	d.DB.Exec(`INSERT INTO chats (chat_key, customer_id, name, role, linked_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(chat_key) DO UPDATE SET customer_id = excluded.customer_id, name = excluded.name, role = excluded.role,
		linked_at = excluded.linked_at`, chatKey(chat), customerID, who, role, store.Now())
	if role == "owner" {
		d.DB.Exec(`UPDATE pair_codes SET used_at = ? WHERE code = ?`, store.Now(), code)
		d.DB.Event("chat.linked", "owner", who, map[string]string{"chat": chatKey(chat), "role": "owner"})
		d.say(ctx, chat, "✅ Ce compte Telegram est maintenant lié comme <b>propriétaire</b>.\n\n"+ownerHelp)
		return
	}
	d.DB.Exec(`UPDATE pair_codes SET used_at = ? WHERE code = ?`, store.Now(), code)
	d.DB.Event("chat.linked", customerID, who, map[string]string{"chat": chatKey(chat), "customer": name})
	d.say(ctx, chat, fmt.Sprintf("✅ Ce compte Telegram est maintenant lié à <b>%s</b>.\n\n%s", esc(name), helpText))
}

const ownerHelp = `Envoyez ici les <b>factures fournisseurs</b> : une photo nette ou le PDF.
Order Desk lit chaque ligne, la rapproche du catalogue, vérifie prix et conditionnements, puis prépare le bon de réception.
Les lignes incertaines vous sont posées en question ; tout est aussi visible dans le panneau (Invoices).`

// onOwnerMessage: the owner sends supplier invoices (photo or PDF).
func (d *Desk) onOwnerMessage(ctx context.Context, msg *telegram.TgMessage) {
	chat := msg.Chat.ID
	key := chatKey(chat)
	var fileID, ext string
	switch {
	case len(msg.Photo) > 0:
		fileID, ext = msg.Photo[len(msg.Photo)-1].FileID, "jpg"
	case msg.Document != nil && (strings.HasPrefix(msg.Document.MimeType, "image/") || msg.Document.MimeType == "application/pdf"):
		fileID, ext = msg.Document.FileID, "pdf"
		if strings.HasPrefix(msg.Document.MimeType, "image/") {
			ext = "jpg"
		}
	default:
		d.say(ctx, chat, ownerHelp)
		return
	}
	d.TG.Typing(ctx, chat)
	data, err := d.TG.Download(ctx, fileID)
	if err != nil {
		d.fail(ctx, chat, "je n'ai pas pu récupérer le fichier", err)
		return
	}
	media := d.keep(key, ext, data)
	draft, err := d.IngestInvoice(ctx, key, data, media)
	if err != nil {
		d.fail(ctx, chat, "je n'arrive pas à lire cette facture", err)
		return
	}
	d.next(ctx, chat, draft)
}

const helpText = `Comment commander :
• écrivez : « 3 cartons Dove shampooing 250 ml, 2 Nivea crème »
• ou envoyez une <b>note vocale</b> (darja, français, arabe)
• ou une <b>photo de votre liste</b>
• « comme la dernière fois » reprend votre dernière commande

/commande voir la commande en cours · /annuler l'annuler · /deconnecter délier ce compte`

// ---- helpers ---------------------------------------------------------------------------

func (d *Desk) say(ctx context.Context, chat int64, text string) {
	if _, err := d.TG.Send(ctx, chat, text, nil); err != nil {
		log.Printf("[desk] send: %v", err)
	}
}

func (d *Desk) fail(ctx context.Context, chat int64, what string, err error) {
	log.Printf("[desk] %s: %v", what, err)
	if err == meta.ErrBudget {
		what = "le budget IA du jour est atteint, votre message sera traité par le distributeur"
	}
	d.say(ctx, chat, "😕 Désolé, "+what+".")
}

// Keep stores a received file (voice note, photo, invoice) as evidence.
func (d *Desk) Keep(key, ext string, data []byte) string { return d.keep(key, ext, data) }

func (d *Desk) keep(key, ext string, data []byte) string {
	dir := filepath.Join(d.DataDir, "media", strings.ReplaceAll(key, ":", "_"))
	if os.MkdirAll(dir, 0o755) != nil {
		return ""
	}
	p := filepath.Join(dir, time.Now().Format("20060102-150405.000")+"."+ext)
	if os.WriteFile(p, data, 0o644) != nil {
		return ""
	}
	return p
}

// toWAV converts a voice note (OGG/Opus) to 16 kHz mono WAV with ffmpeg.
func toWAV(ctx context.Context, audio []byte) ([]byte, error) {
	bin := os.Getenv("FFMPEG")
	if bin == "" {
		bin = "ffmpeg"
	}
	cmd := exec.CommandContext(ctx, bin, "-hide_banner", "-loglevel", "error", "-i", "pipe:0", "-ac", "1", "-ar", "16000", "-f", "wav", "pipe:1")
	cmd.Stdin = bytes.NewReader(audio)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %v %s", err, errb.String())
	}
	return out.Bytes(), nil
}

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// Money formats centimes as « 12 450 DA ».
func Money(centimes int64) string {
	da := (centimes + 50) / 100
	s := fmt.Sprint(da)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	return b.String() + " DA"
}

func trimQty(milli int64) string {
	if milli%1000 == 0 {
		return fmt.Sprint(milli / 1000)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", float64(milli)/1000), "0"), ".")
}
