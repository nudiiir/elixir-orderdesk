"""Supermarket and electronics supplier documents for the benchmark (to print and photograph).

Supermarket: the formats copy the real delivery notes / sale slips photographed for the supermarket reading
benchmark (column sets, packaging columns like CART x COLISS, Qt./U. with 'P' for pieces, Fardeau(24),
thermal receipt, Arabic delivery note), and products are written the way those suppliers write them
(shorthand, pack in the name: "HERISSA CNAB 400G", "BINGO MATIC LIQ 2,5LX4").
Electronics: phone-accessory / IT wholesale formats (warranty column, colis x pieces, family sections).
No company ids, no phone numbers; suppliers are fictional and every page says so.

    GS1_CSV=<GS1 Algeria export.csv> python bench/scripts/gen2.py

GS1_CSV is a GS1 Algeria product export (barcode, brand, name_ar columns) for the official Arabic names; it is
not shipped. Needs openpyxl and a Chromium-based browser ($PDF_BROWSER, or Edge on Windows). Reads
bench/catalogs/; writes invoices-<kind>.pdf, truth-<kind>.json/.xlsx and aliases-<kind>.csv (the written
aliases) to $BENCH_OUT (default ./bench-out).
"""
import csv, json, os, random, re, html, subprocess, collections, sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))
from gen import words_da, fmt, EDGE, CITIES
sys.stdout.reconfigure(encoding="utf-8")
import paths
HERE = paths.out_dir() if __name__ == "__main__" else Path(__file__).parent
R = random.Random(20261104)
e = html.escape

# ------------------------------------------------------------------ aliases
SM_DROP = set("PREPARATION ALIMENTAIRE PRODUIT BOISSON GAZEIFIEE GAZEIFIER AROMATISE AROMATISEE CONFISERIE CONSERVE CONSERVERIE "
              "SPECIALITE FROMAGERE EN POUDRE SOLUBLE MOULU NATURE NATUREL CLASSIC CLASSIQUE".split())
SM_ABBR = {"CHOCOLAT": ["CHOCO", "CHOC"], "CONFITURE": ["CONFI", "CONF"], "BISCUIT": ["BISC", "BISCUIT"], "FROMAGE": ["FROM", "FRMG"],
           "LIMONADE": ["LIM", "LIMO"], "SHAMPOOING": ["SHAMP", "SHMP"], "LESSIVE": ["LESS"], "DETERGENT": ["DET"], "LIQUIDE": ["LIQ"],
           "VINAIGRE": ["VINAIG", "VNGR"], "MAYONNAISE": ["MAYO"], "CONCENTRE": ["CONC", "CC"], "YAOURT": ["YAOU", "YAOURT"],
           "GAUFRETTE": ["GAUF"], "MARGARINE": ["MARG"], "SARDINES": ["SARD"], "SARDINE": ["SARD"], "CAFE": ["CAFE"], "TOMATE": ["TOM", "TOMATE"],
           "COUCHE": ["CHE", "COUCHES"], "BEBE": ["BB"], "PAPIER": ["PAP"], "HYGIENIQUE": ["HYG"], "NETTOYANT": ["NETT"], "DESINFECTANT": ["DESINF"],
           "CREME": ["CR"], "BOUTEILLE": ["BTL"], "CANETTE": ["CNT", "CAN"], "SACHET": ["SACH"], "POMME": ["POM"], "TERRE": ["T"], "PATES": ["PATES", "PATE"],
           "CHEVEUX": ["CHEV"], "ORANGE": ["ORG"], "FRAISE": ["FRS"], "VANILLE": ["VAN"], "CITRON": ["CIT"]}
STOPW = set("DE DU LA LE LES DES ET A AU AUX EN L D POUR AVEC".split())
SIZE = re.compile(r"\b(\d+(?:[.,]\d+)?)(KG|G|L|CL|ML|P|M|W|CM|MM)\b")

def sm_aliases(name, brand, colis):
    w = [x for x in name.split() if x not in STOPW]
    size = next((x for x in w if SIZE.fullmatch(x)), "")
    core = [x for x in w if x != size and x not in SM_DROP]
    b = [x for x in brand.split() if x in core] or core[:1]
    rest = [x for x in core if x not in b]
    def ab(ws, p=0.6): return [R.choice(SM_ABBR[x]) if x in SM_ABBR and R.random() < p else x for x in ws]
    def sz(s, style):
        m = SIZE.fullmatch(s) if s else None
        if not m: return ""
        n, u = m.group(1), m.group(2)
        return {0: n + u, 1: n + (" GR" if u == "G" else " " + u), 2: n + ("GR" if u == "G" else u), 3: f"{n}{u}X{colis}", 4: f"{n}{u}*{colis}"}[style]
    out = set()
    tries = 0
    while len(out) < 7 and tries < 40:
        tries += 1
        st = R.randint(0, 4)
        form = R.randint(0, 3)
        if form == 0: t = ab(rest[:3]) + b + [sz(size, st)]
        elif form == 1: t = b + ab(rest[:3]) + [sz(size, st)]
        elif form == 2: t = b + ab(rest[:2], 0.9) + [sz(size, st)]
        else: t = ab(rest[:1]) + b + ab(rest[1:3]) + [sz(size, st)]
        s = re.sub(r"\s+", " ", " ".join(x for x in t if x)).strip()
        if s and s != name and len(s.split()) >= 2: out.add(s)
    return sorted(out)

