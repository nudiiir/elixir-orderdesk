"""Screenshots of one order going from Order Desk into each ERP, for the README.

    go build -o orderdesk.exe ./cmd/orderdesk          (or -o orderdesk on macOS / Linux)
    python tools/capture_erps.py odoo erpnext dolibarr inventree tryton

Starts a fresh desk per ERP (in desks/ under the system temp folder), connects it with the local Docker test
login, sends the same shop order through the panel's real pipeline, answers the question, confirms, then opens
the created order in the ERP's own screens. Writes docs/media/<erp>-1-question.jpg, -2-draft.jpg, -3-erp.jpg.
Needs Playwright with Chrome (pip install playwright) and the ERPs from erps/ running and seeded.
SHOP=<name part> picks the shop (default: a shop with no order history, so the question is asked);
KEYS=<file> passes a keys file to Order Desk (default: .env if present)."""
import json
import os
import tempfile
import shutil
import subprocess
import sys
import time
import urllib.request
from pathlib import Path

from playwright.sync_api import sync_playwright

REPO = Path(__file__).resolve().parent.parent
EXE = REPO / ("orderdesk.exe" if os.name == "nt" else "orderdesk")
KEYS = Path(os.environ.get("KEYS", REPO / ".env"))
OUT = REPO / "docs" / "media"
DESKS = Path(tempfile.gettempdir()) / "orderdesk-capture"
SHOP = os.environ.get("SHOP", "Orangers")
ORDER = "3 cartons selecto 2l\n2 ifri eau minerale 1.5\n4 rouiba orange 1l\n2 ngaous abricot"

ERPS = {
    "odoo": dict(port=8621, connect=dict(kind="odoo", url="http://127.0.0.1:8069", database="orderdesk", username="admin", password="admin")),
    "erpnext": dict(port=8622, connect=dict(kind="erpnext", url="http://127.0.0.1:8090", username="Administrator", password="admin")),
    "dolibarr": dict(port=8623, connect=dict(kind="dolibarr", url="http://127.0.0.1:8081", username="admin", password="admin")),
    "inventree": dict(port=8624, connect=dict(kind="inventree", url="http://127.0.0.1:8083", username="admin", password="admin")),
    "tryton": dict(port=8625, connect=dict(kind="tryton", url="http://127.0.0.1:8000", database="orderdesk", username="admin", password="admin")),
}


def api(port, path, body=None, method=None):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(f"http://127.0.0.1:{port}/api{path}", data=data, method=method or ("POST" if body is not None else "GET"),
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=300) as r:
        return json.loads(r.read() or b"{}")


def login(page, erp, c):
    u, p = c["username"], c["password"]
    base = c["url"]
    if erp == "odoo":
        page.goto(base + "/web/login")
        page.fill("#login", u)
        page.fill("#password", p)
        page.click("button[type=submit]")
    elif erp == "erpnext":
        page.goto(base + "/login")
        page.fill("#login_email", u)
        page.fill("#login_password", p)
        page.locator("form button[type=submit]").first.click()
    elif erp == "dolibarr":
        page.goto(base + "/index.php")
        page.fill("input[name=username]", u)
        page.fill("input[name=password]", p)
        page.click("input[type=submit], button[type=submit]")
    elif erp == "inventree":
        page.goto(base + "/web/login")
        page.get_by_placeholder("Your username").wait_for(timeout=60000)
        page.get_by_placeholder("Your username").fill(u)
        page.get_by_placeholder("Your password").fill(p)
        page.get_by_role("button", name="Log In").click()
        page.wait_for_url("**/web/home**", timeout=60000)
    elif erp == "tryton":
        page.goto(base + "/")
        page.wait_for_selector("input#login", timeout=60000)
        page.wait_for_timeout(1500)
        dbi = page.locator("input#database").first
        if dbi.is_visible():
            if dbi.is_editable():
                dbi.fill(c["database"])
        else:
            page.locator("select#database").first.select_option(c["database"])
        page.locator("input#login, input[name=login]").first.fill(u)
        page.keyboard.press("Enter")
        page.wait_for_selector("input[type=password]", timeout=60000)
        page.locator("input[type=password]").first.fill(p)
        page.keyboard.press("Enter")
    page.wait_for_timeout(4000)


def run(erp):
    cfg = ERPS[erp]
    port, c = cfg["port"], cfg["connect"]
    data = DESKS / erp
    shutil.rmtree(data, ignore_errors=True)
    data.mkdir(parents=True)
    log = open(DESKS / f"{erp}.log", "w")
    args = [str(EXE), "-addr", f"127.0.0.1:{port}", "-data", str(data), "-no-telegram"]
    if KEYS.exists():
        args += ["-keys", str(KEYS)]
    proc = subprocess.Popen(args, stdout=log, stderr=log)
    try:
        for _ in range(60):
            try:
                api(port, "/overview")
                break
            except Exception:
                time.sleep(0.5)
        ov = api(port, "/erp/connect", c)
        print(erp, "connected:", ov["erp"]["connected"], ov["products"], "products")
        cust = next(x for x in api(port, "/customers")["items"] if SHOP in x["name"])
        tr = api(port, "/try", {"customer_id": cust["id"], "text": ORDER})
        draft = tr["draft"]
        OUT.mkdir(parents=True, exist_ok=True)
        with sync_playwright() as pw:
            b = pw.chromium.launch(channel="chrome", headless=True)
            page = b.new_page(viewport={"width": 1500, "height": 940}, device_scale_factor=1)
            page.goto(f"http://127.0.0.1:{port}/#/inbox/{draft}")
            page.wait_for_timeout(2500)
            page.screenshot(path=str(OUT / f"{erp}-1-question.jpg"), type="jpeg", quality=82)
            for l in api(port, f"/drafts/{draft}")["lines"]:
                if l["status"] == "ask" and l.get("options"):
                    api(port, f"/lines/{l['id']}/choose", {"product_id": l["options"][0], "correction": False})
            page.reload()
            page.wait_for_timeout(2500)
            page.screenshot(path=str(OUT / f"{erp}-2-draft.jpg"), type="jpeg", quality=82)
            ref = api(port, f"/drafts/{draft}/confirm", {})
            print(erp, "order", ref.get("number"), ref.get("url"))
            erp_page = b.new_page(viewport={"width": 1500, "height": 940}, device_scale_factor=1)
            login(erp_page, erp, c)
            erp_page.goto(ref["url"])
            erp_page.wait_for_timeout(7000)
            erp_page.screenshot(path=str(OUT / f"{erp}-3-erp.jpg"), type="jpeg", quality=82)
            b.close()
        return ref
    finally:
        proc.terminate()
        proc.wait(10)
        log.close()


if __name__ == "__main__":
    for e in sys.argv[1:]:
        run(e)
