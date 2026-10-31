"""Load the shared distributor seed into the Odoo test database 'orderdesk' (admin / admin, local only)."""
import json, sys, urllib.request

URL, DB, USER, PWD = "http://127.0.0.1:8069/jsonrpc", "orderdesk", "admin", "admin"


def rpc(service, method, *args):
    body = json.dumps({"jsonrpc": "2.0", "method": "call", "params": {"service": service, "method": method, "args": list(args)}}).encode()
    r = json.loads(urllib.request.urlopen(urllib.request.Request(URL, body, {"Content-Type": "application/json"}), timeout=300).read())
    if "error" in r:
        raise RuntimeError(json.dumps(r["error"].get("data", r["error"]).get("message", r["error"]))[:600])
    return r.get("result")


uid = rpc("common", "login", DB, USER, PWD)
X = lambda model, method, *a, **kw: rpc("object", "execute_kw", DB, uid, PWD, model, method, list(a), kw)
seed = json.load(open(sys.argv[1], encoding="utf-8"))

# packagings are hidden behind a setting group; turn it on so they show in the UI
try:
    s = X("res.config.settings", "create", {"group_stock_packaging": True})
    X("res.config.settings", "execute", [s])
except RuntimeError as e:
    print("packaging setting:", e)

tax = {}
for t in X("account.tax", "search_read", [["type_tax_use", "=", "sale"], ["amount_type", "=", "percent"]], fields=["amount"]):
    tax.setdefault(round(t["amount"]), t["id"])
ptax = {}
for t in X("account.tax", "search_read", [["type_tax_use", "=", "purchase"], ["amount_type", "=", "percent"]], fields=["amount"]):
    ptax.setdefault(round(t["amount"]), t["id"])
print("sale taxes:", sorted(tax), "purchase taxes:", sorted(ptax))

parent = X("product.category", "search", [["name", "=", "All"]], limit=1)[0]
categ = {}
for b in sorted({p["brand"] for p in seed["products"] if p["brand"]}):
    ids = X("product.category", "search", [["name", "=", b], ["parent_id", "=", parent]], limit=1)
    categ[b] = ids[0] if ids else X("product.category", "create", {"name": b, "parent_id": parent})

prod = {}
for p in seed["products"]:
    found = X("product.product", "search", [["default_code", "=", p["sku"] or p["ref"][:20]]], limit=1)
    if found:
        prod[p["ref"]] = found[0]
        continue
    vals = {"name": p["name"], "default_code": p["sku"] or p["ref"][:20], "type": "consu", "is_storable": True,
            "list_price": p["price"], "standard_price": p["cost"], "categ_id": categ.get(p["brand"], parent),
            "sale_ok": True, "purchase_ok": True}
    if p["barcode"]:
        vals["barcode"] = p["barcode"]
    if round(p["vat"]) in tax:
        vals["taxes_id"] = [(6, 0, [tax[round(p["vat"])]])]
    if round(p["vat"]) in ptax:
        vals["supplier_taxes_id"] = [(6, 0, [ptax[round(p["vat"])]])]
    try:
        tmpl = X("product.template", "create", vals)
    except RuntimeError as e:
        if "barcode" in str(e).lower():
            vals.pop("barcode")
            tmpl = X("product.template", "create", vals)
        else:
            raise
    pid = X("product.product", "search", [["product_tmpl_id", "=", tmpl]], limit=1)[0]
    prod[p["ref"]] = pid
    for k in p["packs"]:
        X("product.packaging", "create", {"name": k["name"], "product_id": pid, "qty": k["factor"]})
print("products:", X("product.product", "search_count", [["sale_ok", "=", True]]))

partner = {}
for pt in seed["partners"]:
    found = X("res.partner", "search", [["name", "=", pt["name"]]], limit=1)
    if found:
        partner[pt["ref"]] = found[0]
        continue
    rank = {"customer_rank": 1} if pt["kind"] == "customer" else {"supplier_rank": 1}
    partner[pt["ref"]] = X("res.partner", "create", {"name": pt["name"], "phone": pt["phone"], "is_company": True, **rank})
print("customers:", X("res.partner", "search_count", [["customer_rank", ">", 0]]), "suppliers:", X("res.partner", "search_count", [["supplier_rank", ">", 0]]))

for c in seed["supplier_costs"]:
    pid, sid = prod.get(c["product"]), partner.get(c["supplier"])
    if not pid or not sid:
        continue
    tmpl = X("product.product", "read", [pid], fields=["product_tmpl_id"])[0]["product_tmpl_id"][0]
    if not X("product.supplierinfo", "search", [["product_tmpl_id", "=", tmpl], ["partner_id", "=", sid]], limit=1):
        X("product.supplierinfo", "create", {"product_tmpl_id": tmpl, "partner_id": sid, "price": c["cost"], "min_qty": 0})

loc = X("stock.warehouse", "search_read", [], fields=["lot_stock_id"], limit=1)[0]["lot_stock_id"][0]
for p in seed["products"]:
    if p["stock"] <= 0:
        continue
    pid = prod[p["ref"]]
    if X("stock.quant", "search", [["product_id", "=", pid], ["location_id", "=", loc], ["quantity", ">", 0]], limit=1):
        continue
    q = X("stock.quant", "create", {"product_id": pid, "location_id": loc, "inventory_quantity": p["stock"]})
    X("stock.quant", "action_apply_inventory", [q])
print("products in stock:", X("product.product", "search_count", [["qty_available", ">", 0]]))
