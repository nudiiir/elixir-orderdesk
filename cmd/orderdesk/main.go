// Order Desk: orders from any chat into any ERP, on a catalogue that writes its own matching rules.
//
//	orderdesk -addr 127.0.0.1:8610 -data ./data
//	orderdesk -demo     a trained supermarket desk on a stand-in ERP: no key, no Docker, no ERP
//
// Keys (META_MODEL_API_KEY, TELEGRAM_BOT_TOKEN, ...) come from the environment, from .env, or from the
// files given with -keys. See .env.example.
// The ERP connection is made from the panel (Settings → ERP).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nudiiir/elixir-orderdesk/internal/adapter"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/demo"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/dolibarr"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/elixir"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/erpnext"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/inventree"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/odoo"
	"github.com/nudiiir/elixir-orderdesk/internal/adapter/tryton"
	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/desk"
	"github.com/nudiiir/elixir-orderdesk/internal/engineer"
	"github.com/nudiiir/elixir-orderdesk/internal/meta"
	"github.com/nudiiir/elixir-orderdesk/internal/profile"
	"github.com/nudiiir/elixir-orderdesk/internal/search"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
	"github.com/nudiiir/elixir-orderdesk/internal/telegram"
	"github.com/nudiiir/elixir-orderdesk/internal/twilio"
	"github.com/nudiiir/elixir-orderdesk/internal/web"
	"github.com/nudiiir/elixir-orderdesk/internal/whatsapp"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8610", "panel and API address")
	dataDir := flag.String("data", "data", "data directory (database, voice notes, photos)")
	keys := flag.String("keys", "", "keys files with KEY=value lines, comma-separated (default: .env if present)")
	noTelegram := flag.Bool("no-telegram", false, "do not start the Telegram connector")
	publicRO := flag.Bool("public-readonly", false, "serve the panel read-only to other computers (a public demo, with -addr 0.0.0.0:PORT): "+
		"they can look at everything and change nothing; this computer keeps full access")
	demoMode := flag.Bool("demo", false, "try it with nothing installed: the trained supermarket desk ("+demoDesk+") is copied into the data "+
		"directory (default "+demoData+") on first start, orders and receipts are written to files there instead of an ERP")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of orderdesk:\n\n  go run ./cmd/orderdesk -demo     the one-command demo (no key, no Docker, no ERP)\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *keys == "" {
		if _, err := os.Stat(".env"); err == nil {
			loadKeys(".env")
		}
	} else {
		for _, k := range strings.Split(*keys, ",") {
			loadKeys(strings.TrimSpace(k))
		}
	}

	if *demoMode {
		dataSet := false
		flag.Visit(func(f *flag.Flag) { dataSet = dataSet || f.Name == "data" })
		if !dataSet {
			*dataDir = demoData
		}
		if err := copyDemoDesk(*dataDir); err != nil {
			log.Fatalf("demo: %v (run it from the repository root)", err)
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
		case "tryton":
			c = tryton.New(url, dbName, login, token)
		case "dolibarr":
			c = dolibarr.New(url, token)
		case "inventree":
			c = inventree.New(url, token)
		default:
			c = elixir.New(url, token, nil, func(t string) { db.SetSetting("erp_token", t) })
		}
		mu.Lock()
		erp = c
		mu.Unlock()
	}
	var demoERP *demo.Client
	if *demoMode {
		// the stand-in ERP reads the desk's own catalogue and writes what is booked to files
		if n, err := demo.AddShops(db); err != nil {
			log.Fatal(err)
		} else if n > 0 {
			log.Printf("[demo] %d demo shops added", n)
		}
		demoERP = demo.New(db, *dataDir)
		erp = demoERP
	} else if u, t := db.Setting("erp_url", ""), db.Setting("erp_token", ""); u != "" && t != "" {
		connect(db.Setting("erp_kind", "elixir"), u, db.Setting("erp_db", ""), db.Setting("erp_login", ""), t)
	}
	afterSync := func() {} // set once the desk exists: the matching is rebuilt on the new catalogue
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
		afterSync()
		log.Printf("[sync] %d products from %s", len(cat.Products()), e.Name())
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	eng := &engineer.Engineer{DB: db, Cat: cat, Meta: mc}
	d := &desk.Desk{DB: db, Cat: cat, Meta: mc, ERP: current, DataDir: *dataDir,
		OnLearn: func() { eng.MaybeRun(context.Background()) }, Threshold: 0.9}
	// The matching Muse Spark designed for this desk (first-pass blocks, vocabulary, search method) and the
	// autonomy its steps earned: without them the live desk matched with none of what it learned.
	d.Autonomy = desk.NewAutonomy(0.99)
	json.Unmarshal([]byte(db.Setting("autonomy", "{}")), d.Autonomy)
	afterSync = func() {
		if p := profile.Load(db); p != nil {
			p.Install(cat)
			d.Search = search.Build(cat, p.Search)
		}
	}
	afterSync()
	// Channels: Telegram (polled) and WhatsApp (Cloud API webhook), behind one Messenger.
	mux := &telegram.Mux{}
	d.TG = mux
	if tok := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")); tok != "" && !*noTelegram {
		mux.TG = telegram.NewTelegram(tok)
		d.Poll = mux.TG
		go d.Run(ctx)
	}
	var wa *whatsapp.Client
	var waHook http.Handler
	if sid, tok, from := strings.TrimSpace(os.Getenv("TWILIO_ACCOUNT_SID")), strings.TrimSpace(os.Getenv("TWILIO_AUTH_TOKEN")),
		strings.TrimSpace(os.Getenv("TWILIO_WHATSAPP_FROM")); sid != "" && tok != "" && from != "" {
		// WhatsApp through Twilio (a Twilio sender or the Twilio Sandbox for WhatsApp)
		tw := twilio.New(sid, tok, from)
		tw.PublicURL = strings.TrimSpace(os.Getenv("TWILIO_WEBHOOK_URL"))
		if api := os.Getenv("TWILIO_API_URL"); api != "" {
			tw.API = strings.TrimRight(api, "/") // a local stand-in for tests
		}
		mux.WA = tw
		waHook = tw.Webhook(d.Handle)
		go func() {
			mctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			if who, err := tw.Me(mctx); err == nil {
				db.SetSetting("whatsapp_number", who)
				log.Printf("[desk] WhatsApp on %s (webhook /whatsapp/webhook)", who)
			} else {
				db.SetSetting("whatsapp_number", "")
				log.Printf("[desk] WhatsApp via Twilio: %v", err)
			}
		}()
	} else if tok, phone := strings.TrimSpace(os.Getenv("WHATSAPP_TOKEN")), strings.TrimSpace(os.Getenv("WHATSAPP_PHONE_ID")); tok != "" && phone != "" {
		wa = whatsapp.New(tok, phone, os.Getenv("WHATSAPP_VERIFY_TOKEN"), os.Getenv("WHATSAPP_APP_SECRET"))
		if wa.AppSecret == "" && !localURL(os.Getenv("WHATSAPP_GRAPH_URL")) {
			// without the app secret anyone who finds the webhook could post orders as any shop
			log.Fatal("WHATSAPP_APP_SECRET is required: Meta signs every webhook call with it (Meta app → Settings → Basic)")
		}
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

	srv := &web.Server{DB: db, Cat: cat, Desk: d, Eng: eng, Meta: mc, DataDir: *dataDir, ERP: current, Connect: connect, Sync: sync, PublicReadOnly: *publicRO}
	if wa != nil {
		srv.WhatsApp = wa.Webhook(d.Handle)
	}
	if waHook != nil {
		srv.WhatsApp = waHook
	}
	if !localURL("http://" + *addr) {
		log.Printf("[desk] %s is reachable from other computers: the panel still only answers requests from this one; only /whatsapp/webhook is served to others", *addr)
	}
	// Uploads are capped at 25 MB (a minute is plenty); an invoice read may take the handler's 3 minutes.
	hs := &http.Server{Addr: *addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 60 * time.Second, WriteTimeout: 4 * time.Minute, IdleTimeout: 2 * time.Minute}
	go func() {
		<-ctx.Done()
		hs.Shutdown(context.Background())
	}()
	log.Printf("Order Desk panel on http://%s (Meta: %v, ERP: %v)", *addr, mc.Enabled(), current() != nil)
	if demoERP != nil {
		panelURL := "http://" + *addr
		if h, p, err := net.SplitHostPort(*addr); err == nil && (h == "" || h == "0.0.0.0" || h == "::") {
			panelURL = "http://127.0.0.1:" + p
		}
		log.Printf("[demo] open %s and use \"Try a message\" with one of the demo shops", panelURL)
		if mc.Enabled() {
			log.Printf("[demo] META_MODEL_API_KEY is set: Muse Spark reads what the parser, learned wordings and rules leave open")
		} else {
			log.Printf("[demo] keyless: the parser, the learned wordings and the rules answer; anything else becomes a question for you " +
				"(send a plain list, one product per line: a chatty message, a voice note or a photo needs the key)")
			log.Printf("[demo] set META_MODEL_API_KEY (environment or .env) and Muse Spark joins in: voice, photos, unknown wordings")
		}
		log.Printf("[demo] booked orders are written to %s, goods receipts to %s", demoERP.OrdersFile(), demoERP.ReceiptsFile())
		if mux.TG == nil {
			log.Printf("[demo] Telegram is off (set TELEGRAM_BOT_TOKEN to chat with the desk)")
		}
	}
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// The desk -demo starts from, and where it keeps its copy.
const (
	demoDesk = "bench/trained-desks/supermarket/orderdesk.db"
	demoData = "data-demo"
)

// copyDemoDesk copies the trained supermarket desk into dir, unless dir already has a database.
func copyDemoDesk(dir string) error {
	dst := filepath.Join(dir, "orderdesk.db")
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	in, err := os.Open(demoDesk)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := dst + ".copy"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	log.Printf("[demo] trained supermarket desk copied to %s", dst)
	return nil
}

// localURL tells whether a URL points at this computer (a local simulator or test server).
func localURL(u string) bool {
	p, err := url.Parse(u)
	if err != nil {
		return false
	}
	h := p.Hostname()
	ip := net.ParseIP(h)
	return h == "localhost" || (ip != nil && ip.IsLoopback())
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
