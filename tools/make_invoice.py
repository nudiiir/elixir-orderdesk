"""Draw a fictional test invoice from the demo wholesaler (a fictional company), as a PNG to upload in the panel (Invoices).
It is a specimen, marked as such: no real company issued it.

    python tools/make_invoice.py samples/invoice-specimen.png
"""
import sys
from PIL import Image, ImageDraw, ImageFont

W, H = 1240, 1754
img = Image.new("RGB", (W, H), "white")
d = ImageDraw.Draw(img)
try:
    big, font, small = ImageFont.truetype("arialbd.ttf", 44), ImageFont.truetype("arial.ttf", 28), ImageFont.truetype("arial.ttf", 22)
except OSError:
    big = font = small = ImageFont.load_default()
d.text((80, 80), "EURL Grossiste Mitidja Alimentation", font=big, fill="black")
d.text((80, 140), "Grossiste boissons et alimentation — Blida", font=small, fill="black")
d.text((760, 185), "FACTURE N° FA-2026-1187", font=font, fill="black")
d.text((760, 230), "Date : 30/10/2026", font=font, fill="black")
d.text((80, 230), "Client : Elixir Distribution Test", font=font, fill="black")
lines = [("SEL 2000ML PET X6", 10, 572.34), ("SEL 1L PET X6", 8, 381.60), ("HMD GAZ 2L PET X6", 5, 540.00),
         ("SLIM CIT 1L PET X6", 4, 420.00), ("SEL CAN 33CL X24", 3, 1068.48)]
y = 330
d.line((80, y - 10, W - 80, y - 10), fill="black", width=2)
for x, t in ((80, "Désignation"), (640, "Qté (colis)"), (820, "P.U. HT"), (1000, "Montant HT")):
    d.text((x, y), t, font=font, fill="black")
y += 50
d.line((80, y - 6, W - 80, y - 6), fill="black", width=1)
total = 0
for text, qty, pu in lines:
    m = qty * pu
    total += m
    d.text((80, y), text, font=font, fill="black")
    d.text((660, y), str(qty), font=font, fill="black")
    d.text((820, y), f"{pu:,.2f}".replace(",", " "), font=font, fill="black")
    d.text((1000, y), f"{m:,.2f}".replace(",", " "), font=font, fill="black")
    y += 48
d.line((80, y + 4, W - 80, y + 4), fill="black", width=2)
d.text((760, y + 30), f"Total HT : {total:,.2f} DA".replace(",", " "), font=font, fill="black")
d.text((80, H - 160), "SPÉCIMEN — facture fictive de test (Order Desk demo), sans valeur", font=font, fill=(200, 0, 0))
img.save(sys.argv[1])
print("total HT", round(total, 2))
