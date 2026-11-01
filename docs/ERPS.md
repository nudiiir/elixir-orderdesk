# ERPs

Order Desk talks to an ERP only through an adapter (`internal/adapter`), one per ERP, all behind the same
contract (`adapter.ERP`). Every adapter passes the same live conformance test
(`internal/adapter/adaptertest`):

- read the catalogue, with packs;
- read customers and suppliers, prices and stock;
- create an order and confirm it, then read the history back;
- book a supplier invoice as a goods receipt that moves the stock;
- read the last purchase price back.

Nothing is installed in any ERP: Order Desk uses each one's standard API with a normal user.

| ERP | API | Credentials Order Desk keeps | Order | Supplier invoice |
|---|---|---|---|---|
| Odoo 16–18 | JSON-RPC `/jsonrpc` | login + the secret given (use an Odoo **API key**) | `sale.order`, confirmed | `purchase.order` confirmed + incoming transfer validated |
| ERPNext v14–v16 | Frappe REST `/api` | API key + secret, generated at connect | `Sales Order`, submitted | `Purchase Receipt`, submitted |
| Dolibarr 17+ | REST `/api/index.php` | the user's API key (DOLAPIKEY), read or created at connect | customer order, validated | supplier order + reception, validated |
| InvenTree 1.x | REST `/api` | a named API token, issued at connect | Sales Order, issued | Purchase Order, received into stock |
| Tryton 6.0–7.x | JSON-RPC | a user session, opened at connect | `sale.sale`, confirmed | `purchase.purchase` + supplier shipment, done |
| Elixir Gestion | `/api/v1` | a user session | sales order | goods receipt, posted |

For every ERP except Odoo, the password is used once and never stored. Odoo's API needs a secret on every
call, so Order Desk keeps the one you give it. Give it an API key (Odoo → your profile → Account Security →
New API key) and revoke the key in Odoo to cut access.

The shop's own words travel with the order: they go in the line notes, the order note or a comment,
depending on the ERP. After a confirmation the panel links to the order in the ERP's own screens.

## Local test ERPs (Docker)

The five open-source ERPs run from `erps/`. Each listens on 127.0.0.1 only and uses throwaway test logins.
Run every command from the repository root. `python` means Python 3 (`py` on Windows if `python` is not
on the PATH). The seed scripts use only the standard library: there is nothing to `pip install`.

All five load the same demo distributor from `erps/seed/seed.json`:

- 120 drinks and dairy products, with packs, prices, VAT, barcodes and stock;
- 15 shops (customers) and 10 suppliers.

### Odoo 18 — http://127.0.0.1:8069 · database `orderdesk` · admin / admin

```bash
docker compose -f erps/odoo/docker-compose.yml up -d
# wait until http://127.0.0.1:8069 answers (about 30 s on first start)
python erps/seed/odoo_setup.py                      # creates the database with Sales, Purchase, Inventory (2–4 min)
python erps/seed/odoo_seed.py erps/seed/seed.json
```

### ERPNext v16 — http://127.0.0.1:8090 · Administrator / admin

It needs 4 GB of free memory. The first start creates the site, which takes 3–5 minutes.

```bash
docker compose -p od-erpnext -f erps/erpnext/pwd.yml up -d
docker compose -p od-erpnext -f erps/erpnext/pwd.yml logs -f create-site   # wait for it to exit
python erps/seed/erpnext_setup.py erps/seed/seed.json                       # setup wizard + seed
```

### Dolibarr 24 — http://127.0.0.1:8081 · admin / admin

```bash
docker compose -f erps/dolibarr/docker-compose.yml up -d
python erps/seed/dolibarr_setup.py erps/seed/seed.json       # also prints the admin API key (DOLIBARR_KEY)
```

### InvenTree 1.6 — http://127.0.0.1:8083 · admin / admin

```bash
docker compose -f erps/inventree/docker-compose.yml up -d
# first start runs the migrations (1–2 min): wait until http://127.0.0.1:8083/api/ answers
python erps/seed/inventree_setup.py erps/seed/seed.json
```

### Tryton 7.0 — http://127.0.0.1:8000 · database `orderdesk` · admin / admin

```bash
docker compose -f erps/tryton/docker-compose.yml up -d db
docker compose -f erps/tryton/docker-compose.yml run --rm init     # once: schema + modules (2–3 min)
docker compose -f erps/tryton/docker-compose.yml up -d tryton
python erps/seed/tryton_setup.py erps/seed/seed.json
```

## Connecting from the panel

**Settings → ERP connection**: choose the ERP, then fill in the address, the database (Odoo and Tryton only),
the user and the password. Connecting syncs the catalogue right away. **Sync now** syncs it again.

| ERP | Address | Database | User | Password |
|---|---|---|---|---|
| Odoo | `http://127.0.0.1:8069` | `orderdesk` | `admin` | `admin` |
| ERPNext | `http://127.0.0.1:8090` | | `Administrator` | `admin` |
| Dolibarr | `http://127.0.0.1:8081` | | `admin` | `admin` |
| InvenTree | `http://127.0.0.1:8083` | | `admin` | `admin` |
| Tryton | `http://127.0.0.1:8000` | `orderdesk` | `admin` | `admin` |

## Live adapter tests

`go test` does not read `.env`, so set the variables in the shell that runs the tests. Without them, the
live tests are skipped and only the offline tests run.

```bash
ODOO_URL=http://127.0.0.1:8069 ODOO_DB=orderdesk ODOO_LOGIN=admin ODOO_KEY=admin go test ./internal/adapter/odoo -v
ERPNEXT_URL=http://127.0.0.1:8090 ERPNEXT_USER=Administrator ERPNEXT_PASS=admin go test ./internal/adapter/erpnext -v
DOLIBARR_URL=http://127.0.0.1:8081 DOLIBARR_KEY=<printed by dolibarr_setup.py> go test ./internal/adapter/dolibarr -v
INVENTREE_URL=http://127.0.0.1:8083 INVENTREE_USER=admin INVENTREE_PASS=admin go test ./internal/adapter/inventree -v
TRYTON_URL=http://127.0.0.1:8000 TRYTON_DB=orderdesk TRYTON_USER=admin TRYTON_PASS=admin go test ./internal/adapter/tryton -v
```

In Windows PowerShell, set each variable first, for example `$env:ODOO_URL="http://127.0.0.1:8069"`, then
run `go test …`.

## Writing an adapter for another ERP

Implement `adapter.ERP` (`internal/adapter/adapter.go`) in a new package and run the conformance test
against a test instance of that ERP: `adaptertest.Run(t, client)`. The existing adapters are 500–1,100
lines each. Tryton's is the most complete example of an ERP with strict accounting rules.
