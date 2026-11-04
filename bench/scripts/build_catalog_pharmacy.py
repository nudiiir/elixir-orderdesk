"""Pharmacy catalogue: a wholesaler's stock listing (PDF, 3 columns per page), cleaned.

Each entry is "DESIGNATION  PPA  expiry  bonus"; long designations wrap onto the next line. Only the
designation is kept (no price, expiry or bonus). Output catalog.csv: id, name, brand (first word).
"""
import csv, re, sys
from pathlib import Path
import pdfplumber
sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).parent
ROW = re.compile(r"^(.*?)\s+(\d[\d ]*,\d{2})\s+(\d{2}/\d{2}/\d{2})\s+(\d+)\s*(.*)$")
items = []
with pdfplumber.open(HERE / "raw-private.pdf") as pdf:
    for page in pdf.pages:
        heads = [w for w in page.extract_words() if w["text"].startswith("Désignation")]
        xs = sorted(w["x0"] for w in heads)
        if not xs: continue
        top = max(w["bottom"] for w in heads) + 1
        bounds = xs + [page.width]
        for i in range(len(xs)):
            col = page.crop((bounds[i] - 2, top, bounds[i + 1] - 2, page.height))
            for line in (col.extract_text() or "").splitlines():
                line = line.strip()
                m = ROW.match(line)
                if m:
                    items.append(m.group(1) + (" " + m.group(5) if m.group(5) else ""))
                elif items and line and not re.match(r"^(Page|Date|Listing)", line):
                    items[-1] += " " + line
def clean(n):
    n = re.sub(r"\(\s*\d+\s*%\s*\)", " ", n)          # bonus percentage notes
    n = re.sub(r"\s+", " ", n).strip(" .-")
    return n.upper()
seen, out = set(), []
for n in map(clean, items):
    if len(n) < 4 or n in seen: continue
    seen.add(n); out.append(n)
with open(HERE / "catalog.csv", "w", encoding="utf-8-sig", newline="") as f:
    w = csv.writer(f); w.writerow(["id", "name", "brand"])
    for i, n in enumerate(sorted(out), 1): w.writerow([f"PH-{i:04}", n, n.split()[0]])
print(len(items), "entries ->", len(out), "products")
