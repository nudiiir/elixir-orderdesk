"""Find, for every catalogue product, the matching Elixir catalogue product and the real spellings pharmacies
use for it (aliases from pharmacy software, scanned invoices, nomenclatures). Writes matches-<kind>.json."""
import os, csv, re, sys, math, json, collections
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
csv.field_size_limit(10**8)
HERE = Path(__file__).parent
AL = Path(os.environ.get("SPELLINGS_DIR", "spellings"))  # the Elixir spelling export (not shipped)
KIND = sys.argv[1]  # pharmacy | cosmetics
CAT = HERE.parent / ("pharmacy" if KIND == "pharmacy" else "cosmetics") / "catalog.csv"

def norm(s): return re.sub(r"\s+", " ", re.sub(r"[^A-Z0-9%/.,+ ]", " ", s.upper().replace("É", "E").replace("È", "E").replace("Ê", "E"))).strip()
def toks(s): return re.findall(r"[A-Z]{2,}|\d+(?:[.,]\d+)?", norm(s))
def nums(s):
    out = set()
    for m in re.finditer(r"(\d+(?:[.,]\d+)?)", norm(s)):
        v = m.group(1).replace(",", ".")
        try: f = float(v)
        except ValueError: continue
        out.add(("%g" % f))
    return out
FORMWORDS = set("CP CPR COMP COMPRIME COMPRIMES GLE GLES GELULE GELULES SP SIROP SACH SACHET SACHT INJ AMP SUPP SUPPO PDE POMMADE GTT GTTES GOUTTES SOL SUSP BUV FL T B BT BTE PELLI SEC EFF DISP OROD ENROB".split())
prods = {}
for r in csv.DictReader(open(AL / "products.csv", encoding="utf-8")):
    prods[r["id"]] = r
def final(pid):
    seen = 0
    while pid in prods and prods[pid]["merged_into"] and seen < 5: pid = prods[pid]["merged_into"]; seen += 1
    return pid
want_kind = {"pharmacy": {"medicament"}, "cosmetics": {"parapharmacie", "dispositif", "autre"}}[KIND]
aliases = collections.defaultdict(list)
for r in csv.DictReader(open(AL / "aliases.csv", encoding="utf-8")):
    pid = final(r["product_id"])
    if pid not in prods: continue
    aliases[pid].append((r["alias_text"], int(r["witnesses"] or 0), r["sources"]))
for pid, p in prods.items():
    if p["canonical_label"]: aliases[pid].append((p["canonical_label"], 0, "label"))
cand = [pid for pid in aliases if prods.get(pid, {}).get("kind") in want_kind and not prods[pid]["merged_into"]]
df = collections.Counter(); inv = collections.defaultdict(set)
for pid in cand:
    ts = set()
    for a, _, _ in aliases[pid]: ts |= set(toks(a))
    for t in ts: df[t] += 1; inv[t].add(pid)
N = len(cand); idf = lambda t: math.log((N + 1) / (df[t] + 1))
res, stats = [], collections.Counter()
for row in csv.DictReader(open(CAT, encoding="utf-8-sig")):
    name = row["name"]; qt = [t for t in toks(name)]; qn = nums(name)
    brand = next((t for t in qt if t.isalpha() and t not in FORMWORDS and len(t) > 2), None)
    if not brand: stats["no brand"] += 1; res.append(dict(row, match=None)); continue
    pool = inv.get(brand, set())
    best, bs = None, 0
    for pid in pool:
        texts = aliases[pid]
        sc = 0
        for a, w, src in texts[:200]:
            at = set(toks(a)); an = nums(a)
            if brand not in at: continue
            if KIND == "pharmacy":
                dq = {x for x in qn if x not in ("1",)}; da = an
                if not dq <= da | {"30", "20", "10", "14", "28", "12", "60"} and not dq & da: continue
                if dq and not (dq & da): continue
            s = sum(idf(t) for t in set(qt) & at) / (sum(idf(t) for t in set(qt)) + 1e-9)
            if KIND == "pharmacy":
                miss = [x for x in qn if x not in an and not re.fullmatch(r"(10|12|14|20|28|30|60|90|100)", x)]
                s -= 0.25 * len(miss)
            sc = max(sc, s)
        if sc > bs: best, bs = pid, sc
    ok = best and bs >= (0.55 if KIND == "pharmacy" else 0.6)
    stats["matched" if ok else "no match"] += 1
    res.append(dict(row, match=best if ok else None, score=round(bs, 3), elixir_label=prods[best]["canonical_label"] if ok else ""))
json.dump({"rows": res, "aliases": {r["match"]: aliases[r["match"]] for r in res if r.get("match")}}, open(HERE / f"matches-{KIND}.json", "w", encoding="utf-8"), ensure_ascii=False)
print(KIND, dict(stats))
