"""Create the Odoo test database 'orderdesk' (no demo data) with Sales, Purchase and Inventory."""
import json, urllib.request

URL = "http://127.0.0.1:8069/jsonrpc"


def rpc(service, method, *args):
    body = json.dumps({"jsonrpc": "2.0", "method": "call", "params": {"service": service, "method": method, "args": list(args)}}).encode()
    r = json.loads(urllib.request.urlopen(urllib.request.Request(URL, body, {"Content-Type": "application/json"}), timeout=900).read())
    if "error" in r:
        raise SystemExit(json.dumps(r["error"])[:800])
    return r.get("result")


if "orderdesk" not in rpc("db", "list"):
    rpc("db", "create_database", "admin", "orderdesk", False, "fr_FR", "admin", "admin", "DZ")
uid = rpc("common", "login", "orderdesk", "admin", "admin")
mods = ["sale_management", "purchase", "stock"]
ids = rpc("object", "execute_kw", "orderdesk", uid, "admin", "ir.module.module", "search", [[["name", "in", mods], ["state", "!=", "installed"]]])
if ids:
    rpc("object", "execute_kw", "orderdesk", uid, "admin", "ir.module.module", "button_immediate_install", [ids])
print("uid", uid, rpc("object", "execute_kw", "orderdesk", uid, "admin", "ir.module.module", "search_read",
                      [[["name", "in", mods]]], {"fields": ["name", "state"]}))
