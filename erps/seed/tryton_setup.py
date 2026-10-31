"""Configure the Tryton 7.0 test database 'orderdesk' and load the shared distributor seed, through
Tryton's JSON-RPC API (admin / admin, local only). Run it again safely: what exists is kept.

    python tryton_setup.py seed.json

company "Elixir Distribution (test)" in DZD, the minimal chart of accounts, fiscal years 2026-2027,
one 19 % VAT used on sales and purchases, SO/ PO/ IN/ OUT/ number prefixes, an accounting category per VAT rate, brands as product
categories under "Marques", packs as units of measure ("Carton x6" = 6 units) set as the purchase
unit, parties in the "Clients" / "Fournisseurs" categories, supplier prices (per carton, Tryton's
purchase unit) and the initial stock booked by an inventory in the warehouse's storage zone.
"""
import base64, datetime, json, sys, urllib.request
from decimal import Decimal

URL, DB, USER, PWD = "http://127.0.0.1:8000", "orderdesk", "admin", "admin"


def enc(v):
    if isinstance(v, Decimal):
        return {"__class__": "Decimal", "decimal": str(v)}
    if isinstance(v, datetime.date):
        return {"__class__": "date", "year": v.year, "month": v.month, "day": v.day}
    raise TypeError(type(v))


def dec(d):
    if d.get("__class__") == "Decimal":
        return Decimal(d["decimal"])
    if d.get("__class__") == "date":
        return datetime.date(d["year"], d["month"], d["day"])
    return d


AUTH = None


def rpc(method, *params):
    h = {"Content-Type": "application/json"}
    if AUTH:
        h["Authorization"] = "Session " + base64.b64encode(AUTH.encode()).decode()
    body = json.dumps({"method": method, "params": list(params), "id": 1}, default=enc).encode()
    r = json.loads(urllib.request.urlopen(urllib.request.Request(f"{URL}/{DB}/", body, h), timeout=600).read(), object_hook=dec)
    if "error" in r:
        raise RuntimeError(f"{method}: {json.dumps(r['error'], default=str)[:800]}")
    return r["result"]


uid, session = rpc("common.db.login", USER, {"password": PWD}, "en")
AUTH = f"{USER}:{uid}:{session}"
CTX = {}


def M(model, method, *args):
    return rpc(f"model.{model}.{method}", *args, CTX)


def search(model, domain, limit=None, order=None):
    return M(model, "search", domain, 0, limit, order)


def read(model, domain, fields, limit=None):
    return M(model, "search_read", domain, 0, limit, None, fields)


def one(model, domain):
    ids = search(model, domain, 1)
    return ids[0] if ids else None


def create(model, vals):
    return M(model, "create", [vals])[0]


def get_or_create(model, domain, vals):
    return one(model, domain) or create(model, vals)


seed = json.load(open(sys.argv[1] if len(sys.argv) > 1 else "seed.json", encoding="utf-8"))

# ---- company, currency, user ------------------------------------------------------------------
dzd = get_or_create("currency.currency", [("code", "=", "DZD")],
                    {"name": "Algerian Dinar", "code": "DZD", "symbol": "DA", "rounding": Decimal("0.01"), "digits": 2})
company = one("company.company", [])
if not company:
    cparty = get_or_create("party.party", [("name", "=", "Elixir Distribution (test)")],
                           {"name": "Elixir Distribution (test)", "addresses": [["create", [{"city": "Alger"}]]]})
    company = create("company.company", {"party": cparty, "currency": dzd, "timezone": "Africa/Algiers"})
M("res.user", "write", [uid], {"companies": [["add", [company]]], "company": company})
CTX = {"company": company, "companies": [company], "language": "en"}
print("company", company, "currency DZD", dzd)

# ---- chart of accounts (wizard), fiscal years -------------------------------------------------
if not search("account.account", [("company", "=", company)], 1):
    root_tmpl = one("account.account.template", [("parent", "=", None), ("name", "=", "Minimal Account Chart")])
    sid, _, _ = rpc("wizard.account.create_chart.create", CTX)
    w = lambda data, state: rpc("wizard.account.create_chart.execute", sid, data, state, CTX)
    w({}, "start")
    w({"start": {}}, "account")
    w({"account": {"company": company, "account_template": root_tmpl}}, "create_account")
    rec = one("account.account", [("company", "=", company), ("type.receivable", "=", True)])
    pay = one("account.account", [("company", "=", company), ("type.payable", "=", True)])
    w({"properties": {"company": company, "account_receivable": rec, "account_payable": pay}}, "create_properties")
    rpc("wizard.account.create_chart.delete", sid, CTX)
