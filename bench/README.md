# Benchmark data and runs

Everything behind the [benchmark in the README](../README.md#benchmark-four-industries-invoices-the-desk-never-saw):
the four catalogues, every invoice and its answer key, the 200 shop orders with their voice notes and
screenshots, the runs that produced every number, and the desks those runs trained.

**Check the numbers in one second, with no API key:**

```bash
python bench/check.py
```

It reads the committed run reports and prints every figure of the README's tables: 93 % automatic, 58 % free,
3 wrong in 2,169 lines, the Spark-alone baseline, the photographed set, and the 200 orders by channel.

## What is here

| Folder | What it holds |
|---|---|
| `catalogs/` | The four catalogues: electronics (796 products), pharmacy (524), cosmetics & parapharmacy (586), supermarket (587). Real product lists, cleaned: one row per product, `id,name,brand`. |
| `wordings/` | How each product is written in the wild: real spellings (`source=real`, with how many independent sources saw it) and written variants. The invoice designations were drawn from these. |
| `invoices/` | `truth-<industry>.json`: 40 invoices per industry with, for every line, the text printed on the paper, the right catalogue id, quantity and prices. `invoices-<industry>.pdf`: the same invoices as printed and photographed (15 layouts; suppliers and their registration numbers are fictional). |
| `replay/` | The inputs of `cmd/adaptive` and `cmd/sparkalone`. `stream-<industry>.json`: 150 invoices from 15 suppliers (120 to learn from, 30 locked). `photographed-<industry>.json`: the 40 printed invoices (30 to learn from, 10 locked). The split is fixed in the file. |
| `orders/` | `orders.json`: 200 shop orders, 1,272 lines, with the answer key, the text as the shop typed it and the channel. `voice/`: the 40 voice notes (8 recorded by people, 26 Gemini TTS, 6 another voice generator; `sources.json` says which). `screens/`: the 40 screenshots. |
| `results/` | One folder per run and industry: `report.json` (every line, every phase, every rule and lexicon entry, the cost), `report.md` (the same, readable), `run.log` (the run as it printed). `results-*.json` are the summaries the charts were drawn from. |
| `trained-desks/` | The desk each stream run trained (SQLite, the same file the server uses): the Matching Profile Muse Spark designed, the aliases, the rules and their status, the earned autonomy. The order benchmark runs on these. |
| `scripts/` | How the data was built: catalogue cleaning, wordings, invoice generation (`gen.py`, `gen2.py`), the stream (`gen_stream.py`), the replay split (`make_replay.py`), the orders (`gen_orders.py`, `restyle.py`, `assign_channels.py`, `tts.py`), and the summaries and charts. The raw sources the catalogues were cleaned from are not shipped (they name distributors). |

| Run (`results/`) | Command | Meta cost |
|---|---|---|
| `stream/` | `adaptive -in bench/replay/stream-<industry>.json -calib 30,60,90,120 -evalat 0,30,60,90,120` | $2.0–2.9 per industry |
| `photographed/` | `adaptive -in bench/replay/photographed-<industry>.json -calib 10,20,30 -evalat 0,10,20,30` | about $1.9 per industry |
| `spark-alone-stream/`, `spark-alone-photographed/` | `sparkalone -in bench/replay/<set>-<industry>.json` | $0.6–0.7 (stream), about $0.2 (photographed) per industry |
| `orders/` | `orderbench -orders bench/orders/orders.json -industry <industry> -desk bench/trained-desks/<industry> -voice bench/orders/voice -screens bench/orders/screens` | about $0.5 per industry |

## Run it again

Each run replaces its `-out` folder and needs a Meta Model API key (`META_MODEL_API_KEY` in a keys file, as in
`.env.example`). From the repository root:

```bash
go build -o adaptive ./cmd/adaptive && go build -o sparkalone ./cmd/sparkalone && go build -o orderbench ./cmd/orderbench
```

```bash
./adaptive -in bench/replay/stream-pharmacy.json -out run-stream-pharmacy -keys .env -calib 30,60,90,120 -evalat 0,30,60,90,120
```

```bash
./sparkalone -in bench/replay/stream-pharmacy.json -out run-alone-pharmacy -keys .env
```

```bash
./orderbench -orders bench/orders/orders.json -industry pharmacy -desk bench/trained-desks/pharmacy -voice bench/orders/voice -screens bench/orders/screens -out run-orders-pharmacy -keys .env
```

`orderbench` copies the desk before using it, so `trained-desks/` is never changed. Model answers vary a little
from run to run: expect the same picture within a point or two, and the wrong bookings to stay at a handful.

## How the data was made

- **Catalogues.** Real product lists, cleaned into one canonical name per product (the `build_catalog_*.py`
  scripts say exactly how): a pharmacy wholesaler's stock listing, the product names a cosmetics &
  parapharmacy distributor prints on its invoices, an electronics & phone-accessories shop's product list, and
  a supermarket's assortment whose barcodes were checked against GS1 Algeria. Only product names are kept: no
  prices, customers or dates.
- **Wordings.** For pharmacy and cosmetics, every product was matched against about 455,000 spellings seen in
  pharmacy software lists, scanned invoices and nomenclatures; a spelling is kept only if it fits that product
  and no other one. Products with few real spellings got written variants built from the rewrite patterns those
  spellings use most. Electronics and supermarket wordings are written variants, plus the official Arabic names
  (GS1) for the supermarket.
- **Invoices.** Each supplier sells a range of about 100 products and prints each one with one fixed
  designation, as supplier software does; popular products are reordered more often. Every line keeps the
  printed text and the right answer.
- **Orders.** Written as shops write on WhatsApp: French, Darja in Latin letters, Darja mixed with French in
  Arabic script, screenshots (a notes app, a spreadsheet, a purchase order) and voice notes. Shop names are
  fictional.
