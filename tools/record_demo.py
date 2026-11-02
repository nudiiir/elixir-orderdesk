"""Record the runs the demo page replays (docs/demo). Everything is the real pipeline: Order Desk with live Meta
Model API calls, WhatsApp through the local simulator (tools/whatsapp-sim), orders and receipts booked in the Odoo
test ERP from erps/. Writes docs/demo/data.json and screenshots of the booked documents in Odoo.

    python tools/record_demo.py            (KEYS=<keys file with META_MODEL_API_KEY>, default .env)

Needs: orderdesk(.exe) built at the repo root, Odoo from erps/odoo running and seeded, ffmpeg, Playwright + Chrome.
"""
import hashlib
import hmac
import json
import os
import shutil
import sqlite3
import subprocess
import sys
import tempfile
import time
import urllib.parse
import urllib.request
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")
REPO = Path(__file__).resolve().parent.parent
SIM = REPO / "tools" / "whatsapp-sim"
OUTBOX = SIM / "outbox.jsonl"
OUT = REPO / "docs" / "demo"
EXE = REPO / ("orderdesk.exe" if os.name == "nt" else "orderdesk")
KEYS = Path(os.environ.get("KEYS", REPO / ".env"))
PORT, GRAPH = 8640, 8699
DESK = f"http://127.0.0.1:{PORT}"
DATA = Path(tempfile.gettempdir()) / "orderdesk-demo-desk"
ODOO = dict(kind="odoo", url="http://127.0.0.1:8069", database="orderdesk", username="admin", password="admin")


def api(method, path, body=None):
    req = urllib.request.Request(DESK + "/api" + path, json.dumps(body).encode() if body is not None else None,
                                 {"Content-Type": "application/json"}, method=method)
    return json.loads(urllib.request.urlopen(req, timeout=300).read() or b"{}")


def outbox(phone):
    if not OUTBOX.exists():
        return []
    return [json.loads(l) for l in open(OUTBOX, encoding="utf-8") if l.strip() and json.loads(l).get("to") == phone]


n = 0


def deliver(phone, name, message):
    global n
    n += 1
    message = {"from": phone, "id": f"wamid.demo{int(time.time() * 1000)}{n}", "timestamp": str(int(time.time())), **message}
    body = json.dumps({"object": "whatsapp_business_account", "entry": [{"id": "WABA", "changes": [{"field": "messages", "value": {
        "messaging_product": "whatsapp", "metadata": {"phone_number_id": "PHONE"},
        "contacts": [{"wa_id": phone, "profile": {"name": name}}], "messages": [message]}}]}]}).encode()
    sig = "sha256=" + hmac.new(b"SECRET", body, hashlib.sha256).hexdigest()
    urllib.request.urlopen(urllib.request.Request(DESK + "/whatsapp/webhook", body,
                                                  {"Content-Type": "application/json", "X-Hub-Signature-256": sig}, method="POST"), timeout=30).read()


def wait_replies(phone, since, quiet=2.5, limit=180):
    t0, last, count = time.time(), time.time(), since
    while time.time() - t0 < limit:
        got = outbox(phone)
        if len(got) > count:
            count, last = len(got), time.time()
        elif count > since and time.time() - last > quiet:
            return got[since:]
        time.sleep(0.3)
    return outbox(phone)[since:]


def view(m):
    """A sent WhatsApp message as the phone shows it: text, and buttons or list rows."""
    if m["type"] == "text":
        return {"text": m["text"]["body"], "buttons": []}
    if m["type"] == "image":
        return {"text": m["image"].get("caption", ""), "image": m["image"].get("link", ""), "buttons": []}
    it = m["interactive"]
    if it["type"] == "button":
        bt = [{"id": b["reply"]["id"], "title": b["reply"]["title"], "kind": "button_reply"} for b in it["action"]["buttons"]]
    else:
        bt = [{"id": r["id"], "title": r["title"], "kind": "list_reply"} for s in it["action"]["sections"] for r in s["rows"]]
    return {"text": it["body"]["text"], "buttons": bt, "list": it["type"] == "list"}


