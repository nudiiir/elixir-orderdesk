"""Final channels, 10 per channel per industry (seed 2031). Voice orders are left alone.

Of the other 40 orders of each industry: 10 in French, 10 in Darja written in Latin letters, 10 mixed
(Darja in Arabic script with French words), and 10 sent as a screenshot of the list (a phone notes app,
a spreadsheet, or a printed purchase order), rendered as PNG. The products of an order never change.

    python -I restyle.py      -> orders.json, orders-<industry>.json, E:\\invoices\\orders-screens\\<id>.png
"""
import sys, json, random, subprocess, html
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).parent
SHOTS = Path(r"E:\invoices\orders-screens"); SHOTS.mkdir(parents=True, exist_ok=True)
EDGE = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
R = random.Random(2031)
orders = json.load(open(HERE / "orders.json", encoding="utf-8"))
e = html.escape

FR_UNIT = {"carton": ("carton", "cartons", "ctn"), "pack": ("pack", "packs", "fdx"),
           "unit": {"pharmacy": ("boîte", "boîtes", "bt"), "electronics": ("pièce", "pièces", "pcs"), "cosmetics": ("pièce", "pièces", "pcs"), "supermarket": ("pièce", "pièces", "pcs")}}
def fr_unit(o, l, short=False):
    u = FR_UNIT[l["unit"]]
    if isinstance(u, dict): u = u[o["industry"]]
    return u[2] if short else (u[0] if l["qty"] == 1 else u[1])

def french(o):
    short = R.random() < 0.35
    hello = R.choice(["Bonjour,", "Bonjour", "Salam, bonjour", "Bonsoir,"])
    intro = R.choice(["Merci de me livrer :", "Voici ma commande :", "Je voudrais :", "Commande pour demain svp :", ""])
    rows = []
    for l in o["lines"]:
        b = R.choice(["- ", "", "• "]) if not short else ""
        rows.append(f"{b}{l['qty']} {fr_unit(o, l, short)} {l['say']}" if R.random() < 0.8 else f"{b}{l['say']} : {l['qty']} {fr_unit(o, l, short)}")
    bye = R.choice(["Merci", "Merci d'avance", "Merci, bonne journée", "Cordialement", f"Merci\n{o['shop']}"])
    return "\n".join(x for x in [hello, intro] + rows + [bye] if x)

AR_UNIT = {"carton": ("كرطون", "كراطن"), "pack": ("فاردو", "فاردوات"),
           "unit": {"pharmacy": ("علبة", "علب"), "electronics": ("حبة", "حبات"), "cosmetics": ("حبة", "حبات"), "supermarket": ("حبة", "حبات")}}
AR_NUM = {1: "واحد", 2: "زوج", 3: "ثلاثة", 4: "ربعة", 5: "خمسة", 6: "ستة", 10: "عشرة"}
def mixed(o):
    rows = []
    for l in o["lines"]:
        q = AR_NUM[l["qty"]] if l["qty"] in AR_NUM and R.random() < 0.3 else str(l["qty"])
        if R.random() < 0.45:  # unit in French
            u = fr_unit(o, l)
        else:
            u = AR_UNIT[l["unit"]]; u = u[o["industry"]] if isinstance(u, dict) else u; u = u[0] if l["qty"] == 1 or l["qty"] > 10 else u[1]
        rows.append(f"{q} {u} {l['say']}" if R.random() < 0.8 else f"{l['say']} {q} {u}")
    head = [R.choice(["السلام عليكم", "Salam خويا", "صباح الخير", "Bonjour خويا"]),
            R.choice(["ابعثلي svp:", "la commande تاعي:", "نحتاج هادو:", "حضرلي من فضلك:", "commande:"])]
    bye = R.choice(["بارك الله فيك", "merci خويا", "صحة", "ابعثهم اليوم إن شاء الله", "merci"])
    return "\n".join(head + rows + [bye])

LA_UNIT = {"carton": ("kartona", "kartonat"), "pack": ("fardou", "fardouyat"),
           "unit": {"pharmacy": ("boite", "boites"), "electronics": ("habba", "habbat"), "cosmetics": ("habba", "habbat"), "supermarket": ("habba", "habbat")}}
