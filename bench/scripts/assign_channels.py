"""Give every order the channel it arrives on, 50 per industry: 10 voice notes, 15 WhatsApp messages in
Darja written in Arabic script, 15 in Darja written in Latin letters only (no 3/7/9 for letters), and 10
photos of handwritten lists. Orders already recorded stay voice. The products of an order never change
(recordings depend on them); only its message is rewritten. Seed 2029.

    python -I assign_channels.py [voice dir]   -> orders.json, orders-<industry>.json, photo-sheets.html/.pdf
"""
import sys, json, random, re, subprocess
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).parent
VOICE = Path(sys.argv[1] if len(sys.argv) > 1 else r"E:\invoices\orders-voice")
R = random.Random(2029)
orders = json.load(open(HERE / "orders.json", encoding="utf-8"))
recorded = {p.stem for p in VOICE.iterdir() if p.suffix in (".webm", ".mp3", ".wav", ".ogg", ".m4a", ".opus", ".aac")}

AR_UNIT = {"carton": ("كرطون", "كراطن"), "pack": ("فاردو", "فاردوات"),
           "unit": {"pharmacy": ("علبة", "علب"), "electronics": ("حبة", "حبات"), "cosmetics": ("حبة", "حبات"), "supermarket": ("حبة", "حبات")}}
LA_UNIT = {"carton": ("kartona", "kartonat"), "pack": ("fardou", "fardouyat"),
           "unit": {"pharmacy": ("boite", "boites"), "electronics": ("habba", "habbat"), "cosmetics": ("habba", "habbat"), "supermarket": ("habba", "habbat")}}
AR_NUM = {1: "واحد", 2: "زوج", 3: "ثلاثة", 4: "ربعة", 5: "خمسة", 6: "ستة", 10: "عشرة"}

def unit(table, o, l):
    u = table[l["unit"]]
    if isinstance(u, dict): u = u[o["industry"]]
    return u[0] if l["qty"] == 1 or l["qty"] > 10 else u[1]

def darja_arabic(o):
    rows = []
    for l in o["lines"]:
        q = AR_NUM[l["qty"]] if l["qty"] in AR_NUM and R.random() < 0.3 else str(l["qty"])
        rows.append(f"{q} {unit(AR_UNIT, o, l)} {l['say']}" if R.random() < 0.8 else f"{l['say']} {q} {unit(AR_UNIT, o, l)}")
    head = [R.choice(["السلام عليكم", "سلام خويا", "صباح الخير", "مساء الخير خويا"]), R.choice(["ابعثلي:", "عطيني:", "حضرلي هادو:", "نحتاج:"])]
    return "\n".join(head + rows + [R.choice(["بارك الله فيك", "صحة خويا", "ربي يحفظك", "وابعثهم اليوم ان شاء الله"])])

def darja_latin(o):
    rows = []
    for l in o["lines"]:
        rows.append(f"{l['qty']} {unit(LA_UNIT, o, l)} {l['say']}" if R.random() < 0.75 else f"{l['say']} {l['qty']} {unit(LA_UNIT, o, l)}")
    sep = R.choice(["\n", "\n", ", w "])
    body = sep.join(rows)
    head = R.choice(["salam", "slm khouya", "salam alikoum", "sbah lkhir khouya"]) + (",\n" if sep == "\n" else ", ") + R.choice(["abaathli", "aatini", "hadarli", "nhab"]) + (":\n" if sep == "\n" else " ")
    return head + body + ("\n" if sep == "\n" else ". ") + R.choice(["saha", "baraka allahou fik", "rabi yhafdek", "w abaathhom lyoum inchallah"])

out = []
for ind in ["supermarket", "pharmacy", "cosmetics", "electronics"]:
    os_ = [o for o in orders if o["industry"] == ind]
    voice = [o for o in os_ if o["id"] in recorded]
    rest = [o for o in os_ if o["id"] not in recorded]
    R.shuffle(rest)
    voice += rest[:10 - len(voice)]; rest = rest[10 - min(10, len(voice)):] if len(voice) <= 10 else rest
    rest = [o for o in os_ if o not in voice]; R.shuffle(rest)
    plan = [("voice", voice)] + [("darja_arabic", rest[:15]), ("darja_latin", rest[15:30]), ("photo", rest[30:40])]
    for ch, group in plan:
        for o in group:
            o["channel"] = ch
            if ch == "darja_arabic": o["text"], o["text_style"] = darja_arabic(o), "darja_arabic"
            if ch == "darja_latin": o["text"], o["text_style"] = darja_latin(o), "darja_latin"
    os_.sort(key=lambda o: o["id"])
    (HERE / f"orders-{ind}.json").write_text(json.dumps(os_, ensure_ascii=False, indent=1), encoding="utf-8")
    out += os_
    c = {ch: len(g) for ch, g in plan}
    print(ind, c, "| voice already recorded:", sorted(o["id"] for o in voice if o["id"] in recorded))
(HERE / "orders.json").write_text(json.dumps(out, ensure_ascii=False, indent=1), encoding="utf-8")

# printable sheet for the photo orders: what to write by hand
UNIT_FR = {"carton": ("carton", "cartons"), "pack": ("pack", "packs")}
def fr(o, l):
    if l["unit"] in UNIT_FR: return UNIT_FR[l["unit"]][l["qty"] > 1]
    return {"pharmacy": "boîte", "electronics": "pièce", "cosmetics": "pièce", "supermarket": "pièce"}[o["industry"]] + ("s" if l["qty"] > 1 else "")
cards = []
for o in [o for o in out if o["channel"] == "photo"]:
    rows = "".join(f"<tr><td class=q>{l['qty']} {fr(o, l)}</td><td>{l['say']}</td></tr>" for l in o["lines"])
    cards.append(f"<div class=card><div class=h><b>{o['id']}</b><span>{o['shop']}</span></div><table>{rows}</table></div>")
html = f"""<!doctype html><meta charset=utf-8><title>Photo orders</title><style>
@page {{ size:A4; margin:12mm }} body {{ font:11pt/1.4 'Segoe UI',Arial,sans-serif; color:#111 }}
h1 {{ font-size:16pt; margin:0 0 4px }} p {{ margin:0 0 10px; color:#444 }}
.grid {{ display:grid; grid-template-columns:1fr 1fr; gap:10px }} .card {{ border:1px solid #bbb; border-radius:6px; padding:8px 10px; break-inside:avoid }}
.h {{ display:flex; justify-content:space-between; border-bottom:1px solid #ddd; margin-bottom:4px; padding-bottom:3px }} .h span {{ color:#555; font-size:10pt }}
table {{ border-collapse:collapse; width:100% }} td {{ padding:2px 4px; font-size:10.5pt; vertical-align:top }} td.q {{ width:90px; font-weight:600; white-space:nowrap }}
</style><h1>Photo orders: 40 handwritten lists</h1>
<p>Write each list by hand on paper, the way a shop owner would (any order, abbreviations, Darja or French), then photograph it and name the photo by its number (SUP-O05.jpg). Every product and quantity must be on the paper.</p>
<div class=grid>{''.join(cards)}</div>"""
(HERE / "photo-sheets.html").write_text(html, encoding="utf-8")
pdf = HERE / "photo-sheets.pdf"
subprocess.run([r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe", "--headless", "--disable-gpu", "--no-pdf-header-footer", f"--print-to-pdf={pdf}", (HERE / "photo-sheets.html").as_uri()], capture_output=True)
print("photo sheet:", pdf, pdf.exists())
