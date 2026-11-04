"""Score the two invoice readers on the real delivery-note photos, from the committed run (no key, no model call).

    python bench/real-photos/score.py

Photos: bench/real-photos/SM-*.jpg, 23 delivery notes, sales slips and invoices from wholesalers to one supermarket,
photographed on a phone, cropped to the product table (names, phones, balances, signatures cut; no EXIF).
Answer key: truth.json, every printed line with its quantity, unit price and the row's printed columns.
Run: bench/results/real-photos/report.json, what Muse Spark and Muse Wonder each read (cmd/readbench).

Two scores are printed. Strict is readbench's own rule: quantity and unit price equal to the key's, 80 % of the
words shared. Fair accepts two things that are on the slip and not reading errors: a reference code printed before
the name ("ESS/ECO/NEW", "ر3") kept or left out, and the quantity as printed in packs (20) or as units (20 x 10).
"""
import json
import re
import sys
import unicodedata
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).resolve().parent
truth = {i["image"]: i for i in json.load(open(HERE / "truth.json", encoding="utf-8"))["invoices"]}
report = json.load(open(HERE.parent / "results" / "real-photos" / "report.json", encoding="utf-8"))


def words(s, fair):
    s = unicodedata.normalize("NFKC", s).upper()
    ws = re.findall(r"\w+(?:[/.,]\w+)*", s)
    if not fair:
        return ws
    return [w for w in ws if "/" not in w and not re.fullmatch(r"\d+|[^\W\d]{1,2}\d+|\d+[^\W\d]{1,2}", w)
            or re.fullmatch(r"\d+(G|KG|L|ML|CL|GR|RLX|غ|كغ|كلغ)", w)]


def overlap(a, b, fair):
    a, b = set(words(a, fair)), set(words(b, fair))
    return len(a & b) / max(len(a), len(b)) if a and b else 0.0


def nums(s):
    return {float(x.replace(" ", "").replace(",", ".")) for x in re.findall(r"\d[\d ]*(?:[.,]\d+)?", s or "")}


def same(a, b):
    return abs(a - b) <= max(0.01, 0.005 * abs(b))


def strip_code(line):
    """Leave out a reference code printed in its own column before the name."""
    for c in (line.get("printed") or "").split(" | ")[:3]:
        c = c.strip()
        if c and " " not in c and len(c) <= 16 and line["text"].startswith(c + " "):
            return line["text"][len(c) + 1:]
    return line["text"]


def score(read, key, fair):
    right, used = [], set()
    for l in read:
        ok = False
        for j, k in enumerate(key):
            if j in used or not same(l["unit_price"], k["unit_price"]):
                continue
            q_ok = same(l["qty"], k["qty"]) or (fair and any(same(l["qty"], n) for n in nums(k.get("printed"))))
            if q_ok and overlap(l["text"], strip_code(k) if fair else k["text"], fair) >= 0.8:
                ok = True
                used.add(j)
                break
        right.append(ok)
    return right


for fair in (False, True):
    t = dict(lines=0, spark=0, wonder=0, agree=0, agree_wrong=0, caught=0, ar=0, ar_spark=0)
    for inv in report["invoices"]:
        key = truth[inv["image"]]
        s, w = inv["spark"]["lines"], inv["wonder"]["lines"]
        sr, wr = score(s, key["lines"], fair), score(w, key["lines"], fair)
        agree = score(s, [dict(l, printed="") for l in w], fair)
        for i, ok in enumerate(sr):
            if agree[i]:
                t["agree"] += 1
                t["agree_wrong"] += not ok
            elif not ok:
                t["caught"] += 1
        t["lines"] += len(key["lines"])
        t["spark"] += sum(sr)
        t["wonder"] += sum(wr)
        if key["language"] in ("ar", "mixed"):
            t["ar"] += len(key["lines"])
            t["ar_spark"] += sum(sr)
    L = t["lines"]
    print(f"{'fair' if fair else 'strict'}: {L} printed lines on {len(report['invoices'])} photos")
    print(f"  Muse Spark read right  {t['spark']} ({100 * t['spark'] / L:.1f} %), Arabic or mixed slips {t['ar_spark']}/{t['ar']}")
    print(f"  Muse Wonder read right {t['wonder']} ({100 * t['wonder'] / L:.1f} %)")
    print(f"  both readers agree on {t['agree']} lines, {t['agree_wrong']} of them wrong; "
          f"{t['caught']} wrong Spark readings the two-reader check sends to a person")
