"""Aliases for every catalogue product: how pharmacies actually type it.

1. Real spellings: every spelling in the Elixir catalogue (pharmacy software lists, scanned invoices,
   nomenclatures; 455k spellings, no pharmacy ids) is checked against our products one by one. A spelling is
   kept for a product only if it has the product's brand, every number in it is one of the product's numbers
   (no other dose, size or pack), it carries the product's dose (medicines), every other word is the
   product's own word or a known abbreviation of it, and no other product of the catalogue fits it as well.
2. When a product has fewer than MIN real spellings, variants are written with the rewrite patterns
   pharmacies use most (counted in the same 455k spellings): form abbreviations (CP/COMP/CPR), pack
   formats (B/30, BT/30, BTE 30), unit spacing (10MG / 10 MG), dropped pack or secondary words.

    python -I aliases.py pharmacy|cosmetics
Writes aliases-<kind>.xlsx (one row per product, alias_1..alias_N) and aliases-<kind>.csv (long form).
"""
import os, csv, re, sys, math, collections
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
csv.field_size_limit(10**8)
HERE = Path(__file__).parent
AL = Path(os.environ.get("SPELLINGS_DIR", "spellings"))  # the Elixir spelling export (not shipped)
KIND = sys.argv[1]
CAT = HERE.parent / ("pharmacy" if KIND == "pharmacy" else "cosmetics") / "catalog.csv"
MIN, MAX = 5, 15

ABBR = {  # full word -> abbreviations seen in pharmacy spellings
    "COMPRIME": "CP CPR COMP CPS COMPRIMES", "GELULE": "GLE GLES GEL GELS GELULES CAPS", "SIROP": "SP SIR SRP",
    "SACHET": "SACH SACHT SAC SACHETS", "POMMADE": "PDE POM", "INJECTABLE": "INJ", "AMPOULE": "AMP AMPOULES",
    "SUPPOSITOIRE": "SUPP SUPPO SUPPOS", "GOUTTES": "GTT GTTES GTTE GOUT", "COLLYRE": "COLL COLLY",
    "PELLICULE": "PELLI PEL PELL PELLIC", "SECABLE": "SEC", "EFFERVESCENT": "EFF EFFERV EFFER",
    "DISPERSIBLE": "DISP DISPER", "OROD": "ORODISP", "BOITE": "B BT BTE", "FLACON": "FL FLC", "TUBE": "T TB",
    "SOLUTION": "SOL SOLU", "BUVABLE": "BUV", "SUSPENSION": "SUSP", "CREME": "CR CRM CREM", "SHAMPOING": "SHAMP SHP SHAMPOOING SHAMPO",
    "DENTIFRICE": "DENT DENTIF", "BEBE": "BB", "ENFANT": "ENF ENFANTS", "ADULTE": "AD ADULTES ADULT", "VITAMINE": "VIT VITAMINES",
    "MAGNESIUM": "MAG", "NOURRISSON": "NOUR NRS", "LIBERATION": "LIB", "PROLONGEE": "PROL LP", "GASTRO": "GAST",
    "RESISTANT": "RES", "OPHTALMIQUE": "OPHT OPH", "INHALATION": "INHAL", "SPRAY": "SPR", "LOTION": "LOT", "SERUM": "SER",
    "BROSSE": "BAD BR", "GRANULES": "GRANU GRAN", "OVULE": "OVUL OV", "SAVON": "SAV", "LIQUIDE": "LIQ", "ANTI": "A",
}
if KIND == "cosmetics":  # GEL is a gel, COMP a compress: the medicine forms do not apply
    for k in ("COMPRIME", "GELULE", "SIROP", "SUPPOSITOIRE", "INJECTABLE", "AMPOULE", "OVULE"): ABBR.pop(k)
EQUIV = {}
for full, ab in ABBR.items():
    grp = {full} | set(ab.split())
    for w in grp: EQUIV.setdefault(w, set()).update(grp)
STOP = set("DE DU LA LE LES DES ET A AU AUX EN POUR AVEC SANS L D X ML MG G GR GM MCG UG UI MUI %".split())
FORM = set(EQUIV) - {"ANTI", "A", "VITAMINE", "VIT", "VITAMINES", "MAGNESIUM", "MAG", "BEBE", "BB"}