accounts = {a["name"]: a["id"] for a in read("account.account", [("company", "=", company)], ["name"])}
print("accounts:", sorted(accounts))

move_seq_type = one("ir.sequence.type", [("name", "=", "Account Move")])
for year in (2026, 2027):
    if one("account.fiscalyear", [("company", "=", company), ("name", "=", str(year))]):
        continue
    seq = create("ir.sequence", {"name": f"Account Move {year}", "sequence_type": move_seq_type, "company": company})
    fy = create("account.fiscalyear", {"name": str(year), "company": company, "post_move_sequence": seq,
                                       "start_date": datetime.date(year, 1, 1), "end_date": datetime.date(year, 12, 31)})
    M("account.fiscalyear", "create_period", [fy])
print("fiscal years:", [f["name"] for f in read("account.fiscalyear", [], ["name"])])

# readable document numbers (Tryton's default sequences have no prefix)
for model, field, prefix in (("sale.configuration", "sale_sequence", "SO/"), ("purchase.configuration", "purchase_sequence", "PO/"),
                             ("stock.configuration", "shipment_in_sequence", "IN/"), ("stock.configuration", "shipment_out_sequence", "OUT/")):
    seq = M(model, "read", [1], [field])[0][field]
    if seq and not M("ir.sequence", "read", [seq], ["prefix"])[0]["prefix"]:
        M("ir.sequence", "write", [seq], {"prefix": prefix})

# ---- VAT and accounting categories -------------------------------------------------------------
tax_account = accounts.get("Main Tax")
taxes, acc_categ = {}, {}
for rate in sorted({round(p["vat"]) for p in seed["products"]}):
    name = f"TVA {rate} %"
    taxes[rate] = get_or_create("account.tax", [("name", "=", name), ("company", "=", company)],
                                {"name": name, "description": name, "type": "percentage", "rate": Decimal(rate) / 100,
                                 "company": company, "invoice_account": tax_account, "credit_note_account": tax_account})
    cname = f"Marchandises TVA {rate} %"
    acc_categ[rate] = get_or_create("product.category", [("name", "=", cname), ("accounting", "=", True)],
                                    {"name": cname, "accounting": True, "account_revenue": accounts["Main Revenue"],
                                     "account_expense": accounts["Main Expense"],
                                     "customer_taxes": [["add", [taxes[rate]]]], "supplier_taxes": [["add", [taxes[rate]]]]})
print("taxes:", taxes)

# ---- units: packs are units of the "Units" category --------------------------------------------
units_categ = one("product.uom.category", [("name", "=", "Units")])
unit = one("product.uom", [("category", "=", units_categ), ("factor", "=", 1)])
uoms = {}
for k in sorted({(k["name"], k["factor"]) for p in seed["products"] for k in p["packs"]}, key=lambda x: x[1]):
    uoms[k[0]] = get_or_create("product.uom", [("name", "=", k[0])],
                               {"name": k[0], "symbol": "ct" + str(int(k[1])), "category": units_categ,
                                "factor": float(k[1]), "rate": round(1 / float(k[1]), 12), "rounding": 1, "digits": 0})
print("pack units:", uoms)

# ---- brands -----------------------------------------------------------------------------------
brands_root = get_or_create("product.category", [("name", "=", "Marques"), ("parent", "=", None), ("accounting", "=", False)], {"name": "Marques"})
brand = {}
for b in sorted({p["brand"] for p in seed["products"] if p["brand"]}):
    brand[b] = get_or_create("product.category", [("name", "=", b), ("parent", "=", brands_root)], {"name": b, "parent": brands_root})