EL_SYN = {"CABLE": ["CBL", "CABLE", "CÂBLE"], "CHARGEUR": ["CHARGER", "CHARG", "CHARGEUR"], "CHARGER": ["CHARGEUR", "CHARG"],
          "ECOUTEUR": ["ECOUT", "KIT", "EARPHONE"], "IP": ["IPHONE", "LIGHTNING", "IP"], "T-C": ["TYPE-C", "USB-C", "TYPE C"],
          "TYPE-C": ["T-C", "USB-C", "TYPE C"], "MICRO": ["MICRO USB", "V8", "MICRO"], "LAPTOP": ["PC PORTABLE", "NOTEBOOK", "LAPTOP"],
          "SOURIS": ["MOUSE", "SOURIS"], "CLAVIER": ["KEYBOARD", "CLAVIER"], "BAFFLE": ["HAUT PARLEUR", "SPEAKER", "BAFFLE"],
          "CASQUE": ["HEADSET", "CASQUE"], "POCHETTE": ["COQUE", "ETUI", "POCHETTE"], "GLASS": ["VERRE TREMPE", "GLASS", "PROTECTION ECRAN"],
          "SUPPORT": ["HOLDER", "SUPPORT"], "SAMSUNG": ["SAMSUNG GALAXY", "SAMSUNG", "SAM"], "REDMI": ["XIAOMI REDMI", "REDMI"],
          "MONTRE": ["WATCH", "MONTRE"], "TABLETTE": ["TAB", "TABLETTE"], "SANS-FIL": ["S-FIL", "WIRELESS", "SANS FIL"], "S-FIL": ["SANS FIL", "WIRELESS"],
          "BL-TOOTH": ["BLUETOOTH", "BT"], "FLASH": ["CLE USB", "FLASH"], "DISK": ["DISQUE", "DISK"], "POWER": ["POWERBANK", "POWER"], "MOBILE": ["TEL", "SMARTPHONE", ""]}
def el_aliases(name, brand):
    toks = name.split()
    out = set(); tries = 0
    while len(out) < 7 and tries < 40:
        tries += 1
        t = []
        for x in toks:
            if x in EL_SYN and R.random() < 0.6: x = R.choice(EL_SYN[x])
            m = re.fullmatch(r"(\d+)/(\d+)(GB|G)?", x)
            if m and R.random() < 0.7: x = R.choice([f"{m.group(1)}+{m.group(2)}", f"{m.group(1)}GB/{m.group(2)}GB", f"{m.group(2)}GB", f"{m.group(1)}-{m.group(2)}"])
            x = {"7EME": R.choice(["7TH", "7G", "7EME"]), "8TH": R.choice(["8EME", "8G", "8TH"]), "10TH": R.choice(["10EME", "10G"]), "11TH": R.choice(["11EME", "11G"]),
                 "12TH": R.choice(["12EME", "12G"]), "I5": R.choice(["CORE I5", "I5"]), "I7": R.choice(["CORE I7", "I7"])}.get(x, x)
            if x: t.append(x)
        if brand and brand in t and R.random() < 0.35:  # brand moved to the end / model first
            t.remove(brand); t.append(brand)
        s = re.sub(r"\s+", " ", " ".join(t)).strip()
        if s != name and len(s.split()) >= 2: out.add(s)
    return sorted(out)

AR = [("YAOURT", "ياغورت"), ("FROMAGE", "جبن"), ("BISCUIT", "بسكويت"), ("CONFITURE", "مربى"), ("HUILE", "زيت"), ("LAIT", "حليب"),
      ("CAFE", "قهوة"), ("SUCRE", "سكر"), ("PATES", "عجائن"), ("COUCHE", "حفاظات"), ("SHAMPOOING", "شامبو"), ("SAVON", "صابون"), ("LIMONADE", "مشروب غازي"),
      ("JUS", "عصير"), ("EAU", "ماء"), ("CHOCOLAT", "شوكولاطة"), ("GAUFRETTE", "قوفريت"), ("MARGARINE", "مرغرين"), ("SARDINE", "سردين"), ("TOMATE", "طماطم"),
      ("VINAIGRE", "خل"), ("MAYONNAISE", "مايونيز"), ("MOUTARDE", "خردل"), ("FARINE", "فرينة"), ("SEMOULE", "سميد"), ("RIZ", "رز"), ("LENTILLE", "عدس"),
      ("HARICOT", "لوبيا"), ("POIS CHICHE", "حمص"), ("THE", "شاي"), ("LESSIVE", "مسحوق غسيل"), ("JAVEL", "جافيل"), ("DETERGENT", "منظف"), ("NETTOYANT", "منظف"),
      ("PAPIER HYGIENIQUE", "ورق صحي"), ("MOUCHOIR", "مناديل"), ("GLACE", "مثلجات"), ("CHIPS", "شيبس"), ("BONBON", "حلوى"), ("CEREALE", "رقائق"),
      ("FLOCONS", "شوفان"), ("PUREE", "بيري"), ("SAUCE", "صلصة"), ("HARISSA", "هريسة"), ("EPICE", "توابل"), ("CANNELLE", "قرفة"), ("CAHIER", "كراس"),
      ("GANT", "قفازات"), ("SAC POUBELLE", "أكياس قمامة"), ("DENTIFRICE", "معجون أسنان"), ("BROSSE", "فرشاة"), ("DEODORANT", "مزيل عرق"), ("LAMPE", "مصباح")]
def ar_written(name, brand):
    for fr_w, ar_w in AR:
        if fr_w in name:
            size = next((x for x in name.split() if SIZE.fullmatch(x)), "")
            return re.sub(r"\s+", " ", f"{ar_w} {brand} {size}").strip()
    return ""

# ------------------------------------------------------------------ data
def load_sm():
    cat = list(csv.DictReader(open(paths.catalog("supermarket"), encoding="utf-8-sig")))
    ar = {}
    for r in csv.DictReader(open(os.environ["GS1_CSV"], encoding="utf-8-sig")):
        if r.get("name_ar"): ar[r["barcode"]] = (r["brand"] if r["brand"] and not re.search(r"[A-Za-z]", r["brand"]) else "") + " " + r["name_ar"]
    for p in cat:
        p["colis"] = R.choice([6, 8, 10, 12, 12, 16, 20, 24, 24, 48])
        p["ar"] = ar.get(p["id"].replace("SM-", ""), "").strip() or ar_written(p["name"], p["brand"] or p["name"].split()[0])
        p["aliases"] = sm_aliases(p["name"], p["brand"] or p["name"].split()[0], p["colis"])
        n = p["name"]
        p["price"] = round(R.uniform(*((25, 120) if re.search(r"YAOURT|BOISSON|LIMONADE|CNT|33CL|24CL|BISCUIT|BONBON|DRAGE", n) else
                                     (150, 900) if re.search(r"KG|1L|2L|1.5L|CAFE|COUCHE|LESSIVE|SHAMP|HUILE", n) else (60, 450))), 2)
    return cat