def norm(s):
    s = s.upper().replace("É", "E").replace("È", "E").replace("Ê", "E").replace("Μ", "M").replace("µ", "M")
    s = re.sub(r"(?<=\d) (?=\d{3}\b)", "", s)            # 1 000 000 -> 1000000
    return re.sub(r"\s+", " ", re.sub(r"[^A-Z0-9%/.,+ ]", " ", s)).strip()
def words(s): return re.findall(r"[A-Z]+", norm(s))
def nums(s):
    out = set()
    for m in re.finditer(r"\d+(?:[.,]\d+)?", norm(s)):
        try: out.add("%g" % float(m.group(0).replace(",", ".")))
        except ValueError: pass
    return out
def doses(s):  # numbers that are a strength (followed by a unit or %)
    return {"%g" % float(m.group(1).replace(",", ".")) for m in re.finditer(r"(\d+(?:[.,]\d+)?)\s?(MG|G|MCG|UG|UI|MUI|%|ML)\b", norm(s))}
def same(a, b):
    if a == b or b in EQUIV.get(a, ()) or a in EQUIV.get(b, ()): return True
    return len(a) >= 3 and len(b) >= 4 and (b.startswith(a) or a.startswith(b))
def brand_of(name):
    for w in words(name):
        if w not in STOP and w not in FORM and len(w) > 2: return w
    return None

cat = list(csv.DictReader(open(CAT, encoding="utf-8-sig")))
for it in cat:
    it["_w"] = [w for w in words(it["name"]) if w not in STOP]
    it["_n"], it["_d"], it["_b"] = nums(it["name"]), doses(it["name"]), brand_of(it["name"])
by_brand = collections.defaultdict(list)
for it in cat:
    if it["_b"]: by_brand[it["_b"]].append(it)

def fit(alias, it):
    """score of alias for item, or None if the alias cannot be this item"""
    aw = [w for w in words(alias) if w not in STOP]
    if it["_b"] not in aw: return None
    if len(aw) < 2 and len(it["_w"]) > 1: return None       # the brand alone says too little
    t = norm(alias).split()
    if any(t[i:i + 3] == t[j:j + 3] for i in range(len(t) - 2) for j in range(i + 3, len(t) - 2)): return None  # OCR doubled text
    an = nums(alias)
    if not an <= it["_n"]: return None                       # another dose, size or pack
    if KIND == "pharmacy" and it["_d"] and not (it["_d"] <= an): return None
    for w in aw:                                             # every alias word explained by the product
        if not any(same(w, x) for x in it["_w"]) and not (w in FORM and len(w) <= 5): return None
    core = [x for x in it["_w"] if x not in FORM and x not in EQUIV]
    hit = sum(1 for x in core if any(same(x, w) for w in aw))
    return hit / max(1, len(core))

real = collections.defaultdict(dict)   # item id -> alias -> (witnesses, sources)
form_freq = collections.Counter()
want = {"pharmacy": {"medicament"}, "cosmetics": {"parapharmacie", "dispositif", "autre"}}[KIND]
kind_of = {}
for r in csv.DictReader(open(AL / "products.csv", encoding="utf-8")): kind_of[r["id"]] = r["kind"]
for r in csv.DictReader(open(AL / "aliases.csv", encoding="utf-8")):
    a = norm(r["alias_text"])
    for w in words(a): form_freq[w] += 1
    if kind_of.get(r["product_id"]) not in want and KIND == "pharmacy": continue
    cands = set()
    for w in set(words(a)):
        if w in by_brand: cands.update(id(x) for x in by_brand[w])
    if not cands: continue
    scored = []
    for w in set(words(a)):
        for it in by_brand.get(w, []):
            s = fit(a, it)
            if s is not None: scored.append((s, it))
    if not scored: continue
    scored.sort(key=lambda x: -x[0])
    if scored[0][0] < 0.5 or (len(scored) > 1 and scored[1][0] == scored[0][0] and scored[1][1] is not scored[0][1]): continue
    it = scored[0][1]
    if a == norm(it["name"]): continue
    w0, s0 = real[it["id"]].get(a, (0, ""))
    real[it["id"]][a] = (w0 + int(r["witnesses"] or 0), "+".join(sorted(set(filter(None, (s0 + "+" + r["sources"]).split("+"))))))

