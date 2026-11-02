"""Screenshots of the documents the demo run booked in Odoo (docs/demo/erp-<scene>.jpg), for the demo page.

    python tools/demo_shots.py          (after tools/record_demo.py; needs Playwright with Chrome)
"""
import json
import urllib.request
from pathlib import Path

from playwright.sync_api import sync_playwright

REPO = Path(__file__).resolve().parent.parent
OUT = REPO / "docs" / "demo"
URL, DB, USER, PWD = "http://127.0.0.1:8069", "orderdesk", "admin", "admin"


def rpc(model, method, *args):
    body = json.dumps({"jsonrpc": "2.0", "method": "call", "params": {"service": "object", "method": "execute_kw",
                                                                    "args": [DB, 2, PWD, model, method, list(args)]}}).encode()
    r = json.loads(urllib.request.urlopen(urllib.request.Request(URL + "/jsonrpc", body, {"Content-Type": "application/json"})).read())
    return r["result"]


data = json.loads((OUT / "data.json").read_text(encoding="utf-8"))
with sync_playwright() as pw:
    b = pw.chromium.launch(channel="chrome", headless=True)
    page = b.new_page(viewport={"width": 1440, "height": 900})
    page.goto(URL + "/web/login")
    page.fill("#login", USER)
    page.fill("#password", PWD)
    page.click("button[type=submit]")
    page.wait_for_timeout(3000)
    for s in data["scenes"]:
        number = (s["inside"]["drafts"] or [{}])[-1].get("erp_order_number") or ""
        if not number:
            continue
        if s["id"] == "invoice":
            ids = rpc("purchase.order", "search", [["name", "=", number.split(" · ")[0]]])
            link = f"{URL}/odoo/purchase/{ids[0]}"
        else:
            ids = rpc("sale.order", "search", [["name", "=", number]])
            link = f"{URL}/odoo/sales/{ids[0]}"
        page.goto(link)
        page.wait_for_timeout(5000)
        page.screenshot(path=str(OUT / f"erp-{s['id']}.jpg"), type="jpeg", quality=80)
        s["erp_shot"] = f"erp-{s['id']}.jpg"
        print(s["id"], number, link)
    b.close()
(OUT / "data.json").write_text(json.dumps(data, ensure_ascii=False, indent=1), encoding="utf-8")