class Chat:
    def __init__(self, phone, name):
        self.phone, self.name, self.log = phone, name, []
        self.t0 = time.time()

    def _send(self, entry, message):
        since = len(outbox(self.phone))
        entry["t"] = round(time.time() - self.t0, 1)
        self.log.append(entry)
        deliver(self.phone, self.name, message)
        started = time.time()
        got = wait_replies(self.phone, since)
        took = round(time.time() - started, 1)
        out = []
        for m in got:
            v = view(m)
            v.update(dir="out", t=round(time.time() - self.t0, 1), took=took)
            self.log.append(v)
            out.append(v)
        return out

    def say(self, text):
        print("📱", text.replace("\n", " / "))
        return self._send({"dir": "in", "text": text}, {"type": "text", "text": {"body": text}})

    def voice(self, media_id, seconds):
        print("🎤 voice note")
        return self._send({"dir": "in", "voice": True, "seconds": seconds}, {"type": "audio", "audio": {"id": media_id, "mime_type": "audio/ogg"}})

    def photo(self, media_id, caption, src):
        print("📷 photo")
        return self._send({"dir": "in", "image": src, "text": caption},
                          {"type": "image", "image": {"id": media_id, "mime_type": "image/png", "caption": caption}})

    def tap(self, button):
        print("👆", button["title"])
        return self._send({"dir": "tap", "text": button["title"]},
                          {"type": "interactive", "interactive": {"type": button["kind"], button["kind"]: {"id": button["id"], "title": button["title"]}}})


def buttons(msgs):
    return [b for m in msgs for b in m.get("buttons", [])]


def pick(msgs, *words):
    bs = buttons(msgs)
    for w in words:
        for b in bs:
            if w.lower() in b["title"].lower() or b["id"].startswith(w):
                return b
    return None


def converse(chat, msgs, prefer, goal, rounds=10):
    """Answer questions with the preferred option, then confirm, until the goal text appears."""
    for _ in range(rounds):
        if any(goal in m["text"] for m in msgs):
            return True
        b = pick(msgs, *prefer) or pick(msgs, "ok:") or (buttons(msgs) or [None])[0]
        if not b:
            # still working (a voice note or a photo): wait for the next messages
            since = len(outbox(chat.phone))
            more = wait_replies(chat.phone, since, quiet=3, limit=120)
            if not more:
                return False
            msgs = []
            for m in more:
                v = view(m)
                v.update(dir="out", t=round(time.time() - chat.t0, 1))
                chat.log.append(v)
                msgs.append(v)
            continue
        msgs = chat.tap(b)
    return any(goal in m["text"] for m in msgs)


def db():
    c = sqlite3.connect(DATA / "orderdesk.db")
    c.row_factory = sqlite3.Row
    return c


def internals(chat_key, since):
    """What happened inside the desk for one chat since a time: messages, lines, AI calls, ERP documents."""
    c = db()
    msgs = [dict(r) for r in c.execute("SELECT id, kind, text, extraction, created_at FROM messages WHERE chat_key = ? AND created_at >= ? ORDER BY created_at", (chat_key, since))]
    ids = [m["id"] for m in msgs]
    drafts = [dict(r) for r in c.execute("SELECT id, kind, status, erp_order_number, total_ttc, supplier_id, doc_number, doc_total FROM drafts WHERE chat_key = ? AND created_at >= ? ORDER BY created_at", (chat_key, since))]
    out_drafts = []
    for d in drafts:
        lines = []
        for r in c.execute("SELECT * FROM lines WHERE draft_id = ? ORDER BY position", (d["id"],)):
            r = dict(r)
            p = c.execute("SELECT name FROM products WHERE id = ?", (r.get("product_id") or "",)).fetchone()
            opts = json.loads(r.get("options") or "[]")
            names = [(c.execute("SELECT name FROM products WHERE id = ?", (o,)).fetchone() or [o])[0] for o in opts]
            lines.append({"said": r["said"], "norm": r["norm"], "qty": r["qty"], "unit": r.get("unit"), "product": p[0] if p else "",
                          "pack": r.get("pack_name"), "level": r["level"], "used": json.loads(r.get("used") or "[]"),
                          "confidence": r.get("confidence"), "question": r.get("question"), "options": names,
                          "status": r["status"], "outcome": r.get("outcome"), "unit_price": r.get("unit_price"),
                          "line_ttc": r.get("line_ttc"), "checks": json.loads(r.get("checks") or "{}"), "ai_cost": r.get("ai_cost")})
        d["lines"] = lines
        out_drafts.append(d)
    calls = [dict(r) for r in c.execute("SELECT at, model, purpose, tokens_in, tokens_out, audio_sec, cost, ref FROM ai_usage WHERE at >= ? ORDER BY at", (since,))]
    return {"messages": msgs, "drafts": out_drafts, "ai_calls": calls}


def now():
    return time.strftime("%Y-%m-%d %H:%M:%S")  # the desk stamps rows in local time


