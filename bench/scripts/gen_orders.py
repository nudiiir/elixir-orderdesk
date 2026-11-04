"""200 shop orders (50 per industry) for the Order Desk orders benchmark, seed 2028.

Each order: a shop, 3-10 products from the industry catalogue with a quantity and the unit the shop
counts in (the answer key), a written WhatsApp message in shop style, and a suggested voice script.
Shops write the way they talk: brand first, short, no packaging codes, sizes as they say them; some
names come from the real alias lists. Darja (Latin and Arabic script) and French, mixed.

    python bench/scripts/gen_orders.py      -> orders-<industry>.json, orders.json in $BENCH_OUT (default ./bench-out)

Reads bench/catalogs/<industry>.csv and bench/wordings/aliases-<industry>.csv.
"""
import sys, json, csv, random, re, collections
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
sys.path.insert(0, str(Path(__file__).resolve().parent))
import paths
OUT = paths.out_dir()
R = random.Random(2028)
CAT = {ind: paths.need(paths.catalog(ind), __doc__, "catalogue") for ind in paths.INDUSTRIES}
ALIAS = {ind: paths.need(paths.wordings(ind), __doc__, "wordings") for ind in paths.INDUSTRIES}
PFX = {"supermarket": "SUP", "pharmacy": "PHA", "cosmetics": "COS", "electronics": "ELE"}
SHOPS = {
    "supermarket": ["Supérette Ennour", "Alimentation El Baraka", "Épicerie Amine", "Supérette Yasmine", "Alimentation Djurdjura",
                    "Mini-market Rayan", "Épicerie du Coin", "Supérette Assalam", "Alimentation Bouzid", "Supérette El Hayat"],
    "pharmacy": ["Pharmacie Benali", "Pharmacie El Chifa", "Pharmacie du Centre", "Pharmacie Hamdi", "Pharmacie Ibn Sina",
                 "Pharmacie Ennakhil", "Pharmacie Saidi", "Pharmacie El Amel", "Pharmacie Rahmani", "Pharmacie de la Gare"],
    "cosmetics": ["Parapharmacie Lina", "Cosmétique Nour", "Para Beauté Plus", "Parfumerie Yasmine", "Para Santé Amira",
                  "Beauty Shop Sarah", "Para El Wafa", "Cosmétique Malak", "Para Bien-Être", "Hygiène Plus"],
    "electronics": ["Phone Store Adam", "Mobile City", "Accessoires Rayan", "Info Pro Bab Ezzouar", "Tech Shop Anis",
                    "Galaxy Phone", "Multiservices Yanis", "Smart Corner", "Phone Center Karim", "Univers Mobile"],
}

def strip_pack(s):
    s = re.sub(r"\([^)]*\)", " ", s)
    s = re.sub(r"(?i)(\d(?:G|GR|L|ML|CL|KG))\s?[X*]\s?\d{1,3}\b", r"\1", s)   # 700GX12: the supplier's pack
    s = re.sub(r"(\d)\*(?!\d)", r"\1", s)                                    # 11* = 11 inch
    s = re.sub(r"(?i)\b(?:B|BT|BTE|BOITE|C|S|T|FL|FLC|CRT|CTN)\s*/?\s*\d+\b|\bX\s?\d+\b|\*\s?\d+|\bOFFRE SPECIALE\b|\bPROMO\b", " ", s)
    return re.sub(r"\s+", " ", s).strip(" -/.,")

PHARMA_FORM_DROP = {"CP", "CPR", "COMP", "COMPRIME", "COMPRIMES", "PELLI", "PEL", "ENROB", "SEC", "GLES", "GELULES", "PDRE",
                    "SACHT", "BT", "BTE", "FL", "DISP", "TUBE", "T"}

def shop_name(ind, name, pool):
    """how a shop writes this product"""
    src = R.choice(pool) if pool and R.random() < 0.45 else name
    s = strip_pack(src.upper())
    if ind == "pharmacy":
        s = re.sub(r"(\d+(?:[.,]\d+)?)\s*MG\s*/\s*(\d+(?:[.,]\d+)?)\s*MG", lambda m: f"{m[1]}/{m[2]}" if R.random() < 0.6 else m[0], s)
        if R.random() < 0.5: s = re.sub(r"(\d+)\s*MG\b", r"\1", s)
        w = [x for x in s.split() if not (x in PHARMA_FORM_DROP and R.random() < 0.75)]
    else:
        w = s.split()
    keep = [w[0]] if w else []
    rest = w[1:]
    limit = {"supermarket": 3, "pharmacy": 4, "cosmetics": 4, "electronics": 4}[ind]
    descr = digits = 0
    for x in rest:
        if re.search(r"\d", x):
            if digits < 3: keep.append(x); digits += 1
        elif descr < limit: keep.append(x); descr += 1
    s = " ".join(keep)
    s = re.sub(r"(\d),(\d)", r"\1.\2", s)
    if ind == "supermarket" and R.random() < 0.4:
        s = re.sub(r"\b1L\b|\b1 L\b", "1l", s); s = re.sub(r"\b1\.5L\b|\b1500ML\b", "1.5", s)
    return s.lower() if R.random() < 0.8 else s