def load_el():
    cat = list(csv.DictReader(open(paths.catalog("electronics"), encoding="utf-8-sig")))
    for p in cat:
        p["aliases"] = el_aliases(p["name"], p["brand"])
        f, n = p["family"], p["name"]
        if f == "MOBILE": p["price"], p["qr"] = round(R.uniform(14000, 95000), -2), (2, 25)
        elif f == "TABLETTE": p["price"], p["qr"] = round(R.uniform(12000, 60000), -2), (2, 15)
        elif "DISQUE" in n or "SSD NVME" in n: p["price"], p["qr"] = round(R.uniform(5500, 22000), -1), (5, 60)
        elif re.search(r"LAPTOP|NOTEBOOK|THINKPAD|LATITUDE|ELITEBOOK|PROBOOK|VOSTRO|IDEAPAD|MACBOOK|INSPIRON|PAVILION|RYZEN|X1 CARBON|\bULTRA [579]\b|\bI[3579]\b|\d+/\d+ ?SSD", n): p["price"], p["qr"] = round(R.uniform(45000, 160000), -2), (1, 8)
        elif f == "INFORMATIQUE": p["price"], p["qr"] = round(R.uniform(400, 9000), -1), (5, 120)
        elif f in ("Montre", "AUDIOVISUEL", "CONSOLE"): p["price"], p["qr"] = round(R.uniform(1500, 25000), -1), (3, 40)
        else: p["price"], p["qr"] = round(R.uniform(150, 3500), -1), (10, 300)
        p["colis"] = R.choice([10, 20, 50, 100]) if p["price"] < 4000 else 1
    return cat

SUP_SM = ["EURL DIST ALIMENTAIRE EL BARAKA", "SARL HODNA DISTRIBUTION", "ETS BENSALEM & FILS", "SARL ATLAS GROS", "EURL AMANE FOOD", "SNC FRERES DJEBBAR",
          "SARL NEDJMA DISTRIBUTION", "EURL SAHARA HYGIENE", "SARL EL MANAR GROS", "ETS TELL ALIMENTATION", "SARL MEDJANA DIS", "EURL ZIBAN FOOD",
          "SARL AURES PRODUITS LAITIERS", "EURL RIMEL DISTRIBUTION", "SARL ESSALEM GROS"]
SUP_EL = ["SARL TECHNO PHONE DZ", "EURL MOBILAND GROS", "SARL DIGITAL WORLD", "EURL ACCESS PLUS", "SARL INFO TECH ALGERIE", "EURL SMART CONNECT",
          "SARL GSM CENTER GROS", "EURL PC LAND", "SARL ELECTRO MOBILE", "EURL HIGH TECH DIS", "SARL ROYAL PHONE", "EURL NET INFORMATIQUE",
          "SARL WIFI STORE GROS", "EURL CYBER ACCESSOIRES", "SARL ORBIT MOBILE"]
FOOT = "<div class=foot>Document de test — jeu de factures Order Desk (fournisseurs fictifs)</div>"

def pick_lines(cat, n, kind):
    lines = []
    for j, p in enumerate(R.sample(cat, n), 1):
        r = R.random()
        if r < 0.08 or not p["aliases"]: text, how = p["name"], "catalogue name"
        else: text, how = R.choice(p["aliases"]), "written alias"
        if kind == "sm":
            ctn = R.choice([1, 1, 2, 2, 3, 4, 5, 6, 8, 10, 12, 15, 20]); col = p["colis"]
            qty = ctn * col; pu = p["price"]
        else:
            ctn = 0; col = p["colis"]; qty = R.randint(*p["qr"])
            if col > 1: qty = max(col, round(qty / col) * col)
            pu = p["price"]
        lines.append(dict(no=j, product_id=p["id"], product=p["name"], printed=text, how=how, ctn=ctn, colis=col, qty=qty, pu=pu,
                          mt=round(qty * pu, 2), ar=p.get("ar", ""), family=p.get("family", ""), brand=p.get("brand", ""), ref=f"{R.randint(100, 99999)}"))
    return lines

# ------------------------------------------------------------------ supermarket formats (from the photographed documents)
def table(heads, rows, cls=""):
    return f"<table class='lines {cls}'><thead><tr>{''.join(f'<th>{h}</th>' for h in heads)}</tr></thead><tbody>" + \
           "".join("<tr>" + "".join(f"<td class='{c}'>{v}</td>" for c, v in r) + "</tr>" for r in rows) + "</tbody></table>"
N = lambda x, s="fr", d=2: fmt(x, s, d)
def date_s(): return f"{R.randint(1, 28):02}/{R.choice(['09', '10'])}/2026"

def sm_gecow(inv, var):  # Quantite | Qt./U. | Produit | Prix | Total
    rows = []
    for l in inv["lines"]:
        piece = R.random() < 0.15
        q, qu = (l["qty"], "P") if piece else (l["ctn"], str(l["colis"]))
        if piece: l["ctn"] = 0
        rows.append([("r", str(q)), ("r", qu), ("", e(l["printed"])), ("r", N(l["pu"])), ("r", N(l["mt"]))])
    head = f"<div class=gh><b>Bon de vente N° {R.randint(1000, 9999)}</b><span>Le {inv['date']} {R.randint(8, 18):02}:{R.randint(0, 59):02}:{R.randint(0, 59):02}</span></div><div>Client : {e(inv['client'])}</div>"
    solde = f"<div class=solde>Total : <b>{N(inv['ht'])}</b><br>Ancien solde : {N(inv['old'])}<br>Nouveau solde : {N(inv['old'] + inv['ht'])}</div>" if var else f"<div class=solde>Total : <b>{N(inv['ht'])}</b></div>"
    return head + table(["Quantite", "Qt./U.", "Produit", "Prix", "Total"], rows, "gecow") + solde

def sm_facn(inv, var):  # ARTICLE | CART | COLISS | QTT | PR UNIT | MONTANT
    rows = [[("", e(l["printed"])), ("c", str(l["ctn"])), ("c", str(l["colis"])), ("c", str(l["qty"])), ("r", N(l["pu"])), ("r", N(l["mt"]))] for l in inv["lines"]]
    head = f"<div class=hrow><div><div class=sname>{e(inv['supplier'])}</div><div>{e(inv['city'])}</div></div><div><b>FAC N : {R.randint(1, 120):02}</b><br>Date : {inv['date']}</div></div><div>Client : {e(inv['client'])}</div>"
    return head + table(["ARTICLE", "CART", "COLISS", "QTT", "PR UNIT", "MONTANT"], rows, "facn") + f"<div class=solde>TOTAL A PAYER : <b>{N(inv['ht'])}</b></div>"