def main():
    for p in (OUTBOX,):
        p.unlink(missing_ok=True)
    shutil.rmtree(DATA, ignore_errors=True)
    DATA.mkdir(parents=True)
    OUT.mkdir(parents=True, exist_ok=True)
    media = SIM / "media"
    shutil.copy(REPO / "samples" / "invoice-specimen.png", media / "IMGINV1")
    shutil.copy(REPO / "samples" / "voice-order-darja.mp3", media / "VOICE1")

    graph = subprocess.Popen([sys.executable, str(SIM / "fakegraph.py"), str(GRAPH)])
    env = dict(os.environ, WHATSAPP_GRAPH_URL=f"http://127.0.0.1:{GRAPH}", WHATSAPP_TOKEN="TOKEN", WHATSAPP_PHONE_ID="PHONE",
               WHATSAPP_VERIFY_TOKEN="VERIFY", WHATSAPP_APP_SECRET="SECRET")
    args = [str(EXE), "-addr", f"127.0.0.1:{PORT}", "-data", str(DATA), "-no-telegram"]
    if KEYS.exists():
        args += ["-keys", str(KEYS)]
    log = open(DATA / "desk.log", "w", encoding="utf-8")
    desk = subprocess.Popen(args, env=env, stdout=log, stderr=log)
    try:
        for _ in range(60):
            try:
                api("GET", "/overview")
                break
            except Exception:
                time.sleep(0.5)
        ov = api("POST", "/erp/connect", ODOO)
        print("ERP:", ov["erp"]["name"], ov["products"], "products")
        custs = api("GET", "/customers")["items"]

        def shop(part):
            return next(c for c in custs if part in c["name"])

        scenes = []

        # 1. a typed order, one question, booked
        c1 = shop("El Amel")
        code = api("POST", "/customers/" + urllib.parse.quote(c1["id"], safe="") + "/pair-code")["code"]
        chat = Chat("213555100201", c1["name"])
        since = now()
        chat.say(code)
        chat.log.clear()
        chat.t0 = time.time()
        order = "salam, 3 cartons selecto 2l\n2 ifri eau minerale 1.5\n4 rouiba orange 1l\n2 ngaous abricot"
        msgs = chat.say(order)
        ok = converse(chat, msgs, ["Abricot 1L"], "enregistrée")
        scenes.append({"id": "typed", "shop": c1["name"], "booked": ok, "chat": chat.log, "inside": internals("wa:213555100201", since)})

        # 2. a voice note in Darja
        c2 = shop("Errahma")
        code = api("POST", "/customers/" + urllib.parse.quote(c2["id"], safe="") + "/pair-code")["code"]
        chat = Chat("213555100202", c2["name"])
        since = now()
        chat.say(code)
        chat.log.clear()
        chat.t0 = time.time()
        msgs = chat.voice("VOICE1", 9)
        ok = converse(chat, msgs, ["Selecto 2L", "Cocktail", "Elio Huile 5L"], "enregistrée")
        scenes.append({"id": "voice", "shop": c2["name"], "booked": ok, "audio": "voice-order-darja.mp3", "chat": chat.log,
                       "inside": internals("wa:213555100202", since)})

        # 3. the owner sends a supplier invoice photo
        code = api("POST", "/owner/pair-code")["code"]
        chat = Chat("213661100300", "Owner")
        since = now()
        chat.say(code)
        chat.log.clear()
        chat.t0 = time.time()
        msgs = chat.photo("IMGINV1", "facture", "invoice-specimen.png")
        ok = converse(chat, msgs, ["Mitidja", "ok:", "Valider", "Approuver"], "enregistr")
        scenes.append({"id": "invoice", "shop": "Distributor (owner)", "booked": ok, "chat": chat.log, "inside": internals("wa:213661100300", since)})

        # 4. the first shop orders again: what it taught is now free
        chat = Chat("213555100201", c1["name"])
        since = now()
        msgs = chat.say("3 cartons selecto 2l\n2 ngaous abricot\n2 ifri 1.5")
        ok = converse(chat, msgs, ["Abricot 1L"], "enregistrée")
        scenes.append({"id": "learned", "shop": c1["name"], "booked": ok, "chat": chat.log, "inside": internals("wa:213555100201", since)})

        ov = api("GET", "/overview")
        data = {"recorded_at": time.strftime("%Y-%m-%d %H:%M"), "erp": "Odoo 18 (erps/odoo)", "products": ov.get("products"),
                "scenes": scenes}
        (OUT / "data.json").write_text(json.dumps(data, ensure_ascii=False, indent=1), encoding="utf-8")
        print("wrote", OUT / "data.json", [(s["id"], s["booked"]) for s in scenes])
        return data
    finally:
        desk.terminate()
        graph.terminate()
        log.close()


if __name__ == "__main__":
    main()
