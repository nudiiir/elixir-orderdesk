"""Recompute every headline number of the README's benchmark from the committed run reports and replay files.
No API key, no network, Python 3 standard library only.

    python bench/check.py
"""
import json
import sys
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).parent
INDS = ["electronics", "cosmetics", "supermarket", "pharmacy"]
pct = lambda a, b: f"{100 * a / b:.0f} %"
# ASSUMPTION, not a measurement: a clerk finds the product in the ERP and types one line in about 20 seconds
SECONDS_PER_LINE = 20


def load(*p):
    return json.load(open(HERE.joinpath(*p), encoding="utf-8"))


def tests(report):
    """Locked-invoice measurements, by number of training invoices read before them."""
    out = {}
    for k, a in report["phases"].items():
        if k.startswith("test after"):
            out[int(k.split()[-1])] = a
    return dict(sorted(out.items()))


auto_of = lambda a: a["Alias"] + a["Rules"] + a["AI"]

print("Invoice stream: 30 locked invoices per industry, scored before training and after 120 learned")
print(f"{'industry':12} {'lines':>5}  {'automatic':>17}  {'free':>5}  {'asked':>5}  {'wrong':>5}  {'$/invoice':>9}   "
      f"Spark alone: auto, wrong, $/invoice")
T = dict(lines=0, auto=0, free=0, wrong=0, cost=0.0, inv=0, a_auto=0, a_wrong=0, a_cost=0.0, a_inv=0)
for ind in INDS:
    t = tests(load("results", "stream", ind, "report.json"))
    a0, a = t[min(t)], t[max(t)]
    auto0, auto = auto_of(a0), auto_of(a)
    # "free" = booked by the code with no model call at all: Spark's spot checks of code answers are model calls
    free = a["Alias"] + a["Rules"] - a["AgreeRight"]
    n = a["Lines"]
    alone = load("results", "spark-alone-stream", ind, "report.json")
    print(f"{ind:12} {n:>5}  {pct(auto0, n):>6} -> {pct(auto, n):>6}  {pct(free, n):>5}  {pct(a['Asked'], n):>5}  "
          f"{a['Wrong']:>5}  {a['Cost'] / 30:>9.4f}   {pct(alone['RightAt'] + alone['WrongAt'], alone['Lines'])}, "
          f"{alone['WrongAt']}, {alone['cost'] / alone['Invoices']:.3f}")
    T["lines"] += n; T["auto"] += auto; T["free"] += free; T["wrong"] += a["Wrong"]; T["cost"] += a["Cost"]; T["inv"] += 30
    T["a_auto"] += alone["RightAt"] + alone["WrongAt"]; T["a_wrong"] += alone["WrongAt"]
    T["a_cost"] += alone["cost"]; T["a_inv"] += alone["Invoices"]
print(f"{'all':12} {T['lines']:>5}  automatic {pct(T['auto'], T['lines'])}, free {pct(T['free'], T['lines'])}, "
      f"wrong {T['wrong']} of {T['lines']}, ${T['cost'] / T['inv']:.4f}/invoice")
print(f"{'Spark alone':12} {T['lines']:>5}  automatic {pct(T['a_auto'], T['lines'])}, wrong {T['a_wrong']} of {T['lines']}, "
      f"${T['a_cost'] / T['a_inv']:.4f}/invoice")

print("\nThe learning curve on the locked invoices: automatic, wrong, $ per invoice, after each round of 30")
for ind in INDS:
    t = tests(load("results", "stream", ind, "report.json"))
    print(f"{ind:12} " + "  ".join(f"{k:>3}: {pct(auto_of(a), a['Lines']):>4} {a['Wrong']} ${a['Cost'] / a['Invoices']:.4f}"
                                   for k, a in t.items()))

print("\nLocked lines whose (supplier, printed wording) pair already appeared in training")
for st in ("stream", "photographed"):
    n_all = seen_all = 0
    row = []
    for ind in INDS:
        r = load("replay", f"{st}-{ind}.json")
        seen = {(i["supplier"], l["text"]) for i in r["invoices"] if i["split"] == "train" for l in i["lines"]}
        locked = [(i["supplier"], l["text"]) for i in r["invoices"] if i["split"] == "test" for l in i["lines"]]
        k = sum(x in seen for x in locked)
        n_all += len(locked); seen_all += k
        row.append(f"{ind} {pct(k, len(locked))}")
    print(f"{st:12} " + ", ".join(row) + f"; all {seen_all} of {n_all} ({pct(seen_all, n_all)})")

