"""Collect the round-by-round runs (cmd/adaptive) and the Spark-alone baseline (cmd/sparkalone) into one
summary: bench/results.json and a printed table per industry.

    python -I summarize.py E:\\invoices\\bench
"""
import sys, json
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
B = Path(sys.argv[1]); PFX = sys.argv[2]; HERE = Path(__file__).parent
RP = "replay-stream-{}.json" if PFX == "stream" else "replay-{}.json"
INDS = ["supermarket", "pharmacy", "electronics", "cosmetics"]
out = {}
for ind in INDS:
    rp = B / f"{PFX}-{ind}" / "report.json"
    if not rp.exists():
        print(ind, "not finished"); continue
    r = json.load(open(rp, encoding="utf-8"))
    names = {p["id"]: p["name"] for p in json.load(open(HERE / RP.format(ind), encoding="utf-8"))["products"]}
    curve = []
    for ph in sorted((k for k in r["phases"] if k.startswith("test after")), key=lambda k: int(k.split()[-1])):
        a, n = r["phases"][ph], int(ph.split()[-1])
        free = a["Alias"] + a["Rules"]
        curve.append({"after": n, "lines": a["Lines"], "free": free, "spark": a["AI"], "asked": a["Asked"], "wrong": a["Wrong"],
                      "auto": free + a["AI"], "checked": a["AgreeRight"], "cost": round(a["Cost"], 4)})
    train = [r["phases"][k] for k in ("spark-checks-all", "learning") if k in r["phases"]]
    errs = [{"invoice": x["invoice"], "after": x["phase"], "said": e["said"], "got": names.get(e["got"], e["got"]), "want": names.get(e["want"], e["want"]), "level": e["level"]}
            for x in r["rows"] if x["phase"].startswith("test") for e in (x.get("errors") or [])]
    rules = [{"id": x.get("ID") or x.get("id"), "status": x.get("Status") or x.get("status"), "sentence": x.get("Sentence") or x.get("sentence"),
              "author": x.get("Author") or x.get("author")} for x in (r.get("rules") or [])]
    lex = [{"from": e.get("From") or e.get("from"), "to": e.get("To") or e.get("to"), "status": e.get("Status") or e.get("status")}
           for e in ((r.get("profile") or {}).get("lexicon") or (r.get("profile") or {}).get("Lexicon") or [])]
    al = B / f"alone-{PFX}-{ind}" / "report.json"
    alone = json.load(open(al, encoding="utf-8")) if al.exists() else None
    out[ind] = {"curve": curve, "train_lines": sum(t["Lines"] for t in train), "train_wrong": sum(t["Wrong"] for t in train),
                "spent": round(r["spent"], 4), "errors": errs, "rules": rules, "lexicon": lex,
                "industry": (r.get("profile") or {}).get("industry") or (r.get("profile") or {}).get("Industry"),
                "alone": alone and {k: alone.get(k) for k in ("Lines", "Right", "Wrong", "Unsure", "RightAt", "WrongAt", "AskedAt", "cost")},
                "alone_errors": alone and [{"said": e["said"], "got": names.get(e["got"], e["got"]), "want": names.get(e["want"], e["want"]), "conf": e["confidence"]} for e in alone.get("errors") or []]}
    o = out[ind]
    print(f"\n== {ind} ({o['industry']}) — locked {curve[0]['lines'] if curve else '?'} lines, run cost ${o['spent']}")
    print("  after | free | Spark | asked | wrong | auto %")
    for c in curve:
        print(f"  {c['after']:>5} | {c['free']:>4} | {c['spark']:>5} | {c['asked']:>5} | {c['wrong']:>5} | {100*c['auto']/c['lines']:.0f} %")
    if o["alone"]:
        a = o["alone"]; print(f"  Spark alone: right {a['Right']}, wrong {a['Wrong']}, unsure {a['Unsure']}; at 0.9: right {a['RightAt']}, wrong {a['WrongAt']}, asked {a['AskedAt']} (${a['cost']:.4f})")
    for e in errs: print("  WRONG", e["after"], e["invoice"], "|", e["said"], "→", e["got"], "| want", e["want"], "|", e["level"])
    print("  rules:", len(rules), [x["sentence"] for x in rules][:12])
(HERE / f"results-{PFX}.json").write_text(json.dumps(out, ensure_ascii=False, indent=1), encoding="utf-8")
