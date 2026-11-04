"""Read-only Odoo user for the judges, and a strong admin password, on the demo Odoo (through a port-forward).

judge / <JUDGE_PASSWORD>: an internal user in one custom group with read-only access to sales, purchases,
stock and products, the menus to reach them, and rules refusing any write on products and contacts.
Then the script checks: judge can read orders, and every write is refused.

    python3 odoo_judge.py <url> <admin password now> <new admin password> <judge password>
"""
import json, sys, urllib.request

URL, ADMIN_OLD, ADMIN_NEW, JUDGE_PWD = sys.argv[1] + "/jsonrpc", sys.argv[2], sys.argv[3], sys.argv[4]
DB = "orderdesk"


def rpc(service, method, *args):
    body = json.dumps({"jsonrpc": "2.0", "method": "call", "params": {"service": service, "method": method, "args": list(args)}}).encode()
    r = json.loads(urllib.request.urlopen(urllib.request.Request(URL, body, {"Content-Type": "application/json"}), timeout=300).read())
    if "error" in r:
        raise RuntimeError(json.dumps(r["error"].get("data", {}).get("message", r["error"]))[:300])
    return r.get("result")


uid = rpc("common", "login", DB, "admin", ADMIN_OLD) or rpc("common", "login", DB, "admin", ADMIN_NEW)
pwd = ADMIN_OLD if rpc("common", "login", DB, "admin", ADMIN_OLD) else ADMIN_NEW
X = lambda model, method, *a, **kw: rpc("object", "execute_kw", DB, uid, pwd, model, method, list(a), kw)
ref = lambda xmlid: (lambda m, n: (X("ir.model.data", "search_read", [["module", "=", m], ["name", "=", n]], fields=["res_id"], limit=1) or [{"res_id": False}])[0]["res_id"])(*xmlid.split("."))

group = (X("res.groups", "search", [["name", "=", "Demo read-only (judges)"]], limit=1) or [None])[0]
if not group:
    group = X("res.groups", "create", {"name": "Demo read-only (judges)", "implied_ids": [(6, 0, [ref("base.group_user")])]})
READ = ["sale.order", "sale.order.line", "purchase.order", "purchase.order.line", "stock.quant", "stock.picking", "stock.move",
        "stock.move.line", "stock.location", "stock.warehouse", "stock.picking.type", "stock.route", "stock.lot", "product.packaging",
        "account.tax", "account.move", "account.move.line", "account.payment.term", "account.fiscal.position", "crm.team",
        "product.pricelist", "product.pricelist.item", "product.supplierinfo", "uom.uom", "res.currency", "sale.order.template",
        "stock.warehouse.orderpoint", "account.analytic.account", "account.analytic.plan"]
for m in READ:
    mid = X("ir.model", "search", [["model", "=", m]], limit=1)
    if mid and not X("ir.model.access", "search", [["model_id", "=", mid[0]], ["group_id", "=", group]], limit=1):
        X("ir.model.access", "create", {"name": f"demo read {m}", "model_id": mid[0], "group_id": group,
                                        "perm_read": True, "perm_write": False, "perm_create": False, "perm_unlink": False})
# products and contacts are writable by every internal user: refuse writes for this group
for m in ["res.partner", "product.template", "product.product"]:
    mid = X("ir.model", "search", [["model", "=", m]], limit=1)[0]
    if not X("ir.rule", "search", [["model_id", "=", mid], ["groups", "in", [group]]], limit=1):
        X("ir.rule", "create", {"name": f"demo no write {m}", "model_id": mid, "groups": [(6, 0, [group])], "domain_force": "[(0, '=', 1)]",
                                "perm_read": False, "perm_write": True, "perm_create": True, "perm_unlink": True})
for xid in ["sale.sale_menu_root", "sale.sale_order_menu", "sale.menu_sale_order", "sale.menu_sale_quotations", "sale.product_menu_catalog",
            "sale.menu_product_template_action", "stock.menu_stock_root", "stock.menu_stock_inventory_control", "stock.menu_product_variant_config_stock",
            "stock.menu_stock_warehouse_mgmt", "stock.menu_valuation", "stock.all_picking", "purchase.menu_purchase_root",
            "purchase.menu_procurement_management", "purchase.menu_purchase_form_action", "purchase.menu_purchase_rfq", "contacts.menu_contacts"]:
    mid = ref(xid)
    if mid:
        X("ir.ui.menu", "write", [mid], {"groups_id": [(4, group)]})

judge = (X("res.users", "search", [["login", "=", "judge"]], limit=1) or [None])[0]
vals = {"name": "Judge (read-only)", "login": "judge", "password": JUDGE_PWD, "groups_id": [(6, 0, [group])]}
if judge:
    X("res.users", "write", [judge], vals)
else:
    judge = X("res.users", "create", vals)

# the checks, as judge
juid = rpc("common", "login", DB, "judge", JUDGE_PWD)
J = lambda model, method, *a, **kw: rpc("object", "execute_kw", DB, juid, JUDGE_PWD, model, method, list(a), kw)
print("judge reads orders:", len(J("sale.order", "search_read", [], fields=["name", "partner_id", "amount_total"], limit=5)),
      "| products:", J("product.product", "search_count", []), "| stock lines:", J("stock.quant", "search_count", []))
for model, op in [("sale.order", "create"), ("product.product", "write"), ("res.partner", "write"), ("stock.quant", "create")]:
    try:
        if op == "create":
            J(model, "create", {"partner_id": 1} if model == "sale.order" else {"product_id": 1, "location_id": 1, "quantity": 1})
        else:
            ids = J(model, "search", [], limit=1)
            J(model, "write", ids, {"name": "hacked"})
        print("WRITE ALLOWED (bad):", model, op)
    except RuntimeError as e:
        print("refused as it should:", model, op)

# a strong admin password (Odoo is about to be reachable from the internet)
if pwd != ADMIN_NEW:
    X("res.users", "write", [uid], {"password": ADMIN_NEW})
print("admin password changed:", bool(rpc("common", "login", DB, "admin", ADMIN_NEW)))
