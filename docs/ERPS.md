# ERPs

Order Desk talks to an ERP only through an adapter (`internal/adapter`). Three exist; each passes the
same live conformance test (`internal/adapter/adaptertest`): catalogue with packs, customers,
suppliers, price, stock, an order created and confirmed, the history read back, a supplier invoice
booked as a goods receipt that moves the stock, and the last purchase price read back.

| | Elixir Gestion | Odoo 16–18 | ERPNext 14–16 |
|---|---|---|---|
| API | `/api/v1` (Bearer session) | JSON-RPC `/jsonrpc` | Frappe REST `/api` |
| Credentials kept | session token | login + API key | API key + secret (generated at connect) |
| Products | products + packagings | `product.product` + `product.packaging`, brand = category | `Item` + UOM conversions, brand |
| Order | sales order | `sale.order`, confirmed | `Sales Order`, submitted |
| Supplier invoice | goods receipt, posted | `purchase.order` confirmed + incoming transfer validated | `Purchase Receipt`, submitted |
| Shop's words | line notes | order note | comment on the order |

Connect from the panel: **Settings → ERP connection**, choose the ERP, address, login.

- **Odoo**: database name, login, and an **API key** (profile → Account Security → New API key). The key
  is kept because Odoo's API needs it on every call; revoke it in Odoo to cut access.
- **ERPNext**: username and password. The password is used once to generate an API key and secret for
  that user (this replaces the user's previous secret); only key and secret are kept.

## Local test ERPs (Docker)

`D:\Documents\ELIXIR SYSTEM\orderdesk-erps` (outside the repo):

- `odoo/docker-compose.yml` — Odoo 18 + Postgres on http://127.0.0.1:8069, database `orderdesk`,
  admin / admin. `docker compose -p od-odoo -f odoo/docker-compose.yml up -d`
- `erpnext/pwd.yml` — frappe_docker's single-machine compose, ERPNext v16 on http://127.0.0.1:8090,
  Administrator / admin. `docker compose -p od-erpnext -f erpnext/pwd.yml up -d`
- `seed/` — the same distributor in both (120 products with packs, prices and stock, 15 customers,
  10 suppliers): `export_seed.py`, `odoo_setup.py`, `odoo_seed.py`, `erpnext_setup.py`.

Live adapter tests:

```
ODOO_URL=http://127.0.0.1:8069 ODOO_DB=orderdesk ODOO_LOGIN=admin ODOO_KEY=admin go test ./internal/adapter/odoo -run Live -v
ERPNEXT_URL=http://127.0.0.1:8090 ERPNEXT_USER=Administrator ERPNEXT_PASS=admin go test ./internal/adapter/erpnext -run Live -v
```