def sm_cnd(inv, var):  # REF | DESIGNATION | PRIX UNITAIRE | QTE CND | CDN | QTE UNITAIRE | UNITE | TOTAL (+ REMISE)
    rows = []
    for l in inv["lines"]:
        cnd = R.choice(["Fardeau", "Carton", "Paquet"])
        rem = R.choice([0, 0, 0, 5, 8, 10]) if var else 0
        l["rem"] = rem; l["mt"] = round(l["qty"] * l["pu"] * (1 - rem / 100), 2)
        r = [("", l["ref"]), ("des", e(l["printed"])), ("r", N(l["pu"])), ("r", N(l["ctn"], "fr", 2)), ("", cnd)]
        if var: r.append(("r", N(rem)))
        r += [("r", N(l["qty"], "fr", 1)), ("", "Unité(s)" if var else "Paquet"), ("r", N(l["mt"]))]
        rows.append(r)
    heads = ["REF", "DESIGNATION", "PRIX UNITAIRE", "QTE CND", "CDN"] + (["REMISE(%)"] if var else []) + ["QTE UNITAIRE", "UNITE", "TOTAL"]
    inv["ht"] = round(sum(l["mt"] for l in inv["lines"]), 2)
    head = f"<div class=hrow><div class=sname>{e(inv['supplier'])}</div><div><b>BL/{R.randint(100, 9999)}/{R.randint(10**8, 10**9)}</b><br>{inv['date']} {R.randint(8, 18):02}:{R.randint(0, 59):02}:{R.randint(0, 59):02}</div></div><div>Liv : {e(inv['city'])}</div>"
    return head + table(heads, rows, "cnd") + f"<div class=solde>Montant total : <b>{N(inv['ht'])}</b></div>"

def sm_bonpour(inv, var):  # N° | Désignation | Nb Casiers | Unit/Cas | Unités | Prix Unit | Montant
    rows = [[("c", str(l["no"])), ("", e(l["printed"])), ("c", str(l["ctn"])), ("c", str(l["colis"])), ("c", str(l["qty"])), ("r", N(l["pu"], "plain")), ("r", N(l["mt"], "plain"))] for l in inv["lines"]]
    head = f"<div class=hrow><div class=logo2>Distribution<br>Produits</div><div><b>Bon Pour N°: {R.randint(1, 60)}/2026</b><br>Date : {inv['date']}</div></div>"
    return head + table(["N°", "Désignation", "Nb Casiers", "Unit/Cas", "Unités", "Prix Unit", "Montant"], rows, "bonpour") + f"<div class=solde>Montant Total : <b>{N(inv['ht'], 'plain')}</b></div>"

def shortest(l, cat_by_id):
    p = cat_by_id[l["product_id"]]; opts = sorted(p["aliases"], key=len)
    if opts and len(l["printed"]) > 24: l["printed"], l["how"] = opts[0], "written alias"
def sm_thermal(inv, var):  # thermal receipt
    for l in inv["lines"]: shortest(l, CAT_BY_ID)
    rows = "".join(f"<tr><td>{l['no']}</td><td>{e(l['printed'])}</td><td class=r>{l['ctn']}</td><td class=r>{l['colis']}</td><td class=r>{N(l['pu'], 'plain')}</td><td class=r>{N(l['mt'], 'plain')}</td></tr>" for l in inv["lines"])
    caisses = sum(l["ctn"] for l in inv["lines"])
    return (f"<div class=thermal><div class=c><b>BON DE LIVRAISON</b><br>BL N° {R.randint(10**7, 10**8)}<br>{inv['date']}</div><hr>"
            f"<table class=th><tr><th>Num</th><th>Produit</th><th>Qte.C</th><th>Qte.U</th><th>P.U</th><th>Montant</th></tr>{rows}</table><hr>"
            f"<div>Total des Caisses : {caisses}</div><div>Total TTC : {N(inv['ht'], 'plain')}</div><div><b>NET A PAYER : {N(inv['ht'], 'plain')}</b></div></div>")

def sm_arabic(inv, var):  # وصل التسليم, right to left, quantity written packs x units
    rows = []
    for l in inv["lines"]:
        name = l["ar"] if l["ar"] and R.random() < 0.85 else l["printed"]
        if name == l["ar"]: l["printed"], l["how"] = name, "arabic name"
        rows.append([("c", str(l["no"])), ("", l["ref"]), ("des", f"<bdi>{e(name)}</bdi>"), ("c", "Pcs"), ("c", f"<bdi>{l['ctn']}x{l['colis']}</bdi>"), ("r", f"<bdi>{N(l['pu'], 'plain')}</bdi>"), ("c", "Exonéré"), ("r", f"<bdi>{N(l['mt'], 'plain')}</bdi>")])
    head = f"<div class=hrow><div class=sname>{e(inv['supplier'])}</div><div><b>وصل التسليم</b> رقم {R.randint(100, 9999)}<br>التاريخ : {inv['date']}</div></div><div>الزبون : {e(inv['client'])}</div>"
    return f"<div dir=rtl class=ar>{head}{table(['الرقم', 'الرمز', 'التعيين', 'الوحدة', 'الكمية', 'سعر الوحدة', 'الرسم', 'الإجمالي'], rows, 'arab')}<div class=solde>المجموع : <b><bdi>{N(inv['ht'], 'plain')}</bdi></b></div></div>"

def sm_bonhash(inv, var):  # N | Ref | Designation | Prix | Qtt | Qtt/ctn | ctns | Montant (lowercase trading name)
    rows = [[("c", str(l["no"])), ("", l["ref"]), ("", e(l["printed"].lower() if R.random() < 0.5 else l["printed"])), ("r", N(l["pu"])), ("c", str(l["qty"])), ("c", str(l["colis"])), ("c", str(l["ctn"])), ("r", N(l["mt"]))] for l in inv["lines"]]
    head = f"<div class=hrow><div class=sname style='text-transform:lowercase'>{e(inv['supplier'])}</div><div><b>Bon # {R.randint(100, 999)}</b> page 1/1<br>{inv['date']}</div></div>"
    return head + table(["N", "Ref", "Designation", "Prix", "Qtt", "Qtt/ctn", "ctns", "Montant"], rows, "hash") + f"<div class=solde>Total : <b>{N(inv['ht'])}</b></div>"

