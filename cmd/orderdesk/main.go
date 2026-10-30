// Order Desk: orders from any chat into any ERP, on a catalogue that writes its own matching rules.
//
//	orderdesk -addr 127.0.0.1:8610 -data ./data -keys ../hackathon-sim/keys.env
//
// Keys (TELEGRAM_BOT_TOKEN, META_MODEL_API_KEY) come from the environment or a keys file.
// The ERP connection is made from the panel (Settings → ERP).
package main

import (
	"bufio"
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/elixir"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/erpnext"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/odoo"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/desk"
	"github.com/nudiiir/elixir-orderdesk/internal/engineer"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
	"github.com/nudiiir/elixir-orderdesk/internal/telegram"
	"github.com/nudiiir/elixir-orderdesk/internal/web"
	"github.com/nudiiir/elixir-orderdesk/internal/whatsapp"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8610", "panel and API address")
	dataDir := flag.String("data", "data", "data directory (database, voice notes, photos)")
	keys := flag.String("keys", "", "optional keys files with KEY=value lines, comma-separated")
	noTelegram := flag.Bool("no-telegram", false, "do not start the Telegram connector")
	flag.Parse()
	if *keys != "" {
		for _, k := range strings.Split(*keys, ",") {
			loadKeys(strings.TrimSpace(k))
		}
	}

	db, err := store.Open(filepath.Join(*dataDir, "orderdesk.db"))
	if err != nil {
		log.Fatal(err)
	}
	cat := catalog.New(db)
	mc := meta.New(os.Getenv("META_MODEL_API_KEY"), db)

	var mu sync.RWMutex
	var erp adapter.ERP
	current := func() adapter.ERP {
		mu.RLock()
		defer mu.RUnlock()
		return erp
	}
	// One adapter per ERP kind; the core never knows which one it talks to.
	connect := func(kind, url, dbName, login, token string) {
		var c adapter.ERP
		switch kind {
		case "odoo":
			c = odoo.New(url, dbName, login, token)
		case "erpnext":
			key, secret, _ := strings.Cut(token, ":")
			c = erpnext.New(url, key, secret)
		default:
			c = elixir.New(url, token, nil, func(t string) { db.SetSetting("erp_token", t) })
		}
		mu.Lock()
		erp = c
		mu.Unlock()
	}
	if u, t := db.Setting("erp_url", ""), db.Setting("erp_token", ""); u != "" && t != "" {
		connect(db.Setting("erp_kind", "elixir"), u, db.Setting("erp_db", ""), db.Setting("erp_login", ""), t)
	}
	sync := func(ctx context.Context) error {
		e := current()
		if e == nil {
			return adapter.ErrNotConnected
		}
		prods, err := e.Products(ctx)
		if err == nil {
			err = catalog.ReplaceProducts(db, prods)
		}
		if err == nil {
			var cs []catalog.Customer
			if cs, err = e.Customers(ctx); err == nil {
				err = catalog.ReplaceCustomers(db, cs)
			}
			if err == nil {
				if ss, serr := e.Suppliers(ctx); serr == nil {
					catalog.ReplaceSuppliers(db, ss)
				}
			}
		}
		if err != nil {
			db.SetSetting("erp_error", err.Error())
			return err
		}
		db.SetSetting("erp_error", "")
		db.SetSetting("erp_synced_at", store.Now())
		cat.Reload()
		log.Printf("[sync] %d products from %s", len(cat.Products()), e.Name())
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	eng := &engineer.Engineer{DB: db, Cat: cat, Meta: mc}
	d := &desk.Desk{DB: db, Cat: cat, Meta: mc, ERP: current, DataDir: *dataDir,
		OnLearn: func() { eng.MaybeRun(context.Background()) }}
	// Channels: Telegram (polled) and WhatsApp (Cloud API webhook), behind one Messenger.
	mux := &telegram.Mux{}
	d.TG = mux
	if tok := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")); tok != "" && !*noTelegram {
		mux.TG = telegram.NewTelegram(tok)
		d.Poll = mux.TG
		go d.Run(ctx)
	}
	var wa *whatsapp.Client
	if tok, phone := strings.TrimSpace(os.Getenv("WHATSAPP_TOKEN")), strings.TrimSpace(os.Getenv("WHATSAPP_PHONE_ID")); tok != "" && phone != "" {
		wa = whatsapp.New(tok, phone, os.Getenv("WHATSAPP_VERIFY_TOKEN"), os.Getenv("WHATSAPP_APP_SECRET"))
		if g := os.Getenv("WHATSAPP_GRAPH_URL"); g != "" {
			wa.Graph = strings.TrimRight(g, "/") // a local stand-in for tests
		}
		mux.WA = wa
		go func() {
			mctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			if who, err := wa.Me(mctx); err == nil {
				db.SetSetting("whatsapp_number", who)
				log.Printf("[desk] WhatsApp on %s (webhook /whatsapp/webhook)", who)
			} else {
				db.SetSetting("whatsapp_number", "")
				log.Printf("[desk] WhatsApp: %v", err)
			}
		}()
	}
	go func() { // keep the catalogue fresh
		for {
			if current() != nil {
				sctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				if err := sync(sctx); err != nil {
					log.Printf("[sync] %v", err)
				}
				cancel()
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(15 * time.Minute):
			}
		}
	}()

	srv := &web.Server{DB: db, Cat: cat, Desk: d, Eng: eng, Meta: mc, DataDir: *dataDir, ERP: current, Connect: connect, Sync: sync}
	if wa != nil {
		srv.WhatsApp = wa.Webhook(d.Handle)
	}
	hs := &http.Server{Addr: *addr, Handler: srv.Handler()}
	go func() {
		<-ctx.Done()
		hs.Shutdown(context.Background())
	}()
	log.Printf("Order Desk panel on http://%s (Meta: %v, ERP: %v)", *addr, mc.Enabled(), current() != nil)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// loadKeys reads KEY=value lines into the environment without printing them.
func loadKeys(path string) {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("keys file: %v", err)
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && os.Getenv(strings.TrimSpace(k)) == "" {
			os.Setenv(strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`))
		}
	}
}