print("\nWho made the free answers possible: every locked line answered free, summed over the five test rounds")
PLAIN = "catalogue match through the matching Muse Spark designed"
C = dict(free=0, spark_found=0, spark_designed=0, person=0, plain=0)
for ind in INDS:
    c = load("results", "stream", ind, "report.json")["credit"]
    plain = c["detail"].get(PLAIN, 0)
    for k in ("free", "spark_found", "spark_designed", "person"):
        C[k] += c[k]
    C["plain"] += plain
    print(f"{ind:12} {c['free']:>5} free  a wording Spark picked or a rule it wrote {pct(c['spark_found'], c['free']):>5}  "
          f"a plain catalogue match {pct(plain, c['free']):>5}  a wording first matched that way "
          f"{pct(c['spark_designed'] - plain, c['free']):>5}  a person {pct(c['person'], c['free']):>4}")
print(f"{'all':12} {C['free']:>5} free  a wording Spark picked or a rule it wrote {pct(C['spark_found'], C['free']):>5}  "
      f"a plain catalogue match {pct(C['plain'], C['free']):>5}  a wording first matched that way "
      f"{pct(C['spark_designed'] - C['plain'], C['free']):>5}  a person {pct(C['person'], C['free']):>4}")
print("  cmd/adaptive/credit.go credits a plain catalogue match to the matching Muse Spark designed (its normaliser,")
print("  vocabulary and search), whatever share the generic normaliser played")

print("\nPhotographed set (cold start: almost no wording repeats): 10 locked invoices per industry, after 30 learned,")
print("the desk vs Muse Spark alone (whole catalogue in every prompt, 0.9 bar) on the same locked lines")
P = dict(lines=0, auto=0, wrong=0, cost=0.0, inv=0, a_auto=0, a_wrong=0, a_cost=0.0, a_inv=0)
for ind in INDS:
    t = tests(load("results", "photographed", ind, "report.json"))
    a = t[max(t)]
    al = load("results", "spark-alone-photographed", ind, "report.json")
    auto, a_auto = auto_of(a), al["RightAt"] + al["WrongAt"]
    print(f"{ind:12} {a['Lines']:>5} lines  desk: automatic {pct(auto, a['Lines']):>4}, asked {a['Asked']:>2}, wrong {a['Wrong']}, "
          f"${a['Cost'] / a['Invoices']:.4f}/invoice   Spark alone: automatic {pct(a_auto, al['Lines']):>4}, asked {al['AskedAt']:>2}, "
          f"wrong {al['WrongAt']}, ${al['cost'] / al['Invoices']:.4f}/invoice")
    P["lines"] += a["Lines"]; P["auto"] += auto; P["wrong"] += a["Wrong"]; P["cost"] += a["Cost"]; P["inv"] += a["Invoices"]
    P["a_auto"] += a_auto; P["a_wrong"] += al["WrongAt"]; P["a_cost"] += al["cost"]; P["a_inv"] += al["Invoices"]
print(f"{'all':12} {P['lines']:>5} lines  desk: automatic {pct(P['auto'], P['lines'])}, wrong {P['wrong']}, "
      f"${P['cost'] / P['inv']:.4f}/invoice   Spark alone: automatic {pct(P['a_auto'], P['lines'])}, wrong {P['a_wrong']}, "
      f"${P['a_cost'] / P['a_inv']:.4f}/invoice")

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

print(f"\nLabour: lines that reach the ERP with nobody typing them (ASSUMPTION: {SECONDS_PER_LINE} s of clerk time per line)")
inv_right = T["auto"] - T["wrong"]
o = ch["all"]
print(f"invoices     per 100: {100 * inv_right / T['inv']:.0f} of {100 * T['lines'] / T['inv']:.0f} lines booked right "
      f"with no typing = {100 * inv_right / T['inv'] * SECONDS_PER_LINE / 3600:.1f} clerk hours")
print(f"shop orders  per 100: {100 * o['Right'] / o['Orders']:.0f} of {100 * o['Lines'] / o['Orders']:.0f} lines booked right "
      f"with no typing = {100 * o['Right'] / o['Orders'] * SECONDS_PER_LINE / 3600:.1f} clerk hours; the rest are asked to the shop "
      f"(buttons) or were missed by the reader; {o['Wrong']} of {o['Lines']} booked wrong")