def qty_unit(ind, name):
    if ind == "supermarket":
        r = R.random()
        if r < 0.6: return R.choice([1, 1, 2, 2, 3, 3, 4, 5, 6, 10]), "carton"
        if r < 0.8: return R.choice([1, 2, 2, 3, 4, 5, 6, 10]), "pack"
        return R.choice([2, 3, 4, 6, 10, 12, 24]), "unit"
    if ind == "pharmacy": return R.choice([1, 2, 2, 3, 3, 4, 5, 5, 6, 10, 10, 12, 20, 30]), "unit"
    if ind == "cosmetics": return R.choice([2, 3, 4, 5, 6, 6, 10, 12, 12, 24]), "unit"
    big = re.search(r"(?i)laptop|notebook|thinkpad|latitude|elitebook|probook|macbook|mobile|tel |iphone \d|galaxy|redmi|oppo|console|xbox|playstation|trottinette|imprimante|ecran|tv\b|onduleur", name)
    return (R.choice([1, 1, 2, 2, 3, 5]) if big else R.choice([2, 3, 5, 5, 10, 10, 12, 20, 24, 50])), "unit"

DZ = {1: "wahed", 2: "jouj", 3: "tlata", 4: "rab3a", 5: "khamsa", 6: "setta", 7: "seb3a", 8: "tmanya", 9: "tes3a", 10: "3achra",
      12: "douze", 20: "vingt", 24: "vingt-quatre", 30: "trente", 50: "cinquante"}
FR = {1: "un", 2: "deux", 3: "trois", 4: "quatre", 5: "cinq", 6: "six", 7: "sept", 8: "huit", 9: "neuf", 10: "dix",
      12: "douze", 20: "vingt", 24: "vingt-quatre", 30: "trente", 50: "cinquante"}
AR_PLURAL = {"علبة": "علب", "حبة": "حبات", "كرطون": "كراطن", "فاردو": "فاردوات"}
AR = {1: "واحد", 2: "زوج", 3: "ثلاثة", 4: "ربعة", 5: "خمسة", 6: "ستة", 10: "عشرة", 12: "اثنا عشر", 20: "عشرين", 24: "أربعة وعشرين", 30: "ثلاثين", 50: "خمسين"}
UNIT_DZ = {"carton": ("kartona", "kartonat"), "pack": ("fardou", "fardouyat"), "unit": ("", "")}
UNIT_FR = {"carton": ("carton", "cartons"), "pack": ("pack", "packs"), "unit": ("", "")}
UNIT_AR = {"carton": "كرطون", "pack": "فاردو", "unit": ""}
UNIT_WORD = {"pharmacy": ("boîte", "boîtes", "boîte", "boîtes", "علبة"), "cosmetics": ("pièce", "pièces", "pièce", "pièces", "حبة"),
             "electronics": ("pièce", "pièces", "pièce", "pièces", "حبة"), "supermarket": ("pièce", "pièces", "pièce", "pièces", "حبة")}

def unit_txt(ind, unit, q, lang):
    if lang == "ar":
        w = UNIT_WORD[ind][4] if unit == "unit" else UNIT_AR[unit]
        return AR_PLURAL.get(w, w) if 2 < q <= 10 else w
    if unit == "unit":
        u = UNIT_WORD[ind]
        return u[0] if q == 1 else u[1]
    t = (UNIT_DZ if lang == "dz" else UNIT_FR)[unit]
    return t[0] if q == 1 else t[1]

def written(ind, lines):
    style = R.choices(["dz", "fr", "compact", "ar"], [0.4, 0.25, 0.2, 0.15])[0]
    if style == "compact":
        parts = [f"{l['say']} x{l['qty']}" if R.random() < 0.5 else f"{l['qty']} {l['say']}" for l in lines]
        return R.choice(["", "slm ", "salam "]) + ", ".join(parts), style
    lang = {"dz": "dz", "fr": "fr", "ar": "ar"}[style]
    hello = {"dz": ["slm", "salam", "salam khouya", "salam alikoum", "sbah lkhir khouya"], "fr": ["Bonjour", "Bonjour,", "Salam, bonjour"],
             "ar": ["السلام عليكم", "صباح الخير", "سلام خويا"]}[lang]
    bye = {"dz": ["saha", "baraka allahou fik", "merci khouya", "", "ab3athli lyoum ida mkach mochkil"], "fr": ["Merci", "Merci d'avance", "Merci, bonne journée", ""],
           "ar": ["بارك الله فيك", "شكرا", "صحة", ""]}[lang]
    rows = []
    for l in lines:
        u = unit_txt(ind, l["unit"], l["qty"], lang)
        bullet = "- " if style == "fr" and R.random() < 0.6 else ""
        rows.append(f"{bullet}{l['qty']} {u} {l['say']}".replace("  ", " ") if u or R.random() < 0.7 else f"{bullet}{l['say']} {l['qty']}")
    return "\n".join([R.choice(hello)] + rows + ([R.choice(bye)] if True else [])).strip(), style

