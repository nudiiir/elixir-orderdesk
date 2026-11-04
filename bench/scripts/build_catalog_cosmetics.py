"""Cosmetics & parapharmacy catalogue: one distributor's own product names, as printed on its invoices.
Only the product name is used: no customer, price, quantity or date. The raw invoice lines stay private
(they name the distributors).

    python bench/scripts/build_catalog_cosmetics.py <raw invoice lines.csv> <distributor name>

Cleaning: invoice annotations removed (PPA 650.00, UG 50%, REF/BR codes), medicines removed (doses, tablets,
syrups, injections...), OCR noise removed (every word must be seen in other invoices too), spelling variants
of the same product merged (the most printed spelling wins). Output catalog.csv: id, name, brand.
"""
import csv, re, sys, collections, difflib
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
sys.path.insert(0, str(Path(__file__).resolve().parent))
import paths
if len(sys.argv) < 3: paths.usage(__doc__, "give the raw invoice lines and the distributor's name")
RAW, DIST = paths.need(sys.argv[1], __doc__, "raw invoice lines"), sys.argv[2].upper()
HERE = paths.out_dir()
rows = list(csv.DictReader(open(RAW, encoding="utf-8")))
def norm(s): return re.sub(r"\s+", " ", re.sub(r"[^A-Z0-9%./+' -]", " ", s.upper())).strip(" .-")
NOTE = re.compile(r"\b(PPA|P\.P\.A)\s*[:=]?\s*\d+(?:[.,]\d+)?|\bUG\s*\d+\s*%?|\b\d+\s*%\s*UG\b|\s\d*\s*UG\s*$|\b(BR|REF|KHA|CB|LOT)\s*[A-Z]?\d+\b|\bBOITE\b|\s[.]+(\s|$)")
# a parapharmacy distributor also sells food supplements (capsules, B/30, vitamin doses): those stay.
# Dropped: what only a pharmacist dispenses (injections, suppositories, eye drops, ointments, syrups).
MED = re.compile(r"\b(INJ|INJECTABLE|SUPPO|SUPPOSITOIRES?|COLLYRE|POMMADE|PDE|SIROPS?|SOL BUV|SUSP|GOUTTES?|GTTES?|AMOXICILLINE|OTIPAX|POLYDEXA)\b|\b\d+\s?MG\b.*\b(CP|CPR|COMP|COMPRIME)\b")
vocab = collections.Counter()
for r in rows:
    for w in set(norm(NOTE.sub(" ", r["designation"])).split()): vocab[w] += 1
cnt = collections.Counter()
for r in rows:
    if DIST not in r["s"].upper(): continue
    d = norm(NOTE.sub(" ", norm(r["designation"])))
    d = re.sub(r"\s+", " ", d).strip()
    if len(d.split()) < 2 or MED.search(d): continue
    if any(len(w) > 2 and re.search(r"[A-Z]", w) and vocab[w] < 2 for w in d.split()[1:]): continue  # first word = brand, may be rare
    cnt[d] += int(r["n"])
names = [n for n, _ in cnt.most_common()]
key = lambda n: " ".join(sorted(set(re.findall(r"[A-Z0-9]+", n))))
kept, dropped = [], 0
for n in names:
    if any(difflib.SequenceMatcher(None, key(n), key(k)).ratio() > 0.93 for k in kept[-400:]): dropped += 1; continue
    kept.append(n)
GENERIC = set("CREME GEL SAVON SHAMP SHAMPOING LAIT HUILE EAU BAUME LOTION SERUM DENTIFRICE BROSSE BAD DEO SPRAY STICK MASQUE".split())
def brand(n):
    for w in n.split():
        if w not in GENERIC and not w[0].isdigit(): return w
    return n.split()[0]
with open(HERE / "catalog.csv", "w", encoding="utf-8-sig", newline="") as f:
    w = csv.writer(f); w.writerow(["id", "name", "brand"])
    for i, n in enumerate(sorted(kept), 1): w.writerow([f"CO-{i:04}", n, brand(n)])
print(DIST, "distinct printed names", sum(1 for r in rows if DIST in r["s"].upper()), "-> clean", len(cnt), "-> after merging spellings", len(kept), f"(merged {dropped})")
