"""Benchmark charts for the README, light and dark (GitHub picks one with <picture>).

Row 1, one panel per industry: what happened to the locked lines after each round of learning, as a share
of lines (free = no model call; Muse Spark = checked or picked by the model; asked = a person). Row 2: Meta
cost per invoice of the trained desk against Muse Spark alone (whole catalogue in every call).

    python -I charts.py <out_dir>
"""
import sys, json
from pathlib import Path
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.ticker import PercentFormatter

HERE = Path(__file__).parent; OUT = Path(sys.argv[1]); OUT.mkdir(parents=True, exist_ok=True)
S = json.load(open(HERE / "results-stream.json", encoding="utf-8"))
INDS = [i for i in ["electronics", "pharmacy", "cosmetics", "supermarket"] if i in S]
LABEL = {"supermarket": "Supermarket", "pharmacy": "Pharmacy", "electronics": "Electronics", "cosmetics": "Cosmetics & parapharmacy"}
THEMES = {
    "light": dict(surface="#fcfcfb", text="#0b0b0b", muted="#52514e", grid="#e4e3df", s1="#2a78d6", s2="#eb6834", s3="#1baf7a", base="#8a8984"),
    "dark": dict(surface="#1a1a19", text="#f0efec", muted="#c3c2b7", grid="#34332f", s1="#3987e5", s2="#d95926", s3="#199e70", base="#8f8e88"),
}

YMAX = 1.2 * max([c["cost"] / 30 for i in INDS for c in S[i]["curve"]] + [S[i]["alone"]["cost"] / 30 for i in INDS if S[i].get("alone")])

def draw(mode):
    t = THEMES[mode]
    plt.rcParams.update({"font.family": "Segoe UI", "font.size": 10, "text.color": t["text"], "axes.labelcolor": t["muted"],
                         "xtick.color": t["muted"], "ytick.color": t["muted"], "axes.edgecolor": t["grid"]})
    fig, axes = plt.subplots(2, len(INDS), figsize=(3.3 * len(INDS), 6.4), facecolor=t["surface"], gridspec_kw={"height_ratios": [1.35, 1]})
    for k, ind in enumerate(INDS):
        o = S[ind]; cv = o["curve"]
        xs = [c["after"] for c in cv]; L = [c["lines"] for c in cv]
        free = [(c["free"] - c.get("checked", 0)) / n for c, n in zip(cv, L)]
        spark = [(c.get("checked", 0) + c["spark"]) / n for c, n in zip(cv, L)]
        asked = [c["asked"] / n for c, n in zip(cv, L)]
        ax = axes[0][k]; ax.set_facecolor(t["surface"])
        pos = list(range(len(xs))); w = 0.62
        b1 = ax.bar(pos, free, w, color=t["s1"], edgecolor=t["surface"], linewidth=2, label="free (no model call)")
        b2 = ax.bar(pos, spark, w, bottom=free, color=t["s2"], edgecolor=t["surface"], linewidth=2, label="Muse Spark (checks or picks)")
        b3 = ax.bar(pos, asked, w, bottom=[a + b for a, b in zip(free, spark)], color=t["s3"], edgecolor=t["surface"], linewidth=2, label="asked a person")
        for p, f in zip(pos, free):
            if f < 0.05: continue
            ax.text(p, f / 2, f"{100 * f:.0f}%", ha="center", va="center", color="#ffffff", fontsize=9, fontweight="bold")
        wrong = sum(c["wrong"] for c in cv[-1:])
        ax.set_title(f"{LABEL[ind]}", loc="left", fontsize=11, fontweight="bold", color=t["text"], pad=18)
        ax.text(0, 1.03, f"{L[-1]} locked lines · wrong at the end: {wrong}", transform=ax.transAxes, fontsize=8.5, color=t["muted"])
        ax.set_xticks(pos, [str(x) for x in xs]); ax.set_ylim(0, 1); ax.yaxis.set_major_formatter(PercentFormatter(1, decimals=0))
        ax.grid(axis="y", color=t["grid"], linewidth=0.8); ax.set_axisbelow(True)
        for s in ("top", "right", "left"): ax.spines[s].set_visible(False)
        if k: ax.set_yticklabels([])
        ax.set_xlabel("invoices learned")
        # cost per invoice
        ax2 = axes[1][k]; ax2.set_facecolor(t["surface"])
        cost = [c["cost"] / 30 for c in cv]
        alone = o["alone"]["cost"] / 30 if o.get("alone") else None
        ax2.plot(xs, cost, color=t["s1"], linewidth=2, marker="o", markersize=7, markeredgecolor=t["surface"], markeredgewidth=2, label="Order Desk")
        if alone:
            ax2.axhline(alone, color=t["base"], linewidth=2, linestyle=(0, (4, 3)), label="Muse Spark alone")
            ax2.text(xs[-1], alone, f"Spark alone ${alone:.3f}", ha="right", va="bottom", fontsize=8.5, color=t["muted"])
        ax2.text(xs[-1], cost[-1], f"${cost[-1]:.4f}", ha="right", va="bottom", fontsize=8.5, color=t["text"])
        ax2.set_ylim(0, YMAX); ax2.set_xticks(xs)
        ax2.yaxis.set_major_formatter(lambda v, _: f"${v:.3f}")
        ax2.grid(axis="y", color=t["grid"], linewidth=0.8); ax2.set_axisbelow(True)
        for s in ("top", "right", "left"): ax2.spines[s].set_visible(False)
        if k: ax2.set_yticklabels([])
        else: ax2.set_ylabel("Meta cost per invoice")
        ax2.set_xlabel("invoices learned")
    h, l = axes[0][0].get_legend_handles_labels()
    h2, l2 = axes[1][0].get_legend_handles_labels()
    fig.legend(h + h2, l + l2, loc="lower center", ncol=5, frameon=False, fontsize=9, labelcolor=t["text"], bbox_to_anchor=(0.5, -0.01))
    fig.suptitle("Locked invoices the desk never learns from: more free, fewer questions, cheaper every round", x=0.01, ha="left", fontsize=12.5, fontweight="bold", color=t["text"])
    fig.tight_layout(rect=(0, 0.05, 1, 0.95))
    fig.savefig(OUT / f"benchmark-{mode}.png", dpi=160, facecolor=t["surface"])
    plt.close(fig)

for m in THEMES: draw(m)
print("written", [str(OUT / f"benchmark-{m}.png") for m in THEMES])
