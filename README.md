# Elixir Order Desk

**Orders from any chat into any ERP, on a catalogue that writes its own matching rules.**

Small shops order from their distributor the way they talk: Darja voice notes, photos of handwritten lists,
"same as last time, plus two cartons of Ifri". Order Desk turns every line of those messages into one precise
product from the distributor's catalogue, checks it with the ERP's own prices and stock, asks only what it must,
and books a confirmed order in the ERP.

The matching is not an AI call on every line forever. Order Desk keeps a memory of the catalogue (every
wording people use for every product) and a set of **rules**. A background agent, the **rule engineer**
(Muse Spark, with Muse Wonder as a fallback), studies the lines the rules could not resolve, writes a new rule in a small rule language,
tests it on every line whose answer is known, and the **code** lets it in only with zero wrong matches. Week
after week, more lines are resolved for free; the AI is called only for what is new.

Built during the Meta Global AI Developer Hackathon (October 26 – November 4, 2026). Everything in this
repository was written during the event. Elixir Distribution, our existing ERP, is not part of it: it is only
one of the six ERPs Order Desk connects to, through its HTTP API. The other five (Odoo, ERPNext, Dolibarr,
InvenTree, Tryton) are open source and run from `erps/`, so the whole demo can be reproduced without it.

## How a line is resolved

| Level | What happens | Cost |
|---|---|---|
| 0 · normalise | lower case, accents, Arabic digits, sizes (`1,5 L` → `1500ml`), live rewrite rules | free |
| 1 · alias | a trusted wording for this shop | free |
| 2 · rules | synonyms, pack patterns, defaults, guards, then an exact catalogue match | free |
| 3 · shortlist | the 8 most likely products, with the shop's habits | free |
| 4 · Muse Spark | picks one id **from the shortlist only**, one call per message | paid |
| 5 · person | one precise question with buttons | — |

Messages are cut into items first. Plain lists use a deterministic parser (no AI). Voice notes go to Muse Voice
Transcribe, then Muse Spark extracts the items, quoting the shop's words and never choosing products, with only
the brand names in the prompt, never the catalogue. "Same as last time" is copied from the ERP by code.

**Photos.** Muse Spark first tells a written list (copied line by line) from a photo of the products themselves,
and names the kinds of packaging it shows. For products, **SAM 3.1** finds every bottle, can or carton (one call
per kind of packaging), duplicate and weak boxes are dropped, and Muse Spark reads all the crops in one call, the
way a shop would type them ("Selecto 2L"). Identical products are counted, a caption like "3 cartons" applies to a
single product, and the lines go through the levels below like any typed order. The shop gets its photo back with
every product boxed in a colour that matches the reply ("🟩 2 × Selecto 2L"). About $0.015 a photo.

## The rule language

Five rule types, each shown as a sentence generated from its structured form:

- **synonym**: `"slkt"` means `"selecto"`
- **rewrite**: `"1l5"` is written as `"1.5l"`
- **pack**: a trailing `"x{n}"` means a carton of `{n}`
- **default**: for Épicerie Nour, `"ifri"` with no size means Ifri 1.5 L
- **guard**: `"citron"` only matches products whose name contains "citron"

Patterns: literal words, `a|b`, `{n}`, `{size}`, `^`, `$`. Scopes: global, tenant, supplier, customer.

A rule's life: proposed → benchmark (every ground-truth line, levels 0–2 only) → **gate** (0 wrong, 0 conflicts,
minimum gain) → shadow mode (watches live lines) → live after N agreements → rolled back automatically after
N corrections by people. Every step is an event, shown as the rule's birth timeline in the panel.

## Run

Needs Go 1.23+, Docker, Python 3 (for the seed script) and `ffmpeg` on the PATH for voice notes.

**1. Start an ERP and load the demo distributor** (120 drinks and dairy products with packs, prices and stock,
15 shops, 10 suppliers). Odoo is the quickest; the other ERPs are in `erps/` with the same seed data.