def sm_bonvente(inv, var):  # Qte | Désignation | Qte.U | Prix.U | Montant
    rows = []
    for l in inv["lines"]:
        name = l["ar"] if var and l["ar"] and R.random() < 0.4 else l["printed"]
        if name == l["ar"] and l["ar"]: l["printed"], l["how"] = name, "arabic name"
        rows.append([("c", str(l["ctn"])), ("des", e(name)), ("r", N(l["colis"], "plain", 3)), ("r", N(l["pu"], "plain")), ("r", N(l["mt"], "plain"))])
    head = f"<div class=hrow><div><b>BON VENTE N° {R.randint(1000, 99999):06}</b><br>{len(inv['lines'])} Produit(s)</div><div>Page 001<br>{inv['date']}</div></div><div>Client : {e(inv['client'])}</div>"
    return head + table(["Qte", "Désignation", "Qte.U", "Prix.U", "Montant"], rows, "bv") + f"<div class=solde>Total : <b>{N(inv['ht'], 'plain')}</b></div>"

def sm_blcrt(inv, var):  # N° | Désignation | Nbr. Crt | Colis | Nbr. U | Total U | UG | P.U | Rem% | Montant
    rows = [[("c", str(l["no"])), ("", e(l["printed"])), ("c", "0"), ("c", str(l["colis"])), ("c", str(l["ctn"])), ("c", str(l["qty"])), ("c", "0"), ("r", N(l["pu"])), ("r", "0,00"), ("r", N(l["mt"]))] for l in inv["lines"]]
    head = f"<div class=hrow><div class=sname>{e(inv['supplier'])}</div><div><b>Bon de Livraison N° {R.randint(1000, 99999):06}</b><br>Edité Le {inv['date']} &nbsp; Livré Le {inv['date']}</div></div>"
    return head + table(["N°", "Désignation", "Nbr. Crt", "Colis", "Nbr. U", "Total U", "UG", "P.U", "Rem%", "Montant"], rows, "blcrt") + f"<div class=solde>Total Bon : <b>{N(inv['ht'])}</b></div>"

def sm_factcarton(inv, var):  # REF | Désignation | Quantite Carton | Colisage | Prix HT | Montant HT
    rows = [[("", ""), ("", e(l["printed"])), ("c", N(l["ctn"], "fr", 1)), ("c", N(l["colis"], "fr", 1)), ("r", N(l["pu"])), ("r", N(l["mt"]))] for l in inv["lines"]]
    head = f"<div class=hrow><div class=boxname>{e(inv['supplier'])}</div><div><b>FACTURE N° :{R.randint(1, 99):03} /2026</b><br>{e(inv['city']).upper()} Le {inv['date']}</div></div>"
    return head + table(["REF", "Désignation", "Quantite Carton", "Colisage", "Prix HT", "Montant HT"], rows, "fc") + \
        f"<div class=solde>Montant HT : {N(inv['ht'])}<br>Montant TTC : <b>{N(inv['ht'])}</b></div><p class=words>Arrêtée la présente facture à la somme de : {words_da(inv['ht'])}</p>"

def sm_fardeau(inv, var):  # N | Réf | Désignation | Unité | Qté | Prix unit | Total TTC (price per fardeau)
    rows = []
    for l in inv["lines"]:
        l["fpu"] = round(l["pu"] * l["colis"], 2)
        rows.append([("c", str(l["no"])), ("", l["ref"]), ("", e(l["printed"])), ("", f"Fardeau({l['colis']})"), ("c", str(l["ctn"])), ("r", N(l["fpu"], "plain", 0)), ("r", N(l["mt"], "plain", 0))])
    head = f"<div class=hrow><div class=sname>{e(inv['supplier'])}</div><div><b>Bon de Livraison N°: {R.randint(1000, 99999)}/2026</b><br>{inv['date']}</div></div>"
    return head + table(["N", "Réf", "Désignation", "Unité", "Qté", "Prix unit", "Total TTC"], rows, "fard") + f"<div class=solde>Montant : {N(inv['ht'], 'plain', 0)}<br><b>NET A PAYER : {N(inv['ht'], 'plain', 0)}</b></div>"

def sm_nbrcolis(inv, var):  # Code | Désignation | NBR/COLIS | Prix Unitaire HT | Montant HT (+ Arabic note)
    rows = [[("", l["ref"]), ("", e(l["printed"])), ("c", str(l["ctn"])), ("r", N(l["pu"] * l["colis"])), ("r", N(l["mt"]))] for l in inv["lines"]]
    for l in inv["lines"]: l["fpu"] = round(l["pu"] * l["colis"], 2)
    head = f"<div class=hrow><div></div><div><b>BL N° {R.randint(1000, 99999)}/2026</b><br>{inv['date']} {R.randint(8, 18):02}:{R.randint(0, 59):02}</div></div>"
    return head + table(["Code", "Désignation", "NBR/COLIS", "Prix Unitaire HT", "Montant HT"], rows, "nbr") + f"<div class=solde>Total : <b>{N(inv['ht'])}</b></div><p dir=rtl class=arnote>الدفع عند التسليم فقط</p>"

SM_FORMATS = [("S01", sm_gecow, 1, "Arial Narrow"), ("S02", sm_facn, 0, "Calibri"), ("S03", sm_cnd, 0, "Tahoma"), ("S04", sm_cnd, 1, "Verdana"),
              ("S05", sm_bonpour, 0, "Arial"), ("S06", sm_thermal, 0, "Courier New"), ("S07", sm_arabic, 0, "Arial"), ("S08", sm_bonhash, 0, "Segoe UI"),
              ("S09", sm_bonvente, 0, "Consolas"), ("S10", sm_blcrt, 0, "Trebuchet MS"), ("S11", sm_factcarton, 0, "Times New Roman"),
              ("S12", sm_fardeau, 0, "Cambria"), ("S13", sm_nbrcolis, 0, "Franklin Gothic Medium"), ("S14", sm_gecow, 0, "Lucida Console"), ("S15", sm_bonvente, 1, "Georgia")]
SM_LINES = {"S06": (8, 14), "S07": (15, 30), "S01": (15, 36), "S14": (15, 36)}

