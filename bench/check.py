"""Recompute every headline number of the README's benchmark from the committed run reports. No API key, no
network, Python 3 standard library only.

    python bench/check.py
"""
import json
import sys
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).parent
INDS = ["electronics", "cosmetics", "supermarket", "pharmacy"]
pct = lambda a, b: f"{100 * a / b:.0f} %"


def load(*p):
    return json.load(open(HERE.joinpath(*p), encoding="utf-8"))


def tests(report):
    """Locked-invoice measurements, by number of training invoices read before them."""
    out = {}
    for k, a in report["phases"].items():
        if k.startswith("test after"):
            out[int(k.split()[-1])] = a
    return dict(sorted(out.items()))


print("Invoice stream: 30 locked invoices per industry, scored before training and after 120 learned")
print(f"{'industry':12} {'lines':>5}  {'automatic':>17}  {'free':>5}  {'asked':>5}  {'wrong':>5}  {'$/invoice':>9}   "
      f"Spark alone: auto, wrong, $/invoice")
T = dict(lines=0, auto=0, free=0, wrong=0, cost=0.0, inv=0)
for ind in INDS:
    t = tests(load("results", "stream", ind, "report.json"))
    a0, a = t[min(t)], t[max(t)]
    auto0 = a0["Alias"] + a0["Rules"] + a0["AI"]
    auto = a["Alias"] + a["Rules"] + a["AI"]
    # "free" = booked by the code with no model call at all: Spark's spot checks of code answers are model calls
    free = a["Alias"] + a["Rules"] - a["AgreeRight"]
    n = a["Lines"]
    alone = load("results", "spark-alone-stream", ind, "report.json")
    print(f"{ind:12} {n:>5}  {pct(auto0, n):>6} -> {pct(auto, n):>6}  {pct(free, n):>5}  {pct(a['Asked'], n):>5}  "
          f"{a['Wrong']:>5}  {a['Cost'] / 30:>9.4f}   {pct(alone['RightAt'] + alone['WrongAt'], alone['Lines'])}, "
          f"{alone['WrongAt']}, {alone['cost'] / alone['Invoices']:.3f}")
    T["lines"] += n; T["auto"] += auto; T["free"] += free; T["wrong"] += a["Wrong"]
print(f"{'all':12} {T['lines']:>5}  automatic {pct(T['auto'], T['lines'])}, free {pct(T['free'], T['lines'])}, "
      f"wrong {T['wrong']} of {T['lines']}")

print("\nPhotographed set (cold start): 10 locked invoices per industry, after 30 learned")
for ind in INDS:
    t = tests(load("results", "photographed", ind, "report.json"))
    a = t[max(t)]
    print(f"{ind:12} {a['Lines']:>5} lines  automatic {pct(a['Alias'] + a['Rules'] + a['AI'], a['Lines'])}, "
          f"asked {a['Asked']}, wrong {a['Wrong']}")

print("\nShop orders: 200 orders, the desks trained on the stream above, nothing learned from the orders")
ch = {}
for ind in INDS:
    for k, s in load("results", "orders", ind, "report.json")["summary"].items():
        c = ch.setdefault(k, dict.fromkeys(["Orders", "Lines", "Right", "QtyOK", "Asked", "Wrong", "Missed", "Cost"], 0))
        for f in c:
            c[f] += s.get(f, 0)
for k in ["french", "darja_latin", "mixed", "screenshot", "voice", "all"]:
    c = ch[k]
    print(f"{k:12} {c['Orders']:>4} orders {c['Lines']:>5} lines  right {pct(c['Right'], c['Lines']):>5}  "
          f"right qty+unit {pct(c['QtyOK'], c['Right']):>5}  asked {pct(c['Asked'], c['Lines']):>5}  wrong {c['Wrong']}  "
          f"missed {pct(c['Missed'], c['Lines']):>4}  ${c['Cost'] / c['Orders']:.3f}/order")
