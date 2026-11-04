"""A realistic stream of supplier invoices for one industry, for cmd/adaptive (seed 2027).

How real wholesale works, and what this stream copies:
- each supplier sells its own range (100 products of the catalogue, ranges overlap);
- a supplier's software prints a product the same way on every invoice (one designation per product,
  taken from the industry's alias list: real wordings seen at least twice, pattern and written variants;
  the catalogue name for about 15 %);
- the distributor reorders popular products more often (Zipf weights within a range).

150 invoices in time order: the first 120 train (rounds of 30), the last 30 are locked (never learned).
A designation that is exactly another product's catalogue name is never used (it would name that
product). Same presentation listed twice in the catalogue: both ids are right.

    python bench/scripts/gen_stream.py supermarket|pharmacy|cosmetics|electronics

Reads bench/catalogs/, bench/wordings/ and the supplier names of bench/invoices/truth-<industry>.json; writes
replay-stream-<industry>.json to $BENCH_OUT (default ./bench-out).
"""
import sys, json, csv, random, collections, re
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
sys.path.insert(0, str(Path(__file__).resolve().parent))
import paths
ind = paths.industry_arg(__doc__)
N_INV, N_TRAIN, RANGE = 150, 120, 100
R = random.Random(2027)
cat = list(csv.DictReader(open(paths.need(paths.catalog(ind), __doc__, "catalogue"), encoding="utf-8-sig")))
apath = paths.need(paths.wordings(ind), __doc__, "wordings")
suppliers = sorted({i["supplier"] for i in json.load(open(paths.need(paths.truth(ind), __doc__, "answer key"), encoding="utf-8"))})

norm = lambda s: re.sub(r"[^A-Z0-9]+", "", s.upper())
name_of = {p["id"]: p["name"] for p in cat}
owner = collections.defaultdict(set)
for p in cat: owner[norm(p["name"])].add(p["id"])
pool = collections.defaultdict(list)
for a in csv.DictReader(open(apath, encoding="utf-8-sig")):
    if a["id"] not in name_of: continue
    if a.get("source") == "real" and int(a.get("witnesses") or 0) < 2: continue  # a wording seen once may be someone's mistake
    if owner.get(norm(a["alias"]), {a["id"]}) - {a["id"]}: continue           # names another product exactly
    pool[a["id"]].append(a["alias"])

def designation(pid):
    if pool[pid] and R.random() > 0.15:
        return R.choice(pool[pid])
    return name_of[pid]

ids = [p["id"] for p in cat]
book = {}
for s in suppliers:
    rng = R.sample(ids, RANGE)
    book[s] = {"range": rng, "w": [1 / (k + 1) ** 0.8 for k in range(RANGE)], "says": {pid: designation(pid) for pid in rng}}

twins = collections.defaultdict(list)
for p in cat: twins[norm(p["name"])].append(p["id"])

invoices = []
for n in range(N_INV):
    s = R.choice(suppliers); b = book[s]
    k = R.randint(12, 25); chosen = []
    while len(chosen) < k:
        pid = R.choices(b["range"], b["w"])[0]
        if pid not in chosen: chosen.append(pid)
    lines = []
    for pid in chosen:
        l = {"text": b["says"][pid], "qty": float(R.choice([6, 12, 24, 36, 48, 60, 96, 120, 144, 240])), "unit_price": round(R.uniform(80, 3000), 2), "product": pid}
        acc = [i for i in twins[norm(name_of[pid])] if i != pid]
        if acc: l["accept"] = acc
        lines.append(l)
    month = f"2026-{1 + n * 10 // N_INV:02}"
    invoices.append({"id": f"{ind[:3].upper()}-S{n + 1:03}", "supplier": s, "month": month, "split": "train" if n < N_TRAIN else "test", "lines": lines})

out = {"invoices": invoices, "products": [{"id": p["id"], "name": p["name"], "brand": p.get("brand", ""), "code": ""} for p in cat]}
(paths.out_dir() / f"replay-stream-{ind}.json").write_text(json.dumps(out, ensure_ascii=False), encoding="utf-8")
L = lambda sp: sum(len(i["lines"]) for i in invoices if i["split"] == sp)
seen = set(); rep = 0
for i in invoices:
    for l in i["lines"]:
        key = (i["supplier"], l["product"])
        if i["split"] == "test" and key in seen: rep += 1
        if i["split"] == "train": seen.add(key)
print(f"{ind}: {len(cat)} products, {len(suppliers)} suppliers x {RANGE}; train {N_TRAIN} inv / {L('train')} lines; locked {N_INV - N_TRAIN} inv / {L('test')} lines; "
      f"locked lines whose supplier already sold that product in training: {rep} ({100 * rep / L('test'):.0f} %); products with a wording pool: {sum(1 for p in ids if pool[p])}")