# ---- products ---------------------------------------------------------------------------------
prod = {}
existing = {r["code"]: r["id"] for r in read("product.product", [], ["code"])}
for p in seed["products"]:
    code = p["sku"] or p["ref"][:20]
    if code in existing:
        prod[p["ref"]] = existing[code]
        continue
    pack = p["packs"][0] if p["packs"] else None
    tmpl = {"name": p["name"], "code": code, "type": "goods", "default_uom": unit, "sale_uom": unit,
            "purchase_uom": uoms[pack["name"]] if pack else unit, "list_price": Decimal(str(p["price"])),
            "salable": True, "purchasable": True, "account_category": acc_categ[round(p["vat"])]}
    if p["brand"]:
        tmpl["categories"] = [["add", [brand[p["brand"]]]]]
    variant = {"cost_price": Decimal(str(p["cost"]))}
    if p["barcode"]:
        variant["identifiers"] = [["create", [{"type": "ean", "code": p["barcode"]}]]]
    tmpl["products"] = [["create", [variant]]]
    try:
        t = create("product.template", tmpl)
    except RuntimeError as e:
        if "identifiers" in variant:  # not a valid EAN: keep it as a plain code
            variant["identifiers"] = [["create", [{"code": p["barcode"]}]]]
            t = create("product.template", tmpl)
        else:
            raise
    prod[p["ref"]] = search("product.product", [("template", "=", t)], 1)[0]
print("products:", len(search("product.product", [("salable", "=", True)])))

# ---- parties ----------------------------------------------------------------------------------
pcateg = {"customer": get_or_create("party.category", [("name", "=", "Clients")], {"name": "Clients"}),
          "supplier": get_or_create("party.category", [("name", "=", "Fournisseurs")], {"name": "Fournisseurs"})}
partner = {}
for pt in seed["partners"]:
    found = one("party.party", [("name", "=", pt["name"])])
    if not found:
        vals = {"name": pt["name"], "categories": [["add", [pcateg[pt["kind"]]]]], "addresses": [["create", [{"city": "Alger"}]]]}
        phone = pt["phone"].replace(" ", "")
        if phone:
            if phone.startswith("0"):
                phone = "+213" + phone[1:]
            vals["contact_mechanisms"] = [["create", [{"type": "mobile" if phone[4:5] in "567" else "phone", "value": phone}]]]
        found = create("party.party", vals)
    partner[pt["ref"]] = found
print("customers:", len(search("party.party", [("categories", "=", pcateg["customer"])])),
      "suppliers:", len(search("party.party", [("categories", "=", pcateg["supplier"])])))

# ---- supplier prices (Tryton keeps them in the purchase unit: per carton) ------------------------
factor = {p["ref"]: (p["packs"][0]["factor"] if p["packs"] else 1) for p in seed["products"]}
tmpl_of = {r["id"]: r["template"] for r in read("product.product", [("id", "in", list(prod.values()))], ["template"])}
for c in seed["supplier_costs"]:
    pid, sid = prod.get(c["product"]), partner.get(c["supplier"])
    if not pid or not sid or one("purchase.product_supplier", [("product", "=", pid), ("party", "=", sid)]):
        continue
    create("purchase.product_supplier", {"template": tmpl_of[pid], "product": pid, "party": sid, "currency": dzd, "company": company,
                                         "prices": [["create", [{"quantity": 0, "unit_price": (Decimal(str(c["cost"])) * factor[c["product"]]).quantize(Decimal("0.0001"))}]]]})
print("supplier prices:", len(search("purchase.product_supplier", [])))

# ---- initial stock ----------------------------------------------------------------------------
wh = read("stock.location", [("type", "=", "warehouse")], ["storage_location"], 1)[0]
with_stock = {r["id"] for r in rpc("model.product.product.search_read", [("id", "in", list(prod.values()))], 0, None, None, ["quantity"],
                                   {**CTX, "locations": [wh["id"]]}) if r["quantity"] > 0}
lines = [{"product": prod[p["ref"]], "quantity": float(p["stock"])} for p in seed["products"] if p["stock"] > 0 and prod[p["ref"]] not in with_stock]
if lines:
    inv = create("stock.inventory", {"location": wh["storage_location"], "company": company, "lines": [["create", lines]]})
    M("stock.inventory", "confirm", [inv])
stocked = rpc("model.product.product.search_read", [("id", "in", list(prod.values()))], 0, None, None, ["quantity"], {**CTX, "locations": [wh["id"]]})
print("products in stock:", sum(1 for r in stocked if r["quantity"] > 0))