# ------------------------------------------------------------------ electronics formats
def el_head(inv, title, extra=""):
    return f"<div class=hrow><div><div class=sname>{e(inv['supplier'])}</div><div>{e(inv['city'])} — Vente en gros téléphonie & informatique</div></div><div><b>{title}</b><br>N° {inv['number']}<br>Date : {inv['date']}{extra}</div></div><div class=cli2>Client : {e(inv['client'])}</div>"
def el_tot(inv, ttc=True):
    t = round(inv["ht"] * 0.19, 2)
    inv["tva"], inv["ttc"] = t, round(inv["ht"] + t, 2)
    return f"<table class=tot2><tr><td>Total HT</td><td>{N(inv['ht'])}</td></tr><tr><td>TVA 19%</td><td>{N(t)}</td></tr><tr><td><b>Total TTC</b></td><td><b>{N(inv['ttc'])}</b></td></tr></table>"
def gar(l): return R.choice(["12 mois", "6 mois", "3 mois", "Sans"]) if l["family"] in ("MOBILE", "TABLETTE", "INFORMATIQUE", "Montre") else R.choice(["1 mois", "Sans", "Echange 7j"])
def el_e1(inv, v): return el_head(inv, "FACTURE") + table(["Réf", "Désignation", "Garantie", "Qté", "PU HT", "Montant HT"], [[("", l["ref"]), ("", e(l["printed"])), ("c", gar(l)), ("c", str(l["qty"])), ("r", N(l["pu"])), ("r", N(l["mt"]))] for l in inv["lines"]], "e1") + el_tot(inv)
def el_e2(inv, v): return el_head(inv, "BON DE LIVRAISON") + table(["Code article", "Désignation", "Marque", "Qté", "PU", "Total"], [[("", f"ART-{l['ref']}"), ("", e(l["printed"])), ("", e(l["brand"])), ("c", str(l["qty"])), ("r", N(l["pu"], "plain")), ("r", N(l["mt"], "plain"))] for l in inv["lines"]], "e2") + f"<div class=solde>Total : <b>{N(inv['ht'], 'plain')}</b></div>"
def el_e3(inv, v):
    for l in inv["lines"]: l["pu_ttc"] = round(l["pu"] * 1.19, 2)
    return el_head(inv, "FACTURE") + table(["N°", "Désignation", "Qté", "PU TTC", "Total TTC"], [[("c", str(l["no"])), ("", e(l["printed"])), ("c", str(l["qty"])), ("r", N(l["pu_ttc"], "en")), ("r", N(l["qty"] * l["pu_ttc"], "en"))] for l in inv["lines"]], "e3") + f"<div class=solde>Net à payer TTC : <b>{N(inv['ht'] * 1.19, 'en')}</b></div>"
def el_e4(inv, v):
    for l in inv["lines"]: shortest(l, CAT_BY_ID)
    rows = "".join(f"<tr><td>{e(l['printed'])}</td><td class=r>{l['qty']}</td><td class=r>{N(l['pu'], 'plain', 0)}</td><td class=r>{N(l['mt'], 'plain', 0)}</td></tr>" for l in inv["lines"])
    return f"<div class=thermal><div class=c><b>{e(inv['supplier'])}</b><br>TICKET N° {inv['number']}<br>{inv['date']}</div><hr><table class=th><tr><th>Article</th><th>Qte</th><th>PU</th><th>Total</th></tr>{rows}</table><hr><div><b>TOTAL : {N(inv['ht'], 'plain', 0)} DA</b></div><div class=c>Merci de votre visite</div></div>"
def el_e5(inv, v):  # family sections
    byf = collections.defaultdict(list)
    for l in inv["lines"]: byf[l["family"] or "DIVERS"].append(l)
    body = ""
    for f, ls in byf.items():
        body += f"<tr class=sec><td colspan=5>{e(f.upper())}</td></tr>" + "".join(f"<tr><td>{l['ref']}</td><td>{e(l['printed'])}</td><td class=r>{l['qty']}</td><td class=r>{N(l['pu'])}</td><td class=r>{N(l['mt'])}</td></tr>" for l in ls)
    return el_head(inv, "FACTURE DE VENTE") + f"<table class='lines e5'><thead><tr><th>Réf</th><th>Désignation</th><th>Qté</th><th>P.U HT</th><th>Montant</th></tr></thead><tbody>{body}</tbody></table>" + el_tot(inv)
