"""ERPNext test site: finish the setup wizard, then load the shared distributor seed.
Local test instance only (Administrator / admin)."""
import json, sys, time, urllib.parse, urllib.request, http.cookiejar

BASE = "http://127.0.0.1:8090"
jar = http.cookiejar.CookieJar()
op = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))


def call(method, path, data=None, form=False):
    body, headers = None, {"Accept": "application/json"}
    if data is not None:
        if form:
            body = urllib.parse.urlencode(data).encode()
            headers["Content-Type"] = "application/x-www-form-urlencoded"
        else:
            body = json.dumps(data).encode()
            headers["Content-Type"] = "application/json"
    req = urllib.request.Request(BASE + path, body, headers, method=method)
    try:
        return json.loads(op.open(req, timeout=300).read() or b"{}")
    except urllib.error.HTTPError as e:
        raise RuntimeError(f"{method} {path}: {e.code} {e.read()[:600]}")


def get(doctype, filters=None, fields=("name",), limit=0):
    q = {"fields": json.dumps(list(fields)), "limit_page_length": limit}
    if filters:
        q["filters"] = json.dumps(filters)
    return call("GET", f"/api/resource/{urllib.parse.quote(doctype)}?" + urllib.parse.urlencode(q))["data"]


def insert(doc):
    return call("POST", f"/api/resource/{urllib.parse.quote(doc['doctype'])}", doc)["data"]


call("POST", "/api/method/login", {"usr": "Administrator", "pwd": "admin"}, form=True)
if not call("GET", "/api/method/frappe.client.get_value?" + urllib.parse.urlencode(
        {"doctype": "System Settings", "fieldname": "setup_complete"}))["message"]["setup_complete"]:
    args = {"language": "English", "country": "Algeria", "timezone": "Africa/Algiers", "currency": "DZD",
            "full_name": "Order Desk Test", "email": "admin@example.com", "password": "admin",
            "company_name": "Elixir Distribution Test", "company_abbr": "EDT", "chart_of_accounts": "Standard",
            "fy_start_date": "2026-01-01", "fy_end_date": "2026-12-31", "setup_demo": 0}
    r = call("POST", "/api/method/frappe.desk.page.setup_wizard.setup_wizard.setup_complete", {"args": json.dumps(args)}, form=True)
    print("setup:", str(r)[:200])
    for _ in range(60):
        v = call("GET", "/api/method/frappe.client.get_value?" + urllib.parse.urlencode(
            {"doctype": "System Settings", "fieldname": "setup_complete"}))["message"]["setup_complete"]
        if v:
            break
        time.sleep(5)
    call("POST", "/api/method/login", {"usr": "Administrator", "pwd": "admin"}, form=True)
print("setup complete; company:", [c["name"] for c in get("Company")])

company = get("Company")[0]["name"]
abbr = call("GET", "/api/method/frappe.client.get_value?" + urllib.parse.urlencode(
    {"doctype": "Company", "filters": json.dumps({"name": company}), "fieldname": "abbr"}))["message"]["abbr"]
wh = f"Stores - {abbr}"
seed = json.load(open(sys.argv[1], encoding="utf-8"))
exists = lambda dt, name: bool(get(dt, [["name", "=", name]]))

for b in sorted({p["brand"] for p in seed["products"] if p["brand"]}):
    if not exists("Brand", b):
        insert({"doctype": "Brand", "brand": b})
for u in sorted({k["name"] for p in seed["products"] for k in p["packs"]}):
    if not exists("UOM", u):
        insert({"doctype": "UOM", "uom_name": u, "must_be_whole_number": 1})
code = {}
for p in seed["products"]:
    c = p["sku"] or p["ref"][:20]
    code[p["ref"]] = c
    if exists("Item", c):
        continue
    doc = {"doctype": "Item", "item_code": c, "item_name": p["name"], "item_group": "Products", "stock_uom": "Nos",
           "is_stock_item": 1, "include_item_in_manufacturing": 0, "brand": p["brand"] or None,
           "standard_rate": p["price"], "valuation_rate": p["cost"],
           "uoms": [{"uom": "Nos", "conversion_factor": 1}] + [{"uom": k["name"], "conversion_factor": k["factor"]} for k in p["packs"]]}
    if p["barcode"]:
        doc["barcodes"] = [{"barcode": p["barcode"]}]
    try:
        insert(doc)
    except RuntimeError as e:
        if "barcode" in str(e).lower():
            doc.pop("barcodes", None)
            insert(doc)
        else:
            raise
print("items:", len(get("Item")))

leaf = lambda dt: get(dt, [["is_group", "=", 0]])[0]["name"]
cgroup, sgroup, territory = leaf("Customer Group"), leaf("Supplier Group"), leaf("Territory")
party = {}
for pt in seed["partners"]:
    dt = "Customer" if pt["kind"] == "customer" else "Supplier"
    rows = get(dt, [[f"{dt.lower()}_name", "=", pt["name"]]])
    if rows:
        party[pt["ref"]] = rows[0]["name"]
        continue
    doc = {"doctype": dt, f"{dt.lower()}_name": pt["name"], "mobile_no": pt["phone"]}
    if dt == "Customer":
        doc.update({"customer_type": "Company", "customer_group": cgroup, "territory": territory})
    else:
        doc.update({"supplier_type": "Company", "supplier_group": sgroup})
    party[pt["ref"]] = insert(doc)["name"]
print("customers:", len(get("Customer")), "suppliers:", len(get("Supplier")))

for c in seed["supplier_costs"]:
    item, sup = code.get(c["product"]), party.get(c["supplier"])
    if item and sup and not get("Item Price", [["item_code", "=", item], ["price_list", "=", "Standard Buying"], ["supplier", "=", sup]]):
        insert({"doctype": "Item Price", "item_code": item, "price_list": "Standard Buying", "supplier": sup, "price_list_rate": c["cost"]})

if not get("Stock Reconciliation", [["docstatus", "=", 1]]):
    items = [{"item_code": code[p["ref"]], "warehouse": wh, "qty": p["stock"], "valuation_rate": p["cost"] or 1}
             for p in seed["products"] if p["stock"] > 0]
    sr = insert({"doctype": "Stock Reconciliation", "company": company, "purpose": "Opening Stock",
                 "expense_account": f"Temporary Opening - {abbr}", "items": items})
    sr["docstatus"] = 1
    call("PUT", f"/api/resource/Stock%20Reconciliation/{urllib.parse.quote(sr['name'])}", {"docstatus": 1})
print("stock bins:", len(get("Bin", [["actual_qty", ">", 0]])))
