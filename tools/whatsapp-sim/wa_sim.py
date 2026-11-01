"""Plays a WhatsApp user against a desk through its webhook (signed like Meta), reading the desk's
replies from the fake Graph API's outbox and tapping buttons like a person would.

  wa_sim.py shop  <desk> <phone> "<order text>"     pair as a shop, order, answer, confirm
  wa_sim.py owner <desk> <phone> <media id>          pair as the owner, send an invoice photo, approve

<desk> is the panel address (http://127.0.0.1:8610); the media id names a file in ./media served by
fakegraph.py. CUST=<n> picks the n-th customer (default 1)."""
import hashlib, hmac, json, os, sys, time, urllib.parse, urllib.request

sys.stdout.reconfigure(encoding="utf-8")  # emoji and accents on a Windows console too

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "outbox.jsonl")
role, desk, phone, payload = sys.argv[1:5]
n = 0
LOG = []  # the conversation (TRANSCRIPT=path writes it as JSON)


def api(method, path, body=None):
    req = urllib.request.Request(desk + "/api" + path, json.dumps(body).encode() if body is not None else None,
                                 {"Content-Type": "application/json"}, method=method)
    return json.loads(urllib.request.urlopen(req, timeout=120).read() or b"{}")


def outbox():
    if not os.path.exists(OUT):
        return []
    return [json.loads(l) for l in open(OUT, encoding="utf-8") if l.strip()]


def deliver(message):
    global n
    n += 1
    message = {"from": phone, "id": f"wamid.sim{int(time.time() * 1000)}{n}", "timestamp": str(int(time.time())), **message}
    body = json.dumps({"object": "whatsapp_business_account", "entry": [{"id": "WABA", "changes": [{"field": "messages", "value": {
        "messaging_product": "whatsapp", "metadata": {"phone_number_id": "PHONE"},
        "contacts": [{"wa_id": phone, "profile": {"name": "Test " + role}}], "messages": [message]}}]}]}).encode()
    sig = "sha256=" + hmac.new(b"SECRET", body, hashlib.sha256).hexdigest()
    req = urllib.request.Request(desk + "/whatsapp/webhook", body, {"Content-Type": "application/json", "X-Hub-Signature-256": sig}, method="POST")
    urllib.request.urlopen(req, timeout=30).read()


def replies(since, wait=90):
    """Messages the desk sent to this phone after `since`, once it has been quiet for 2 s."""
    t0, last, count = time.time(), time.time(), since
    while time.time() - t0 < wait:
        mine = [m for m in outbox() if m.get("to") == phone]
        if len(mine) > count:
            count, last = len(mine), time.time()
        elif count > since and time.time() - last > 2:
            return mine[since:]
        time.sleep(0.3)
    return [m for m in outbox() if m.get("to") == phone][since:]


def show(m):
    if m["type"] == "text":
        return m["text"]["body"]
    it = m["interactive"]
    opts = [b["reply"]["title"] for b in it["action"].get("buttons", [])] or [r["title"] for s in it["action"].get("sections", []) for r in s["rows"]]
    return it["body"]["text"] + "\n   [" + " | ".join(opts) + "]"


def choices(m):
    if m["type"] != "interactive":
        return []
    it = m["interactive"]
    if it["type"] == "button":
        return [("button_reply", b["reply"]["id"], b["reply"]["title"]) for b in it["action"]["buttons"]]
    return [("list_reply", r["id"], r["title"]) for s in it["action"]["sections"] for r in s["rows"]]


def say(text):
    k = len([m for m in outbox() if m.get("to") == phone])
    LOG.append({"dir": "in", "text": text})
    print(f"\n📱 {text}")
    deliver({"type": "text", "text": {"body": text}})
    return k


def tap(kind, id_, title):
    k = len([m for m in outbox() if m.get("to") == phone])
    LOG.append({"dir": "tap", "text": title})
    print(f"\n👆 [{title}]")
    deliver({"type": "interactive", "interactive": {"type": kind, kind: {"id": id_, "title": title}}})
    return k


def converse(k, goal):
    for _ in range(12):
        got = replies(k)
        for m in got:
            LOG.append({"dir": "out", "msg": m})
            print("💬 " + show(m).replace("\n", "\n   "))
        if not got:
            print("(no reply)")
            return False
        if any(goal in show(m) for m in got):
            return True
        opts = [c for m in got for c in choices(m)]
        if not opts:
            return False
        ok = [c for c in opts if c[1].startswith("ok:")]
        k = tap(*(ok[0] if ok else opts[0]))
    return False


if role == "shop":
    cust = api("GET", "/customers")
    cust = (cust.get("items") or cust)[int(os.environ.get("CUST", "1"))]
    code = api("POST", "/customers/" + urllib.parse.quote(cust["id"], safe="") + "/pair-code")["code"]
    print("pair code for", cust["name"])
    converse(say(code), "lié")
    print("\nRESULT:", "order booked" if converse(say(payload), "enregistrée") else "not booked")
else:
    code = api("POST", "/owner/pair-code")["code"]
    converse(say(code), "propriétaire")
    k = len([m for m in outbox() if m.get("to") == phone])
    LOG.append({"dir": "photo", "text": "facture"})
    print("\n📱 [photo de la facture]")
    deliver({"type": "image", "image": {"id": payload, "mime_type": "image/png", "caption": "facture"}})
    print("\nRESULT:", "receipt booked" if converse(k, "enregistr") else "not booked")

if os.environ.get("TRANSCRIPT"):
    json.dump(LOG, open(os.environ["TRANSCRIPT"], "w", encoding="utf-8"), ensure_ascii=False, indent=1)
