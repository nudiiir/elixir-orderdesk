"""Turn one industry's invoices (answer key) and catalogue into a replay file for cmd/adaptive.

Split, fixed before any run (seed 2026): for every supplier with 3 invoices, one invoice is locked away
(test); the other 30 are shuffled into the training order, read in rounds of 10. Each line is the text
printed on the paper, its quantity, unit price and the right catalogue id.

    python bench/scripts/make_replay.py supermarket|pharmacy|cosmetics|electronics

Reads bench/invoices/truth-<industry>.json and bench/catalogs/<industry>.csv; writes replay-<industry>.json to
$BENCH_OUT (default ./bench-out). The committed bench/replay/photographed-<industry>.json came from this split.
"""
import sys, json, csv, random, collections, re
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
sys.path.insert(0, str(Path(__file__).resolve().parent))
import paths
ind = paths.industry_arg(__doc__)
truth = json.load(open(paths.need(paths.truth(ind), __doc__, "answer key"), encoding="utf-8"))
cat = list(csv.DictReader(open(paths.need(paths.catalog(ind), __doc__, "catalogue"), encoding="utf-8-sig")))
R = random.Random(2026)

by_sup = collections.defaultdict(list)
for inv in truth: by_sup[inv["supplier"]].append(inv["id"])
locked = {R.choice(ids) for s, ids in sorted(by_sup.items()) if len(ids) >= 3}
if len(locked) < 10:  # top up from suppliers with 2 invoices: the other one stays in training
    pairs = [ids for s, ids in sorted(by_sup.items()) if len(ids) == 2]
    for ids in R.sample(pairs, 10 - len(locked)): locked.add(R.choice(ids))
train = [inv for inv in truth if inv["id"] not in locked]
R.shuffle(train)
test = [inv for inv in truth if inv["id"] in locked]

def month(d):  # dd/mm/yyyy or dd.mm.yyyy -> yyyy-mm
    dd, mm, yy = re.split(r"[/.\-]", d)[:3]
    return f"{yy[:4]}-{mm}"

# the same presentation listed twice in the catalogue (names equal once spaces and punctuation go): both are right
norm = lambda s: re.sub(r"[^A-Z0-9]+", "", s.upper())
twins = collections.defaultdict(list)
for p in cat: twins[norm(p["name"])].append(p["id"])
ID2N = {p["id"]: norm(p["name"]) for p in cat}

def lines(inv):
    out = []
    for l in inv["lines"]:
        x = {"text": l["printed"], "qty": float(l.get("qty", l.get("qte", 0))), "unit_price": float(l.get("pu") or 0), "product": l["product_id"]}
        acc = [i for i in twins[ID2N[l["product_id"]]] if i != l["product_id"]]
        if acc: x["accept"] = acc
        out.append(x)
    return out

out = {"invoices": [{"id": inv["id"], "supplier": inv["supplier"], "month": month(inv["date"]), "split": split,
                     "layout": inv["layout"], "lines": lines(inv)}
                    for split, invs in (("train", train), ("test", test)) for inv in invs],
       "products": [{"id": p["id"], "name": p["name"], "brand": p.get("brand", ""), "code": ""} for p in cat]}
ids = {p["id"] for p in cat}
bad = [l["product"] for inv in out["invoices"] for l in inv["lines"] if l["product"] not in ids]
assert not bad, f"{len(bad)} answer-key ids not in the catalogue: {bad[:5]}"
(paths.out_dir() / f"replay-{ind}.json").write_text(json.dumps(out, ensure_ascii=False), encoding="utf-8")
n = lambda invs: sum(len(i["lines"]) for i in invs)
print(f"{ind}: {len(cat)} products; train {len(train)} invoices / {n(train)} lines; locked {len(test)} invoices / {n(test)} lines")
print("training order:", " ".join(i["id"] for i in train))
print("locked:", " ".join(f'{i["id"]}({i["layout"]})' for i in test))