def darja_latin(o):  # letters only, no 3/7/9 for letters
    def u(l):
        x = LA_UNIT[l["unit"]]; x = x[o["industry"]] if isinstance(x, dict) else x
        return x[0] if l["qty"] == 1 or l["qty"] > 10 else x[1]
    rows = [f"{l['qty']} {u(l)} {l['say']}" if R.random() < 0.75 else f"{l['say']} {l['qty']} {u(l)}" for l in o["lines"]]
    return "\n".join([R.choice(["salam", "slm khouya", "salam alikoum", "sbah lkhir khouya"]) + ",", R.choice(["abaathli", "aatini", "hadarli", "nhab"]) + ":"]
                     + rows + [R.choice(["saha", "baraka allahou fik", "rabi yhafdek", "w abaathhom lyoum inchallah"])])

# --- screenshots -----------------------------------------------------------------------------------
def notes_app(o):
    items = "".join(f"<li><span class=q>{l['qty']}</span> {e(fr_unit(o, l, True))} {e(l['say'])}</li>" for l in o["lines"])
    title = R.choice([f"Commande {o['shop'].split()[-1]}", "Commande", "A commander", "Commande grossiste"])
    return 390, 844, f"""<style>body{{margin:0;font:17px -apple-system,'Segoe UI',sans-serif;background:#fbfbf8;color:#1c1c1e}}
.bar{{display:flex;justify-content:space-between;padding:14px 22px 6px;font-weight:600;font-size:15px}} .nav{{color:#e0a100;padding:6px 16px;font-size:17px}}
h1{{font-size:28px;margin:6px 20px 2px}} .d{{color:#8e8e93;font-size:13px;margin:0 20px 10px}} ul{{list-style:none;padding:0 20px;margin:0}}
li{{padding:7px 0;border-bottom:1px solid #ececec;overflow-wrap:anywhere}} .q{{font-weight:600;display:inline-block;min-width:26px}}</style>
<div class=bar><span>{R.randint(8, 21)}:{R.randint(10, 59)}</span><span>●●● 4G ▮▮▮</span></div><div class=nav>‹ Notes</div>
<h1>{e(title)}</h1><div class=d>{R.randint(1, 28)} novembre 2026 à {R.randint(8, 20)}:{R.randint(10, 59)}</div><ul>{items}</ul>"""

def sheet(o):
    rows = "".join(f"<tr><td class=n>{i}</td><td>{e(l['say'])}</td><td class=r>{l['qty']}</td><td>{e(fr_unit(o, l))}</td></tr>" for i, l in enumerate(o["lines"], 2))
    h = 150 + 26 * len(o["lines"])
    return 760, max(h, 330), f"""<style>body{{margin:0;font:13px Calibri,'Segoe UI',sans-serif;background:#fff}}
.tb{{background:#217346;color:#fff;padding:7px 12px;font-size:13px}} .rb{{background:#f3f3f3;border-bottom:1px solid #d4d4d4;padding:6px 12px;color:#444}}
table{{border-collapse:collapse;margin:0}} td,th{{border:1px solid #d4d4d4;padding:3px 8px;height:20px}} th{{background:#f3f3f3;font-weight:normal;color:#555}}
td.n,th.n{{background:#f3f3f3;color:#666;text-align:center;width:28px}} td.r{{text-align:right}} tr.h td{{font-weight:bold;background:#e2efda}}</style>
<div class=tb>commande_{e(o['shop'].split()[-1].lower())}.xlsx - Excel</div><div class=rb>Accueil &nbsp; Insertion &nbsp; Mise en page &nbsp; Formules &nbsp; Données</div>
<table><tr><th class=n></th><th style="width:420px">A</th><th style="width:70px">B</th><th style="width:110px">C</th></tr>
<tr class=h><td class=n>1</td><td>Désignation</td><td>Qté</td><td>Unité</td></tr>{rows}</table>"""