def variants(name):
    """rewrites pharmacies make, most frequent forms first"""
    n = norm(name); out = []
    def best(group, cur):
        return sorted((g for g in group if g != cur), key=lambda g: -form_freq[g])
    toks = n.split()
    for i, t in enumerate(toks):                              # one form word rewritten
        if t in EQUIV:
            for alt in best(EQUIV[t], t)[:3]: out.append(" ".join(toks[:i] + [alt] + toks[i + 1:]))
    m = re.search(r"\bB\s?/\s?(\d+)\b", n)
    if m:                                                     # pack formats
        k = m.group(1)
        for f in (f"BT/{k}", f"BTE/{k}", f"B{k}", f"BT {k}", f"X{k}"): out.append(n.replace(m.group(0), f))
        out.append(re.sub(r"\s*\bB\s?/\s?\d+\b", "", n))      # pack left out
    out.append(re.sub(r"(\d)\s?(MG|ML|G|GR|MCG|UI)\b", r"\1 \2", n))   # 10MG -> 10 MG
    out.append(re.sub(r"(\d) (MG|ML|G|GR|MCG|UI)\b", r"\1\2", n))       # 10 MG -> 10MG
    out.append(re.sub(r"(\d),(\d)", r"\1.\2", n)); out.append(re.sub(r"(\d)\.(\d)", r"\1,\2", n))
    out.append(" ".join(t for t in toks if t not in ("PELLI", "SEC", "ENROB", "PELLICULE", "SECABLE")))
    out.append(re.sub(r"\b(FL|T|TB)\s?/\s?", r"\1 ", n))
    seen, res = {n}, []
    for v in out:
        v = re.sub(r"\s+", " ", v).strip()
        if v and v not in seen: seen.add(v); res.append(v)
    return res

rows, long = [], []
nreal = collections.Counter()
for it in cat:
    al = sorted(real[it["id"]].items(), key=lambda x: -x[1][0])[:MAX]
    lst = [(a, "real", w, s) for a, (w, s) in al]
    nreal[min(len(lst), MIN)] += 1
    if len(lst) < MIN:
        have = {a for a, *_ in lst} | {norm(it["name"])}
        for v in variants(it["name"]):
            if len(lst) >= MIN + 3: break
            if v not in have: lst.append((v, "pattern", 0, "")); have.add(v)
    rows.append([it["id"], it["name"], sum(1 for x in lst if x[1] == "real")] + [a for a, *_ in lst])
    for a, src, w, s in lst: long.append([it["id"], it["name"], a, src, w, s])

with open(HERE / f"aliases-{KIND}.csv", "w", encoding="utf-8-sig", newline="") as f:
    w = csv.writer(f); w.writerow(["id", "name", "alias", "source", "witnesses", "seen_in"]); w.writerows(long)
from openpyxl import Workbook
from openpyxl.styles import Font, PatternFill
wb = Workbook(); ws = wb.active; ws.title = "Aliases"
width = max(len(r) for r in rows) - 3
ws.append(["id", "name", "real aliases"] + [f"alias_{i}" for i in range(1, width + 1)])
grey = PatternFill("solid", fgColor="EEEEEE")
for r, it in zip(rows, cat):
    ws.append(r)
    nr = r[2]
    for j in range(4 + nr, len(r) + 1): ws.cell(ws.max_row, j).fill = grey   # pattern-written aliases in grey
for c in ws[1]: c.font = Font(bold=True)
ws.column_dimensions["B"].width = 48; ws.freeze_panes = "C2"
ws2 = wb.create_sheet("Long (with source)")
ws2.append(["id", "name", "alias", "source", "witnesses", "seen_in"])
for l in long: ws2.append(l)
ws3 = wb.create_sheet("How to read")
for line in [__doc__.strip(), "", "Grey cells = written from patterns (no real spelling found). White = real spellings.",
             "witnesses = how many separate lists/invoices carried that spelling."]:
    for l in line.splitlines(): ws3.append([l])
wb.save(HERE / f"aliases-{KIND}.xlsx")
tot_real = sum(1 for l in long if l[3] == "real"); tot_pat = len(long) - tot_real
print(KIND, len(cat), "products |", tot_real, "real aliases,", tot_pat, "pattern aliases | products by real-alias count (5 = 5+):", dict(sorted(nreal.items())))
