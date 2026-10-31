"""InvenTree test instance: currency and parameters, then the shared distributor seed.
Local test instance only (admin / admin). Idempotent: run it again and it only fills what is missing.

    python seed/inventree_setup.py seed/seed.json

How the seed lands in InvenTree (and how the Order Desk adapter reads it back):
  product        Part (IPN = sku), salable + purchaseable, in the category "Boissons & épicerie"
  brand          a manufacturer Company + a ManufacturerPart (MPN = sku)
  VAT, EAN, size Part parameters "TVA" (%), "EAN", "Contenance"; the EAN is also linked as the part's barcode
  sales price    a sale price break at quantity 1 (DZD, HT)
  pack           a SupplierPart per pack ("Carton x6", pack quantity 6) at the supplier's pack price
  stock          one StockItem per part in the location "Magasin principal"
  partners       Companies: is_customer or is_supplier
"""
import base64, json, re, sys, urllib.parse, urllib.request

BASE = "http://127.0.0.1:8083"
CUR = "DZD"


def call(method, path, data=None, auth=None):
    headers = {"Accept": "application/json"}
    if auth:
        headers["Authorization"] = auth
    else:
        headers["Authorization"] = "Token " + TOKEN
    body = None
    if data is not None:
        body = json.dumps(data).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(BASE + path, body, headers, method=method)
    try:
        raw = urllib.request.urlopen(req, timeout=120).read()
        return json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        raise RuntimeError(f"{method} {path}: {e.code} {e.read()[:600]!r}")


def get(path, **q):
    out = call("GET", path + ("?" + urllib.parse.urlencode(q) if q else ""))
    return out["results"] if isinstance(out, dict) and "results" in out else out


def post(path, data):
    return call("POST", path, data)


basic = "Basic " + base64.b64encode(b"admin:admin").decode()
TOKEN = call("GET", "/api/user/me/token/?name=seed", auth=basic)["token"]  # re-issued on every run

# ---- settings: Algerian dinar, instance name ----------------------------------------------------
codes = call("GET", "/api/settings/global/CURRENCY_CODES/")["value"]
if CUR not in codes.split(","):
    call("PATCH", "/api/settings/global/CURRENCY_CODES/", {"value": codes + "," + CUR})
for key, value in [("INVENTREE_DEFAULT_CURRENCY", CUR), ("INVENTREE_INSTANCE", "Elixir Distribution Test")]:
    call("PATCH", f"/api/settings/global/{key}/", {"value": value})
print("currency:", call("GET", "/api/settings/global/INVENTREE_DEFAULT_CURRENCY/")["value"])


def one(path, data, **q):
    """The first object matching q, else a new one from data."""
    rows = get(path, **q)
    return rows[0] if rows else post(path, data)


category = one("/api/part/category/", {"name": "Boissons & épicerie", "description": "Catalogue distributeur (seed)"}, name="Boissons & épicerie")
location = one("/api/stock/location/", {"name": "Magasin principal", "description": "Dépôt (seed)"}, name="Magasin principal")
templates = {}
for name, units, desc in [("TVA", "%", "Taux de TVA"), ("EAN", "", "Code-barres EAN-13"), ("Contenance", "", "Taille / contenance")]:
    rows = [t for t in get("/api/parameter/template/", name=name) if t["name"] == name]
    templates[name] = rows[0]["pk"] if rows else post("/api/parameter/template/", {"name": name, "units": units, "description": desc, "model_type": "part.part"})["pk"]
print("category", category["pk"], "location", location["pk"], "templates", templates)

seed = json.load(open(sys.argv[1], encoding="utf-8"))

# ---- companies ----------------------------------------------------------------------------------
companies = {c["name"]: c for c in get("/api/company/")}


def company(name, **flags):
    if name in companies:
        return companies[name]["pk"]
    c = post("/api/company/", {"name": name, "currency": CUR, "is_customer": False, "is_supplier": False, "is_manufacturer": False, **flags})
    companies[name] = c
    return c["pk"]


brand = {b: company(b, is_manufacturer=True, description="Marque") for b in sorted({p["brand"] for p in seed["products"] if p["brand"]})}
party = {}
for pt in seed["partners"]:
    flags = {"is_customer": True} if pt["kind"] == "customer" else {"is_supplier": True}
    party[pt["ref"]] = company(pt["name"], phone=pt["phone"], **flags)
print("customers:", len(get("/api/company/", is_customer="true")), "suppliers:", len(get("/api/company/", is_supplier="true")),
      "manufacturers:", len(get("/api/company/", is_manufacturer="true")))

# ---- parts --------------------------------------------------------------------------------------
existing = {p["IPN"]: p["pk"] for p in get("/api/part/", category=category["pk"])}
size_re = re.compile(r"(\d+(?:[.,]\d+)?\s?(?:cl|ml|l|kg|g)\b)", re.I)
part = {}
for p in seed["products"]:
    ipn = p["sku"] or p["ref"][:20]
    if ipn in existing:
        part[p["ref"]] = existing[ipn]
        continue
    pk = post("/api/part/", {"name": p["name"], "IPN": ipn, "description": p["name"], "category": category["pk"], "salable": True,
                             "purchaseable": True, "component": False, "active": True, "default_location": location["pk"]})["pk"]
    part[p["ref"]] = pk
    if p["brand"]:
        post("/api/company/part/manufacturer/", {"part": pk, "manufacturer": brand[p["brand"]], "MPN": ipn})
    params = {"TVA": f"{p['vat']:g}", "EAN": p["barcode"]}
    m = size_re.search(p["name"])
    if m:
        params["Contenance"] = m.group(1)
    for name, value in params.items():
        if value:
            post("/api/parameter/", {"template": templates[name], "model_type": "part.part", "model_id": pk, "data": value})
    post("/api/part/sale-price/", {"part": pk, "quantity": 1, "price": p["price"], "price_currency": CUR})
    if p["barcode"]:
        try:
            post("/api/barcode/link/", {"barcode": p["barcode"], "part": pk})
        except RuntimeError as e:
            print("barcode", ipn, str(e)[:120])
    if p["stock"] > 0:
        post("/api/stock/", {"part": pk, "location": location["pk"], "quantity": p["stock"], "purchase_price": p["cost"] or None,
                             "purchase_price_currency": CUR, "notes": "Stock initial (seed)"})
print("parts:", len(get("/api/part/", category=category["pk"])))

# ---- supplier parts: one per pack, at the supplier's price per pack ------------------------------
products = {p["ref"]: p for p in seed["products"]}
have = {(s["supplier"], s["part"], s["SKU"]) for s in get("/api/company/part/")}
for c in seed["supplier_costs"]:
    p, sup = products.get(c["product"]), party.get(c["supplier"])
    if not p or not sup:
        continue
    for k in p["packs"] or [{"name": "Unité", "factor": 1}]:
        sku = f"{p['sku']}-X{k['factor']}"
        if (sup, part[p["ref"]], sku) in have:
            continue
        sp = post("/api/company/part/", {"part": part[p["ref"]], "supplier": sup, "SKU": sku, "packaging": k["name"],
                                         "pack_quantity": str(k["factor"]), "description": p["name"]})
        post("/api/company/price-break/", {"part": sp["pk"], "quantity": 1, "price": round(c["cost"] * k["factor"], 2), "price_currency": CUR})
print("supplier parts:", len(get("/api/company/part/")))
print("stock items:", len(get("/api/stock/", location=location["pk"])))
