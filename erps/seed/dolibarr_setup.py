"""Dolibarr test instance: get the admin API key, then load the shared distributor seed through the
REST API. Idempotent: rerunning it only adds what is missing. Local test instance only (admin / admin).

    python seed/dolibarr_setup.py seed/seed.json

Where the seed lands in Dolibarr:
  brand   product category "Marques" > <brand> (tags)
  size    the name ("2L", "110g"), plus volume / weight on the product card when the name has one
  pack    product extrafield `packs` ("Carton x6"; several separated by ", ")
  VAT     tva_tx 19 (Algeria's dictionary has 0 / 9 / 19)
  stock   one warehouse "Dépôt principal", opening stock as stock movements
  costs   supplier prices (product purchase prices) per supplier

The key is written to dolibarr/test.env (DOLIBARR_URL, DOLIBARR_KEY) for the adapter's live test."""
import json, os, re, sys, urllib.error, urllib.parse, urllib.request

URL = os.environ.get("DOLIBARR_URL", "http://127.0.0.1:8081")
API = URL + "/api/index.php"
KEY = ""


def call(method, path, data=None, ok404=False):
    body = json.dumps(data).encode() if data is not None else None
    headers = {"Accept": "application/json", "Content-Type": "application/json"}
    if KEY:
        headers["DOLAPIKEY"] = KEY
    req = urllib.request.Request(API + path, body, headers, method=method)
    try:
        raw = urllib.request.urlopen(req, timeout=120).read()
        return json.loads(raw) if raw.strip() else None
    except urllib.error.HTTPError as e:
        if e.code == 404 and ok404:
            return None
        raise RuntimeError(f"{method} {path}: {e.code} {e.read()[:600]!r}")


def q(**kw):
    return "?" + urllib.parse.urlencode(kw)



KEY = call("POST", "/login", {"login": "admin", "password": "admin"})["success"]["token"]
print("dolibarr", call("GET", "/status")["success"]["dolibarr_version"])
seed = json.load(open(sys.argv[1], encoding="utf-8"))

# pack extrafield on products
try:
    call("POST", "/setup/extrafields/product/packs", {"label": "Conditionnements", "type": "varchar", "size": "128", "pos": "10",
                                                     "list": "1", "alwayseditable": "1", "help": "Ex. Carton x6 ; plusieurs : Carton x6, Palette x480"})
    print("extrafield packs created")
except RuntimeError as e:
    if " 409 " not in str(e):
        raise

# warehouse
whs = call("GET", "/warehouses" + q(sqlfilters="(t.ref:=:'Dépôt principal')"), ok404=True)
wh = int(whs[0]["id"]) if whs else call("POST", "/warehouses", {"label": "Dépôt principal", "ref": "Dépôt principal", "statut": 1,
                                                                  "description": "Dépôt Order Desk (test)"})
print("warehouse", wh)

# brand categories under "Marques"
cats = call("GET", "/categories" + q(type="product", limit=1000), ok404=True) or []
root = next((int(c["id"]) for c in cats if c["label"] == "Marques" and int(c.get("fk_parent") or 0) == 0), 0)
if not root:
    root = call("POST", "/categories", {"label": "Marques", "type": "product", "description": "Marques des produits"})
brand_cat = {c["label"]: int(c["id"]) for c in cats if int(c.get("fk_parent") or 0) == root}
for b in sorted({p["brand"] for p in seed["products"] if p["brand"]}):
    if b not in brand_cat:
        brand_cat[b] = call("POST", "/categories", {"label": b, "type": "product", "fk_parent": root})
print("brands", len(brand_cat))


def size_fields(name):
    m = re.search(r"(\d+(?:[.,]\d+)?)\s*(l|cl|ml|kg|g)\b", name, re.I)
    if not m:
        return {}
    v, u = float(m.group(1).replace(",", ".")), m.group(2).lower()
    if u in ("l", "cl", "ml"):  # volume_units is a power of ten of m3: -3 = dm3 = litre
        return {"volume": round(v * {"l": 1, "cl": 0.01, "ml": 0.001}[u], 4), "volume_units": -3}
    return {"weight": v, "weight_units": 0} if u == "kg" else {"weight": v, "weight_units": -3}