```bash
docker compose -f erps/odoo/docker-compose.yml up -d        # Odoo 18 on http://127.0.0.1:8069
python erps/seed/odoo_setup.py                               # creates database "orderdesk" (admin / admin)
python erps/seed/odoo_seed.py erps/seed/seed.json            # products, packs, prices, stock, partners
```

**2. Configure and start Order Desk**

```bash
cp .env.example .env        # set META_MODEL_API_KEY, and TELEGRAM_BOT_TOKEN for the Telegram channel
go build -o orderdesk ./cmd/orderdesk
./orderdesk -addr 127.0.0.1:8610 -data ./data
```

**3. Connect and order.** Open http://127.0.0.1:8610:

- **Settings → ERP**: Odoo, `http://127.0.0.1:8069`, database `orderdesk`, user `admin` / `admin`. The password
  is used once to create an API key; only the key is kept. The catalogue syncs right away.
- **Customers → Pairing code**: send the code to your bot on Telegram, then order the way a shop does:
  `3 selecto 2l, 2 cartons ifri, 1 candia`, a voice note, or a photo of the bottles. Without Telegram, the
  **Inbox → Try a message** box takes the same text or photo.
- **Invoices**: upload a supplier invoice photo or PDF to see it become a goods receipt.

| ERP | Start | Seed | Panel settings |
|---|---|---|---|
| Odoo 18 | `erps/odoo` | `odoo_setup.py`, then `odoo_seed.py seed.json` | :8069, database `orderdesk` |
| ERPNext v16 | `erps/erpnext/pwd.yml` | `erpnext_setup.py seed.json` | :8090, `Administrator` / `admin` |
| Dolibarr 24 | `erps/dolibarr` | `dolibarr_setup.py seed.json` | :8081 |
| InvenTree 1.6 | `erps/inventree` | `inventree_setup.py seed.json` | :8083 |
| Tryton 7.0 | `erps/tryton` | `tryton_setup.py seed.json` | :8000, database `orderdesk` |

All test instances use `admin` / `admin` and listen on 127.0.0.1 only. More in [docs/ERPS.md](docs/ERPS.md);
WhatsApp setup in [docs/WHATSAPP.md](docs/WHATSAPP.md).

## Layout

```
cmd/orderdesk/        the single program: API + admin panel + Telegram + workers
internal/catalog/     products mirror, packs, aliases, normalisation, shortlist
internal/rules/       the rule language: types, patterns, interpreter, sentences
internal/resolve/     the resolution levels
internal/engineer/    the rule engineer (Muse Spark + tools), benchmark, gate
internal/desk/        the order agent: extraction, drafts, questions, confirmation, learning, supplier invoices
internal/adapter/     the ERP contract and one adapter per ERP (odoo, erpnext, dolibarr, inventree, tryton, elixir);
                      adaptertest/ is the live conformance test every adapter passes
internal/meta/        Meta Model API client with a cost meter and a daily budget
internal/telegram/    Telegram Bot API (long polling: no public address needed)
internal/whatsapp/    WhatsApp Cloud API webhook; internal/twilio/ the same through Twilio
internal/web/         panel and JSON API
erps/                 Docker setups and seed scripts for the five open-source ERPs
```

## Meta models

Every model Order Desk calls at runtime is a Meta model, through the Meta Model API.

- **Muse Voice Transcribe**: Darja, French and Arabic voice notes to text.
- **SAM 3.1**: finds every product on a photo of the products (boxes from its segmentation output).
- **Muse Spark 1.3**: tells list photos from product photos, copies lists, reads each product SAM found, reads
  supplier invoices, extracts items from conversations, picks among a
  shortlist, and runs the rule engineer, a tool-using agent (`list_residue`, `search_catalogue`, `list_rules`,
  `run_benchmark`, `submit_rule`).
- **Muse Wonder**: the rule engineer's fallback when Muse Spark admits no rule and the unresolved lines keep growing.

Every call is logged with its tokens and cost (Learning & cost screen); a daily budget stops AI calls while
aliases and rules keep working.

## License

Apache-2.0
