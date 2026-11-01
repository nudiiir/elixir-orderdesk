# Elixir Order Desk

**Shops order on WhatsApp the way they talk. Meta's models turn every message into a booked ERP order, and
Muse Spark teaches the desk to do it without them.**

Small shops order from their distributor the way they talk: voice notes in Darja, photos of handwritten
lists, "same as last time, plus two cartons of Ifri". At the distributor, a clerk retypes every line into the
ERP, guessing which of 1,000 products "ifri 1.5" or "VIS TH M8X20 ZN BT100" means. Order Desk does that job:

1. **Muse Voice Transcribe** hears the voice note, primed with the distributor's own brand names.
2. **SAM 3.1** finds each product on a photo and **Muse Spark** reads it; Muse Spark also copies written lists
   and cuts chatty text into items.
3. Every line becomes one precise product from the distributor's catalogue, checked against the ERP's own
   prices and stock. The shop is asked only what is really open.
4. The confirmed order is booked in the ERP: Odoo, ERPNext, Dolibarr, InvenTree or Tryton.

The same desk reads supplier invoices (Muse Spark and Muse Wonder read each one independently) and books them
as goods receipts. It runs on WhatsApp (Meta's Cloud API), Telegram and its own panel.

<p align="center"><img src="docs/media/odoo-1-question.jpg" alt="The Order Desk inbox: a shop's order resolved line by line, one question left" width="900"></p>

### Muse Spark builds the free path

Calling a model on every line of every order forever is slow, costly and never gets better. So Muse Spark
does more than answer: it **teaches the desk**, and the code proves every lesson before it is used.

- **It designs the matching for each industry.** From the catalogue, Muse Spark decides how this trade writes
  its products (inch sizes and `M8X20` screws in hardware, `PET` and pack counts in groceries), the vocabulary
  its suppliers use and the search method. A scratch lab measures every proposal on lines Spark never saw.
- **It finds the products.** When a wording is new, Muse Spark picks the product from a shortlist. Once a
  person confirms it, the wording is remembered and costs nothing next time.
- **It writes rules.** The rule engineer, a Muse Spark agent with tools, studies what the desk could not
  resolve and writes rules in a small rule language. A code gate admits a rule only with zero wrong matches.
- **It attacks its own rules.** Before a rule goes live, a second Muse Spark call, the **breaker**, writes
  look-alike lines for neighbouring products to try to make the rule book the wrong one.
- **It checks the code until the code has earned trust.** Muse Spark double-checks every answer the free steps
  give until their precision clears a 99 % lower bound, and goes back to checking a supplier after a correction.

On unseen invoices, **90 % of the lines the desk resolved for free trace back to Muse Spark**: a product it
found, a rule it wrote, or matching it designed. A person taught the other 10 %. The desk made **0 wrong
automatic bookings** in two industries, at a quarter to an eighth of the cost of asking Muse Spark about every
line ([results](#results-on-replayed-invoices)).

Built during the Meta Global AI Developer Hackathon (October 26 – November 4, 2026). Everything in this
repository was written during the event.

## Contents

- [Order Desk in 60 seconds](#order-desk-in-60-seconds)
- [One order, five ERPs (screenshots)](#one-order-five-erps)
- [Run it: the full demo with Odoo](#run-it-the-full-demo-with-odoo)
- [The other ERPs](#the-other-erps)
- [Chat channels: Telegram and WhatsApp](#chat-channels-telegram-and-whatsapp)
- [Supplier invoices](#supplier-invoices)
- [How a line is resolved](#how-a-line-is-resolved)
- [Adaptive matching: a pipeline Muse Spark designs for each industry](#adaptive-matching-a-pipeline-muse-spark-designs-for-each-industry)
- [Results on replayed invoices](#results-on-replayed-invoices)
- [Meta models: each one's job, and what checks it](#meta-models-each-ones-job-and-what-checks-it)
- [The rule language](#the-rule-language)
- [Limits](#limits)
- [Configuration](#configuration)
- [Tests](#tests)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [Repository layout](#repository-layout)

## Order Desk in 60 seconds

- **One program.** A single Go binary holds the panel, the JSON API, the Telegram and WhatsApp channels and
  the background workers, with SQLite for storage. It has no other service to run.
- **Any ERP.** It works with Odoo, ERPNext, Dolibarr, InvenTree and Tryton through each one's standard API,
  with nothing installed in the ERP. Each adapter passes the same live conformance test. Elixir Distribution,
  our existing ERP, is a sixth adapter. It is not part of the demo: everything here runs on the five
  open-source ERPs in `erps/`.
- **Any message, on Meta's channels.** WhatsApp through Meta's Cloud API (or Twilio), and Telegram. Typed
  lists, chatty text (Muse Spark), voice notes (Muse Voice Transcribe primed with the catalogue's brands),
  photos of written lists (Muse Spark) and photos of the products themselves (SAM 3.1 + Muse Spark).
- **Four Meta models, each with its own job.** Muse Spark reads, picks, designs the matching, writes rules and
  attacks them; Muse Wonder reads every invoice a second time and takes over the rule engineer when a Spark
  session finds nothing; Muse Voice Transcribe hears; SAM 3.1 finds products on photos.
- **Cheapest step first.** Each line climbs a ladder: codes, aliases, rules, a shortlist, one Muse Spark call
  per message, and a person. The free steps are the ones Muse Spark built.
- **The model proposes, the code decides.** Muse Spark picks only from a shortlist the code built. Every rule
  and every change to the matching is measured on known answers before it goes live.
- **Every Meta model call is metered.** The panel shows the tokens and cost of each call. A daily budget stops
  the calls while aliases and rules keep working.

## One order, five ERPs

The same shop order goes through the same desk into each of the five ERPs:

```
3 cartons selecto 2l
2 ifri eau minerale 1.5
4 rouiba orange 1l
2 ngaous abricot
```

Three lines resolve for free (rules, brand and size). "ngaous abricot" matches two products and this shop has
no habit yet, so it is asked which one. After the answer the order is confirmed and opens in the ERP's own screens. The screenshots
were taken by `tools/capture_erps.py` against the Docker ERPs in `erps/`, with no manual step.

| ERP | In Order Desk | The order in the ERP |
|---|---|---|
| **Odoo 18** | <img src="docs/media/odoo-2-draft.jpg" width="420"> | <img src="docs/media/odoo-3-erp.jpg" width="420"> |
| **ERPNext v16** | <img src="docs/media/erpnext-2-draft.jpg" width="420"> | <img src="docs/media/erpnext-3-erp.jpg" width="420"> |
| **Dolibarr 24** | <img src="docs/media/dolibarr-2-draft.jpg" width="420"> | <img src="docs/media/dolibarr-3-erp.jpg" width="420"> |
| **InvenTree 1.6** | <img src="docs/media/inventree-2-draft.jpg" width="420"> | <img src="docs/media/inventree-3-erp.jpg" width="420"> |
| **Tryton 7.0** | <img src="docs/media/tryton-2-draft.jpg" width="420"> | <img src="docs/media/tryton-3-erp.jpg" width="420"> |

Click any image for full size. The question step for each ERP is in `docs/media/<erp>-1-question.jpg`.

## Run it: the full demo with Odoo

### What you need

| Tool | Version | Used for |
|---|---|---|
| [Go](https://go.dev/dl/) | 1.23 or newer | building Order Desk |
| [Docker](https://docs.docker.com/get-docker/) with Compose | any recent | the test ERPs |
| Python | 3.9 or newer, standard library only (no `pip install`) | the seed scripts and the simulators |
| [ffmpeg](https://ffmpeg.org/download.html) | any, on the PATH | voice notes only |
| A Meta Model API key | from https://dev.meta.ai | the AI steps (optional, see below) |

**The Meta key is what makes the desk learn.** Without it the panel, the ERP connection, typed orders,
questions and booking still work, but nothing new is learned: every line the existing aliases and rules
cannot resolve goes to a person, and voice notes, photos, invoices, the matching design and the rule engineer
are off.

Commands below are for a terminal opened in the repository folder. Where Windows differs, both forms are given.
On Windows, use `py` if `python` is not found.

### 1. Get the code

```bash
git clone https://github.com/nudiiir/elixir-orderdesk.git
cd elixir-orderdesk
```

### 2. Start Odoo and load the demo distributor

The demo distributor has 120 drinks and dairy products with packs, prices, VAT, barcodes and stock, plus 15
shops and 10 suppliers.

```bash
docker compose -f erps/odoo/docker-compose.yml up -d
```

Wait until http://127.0.0.1:8069 shows the Odoo page (about 30 seconds on the first start), then:

```bash
python erps/seed/odoo_setup.py
python erps/seed/odoo_seed.py erps/seed/seed.json
```

`odoo_setup.py` creates the database `orderdesk` with Sales, Purchase and Inventory, which takes 2–4 minutes.
The Odoo login is `admin` / `admin`.

### 3. Configure

macOS / Linux:

```bash
cp .env.example .env
```

Windows (PowerShell):

```powershell
Copy-Item .env.example .env
```

Open `.env` and set `META_MODEL_API_KEY`. Add `TELEGRAM_BOT_TOKEN` if you want the Telegram channel (see
[Telegram](#telegram)). Leave everything else empty for now. `.env` is git-ignored.

### 4. Build and start Order Desk

macOS / Linux:

```bash
go build -o orderdesk ./cmd/orderdesk
./orderdesk -addr 127.0.0.1:8610 -data ./data
```

Windows (PowerShell):

```powershell
go build -o orderdesk.exe ./cmd/orderdesk
.\orderdesk.exe -addr 127.0.0.1:8610 -data .\data
```

Expected output:

```
2026/11/01 17:00:00 Order Desk panel on http://127.0.0.1:8610 (Meta: true, ERP: false)
```

### 5. Connect the ERP

Open http://127.0.0.1:8610 → **Settings** → **ERP connection** and fill in:

| Field | Value |
|---|---|
| ERP | Odoo |
| Address | `http://127.0.0.1:8069` |
| Database | `orderdesk` |
| User | `admin` |
| API key or password (kept) | `admin` |

Click **Connect**. The catalogue syncs right away: the side bar shows *ERP connected*, and **Catalogue** lists
120 products. Odoo's API needs a secret on every call, so Order Desk keeps the one you give it. With a real
Odoo, give it an API key (Odoo → your profile → Account Security → New API key), which you can revoke.

### 6. Send an order

Go to **Inbox** → **Try a message**, choose a shop, paste the order below and click **Run**:

```
3 cartons selecto 2l
2 ifri eau minerale 1.5
4 rouiba orange 1l
2 ngaous abricot
```

What happens:

- Each line shows how it was resolved: A alias, R rules, AI pick or P person, with its cost.
- "ngaous abricot" waits for an answer. Click **Ngaous Nectar Abricot 1L**.
- Click **Confirm into ERP →**. The toast shows the Odoo order number, for example `Order S00013 created in the ERP`.
- Open http://127.0.0.1:8069 → Sales: the order is there, with the shop's words in the notes.

Send the same order again: the line you answered is now resolved by an alias, for free. Try chatty text too
("salam, mets moi 2 fardeaux ifri et 3 selecto"), or a photo of bottles with **Choose File**.

### 7. Watch it learn

- **Learning & cost** shows lines resolved for free vs by the AI, and every AI call with its tokens and cost.
- **Rules** lists the rules Muse Spark proposed, the gate's verdict and the breaker's attack lines on each, and its birth timeline:
  proposed → benchmark → shadow → live. **Rules → Run engineer now** starts a session at once. It also runs by itself
  once 50 lines have needed the AI or a person since its last run.
- **Catalogue** → a product shows every wording learned for it and how many confirmations each one has.

## The other ERPs

Each ERP is started and seeded the same way. Full instructions, including the live adapter tests and how to
write an adapter for another ERP, are in [docs/ERPS.md](docs/ERPS.md).

| ERP | Start | Seed | Settings → ERP connection |
|---|---|---|---|
| Odoo 18 | `docker compose -f erps/odoo/docker-compose.yml up -d` | `python erps/seed/odoo_setup.py`, then `python erps/seed/odoo_seed.py erps/seed/seed.json` | `http://127.0.0.1:8069`, database `orderdesk`, `admin` / `admin` |
| ERPNext v16 | `docker compose -p od-erpnext -f erps/erpnext/pwd.yml up -d` (4 GB RAM; first start 3–5 min) | `python erps/seed/erpnext_setup.py erps/seed/seed.json` | `http://127.0.0.1:8090`, `Administrator` / `admin` |
| Dolibarr 24 | `docker compose -f erps/dolibarr/docker-compose.yml up -d` | `python erps/seed/dolibarr_setup.py erps/seed/seed.json` | `http://127.0.0.1:8081`, `admin` / `admin` |
| InvenTree 1.6 | `docker compose -f erps/inventree/docker-compose.yml up -d` | `python erps/seed/inventree_setup.py erps/seed/seed.json` | `http://127.0.0.1:8083`, `admin` / `admin` |
| Tryton 7.0 | `docker compose -f erps/tryton/docker-compose.yml up -d db`, then `… run --rm init`, then `… up -d tryton` | `python erps/seed/tryton_setup.py erps/seed/seed.json` | `http://127.0.0.1:8000`, database `orderdesk`, `admin` / `admin` |

All test instances listen on 127.0.0.1 only, and their logins are throwaway test logins. To switch ERP,
connect another one in Settings: the catalogue is synced again from it.

## Chat channels: Telegram and WhatsApp

Shops and the owner are linked to the desk with a **pairing code** from the panel. A shop's code comes from
**Customers → Pairing code**. The owner's code, used to send supplier invoices, comes from **Invoices →
Pairing code**. Sending the code in a chat links that chat; `/deconnecter` unlinks it.

### Telegram

Telegram uses long polling, so it needs no public address.

1. In Telegram, open [@BotFather](https://t.me/BotFather) → `/newbot` → choose a name and a username. Copy the
   token it gives you.
2. Put it in `.env` as `TELEGRAM_BOT_TOKEN=…` and restart Order Desk. The log shows the bot's name.
3. **Customers** → pick a shop → **Pairing code** → send the code to your bot.
4. Order the way a shop does: `3 selecto 2l, 2 cartons ifri, 1 candia`, a voice note, or a photo. Questions
   come back as buttons. **✅ Confirmer** books the order in the ERP.

### WhatsApp

WhatsApp works through Meta's Cloud API or through Twilio, and both need a public HTTPS address (a tunnel).
To try it **without any WhatsApp account**, `tools/whatsapp-sim` plays both Meta and a phone, locally. The
three setups, from the quickest to the real thing, are in [docs/WHATSAPP.md](docs/WHATSAPP.md).

## Supplier invoices

The owner sends a photo or PDF of a supplier invoice, in the owner's chat or on the **Invoices** screen (*Drop a photo or PDF*). Here is
what happens:

1. Muse Spark reads the supplier, the number, the date and every line with its quantity, price and total.
2. **Muse Wonder reads the same paper on its own.** A line the two readers do not agree on (words, quantity or
   unit price) is never booked automatically: it is shown to the person with its product proposed.
3. Each line is resolved in that supplier's scope, with its own wordings and rules.
4. The quantities are checked against the line totals (packs vs units).
5. Once approved, the invoice is booked in the ERP as a goods receipt that moves the stock.

On 18 invoice photos (`samples/readbench`: small print, skewed, blurred, JPEG), Muse Spark read 179 of 186
lines exactly and Muse Wonder 182. Of Spark's 7 misreadings (`GA5030R` read as `GASO30R`, `ART9303` as
`ART9383`…), the agreement check sent 5 to a person; 2 got through because both models misread them the same
way, and 1 right line was flagged. The second reading costs about $0.026 per invoice (`cmd/readbench`).

Try it with [samples/invoice-specimen.png](samples/invoice-specimen.png). It is a fictional invoice from the
demo wholesaler, marked as a specimen; `python tools/make_invoice.py out.png` draws it again.

## How a line is resolved

| Step | What happens | Cost |
|---|---|---|
| −1 · codes | a printed barcode or reference, or a supplier's article code confirmed before | free |
| 0 · normalise | lower case, accents, Arabic digits, sizes (`1,5 L` → `1500ml`), the desk's Matching Profile, live rewrite rules | free |
| 1 · alias | a trusted wording for this shop or supplier | free |
| 2 · rules | synonyms, packs, defaults, guards, attributes, then an exact catalogue match | free |
| 3 · shortlist | the 8 most likely products the code would accept (same attributes, the brand the line names) | free |
| 4 · Muse Spark | picks one id **from the shortlist only**, one call for all the open lines of a message | paid |
| 5 · person | one precise question with buttons | — |

Messages are cut into items first. Plain lists use a deterministic parser, with no model call. For voice notes,
Muse Voice Transcribe writes the text, then Muse Spark extracts the items. Voice Transcribe is given the
catalogue's brands as keywords, so it writes "Choufan" and "Kool" instead of "chou fin" and "cool": on 30
French voice orders (synthetic voice, `cmd/voicebench`) it heard 65 of 71 brand names with the keywords and 11
of 71 without, at the same price. It quotes the shop's words and never
chooses products, and it gets only the brand names in its prompt, never the catalogue. "Same as last time"
is copied from the ERP by code.

**Photos.** Muse Spark first tells a written list, which it copies line by line, from a photo of the products
themselves. For products:

1. **SAM 3.1** finds every bottle, can or carton, with one call per kind of packaging.
2. Duplicate and weak boxes are dropped.
3. Muse Spark reads all the crops in one call, the way a shop would type them ("Selecto 2L").
4. Identical products are counted, and a caption like "3 cartons" applies to a single product.

The shop gets its photo back with every product boxed in a colour that matches the reply ("🟩 2 × Selecto 2L").
A photo costs about $0.015.

**Learning.** Every confirmed line becomes ground truth and a witness for its wording. A wording becomes a
trusted alias after two confirmations, and a supplier's article code after one. A person's correction blocks
the wrong wording, retires a code that named two products, and counts against the rule that gave the wrong
answer. A rule goes back after repeated corrections.

## Adaptive matching: a pipeline Muse Spark designs for each industry

Every industry writes its products in its own way: strengths and forms, dimensions and inch sizes, flavours
and pack counts. Order Desk does not ship that knowledge. It learns it per desk (`internal/profile`):

- **Bootstrap.** Muse Spark reads a sample of the catalogue and designs a first Matching Profile:
  - which first-pass blocks this industry needs (`strength`, `dimensions`, `inch`, `units_glue`, `pack_count`,
    `shade`, `split_glued`);
  - the vocabulary its suppliers use;
  - the attributes that tell products apart;
  - a search method.
- **Proof.** The code measures every proposal in a scratch lab, on confirmed lines and on lines Spark never
  saw. A proposal is kept only with zero wrong, zero lost and no products made to read alike. Vocabulary
  proposed before any evidence waits in shadow. A word that would drop or change digits is refused, because
  codes go to the code table.
- **Search.** BM25, character trigrams and word similarity are raced on real lines (`internal/search`). The
  winner is kept only if it holds on unseen lines.
- **Codes.** A printed barcode or manufacturer reference resolves a line at once. A supplier's article code is
  learned from one confirmation and retired if it ever names two products. Until a second confirmation, Spark
  still checks it.
- **Earned autonomy.** Muse Spark checks every answer the code gives until the code's precision clears a 99 %
  lower bound (about 270 confirmed answers). After that it spot-checks risky answers, and a correction sends
  that supplier back under supervision (`internal/desk/autonomy.go`).

### Results on replayed invoices

`cmd/adaptive` replays an invoice set through this loop:

1. Muse Spark designs the profile.
2. It checks every line of the training invoices, and a clerk who knows the answers confirms or corrects them.
3. The profile is calibrated twice, and the rule engineer and the breaker run at each calibration.
4. The trained desk is measured on invoices it never saw, with nothing answered or learned.

The same unseen lines were also given to two baselines. **Muse Spark alone** gets the whole catalogue and the
invoice lines in one call, with no desk around it (`cmd/sparkalone`); a line it gives under 0.9 confidence goes
to a person, as on the desk. **The classic desk** has aliases, rules and Spark picks, but no matching designed
by Spark (`cmd/replay`).

| Hardware & electrical: 612 products, 18 unseen invoices, 186 lines | Booked right | Asked a person | **Booked wrong** | Meta model cost |
|---|---|---|---|---|
| Order Desk | 166 (74 % free, 16 % Spark pick) | 20 | **0** | $0.12 |
| Muse Spark alone, whole catalogue in every call | 165 | 20 | **1** | $0.43 |
| Classic desk, no Spark-designed matching | 140 (46 % free) | 45 | **1** | $0.09 |

| Supermarket / FMCG: 1,160 products, 20 unseen invoices, 234 lines | Booked right | Asked a person | **Booked wrong** | Meta model cost |
|---|---|---|---|---|
| Order Desk | 215 (89 % free, 3 % Spark pick) | 19 | **0** | $0.055 |
| Muse Spark alone, whole catalogue in every call | 231 | 3 | **0** | $0.46 |
| Classic desk, no Spark-designed matching | 211 (86 % free) | 23 | **0** | $0.02 |

What the tables say:

- **Muse Spark is an excellent reader.** Alone, with the whole catalogue in front of it, it got 178 of 186
  hardware lines and all 234 supermarket lines right. The desk does not try to beat it at reading.
- **The desk makes Spark's judgement permanent and cheap.** It matched Spark alone in hardware with no wrong
  booking, for a quarter of the cost; in the supermarket it asked about 16 more lines than Spark alone, for an
  eighth of the cost. Spark alone also needs the whole catalogue in every call: about 16,000 tokens per
  invoice for 1,160 products; a distributor with 20,000 products would need about 280,000.
- **The matching Spark designed is what the free path is made of.** Without it (the classic desk), only 46 % of
  the hardware lines were free instead of 74 %, and one was booked wrong.
- **Who made the free answers possible.** The report traces every free answer on the unseen invoices back to
  its origin:

  | | Free answers | Muse Spark found it (a pick a person confirmed, or a rule it wrote) | Through matching Muse Spark designed | Taught by a person |
  |---|---|---|---|---|
  | Hardware | 137 | 36 (26 %) | 88 (64 %) | 13 (9 %) |
  | Supermarket | 209 | 29 (14 %) | 159 (76 %) | 21 (10 %) |

- **Earned autonomy.** In the supermarket, the code's answers cleared the 99 % bar after 31 invoices (Spark
  agreed with the code on all 267 lines it checked), and Spark stopped checking every line. In hardware, one
  wrong answer in training (200 of 201) kept the desk supervised: Spark still checks every code answer there.
- **The breaker.** Against the 6 rules the rule engineer got through the gate in hardware, Muse Spark wrote 46
  look-alike lines (`VIS THC M8X25` for a socket screw against the rule `vis th` → *tête hexagonale*, `bouchon
  ppr dn25` against `ppr {n}` → `ppr d{n}`…). None was booked to the wrong product: on each one the desk asked.

The profile Spark designed for hardware turned on `split_glued`, `inch` (`15/21` → ½"), `dimensions` (`M8X20`),
`units_glue` and `pack_count` (`BT100`), learned `pc` → *prise* and `boulon` → *vis* from evidence, and refused abbreviations like
`zn` and `th` as vocabulary until rules with a supplier scope proved them. The hardware set is synthetic and
shipped in `samples/`; the supermarket catalogue is a real store's and is not shipped, and its invoice lines
were written for the test.

Run it yourself on the hardware set. It costs about $1.50 of Meta Model API credits:

```bash
go build -o adaptive ./cmd/adaptive                 # Windows: adaptive.exe
./adaptive -in samples/replay-hardware.json -out ./run-hardware -keys .env
go build -o sparkalone ./cmd/sparkalone && ./sparkalone -in samples/replay-hardware.json -keys .env   # about $0.45
go build -o readas ./cmd/readas
./readas -data ./run-hardware/desk "VIS TH M8X20 ZN BT100" "ROBINET 15/21"
```

`run-hardware/report.md` lists what Spark identified, every experiment with the gate's verdict, the phase
table and who made the free answers possible. `adaptive -credit <db>` prints that last table for any earlier
run, `adaptive -break <db> -keys .env` runs the breaker against a run's rules, and `readas` prints how the
trained desk reads a line.

## Meta models: each one's job, and what checks it

Every model Order Desk calls is a Meta model, through the Meta Model API, and the chat channel is Meta's own
WhatsApp Cloud API. No model's answer is used unchecked.

| Model | Its jobs | What checks it |
|---|---|---|
| **Muse Spark 1.3** (`muse-spark-1.3`) | reads list photos, product crops and supplier invoices; cuts chatty messages and voice transcripts into items; picks a product from a shortlist; designs and calibrates each desk's Matching Profile; the **rule engineer**, a tool-using agent (`list_residue`, `search_catalogue`, `list_rules`, `run_benchmark`, `submit_rule`); the **breaker** that attacks every admitted rule; checks the free steps' answers until they earn autonomy | its picks only come from a shortlist the code built, and a pick under 0.9 confidence goes to a person; its profile and rules are measured on known and unseen lines and admitted only with 0 wrong; a person confirms every new wording |
| **Muse Wonder 1.0** (`muse-wonder-1.0`) | reads every supplier invoice a second time, on its own; takes over the rule engineer when a Muse Spark session finds no rule the gate admits | the two invoice readings must agree line by line; its rules go through the same gate and breaker |
| **Muse Voice Transcribe** (`muse-voice-transcribe-1.0`) | Darja, French and Arabic voice notes to text, with the catalogue's brands as keywords | the transcript is shown back to the shop with every line, and goes through the same matching as typed text |
| **SAM 3.1** (`sam-3.1`) | finds every bottle, can or carton on a photo of the products | weak and duplicate boxes are dropped; Muse Spark reads each box, and the shop gets its photo back boxed to confirm |

Every call is logged with its model, job, tokens and cost (**Learning & cost**). A daily budget (**Settings**,
$2 by default) stops model calls while aliases and rules keep working.

## The rule language

Eight rule types, each shown in the panel as a sentence generated from its structured form:

| Type | Example sentence |
|---|---|
| **synonym** | `"slkt"` means `"selecto"` |
| **rewrite** | `"1l5"` is written as `"1.5l"` |
| **pack** | a trailing `"x{n}"` means a carton of `{n}` |
| **default** | for Épicerie Nour, `"ifri"` with no size means Ifri 1.5 L |
| **guard** | `"citron"` only matches products whose name contains "citron" |
| **attribute** | `"zero"`, `"light"` each name a variant, in orders, invoices and catalogue names alike |
| **word** | `"pet"` describes a product but never tells two products apart |
| **policy** | Two texts whose colour differs are never the same product. |

Patterns use literal words, `a|b`, `{n}`, `{size}`, `{word}`, `^` and `$`. Scopes are global, tenant, supplier
or customer.

A rule's life is proposed → benchmark → **gate** → shadow → live:

- **benchmark**: the rule runs on every ground-truth line, through the free steps only;
- **gate**: the rule needs 0 wrong, 0 conflicts and a minimum gain;
- **shadow**: the rule watches live lines;
- **live**: after N agreements.

A live rule is rolled back automatically after repeated corrections by people. Every step is an event, shown
as the rule's birth timeline in the panel.

## Limits

What the numbers above do not show yet:

- **No real distributor's orders are measured here.** The hardware invoices are synthetic, and the supermarket
  catalogue is real but its invoice lines were written for the test. Shop orders (text, voice, photos) have no
  measured accuracy in this repository.
- **The voice and reading benches are synthetic.** The 30 voice orders are French read by a speech synthesiser,
  not Darja from a phone; the 18 invoice photos are drawn, blurred and compressed, not photographed.
- **Muse Spark alone asked less in the supermarket.** The desk's caution costs about 16 extra questions on 234
  lines there; the 0.9 threshold is a setting.
- **Two readers can agree on the same misreading.** It happened on 2 of 186 invoice lines. The product match and
  the check against the printed total still apply to those lines, but the agreement check itself misses them.
- **The hardware desk has not earned autonomy.** One wrong answer in training keeps Muse Spark checking every
  code answer there, as designed.
- **Every model runs through the Meta Model API.** There is no open-weights or offline mode yet.

## Configuration

### Command line

```
orderdesk [-addr 127.0.0.1:8610] [-data ./data] [-keys .env,whatsapp.env] [-no-telegram]
```

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `127.0.0.1:8610` | panel, API and webhook address |
| `-data` | `data` | data folder: the SQLite database, voice notes, photos, invoices |
| `-keys` | `.env` if present | keys files with `KEY=value` lines, comma-separated (never printed) |
| `-no-telegram` | off | do not start the Telegram connector even if a token is set |

### Environment (`.env`)

| Variable | Needed for |
|---|---|
| `META_MODEL_API_KEY` | every AI step |
| `TELEGRAM_BOT_TOKEN` | the Telegram channel |
| `WHATSAPP_TOKEN`, `WHATSAPP_PHONE_ID`, `WHATSAPP_VERIFY_TOKEN`, `WHATSAPP_APP_SECRET` | WhatsApp through Meta's Cloud API ([docs/WHATSAPP.md](docs/WHATSAPP.md)) |
| `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_WHATSAPP_FROM`, `TWILIO_WEBHOOK_URL` | WhatsApp through Twilio |
| `FFMPEG` | the path to ffmpeg, if it is not on the PATH |

The ERP connection is made in the panel, not in `.env`. The other variables in `.env.example` are only for
the live tests.

### Panel settings

**Settings** holds the learning thresholds:

- confirmations before an alias is trusted;
- shadow agreements before a rule goes live;
- corrections before a rule is rolled back;
- the gate's minimum gain.

It also holds the rule engineer's models and schedule, the daily AI budget, and whether approved invoices are
posted at once.

## Tests

```bash
go vet ./...
go test ./...
```

The offline tests need nothing. They cover:

- normalisation and the Matching Profile blocks;
- the rule language and the gate;
- codes, the autonomy bound, the voice keywords and the two-reader agreement on invoices;
- the WhatsApp and Twilio channels;
- the resolver's benchmarks.

The live tests run only when their variables are set, and are skipped otherwise:

- the adapter conformance test against each Docker ERP ([docs/ERPS.md](docs/ERPS.md#live-adapter-tests));
- the photo test (`ORDERDESK_PHOTO`, `META_MODEL_API_KEY`).

## Security

- **The panel answers this computer only.** It has no login, so it refuses any request that does not come
  from 127.0.0.1. That covers forwarding headers from a tunnel, another host name (DNS rebinding), and a write
  sent by another site's page (CSRF). It also sets `X-Frame-Options: DENY`.
- **The webhook is the only public route.** A tunnel reaches `/whatsapp/webhook` only. Every call must carry
  Meta's or Twilio's signature, and Order Desk refuses to start the Cloud API channel without
  `WHATSAPP_APP_SECRET`.
- **Inputs are bounded.**
  - Request bodies, downloads (25 MB) and photos (40 megapixels, checked before decoding) have size limits.
  - Media paths are sanitised.
  - ffmpeg only reads its pipe.
- **Secrets stay out of logs.** Keys are read from `.env` or keys files and never printed. The Telegram token
  is stripped from network errors.
- **ERP passwords are not kept.** For ERPNext, Dolibarr, InvenTree and Tryton the password is used once to
  obtain a revocable key. For Odoo, give an API key.
- **Nothing secret is in the repository.** The demo data uses placeholder phone numbers, and `.env`,
  databases and data folders are git-ignored.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `odoo_setup.py` fails with a connection error | Odoo is still starting: wait until http://127.0.0.1:8069 answers, then run it again |
| ERPNext seed fails right after `up -d` | the site is still being created: wait for `create-site` to exit (`docker compose -p od-erpnext -f erps/erpnext/pwd.yml logs -f create-site`) |
| Tryton connect says the database does not exist | run the `init` step once (`docker compose -f erps/tryton/docker-compose.yml run --rm init`) |
| The panel shows `the Order Desk panel only answers on this computer` | open it as `http://127.0.0.1:8610` or `http://localhost:8610`, not through a tunnel or a LAN address |
| `Meta: false` in the start line | `META_MODEL_API_KEY` is not set: check `.env`, or pass `-keys path/to/file` |
| Voice notes fail | install ffmpeg, or set `FFMPEG` to its path |
| WhatsApp channel refuses to start | set `WHATSAPP_APP_SECRET` (Meta app → Settings → Basic) |
| Port 8610 is taken | start with another address: `-addr 127.0.0.1:8620` |

## Repository layout

```
cmd/orderdesk/        the program: panel + JSON API + Telegram + WhatsApp + workers
cmd/adaptive/         replays invoices through the adaptive pipeline and writes what Spark identified,
                      who made the free answers possible (-credit) and the breaker's attacks (-break)
cmd/sparkalone/       the baseline: Muse Spark alone, with the whole catalogue, on the same unseen lines
cmd/voicebench/       Muse Voice Transcribe with and without the catalogue's brands as keywords
cmd/readbench/        Muse Spark and Muse Wonder each read invoice photos; scores the agreement check
cmd/replay/           replays invoices through the classic pipeline (before / learning / after)
cmd/readas/           prints how a trained desk reads a line
internal/catalog/     products mirror, packs, aliases, codes, normalisation, shortlist
internal/profile/     the Matching Profile: first-pass blocks, lexicon, the lab, Spark's design and calibration
internal/search/      BM25, trigram and word-similarity search, raced per desk
internal/rules/       the rule language: types, patterns, interpreter, sentences
internal/resolve/     the resolution ladder, the Muse Spark pick, the sentinel
internal/engineer/    the rule engineer (Muse Spark + tools), benchmark, gate, and the breaker
internal/desk/        the order agent: extraction, photos, drafts, questions, confirmation, learning,
                      supplier invoices read by Muse Spark and Muse Wonder, earned autonomy
internal/adapter/     the ERP contract and one adapter per ERP (odoo, erpnext, dolibarr, inventree,
                      tryton, elixir); adaptertest/ is the live conformance test every adapter passes
internal/meta/        Meta Model API client with a cost meter and a daily budget
internal/telegram/    Telegram Bot API (long polling: no public address needed)
internal/whatsapp/    WhatsApp Cloud API webhook; internal/twilio/ the same through Twilio
internal/web/         panel and JSON API
erps/                 Docker setups and seed scripts for the five open-source ERPs
samples/              a specimen supplier invoice, the hardware replay set and the invoice-photo bench
tools/                WhatsApp simulator, invoice generator, invoice-photo bench, ERP screenshot capture
docs/                 ERPs, WhatsApp, screenshots
```

## License

[Apache-2.0](LICENSE)
