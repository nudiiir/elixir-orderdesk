"""Draws samples/readbench: the hardware test invoices as phone-photo-like JPEGs (small print, skew, blur, JPEG), with
what is printed on them. Run from the repository folder: python tools/make_readbench.py (needs Pillow)."""
import json, random
from PIL import Image, ImageDraw, ImageFont, ImageFilter
d = json.load(open("samples/replay-hardware.json", encoding="utf-8"))
random.seed(3)
try:
    F = ImageFont.truetype("C:/Windows/Fonts/consola.ttf", 17); B = ImageFont.truetype("C:/Windows/Fonts/arialbd.ttf", 26)
except OSError:
    F = B = ImageFont.load_default()
out = []
for inv in [i for i in d["invoices"] if i["split"] == "test"]:
    W, H = 1100, 260 + 34 * len(inv["lines"]) + 160
    im = Image.new("RGB", (W, H), (250, 248, 242)); g = ImageDraw.Draw(im)
    g.text((40, 30), "QUINCAILLERIE " + inv["supplier"].upper(), font=B, fill=(20, 20, 20))
    g.text((40, 70), "Zone industrielle - RC 16/00-0987654B21", font=F, fill=(60, 60, 60))
    g.text((720, 30), f"FACTURE N° {inv['id']}", font=B, fill=(20, 20, 20)); g.text((720, 70), f"Date : {inv['month']}-12", font=F, fill=(40, 40, 40))
    y = 150; g.line((30, y - 8, W - 30, y - 8), fill=(0, 0, 0), width=2)
    for x, h in [(40, "DESIGNATION"), (690, "QTE"), (790, "P.U HT"), (940, "MONTANT")]: g.text((x, y), h, font=F, fill=(0, 0, 0))
    y += 34; total = 0
    for l in inv["lines"]:
        amt = l["qty"] * l["unit_price"]; total += amt
        g.text((40, y), l["text"][:58], font=F, fill=(15, 15, 15)); g.text((690, y), f"{l['qty']:g}", font=F, fill=(15, 15, 15))
        g.text((790, y), f"{l['unit_price']:,.2f}".replace(",", " "), font=F, fill=(15, 15, 15)); g.text((940, y), f"{amt:,.2f}".replace(",", " "), font=F, fill=(15, 15, 15))
        y += 34
    g.line((30, y, W - 30, y), fill=(0, 0, 0), width=2)
    g.text((700, y + 20), f"TOTAL HT : {total:,.2f}".replace(",", " "), font=B, fill=(0, 0, 0))
    im = im.rotate(random.uniform(-2.5, 2.5), expand=True, fillcolor=(200, 196, 188)).filter(ImageFilter.GaussianBlur(random.uniform(0.6, 1.1)))
    im = im.resize((int(im.width * 0.62), int(im.height * 0.62)))
    p = f"{inv['id']}.jpg"; im.save("samples/readbench/" + p, quality=50)
    out.append({"image": p, "lines": [{"text": l["text"], "qty": l["qty"], "unit_price": l["unit_price"]} for l in inv["lines"]]})
json.dump({"invoices": out}, open("samples/readbench/truth.json", "w", encoding="utf-8"), ensure_ascii=False, indent=1)
print(len(out), sum(len(i["lines"]) for i in out))