def el_e6(inv, v): return el_head(inv, "FACTURE") + table(["Désignation", "Colis", "Pcs/Colis", "Qté pcs", "Prix pcs", "Montant"], [[("", e(l["printed"])), ("c", str(max(1, l["qty"] // l["colis"]))), ("c", str(l["colis"])), ("c", str(l["qty"])), ("r", N(l["pu"])), ("r", N(l["mt"]))] for l in inv["lines"]], "e6") + el_tot(inv)
def el_e7(inv, v):
    rows = [[("c", str(l["no"])), ("des", f"<bdi>{e(l['printed'])}</bdi>"), ("c", str(l["qty"])), ("r", f"<bdi>{N(l['pu'], 'plain')}</bdi>"), ("r", f"<bdi>{N(l['mt'], 'plain')}</bdi>")] for l in inv["lines"]]
    return f"<div dir=rtl class=ar><div class=hrow><div class=sname>{e(inv['supplier'])}</div><div><b>فاتورة</b> رقم {inv['number']}<br>التاريخ : {inv['date']}</div></div><div>الزبون : {e(inv['client'])}</div>{table(['الرقم', 'المنتج', 'الكمية', 'السعر', 'المبلغ'], rows, 'arab')}<div class=solde>المجموع : <b><bdi>{N(inv['ht'], 'plain')}</bdi></b> دج</div></div>"
def el_e8(inv, v): return el_head(inv, "BON DE LIVRAISON") + table(["Réf", "Désignation", "Qté cmd", "Qté livrée", "PU", "Montant"], [[("", l["ref"]), ("", e(l["printed"])), ("c", str(l["qty"])), ("c", str(l["qty"])), ("r", N(l["pu"], "en")), ("r", N(l["mt"], "en"))] for l in inv["lines"]], "e8") + f"<div class=solde>Total : <b>{N(inv['ht'], 'en')}</b></div>"
def el_e9(inv, v):
    for l in inv["lines"]:
        l["rem"] = R.choice([0, 0, 2, 3, 5]); l["mt"] = round(l["qty"] * l["pu"] * (1 - l["rem"] / 100), 2)
    inv["ht"] = round(sum(l["mt"] for l in inv["lines"]), 2)
    return el_head(inv, "FACTURE N°") + table(["N", "Désignation", "U", "Qté", "PU HT", "Rem", "Montant HT"], [[("c", str(l["no"])), ("", e(l["printed"])), ("c", "Pcs"), ("c", str(l["qty"])), ("r", N(l["pu"])), ("c", f"{l['rem']}%" if l["rem"] else ""), ("r", N(l["mt"]))] for l in inv["lines"]], "e9") + el_tot(inv)
def el_e10(inv, v): return el_head(inv, "FACTURE D'ACHAT") + table(["Code barre", "Désignation", "Qté", "Prix", "Total"], [[("", f"69{R.randint(10**10, 10**11 - 1)}"), ("", e(l["printed"])), ("c", str(l["qty"])), ("r", N(l["pu"])), ("r", N(l["mt"]))] for l in inv["lines"]], "e10") + el_tot(inv)
def el_e11(inv, v): return el_head(inv, "BON DE VENTE") + table(["Qte", "Article", "PU", "Montant"], [[("c", str(l["qty"])), ("", e(l["printed"])), ("r", N(l["pu"], "plain")), ("r", N(l["mt"], "plain"))] for l in inv["lines"]], "e11") + f"<div class=solde>Total : <b>{N(inv['ht'], 'plain')}</b><br>Versement : {N(round(inv['ht'] * 0.5, -2), 'plain')}<br>Reste : {N(inv['ht'] - round(inv['ht'] * 0.5, -2), 'plain')}</div>"
def el_e12(inv, v): return el_head(inv, "FACTURE") + table(["Désignation", "N° de série", "Garantie", "Qté", "PU HT", "Total HT"], [[("", e(l["printed"])), ("", "................" if l["family"] in ("MOBILE", "TABLETTE") or l["pu"] > 10000 else ""), ("c", gar(l)), ("c", str(l["qty"])), ("r", N(l["pu"])), ("r", N(l["mt"]))] for l in inv["lines"]], "e12") + el_tot(inv)
def el_e13(inv, v): return el_head(inv, "FACTURE") + table(["Désignation", "Qté", "PU HT", "TVA", "Montant TTC"], [[("", e(l["printed"])), ("c", str(l["qty"])), ("r", N(l["pu"])), ("c", "19%"), ("r", N(l["mt"] * 1.19))] for l in inv["lines"]], "e13") + el_tot(inv) + f"<p class=words>Arrêtée la présente facture à la somme de : {words_da(round(inv['ht'] * 1.19, 2))}</p>"
def el_e14(inv, v):  # carnet: handwritten-looking slip on ruled lines
    rows = [[("c", str(l["qty"])), ("", e(l["printed"].title() if R.random() < 0.5 else l["printed"])), ("r", N(l["pu"], "plain", 0)), ("r", N(l["mt"], "plain", 0))] for l in inv["lines"]]
    return f"<div class=carnet><div class=hrow><div><b>{e(inv['supplier'])}</b></div><div>Bon N° {R.randint(1, 500)}<br>Le {inv['date']}</div></div>{table(['Qté', 'Désignation', 'P.U', 'Montant'], rows, 'e14')}<div class=solde>Total : {N(inv['ht'], 'plain', 0)} DA</div></div>"
def el_e15(inv, v): return el_head(inv, "FACTURE / BL") + table(["N°", "Réf", "Désignation", "Famille", "Qté", "PU HT", "Montant HT"], [[("c", str(l["no"])), ("", l["ref"]), ("", e(l["printed"])), ("", e(l["family"].title())), ("c", str(l["qty"])), ("r", N(l["pu"], "en")), ("r", N(l["mt"], "en"))] for l in inv["lines"]], "e15") + el_tot(inv)
EL_FORMATS = [("E01", el_e1, 0, "Segoe UI"), ("E02", el_e2, 0, "Arial"), ("E03", el_e3, 0, "Tahoma"), ("E04", el_e4, 0, "Consolas"), ("E05", el_e5, 0, "Calibri"),
              ("E06", el_e6, 0, "Verdana"), ("E07", el_e7, 0, "Tahoma"), ("E08", el_e8, 0, "Cambria"), ("E09", el_e9, 0, "Trebuchet MS"), ("E10", el_e10, 0, "Arial Narrow"),
              ("E11", el_e11, 0, "Courier New"), ("E12", el_e12, 0, "Georgia"), ("E13", el_e13, 0, "Franklin Gothic Medium"), ("E14", el_e14, 0, "Segoe Print"), ("E15", el_e15, 0, "Times New Roman")]
EL_LINES = {"E04": (10, 16), "E14": (12, 18)}

CSS = """
@page { size: A4; margin: 10mm; }
body { margin:0; color:#000; background:#fff; }
section.inv { page-break-after: always; position:relative; min-height:272mm; }
.hrow { display:flex; justify-content:space-between; gap:12px; margin-bottom:6px; }
.sname { font-weight:700; font-size:1.3em; } .cli2 { border:1px solid #000; padding:3px 8px; margin:4px 0 6px; display:inline-block; }
.boxname { border:1.5px solid #000; padding:8px 14px; font-weight:700; min-width:40%; } .logo2 { border:3px solid #000; border-radius:10px; padding:6px 10px; font-weight:800; text-align:center; }
table.lines { width:100%; border-collapse:collapse; margin-top:6px; } table.lines th, table.lines td { padding:2px 4px; }
td.r { text-align:right; white-space:nowrap; } td.c { text-align:center; white-space:nowrap; }
.solde { margin-top:8px; text-align:right; line-height:1.5; } .words { font-style:italic; } .arnote { text-align:center; margin-top:14px; }
.gh { display:flex; justify-content:space-between; }
.gecow th { border-top:2px solid #000; border-bottom:2px solid #000; text-align:left; } .gecow td { border-bottom:1px solid #000; font-weight:700; }
.facn td, .facn th { border:1px solid #000; } .facn th { background:#ddd; }
.cnd td, .cnd th { border:1px solid #000; vertical-align:top; } .cnd td.des { width:30%; }
.bonpour th { border-bottom:1.5px solid #000; } .bonpour td { border-bottom:.5px solid #999; }
.thermal { width:110mm; margin:0 auto; font-size:9pt; } table.th td, table.th th { padding:1px 6px; } table.th td.r { white-space:nowrap; } .thermal .c { text-align:center; } .thermal hr { border:0; border-top:1px dashed #000; }
table.th { width:100%; border-collapse:collapse; } table.th th { border-bottom:1px dashed #000; text-align:left; }
.ar { font-family:'Arial','Traditional Arabic',sans-serif; } .arab td, .arab th { border:1px solid #000; } .arab th { background:#ddd; }
.hash td { border-bottom:.5px solid #000; } .hash th { border-bottom:2px solid #000; text-align:left; }
.bv th { border:1px solid #000; } .bv td { border-left:1px solid #000; border-right:1px solid #000; border-bottom:.4px dotted #000; }
.blcrt td, .blcrt th { border:.6px solid #000; } .fc td, .fc th { border:1px solid #000; } .fc th { font-weight:700; }
.fard td, .fard th { border:1px solid #000; } .nbr th { border-bottom:1.5px solid #000; text-align:left; } .nbr td { border-bottom:.5px solid #000; }
.e1 td, .e1 th { border:1px solid #000; } .e1 th { background:#000; color:#fff; } .e2 th { border-bottom:2px solid #000; } .e2 td { border-bottom:.5px dashed #000; }
.e3 td, .e3 th { border:1px solid #000; } .e5 td, .e5 th { border-bottom:.5px solid #000; } .e5 tr.sec td { font-weight:800; background:#ddd; border-top:1.5px solid #000; }
.e6 td, .e6 th { border:1px solid #000; } .e6 tbody tr:nth-child(even) td { background:#eee; } .e8 th { border:2px solid #000; } .e8 td { border:.5px solid #000; }
.e9 td { border-bottom:1px solid #000; } .e9 th { border-top:3px double #000; border-bottom:3px double #000; } .e10 td, .e10 th { border:1px solid #000; }
.e11 th { border-bottom:1px solid #000; text-align:left; } .e12 td, .e12 th { border:1px solid #000; } .e13 td { border-bottom:.5px dotted #000; } .e13 th { background:#ccc; }
.e15 td, .e15 th { border:.6px solid #000; } .e15 th { text-transform:uppercase; }
.carnet { border:2px solid #000; padding:10px; } .e14 td { border-bottom:1px solid #777; height:22px; } .e14 th { border-bottom:2px solid #000; }
table.tot2 { margin:8px 0 0 auto; border-collapse:collapse; } table.tot2 td { border:1px solid #000; padding:3px 10px; text-align:right; }
.foot { position:absolute; bottom:0; left:0; right:0; text-align:center; font-size:7px; }
"""

def build(kind):
    global CAT_BY_ID
    cat = load_sm() if kind == "supermarket" else load_el()
    CAT_BY_ID = {p["id"]: p for p in cat}
    formats = SM_FORMATS if kind == "supermarket" else EL_FORMATS
    LL = SM_LINES if kind == "supermarket" else EL_LINES
    sups = SUP_SM if kind == "supermarket" else SUP_EL
    invs, secs = [], []
    for k in range(40):
        fid, fn, var, font = formats[k % 15]
        lo, hi = LL.get(fid, (15, 26))
        lines = pick_lines(cat, R.randint(lo, hi), "sm" if kind == "supermarket" else "el")
        inv = dict(id=f"{'SUP' if kind == 'supermarket' else 'ELE'}-{k + 1:02}", layout=fid, supplier=sups[k % 15], city=R.choice(CITIES),
                   client="SARL ORDER DESK " + ("SUPERETTE" if kind == "supermarket" else "MOBILE"), number=f"{R.randint(100, 9999)}/26", date=date_s(),
                   lines=lines, old=round(R.uniform(0, 400000), 2))
        inv["ht"] = round(sum(l["mt"] for l in lines), 2)
        body = fn(inv, var)
        inv.setdefault("ttc", inv["ht"])
        size = 8.8 if len(lines) > 30 else 9.6
        secs.append(f"<section class=inv style=\"font-family:'{font}',Arial,sans-serif;font-size:{size}pt\">{body}{FOOT}</section>")
        invs.append(inv)
    page = f"<!doctype html><html><head><meta charset=utf-8><style>{CSS}</style></head><body>{''.join(secs)}</body></html>"
    hp = HERE / f"invoices-{kind}.html"; hp.write_text(page, encoding="utf-8")
    pdf = HERE / f"invoices-{kind}.pdf"
    subprocess.run([EDGE, "--headless=new", "--disable-gpu", "--no-pdf-header-footer", f"--print-to-pdf={pdf}", hp.as_uri()], check=True, capture_output=True, timeout=180)
    for i in invs: i.pop("old", None)
    json.dump(invs, open(HERE / f"truth-{kind}.json", "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    with open(HERE / f"aliases-{kind}.csv", "w", encoding="utf-8-sig", newline="") as f:
        w = csv.writer(f); w.writerow(["id", "name", "alias", "source"])
        for p in cat:
            for a in p["aliases"]: w.writerow([p["id"], p["name"], a, "written"])
            if p.get("ar"): w.writerow([p["id"], p["name"], p["ar"], "official arabic (GS1)"])
    from openpyxl import Workbook
    from openpyxl.styles import Font
    wb = Workbook(); ws = wb.active; ws.title = "Answer key"
    ws.append(["invoice", "layout", "supplier", "line", "printed on the document", "how written", "catalogue id", "catalogue product", "cartons", "per carton", "qty (units)", "unit price", "amount"])
    for inv in invs:
        for l in inv["lines"]:
            ws.append([inv["id"], inv["layout"], inv["supplier"], l["no"], l["printed"], l["how"], l["product_id"], l["product"], l["ctn"] or "", l["colis"] if kind == "supermarket" else "", l["qty"], l["pu"], l["mt"]])
    for c in ws[1]: c.font = Font(bold=True)
    wb.save(HERE / f"truth-{kind}.xlsx")
    print(kind, len(invs), "documents,", sum(len(i["lines"]) for i in invs), "lines", dict(collections.Counter(l["how"] for i in invs for l in i["lines"])))

if __name__ == "__main__":
    paths.need(os.environ.get("GS1_CSV"), __doc__, "GS1 export ($GS1_CSV)")
    if not EDGE: paths.usage(__doc__, "no browser to print the PDF: set PDF_BROWSER")
    for k in ("supermarket", "electronics"): build(k)