# products
prod = {}
for p in seed["products"]:
    ref = p["sku"] or p["ref"][:20]
    found = call("GET", "/products/ref/" + urllib.parse.quote(ref), ok404=True)
    if found:
        pid = int(found["id"])
    else:
        doc = {"ref": ref, "label": p["name"], "type": 0, "status": 1, "status_buy": 1, "price": p["price"], "price_base_type": "HT",
               "tva_tx": p["vat"], "cost_price": p["cost"], "barcode": p["barcode"] or None,
               "array_options": {"options_packs": ", ".join(k["name"] for k in p["packs"])}, **size_fields(p["name"])}
        try:
            pid = call("POST", "/products", doc)
        except RuntimeError as e:
            if "barcode" not in str(e).lower():
                raise
            doc.pop("barcode")
            pid = call("POST", "/products", doc)
    prod[p["ref"]] = pid
    if p["brand"]:
        try:
            call("POST", f"/categories/{brand_cat[p['brand']]}/objects/product/{pid}")
        except RuntimeError as e:
            if "already" not in str(e).lower() and " 409 " not in str(e):
                raise
print("products", len(prod))

# customers and suppliers
party = {}
by_name = {t["name"]: int(t["id"]) for t in call("GET", "/thirdparties" + q(limit=1000), ok404=True) or []}
for pt in seed["partners"]:
    if pt["name"] in by_name:
        party[pt["ref"]] = by_name[pt["name"]]
        continue
    doc = {"name": pt["name"], "phone": pt["phone"], "status": 1, "country_code": "DZ"}
    if pt["kind"] == "customer":
        doc.update({"client": 1, "code_client": "-1"})
    else:
        doc.update({"fournisseur": 1, "code_fournisseur": "-1"})
    party[pt["ref"]] = call("POST", "/thirdparties", doc)
print("customers", len(call("GET", "/thirdparties" + q(mode=1, limit=500), ok404=True) or []),
      "suppliers", len(call("GET", "/thirdparties" + q(mode=4, limit=500), ok404=True) or []))

# supplier prices
vat = {p["ref"]: p["vat"] for p in seed["products"]}
sku = {p["ref"]: p["sku"] or p["ref"][:20] for p in seed["products"]}
known = {}
for c in seed["supplier_costs"]:
    pid, sid = prod.get(c["product"]), party.get(c["supplier"])
    if not pid or not sid:
        continue
    if pid not in known:
        known[pid] = {int(x.get("fourn_id") or 0) for x in (call("GET", f"/products/{pid}/purchase_prices", ok404=True) or [])}
    if sid in known[pid]:
        continue
    call("POST", f"/products/{pid}/purchase_prices", {"qty": 1, "buyprice": c["cost"], "price_base_type": "HT", "fourn_id": sid,
                                                      "availability": 0, "ref_fourn": sku[c["product"]], "tva_tx": vat[c["product"]]})
    known[pid].add(sid)
print("supplier prices", sum(len(v) for v in known.values()))

# opening stock
moved = 0
for p in seed["products"]:
    pid = prod[p["ref"]]
    if p["stock"] <= 0:
        continue
    cur = call("GET", f"/products/{pid}")
    if float(cur.get("stock_reel") or 0) != 0:
        continue
    call("POST", "/stockmovements", {"product_id": pid, "warehouse_id": wh, "qty": p["stock"], "label": "Stock initial",
                                     "movementcode": "INIT", "price": p["cost"]})
    moved += 1
print("opening stock movements", moved)

env = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "dolibarr", "test.env")
with open(env, "w", encoding="utf-8") as f:
    f.write(f"DOLIBARR_URL={URL}\nDOLIBARR_KEY={KEY}\n")
print("API key for admin written to", os.path.normpath(env))
