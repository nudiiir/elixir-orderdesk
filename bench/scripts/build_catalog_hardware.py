"""Electronics & phone-accessories catalogue from a real shop/distributor product list (4,825 rows).

Kept: new products in real product families (accessories, IT, phones, tablets, watches, audio, consoles...).
Dropped: second-hand items (OCC, battery-health %), top-ups, SIM cards, transfers, services, labels; names
with a misspelt word (a word seen only once in the whole list that is not a model code); duplicates.
Then up to ~800 products are chosen family by family, preferring products that share a brand with others
(similar names make the benchmark realistic). Output catalog.csv: id, name, brand, family. No prices/stock.
"""
import csv, re, sys, collections, random
from pathlib import Path
import xlrd
sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).parent
sh = xlrd.open_workbook(HERE / "raw-private.xls").sheet_by_index(0)
rows = [dict(zip(sh.row_values(0), sh.row_values(i))) for i in range(1, sh.nrows)]
FAM = {"ACCESOIRE TELEPHONE": 330, "INFORMATIQUE": 250, "MOBILE": 110, "TABLETTE": 30, "Montre": 30,
       "AUDIOVISUEL": 25, "CONSOLE": 8, "TELEPHONE FIXE": 8, "TROTTINETTE": 5, "ACCESOIRE AUTO": 3}
def norm(s): return re.sub(r"\s+", " ", str(s).upper().replace(" ", " ")).strip()
words = collections.Counter(w for r in rows for w in set(re.findall(r"[A-Z]{3,}", norm(r["Désignation"]))))
why, cand = collections.Counter(), []
seen = set()
for r in rows:
    n, fam = norm(r["Désignation"]), r["Famille"]
    if fam not in FAM: why["family out"] += 1; continue
    if re.search(r"\bOCC\b|\d{2,3}\s?%|NEUF|RECOND|---", n): why["second-hand / note"] += 1; continue
    bad = [w for w in re.findall(r"[A-Z]{3,}", n) if words[w] < 2 and not re.search(r"\d", w)]
    if bad: why["misspelt or one-off word"] += 1; continue
    if len(n) < 6 or n in seen: why["short/duplicate"] += 1; continue
    seen.add(n); cand.append((n, fam))
GENERIC = set("CABLE CHARGEUR CHARGER CASQUE ECOUTEUR KIT SUPPORT ADAPTATEUR BATTERIE BAFFLE SOURIS CLAVIER GLASS POCHETTE MOBILE TABLETTE SMART WATCH MONTRE PRISE FLASH DISQUE CLE CARTE MEMOIRE LAMP LED POWER BANK OTG HUB".split())
def brand(n):
    for w in re.findall(r"[A-Z][A-Z0-9-]+", n):
        if w not in GENERIC and len(w) > 1 and not w[0].isdigit(): return w
    return ""
bc = collections.Counter(brand(n) for n, _ in cand)
R = random.Random(20261103)
out = []
for fam, k in FAM.items():
    pool = [(n, f) for n, f in cand if f == fam]
    pool.sort(key=lambda x: (-min(bc[brand(x[0])], 12), R.random()))  # brands with siblings first
    out += pool[:k]
with open(HERE / "catalog.csv", "w", encoding="utf-8-sig", newline="") as f:
    w = csv.writer(f); w.writerow(["id", "name", "brand", "family"])
    for i, (n, fam) in enumerate(sorted(out), 1): w.writerow([f"EL-{i:04}", n, brand(n), fam])
print(len(rows), "rows ->", len(cand), "clean ->", len(out), "kept"); print(dict(why))
