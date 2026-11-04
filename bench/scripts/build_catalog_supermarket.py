"""Supermarket catalogue for the benchmark: a real Algerian store's assortment (Siir), cleaned.

A product is kept only when its name is comprehensible:
  1. the store's barcode has an official GS1 Algeria (Mantooj) entry whose brand or name agrees with the
     store's own name (barcode misuse such as "pates de boeuf" -> a varnish is dropped); the name is
     BRAND + official designation + the flavour/size the store wrote; or
  2. no GS1 entry, but every word of the store's name is a known word (from the 22,769 GS1 names and
     brands) or a size: no typos, no private shorthand.
Output catalog.csv: id, name, brand, size. No prices, no store data.
"""
import csv, json, re, sys, unicodedata, collections
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).parent
M = {r["barcode"]: r for r in csv.DictReader(open(r"D:\other projects\scanner-cloud\tools\mantooj-full\mantooj.csv", encoding="utf-8-sig"))}
store = json.load(open(r"D:\Documents\ELIXIR SYSTEM\orderdesk-replay\data\replay-siir.json", encoding="utf-8"))["products"]

def fold(s):
    s = unicodedata.normalize("NFKD", s); s = "".join(c for c in s if not unicodedata.combining(c))
    s = re.sub(r"[\u0600-\u06FF]+", " ", s)  # Arabic script out of the Latin name
    return re.sub(r"\s+", " ", re.sub(r"[^A-Za-z0-9.,/%'&+ -]", " ", s)).strip().upper()
SIZE = re.compile(r"\b(\d+(?:[.,]\d+)?)\s?(KG|GR|G|GRS|L|CL|ML|LT|LTR|P|PCS|PC|PIECES|M|W|CM|MM)\b")
def size_of(s):
    m = SIZE.search(fold(s))
    if not m: return ""
    n, u = m.group(1).replace(",", "."), m.group(2)
    u = {"GR": "G", "GRS": "G", "LT": "L", "LTR": "L", "PCS": "P", "PC": "P", "PIECES": "P"}.get(u, u)
    n = n.rstrip("0").rstrip(".") if "." in n else n
    return n + u
def clean_brand(b):
    b = fold(b).split("/")[0]
    b = re.sub(r"\b(SARL|SPA|EURL|SNC|ETS|S\.?P\.?A|LES|GROUPE|INDUSTRIE|SOCIETE)\b", " ", b)
    return re.sub(r"\s+", " ", b).strip(" -")
STOP = set("DE DU LA LE LES DES ET A AU AUX EN POUR AVEC SANS L D UN UNE".split())
vocab = collections.Counter()
for r in M.values():
    for w in fold(r["name"] + " " + r["brand"]).split(): vocab[w] += 1
def toks(s): return {w for w in re.findall(r"[A-Z]{3,}", fold(s)) if w not in STOP}
def close(a, b):  # same word allowing one typo or a prefix ("SHOMP"/"SHAMPOOING", "CHIDAR"~no)
    if a == b or (len(a) >= 4 and (b.startswith(a) or a.startswith(b))): return True
    if abs(len(a) - len(b)) > 1 or len(a) < 4: return False
    return sum(x != y for x, y in zip(a, b)) + abs(len(a) - len(b)) <= 1
out, why = [], collections.Counter()
for p in store:
    bc, pos = p["id"][5:], p["name"]
    m = M.get(bc)
    if m and m["brand"] not in ("", "-"):
        brand, off = clean_brand(m["brand"]), fold(m["name"])
        off = re.sub(r"\s*-?\s*PRODUIT (SUCRE|ALLEGE|EDULCORE).*$", "", off)
        if not brand or not any(close(a, b) for a in toks(pos) for b in toks(brand + " " + off)):
            why["gs1 disagrees with the store name"] += 1; continue
        extra = [w for w in fold(pos).split() if len(w) >= 4 and w.isalpha() and w not in STOP
                 and not any(close(w, b) for b in toks(brand + " " + off)) and vocab[w] >= 3]
        size = size_of(pos)
        extra = [w for w in extra if not SIZE.fullmatch(w)]
        body = SIZE.sub(lambda m: "", " ".join([off] + extra[:2]))
        name = " ".join(x for x in [brand, body, size] if x)
        out.append(dict(id=f"SM-{bc}", name=re.sub(r"\s+", " ", name), brand=brand, size=size, source="gs1")); why["gs1"] += 1
    else:
        words = [w for w in fold(pos).replace("[", " ").replace("]", " ").split() if not SIZE.fullmatch(w)]
        alpha = [w for w in words if re.search(r"[A-Z]", w)]
        if len(alpha) < 2 or any(len(w) > 2 and not w.isdigit() and vocab[w] < 3 for w in alpha):
            why["unknown word or too short"] += 1; continue
        brand = fold(p.get("brand") or alpha[0])
        size = size_of(pos)
        body = SIZE.sub("", fold(pos).replace("[", " ").replace("]", " "))
        body = re.sub(r"(FR|PM|GM|MM)$", "", body.strip())
        out.append(dict(id=f"SM-{bc}", name=re.sub(r"\s+", " ", body + " " + size).strip(), brand=brand, size=size, source="store")); why["store name, all words known"] += 1
def dedup_words(n):
    ws, keep = n.split(), []
    for w in ws:
        if w not in keep or w.isdigit(): keep.append(w)
    return " ".join(keep)
for r in out: r["name"] = dedup_words(r["name"])
seen, final = set(), []
for r in out:
    k = r["name"]
    if k in seen: why["duplicate name"] += 1; continue
    seen.add(k); final.append(r)
with open(HERE / "catalog.csv", "w", encoding="utf-8-sig", newline="") as f:
    w = csv.DictWriter(f, fieldnames=["id", "name", "brand", "size", "source"]); w.writeheader(); w.writerows(final)
print(len(store), "store products ->", len(final), "kept"); print(dict(why))