def purchase_order(o):
    rows = "".join(f"<tr><td>{i}</td><td>{e(l['say'])}</td><td class=r>{l['qty']}</td><td>{e(fr_unit(o, l))}</td></tr>" for i, l in enumerate(o["lines"], 1))
    return 800, 300 + 31 * len(o["lines"]), f"""<style>body{{margin:0;background:#e8e8e8;font:14px Arial,sans-serif}} .page{{background:#fff;margin:18px auto;width:720px;padding:34px 40px;box-shadow:0 1px 4px #0003}}
h1{{font-size:20px;margin:0 0 4px;letter-spacing:1px}} .hd{{display:flex;justify-content:space-between;margin-bottom:20px}} .box{{border:1px solid #333;padding:8px 12px;font-size:13px}}
table{{border-collapse:collapse;width:100%}} th,td{{border:1px solid #333;padding:5px 8px;font-size:13px}} th{{background:#eee}} td.r{{text-align:right}} .f{{margin-top:30px;display:flex;justify-content:space-between;font-size:13px}}</style>
<div class=page><div class=hd><div><h1>BON DE COMMANDE</h1>N° {R.randint(100, 999)}/26<br>Date : {R.randint(1, 28):02}/11/2026</div>
<div class=box><b>{e(o['shop'])}</b><br>Client</div></div>
<table><tr><th style="width:36px">N°</th><th>Désignation</th><th style="width:70px">Qté</th><th style="width:100px">Unité</th></tr>{rows}</table>
<div class=f><span>Livraison souhaitée : {R.choice(['demain matin', 'dès que possible', 'cette semaine'])}</span><span>Signature et cachet</span></div></div>"""

CHROME = r"C:\Program Files\Google\Chrome\Application\chrome.exe"

def shoot(oid, w, h, body):
    """Headless Chrome with its own profile (a running Edge swallows headless jobs), at 2x for sharp text."""
    page = SHOTS / f"_{oid}.html"; png = SHOTS / f"{oid}.png"
    page.write_text(f"<!doctype html><meta charset=utf-8>{body}", encoding="utf-8")
    subprocess.run([CHROME, "--headless=new", "--disable-gpu", "--hide-scrollbars", r"--user-data-dir=E:\invoices\chrome-headless",
                    "--force-device-scale-factor=2", f"--window-size={w},{h}", f"--screenshot={png}", page.as_uri()], capture_output=True, timeout=90)
    page.unlink(missing_ok=True)
    return png.exists()

STYLES = [("notes", notes_app), ("sheet", sheet), ("purchase_order", purchase_order)]
for ind in ["supermarket", "pharmacy", "cosmetics", "electronics"]:
    rest = [o for o in orders if o["industry"] == ind and o["channel"] != "voice"]
    R.shuffle(rest)
    for k, o in enumerate(rest):
        ch = ["french", "darja_latin", "mixed", "screenshot"][k // 10]
        o["channel"] = ch
        o.pop("image", None); o.pop("image_style", None)
        if ch == "french": o["text"], o["text_style"] = french(o), "french"
        elif ch == "mixed": o["text"], o["text_style"] = mixed(o), "mixed"
        elif ch == "darja_latin" and o.get("text_style") != "darja_latin":
            o["text"], o["text_style"] = darja_latin(o), "darja_latin"
        elif ch == "screenshot":
            style, fn = STYLES[k % 3]
            w, h, body = fn(o)
            ok = shoot(o["id"], w, h, body)
            o["image"], o["image_style"], o["text"], o["text_style"] = f"{o['id']}.png", style, "", "screenshot"
            if not ok: print("screenshot failed", o["id"])
    print(ind, {c: sum(1 for o in orders if o["industry"] == ind and o["channel"] == c) for c in ["voice", "french", "darja_latin", "mixed", "screenshot"]})
(HERE / "orders.json").write_text(json.dumps(orders, ensure_ascii=False, indent=1), encoding="utf-8")
for ind in ["supermarket", "pharmacy", "cosmetics", "electronics"]:
    (HERE / f"orders-{ind}.json").write_text(json.dumps([o for o in orders if o["industry"] == ind], ensure_ascii=False, indent=1), encoding="utf-8")