SPOKEN = {"amp": "ampoules", "buv": "buvables", "eff": "effervescent", "sp": "sirop", "srp": "sirop", "supp": "suppositoires", "suppo": "suppositoires",
          "inj": "injectable", "pde": "pommade", "crm": "crème", "sol": "solution", "gtt": "gouttes", "gttes": "gouttes", "bb": "bébé", "pm": "petit modèle",
          "gm": "grand modèle", "bl-tooth": "bluetooth", "t-c": "type C", "s-fil": "sans fil", "tonner": "toner"}
SILENT = {"cp", "cpr", "comp", "gles", "pdre", "sacht", "sach", "bt", "bte", "pel", "pelli", "enrob", "sec", "gast-res", "disp", "fl", "flc", "t"}

def speak_name(s):
    s = " ".join(SPOKEN.get(w.lower(), w) for w in s.split() if w.lower() not in SILENT) or s
    s = re.sub(r"(\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)", r"\1 \2", s)
    s = re.sub(r"(?i)(\d)\s*ml\b", r"\1 ml", s); s = re.sub(r"(?i)(\d)\s*mg\b", r"\1 mg", s)
    return s

def voice(ind, lines):
    lang = R.choices(["dz", "fr", "ar"], [0.6, 0.3, 0.1])[0]
    items = []
    for l in lines:
        q = l["qty"]
        if lang == "ar": qw = AR.get(q, str(q))
        else: qw = (DZ if lang == "dz" else FR).get(q, str(q))
        u = unit_txt(ind, l["unit"], q, lang)
        if lang == "fr" and u: u += " de"
        items.append(" ".join(x for x in (qw, u, speak_name(l["say"])) if x))
    if lang == "dz":
        return f"{R.choice(['Salam khouya', 'Salam alikoum', 'Allo, salam'])}, {R.choice(['ab3athli', 'a3tini', 'jibli', 'n7eb'])} " + ", w ".join(items) + f". {R.choice(['Saha', 'Baraka allahou fik', 'Merci'])}.", lang
    if lang == "fr":
        return f"{R.choice(['Bonjour', 'Salam, bonjour'])}, {R.choice(['je voudrais', 'mettez-moi', 'il me faut'])} " + ", ".join(items[:-1]) + (" et " if len(items) > 1 else "") + items[-1] + ". Merci.", lang
    return "السلام عليكم، ابعثلي " + "، و ".join(items) + ". بارك الله فيك.", lang

allo = []
for ind in ["supermarket", "pharmacy", "cosmetics", "electronics"]:
    cat = list(csv.DictReader(open(CAT[ind], encoding="utf-8-sig")))
    pools = collections.defaultdict(list)
    for a in csv.DictReader(open(ALIAS[ind], encoding="utf-8-sig")):
        if a.get("source") == "real" and int(a.get("witnesses") or 0) < 2: continue
        if a.get("source", "").startswith("official arabic"): continue
        pools[a["id"]].append(a["alias"])
    norm = lambda s: re.sub(r"[^A-Z0-9]+", "", s.upper())
    owner = collections.defaultdict(set)
    for p in cat: owner[norm(p["name"])].add(p["id"])
    popular = R.sample(cat, 160)
    weights = [1 / (k + 1) ** 0.7 for k in range(len(popular))]
    out = []
    for n in range(50):
        shop = R.choice(SHOPS[ind]); k = R.randint(3, 10); chosen = []
        while len(chosen) < k:
            p = R.choices(popular, weights)[0]
            if p not in chosen: chosen.append(p)
        lines = []
        for p in chosen:
            q, u = qty_unit(ind, p["name"])
            say = shop_name(ind, p["name"], pools[p["id"]])
            for _ in range(5):  # a wording that is another product's exact catalogue name would name that product
                if not (owner.get(norm(say), {p["id"]}) - {p["id"]}): break
                say = shop_name(ind, p["name"], pools[p["id"]])
            else:
                say = strip_pack(p["name"]).lower()
            lines.append({"product_id": p["id"], "product": p["name"], "qty": q, "unit": u, "say": say})
        text, tstyle = written(ind, lines)
        script, vlang = voice(ind, lines)
        out.append({"id": f"{PFX[ind]}-O{n + 1:02}", "industry": ind, "shop": shop, "lines": lines, "text": text, "text_style": tstyle,
                    "voice_script": script, "voice_lang": vlang})
    (OUT / f"orders-{ind}.json").write_text(json.dumps(out, ensure_ascii=False, indent=1), encoding="utf-8")
    allo += out
    print(ind, len(out), "orders,", sum(len(o["lines"]) for o in out), "lines")
(OUT / "orders.json").write_text(json.dumps(allo, ensure_ascii=False, indent=1), encoding="utf-8")
