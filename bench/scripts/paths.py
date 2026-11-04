"""Where the benchmark scripts read and write, in the repository's own layout.

Inputs come from bench/: catalogs/<industry>.csv, wordings/aliases-<industry>.csv, invoices/truth-<industry>.json.
Outputs go to $BENCH_OUT, or ./bench-out (git-ignored) when it is not set. Raw sources that are not shipped are
passed on the command line or through an environment variable, never hard-coded.
"""
import os
import sys
from pathlib import Path

BENCH = Path(__file__).resolve().parent.parent
INDUSTRIES = ["electronics", "cosmetics", "supermarket", "pharmacy"]


def catalog(ind):
    return BENCH / "catalogs" / f"{ind}.csv"


def wordings(ind):
    return BENCH / "wordings" / f"aliases-{ind}.csv"


def truth(ind):
    return BENCH / "invoices" / f"truth-{ind}.json"


def out_dir():
    d = Path(os.environ.get("BENCH_OUT") or "bench-out")
    d.mkdir(parents=True, exist_ok=True)
    return d


def usage(doc, msg=""):
    """Print the script's docstring (and why it stopped) and exit with status 2."""
    if msg:
        print("error:", msg, file=sys.stderr)
    print((doc or "").strip(), file=sys.stderr)
    sys.exit(2)


def industry_arg(doc):
    """The industry named on the command line, or usage."""
    if len(sys.argv) < 2 or sys.argv[1] not in INDUSTRIES:
        usage(doc, "name one industry: " + " | ".join(INDUSTRIES))
    return sys.argv[1]


def need(path, doc, what):
    """Usage instead of a FileNotFoundError when an input is missing."""
    p = Path(path) if path else None
    if not p or not p.exists():
        usage(doc, f"{what} not found: {path or '(not given)'}")
    return p
