"""Supplier invoices for the benchmark: 40 medicine + 40 cosmetics/parapharmacy, to print and photograph.

Every invoice: one fictional supplier selling to the distributor, 15+ lines, distributor-size quantities.
Lines are written with the product's aliases (real spellings first, pattern-written when none), rarely the
catalogue name itself. 15 layouts per industry, none shared between the two industries; black on white.
Outputs: invoices-<kind>.pdf (one invoice per A4 page) and truth-<kind>.json/.xlsx (the answer key).

    python -I gen.py
"""
import csv, json, random, re, html, subprocess, os, sys, collections
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).parent
CATS = HERE.parent
EDGE = next(p for p in [r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe", r"C:\Program Files\Microsoft\Edge\Application\msedge.exe"] if os.path.exists(p))

# ---------- numbers in French (for "Arrêtée la présente facture à la somme de ...") ----------
U = "zéro un deux trois quatre cinq six sept huit neuf dix onze douze treize quatorze quinze seize dix-sept dix-huit dix-neuf".split()
T = {2: "vingt", 3: "trente", 4: "quarante", 5: "cinquante", 6: "soixante", 8: "quatre-vingt"}
def fr(n):
    if n < 20: return U[n]
    if n < 100:
        d, u = divmod(n, 10)
        if d in (7, 9): return T[d - 1] + ("-et-" if u == 1 and d == 7 else "-") + U[10 + u]
        return T[d] + ("-et-un" if u == 1 and d != 8 else ("-" + U[u] if u else ("s" if d == 8 else "")))
    if n < 1000:
        c, r = divmod(n, 100)
        h = ("cent" if c == 1 else U[c] + " cent" + ("s" if r == 0 else ""))
        return h + (" " + fr(r) if r else "")
    if n < 1_000_000:
        k, r = divmod(n, 1000)
        h = "mille" if k == 1 else fr(k) + " mille"
        return h + (" " + fr(r) if r else "")
    m, r = divmod(n, 1_000_000)
    return fr(m) + (" million" if m == 1 else " millions") + (" " + fr(r) if r else "")
def words_da(x):
    d, c = int(x), round((x - int(x)) * 100)
    s = fr(d).upper() + " DINARS ALGÉRIENS"
    return s + (f" ET {fr(c).upper()} CENTIMES" if c else "")

# ---------- data ----------
def load(kind):
    cat = {r["id"]: r for r in csv.DictReader(open(CATS / ("pharmacy" if kind == "pharmacy" else "cosmetics") / "catalog.csv", encoding="utf-8-sig"))}
    al = collections.defaultdict(list)
    for r in csv.DictReader(open(CATS / "aliases" / f"aliases-{kind}.csv", encoding="utf-8-sig")):
        al[r["id"]].append((r["alias"], r["source"]))
    return cat, al
def pharmacy_ppa():
    import pdfplumber
    ROW = re.compile(r"^(.*?)\s+(\d[\d ]*,\d{2})\s+(\d{2}/\d{2}/\d{2})\s+(\d+)\s*(.*)$")
    ppa, items = {}, []
    with pdfplumber.open(CATS / "pharmacy" / "raw-private.pdf") as pdf:
        for page in pdf.pages:
            heads = [w for w in page.extract_words() if w["text"].startswith("Désignation")]
            xs = sorted(w["x0"] for w in heads)
            if not xs: continue
            top = max(w["bottom"] for w in heads) + 1; b = xs + [page.width]
            for i in range(len(xs)):
                for line in (page.crop((b[i] - 2, top, b[i + 1] - 2, page.height)).extract_text() or "").splitlines():
                    m = ROW.match(line.strip())
                    if m: items.append([m.group(1) + (" " + m.group(5) if m.group(5) else ""), float(m.group(2).replace(" ", "").replace(",", "."))])
                    elif items and line.strip(): items[-1][0] += " " + line.strip()
    for n, p in items:
        n = re.sub(r"\s+", " ", re.sub(r"\(\s*\d+\s*%\s*\)", " ", n)).strip(" .-").upper()
        ppa.setdefault(n, p)
    return ppa

R = random.Random(20261103)
def fake_ids():
    w = R.choice(["16", "09", "31", "19", "25", "06", "15", "35", "42", "13"])
    return dict(rc=f"{w}/00-{R.randint(10, 99)}{R.choice('AB')}{R.randint(1000000, 9999999)}", nif=f"{R.randint(10**14, 10**15 - 1)}",
                ai=f"{w}{R.randint(10**8, 10**9 - 1)}", nis=f"{R.randint(10**14, 10**15 - 1)}", tel=f"0{R.choice('567')}{R.randint(10, 99)} {R.randint(10, 99)} {R.randint(10, 99)} {R.randint(10, 99)}")
CITIES = ["Alger", "Blida", "Sétif", "Oran", "Constantine", "Béjaïa", "Tizi Ouzou", "Batna", "Tlemcen", "Boumerdès", "Médéa", "Bordj Bou Arréridj"]
SUP = {"pharmacy": ["EURL TASSILI MEDICAL DISTRIBUTION", "SARL NOUR PHARMA GROS", "SPA HODNA PHARMACEUTIQUE", "SARL AURES MEDIDIS", "EURL SAHEL SANTE",
                    "SARL ATLAS PHARMA DIFFUSION", "SPA MEDJERDA DISTRIBUTION", "EURL CIRTA MEDICAMENTS", "SARL TAFNA PHARM", "SPA DJURDJURA SANTE",
                    "EURL SOUMMAM PHARMA", "SARL OUARSENIS MEDICAL", "SPA ZIBAN PHARMA", "EURL CHELIFF DISTRIBUTION PHARMACEUTIQUE", "SARL TELL MEDIC"],
       "cosmetics": ["SARL BEAUTE DU SAHEL", "EURL COSMEDIS ALGERIE", "SARL PARA PLUS DISTRIBUTION", "SPA HYGIENA DIFFUSION", "EURL DERMA LINE",
                     "SARL EL WAFA PARAPHARMACIE", "EURL JASMIN COSMETIQUES", "SARL BABY CARE DZ", "SPA ORIENT PARAPHARM", "EURL SAFIR HYGIENE",
                     "SARL NEDJMA BEAUTY", "EURL ROSA DISTRIBUTION", "SARL MEDITERRANEE PARA", "SPA ALOE PARAPHARMACIE", "EURL ZAHRA COSMETIQUE"]}
CLIENT = {"pharmacy": "SARL ORDER DESK DISTRIBUTION PHARMACEUTIQUE", "cosmetics": "EURL ORDER DESK PARAPHARMACIE"}

# ---------- 15 layouts per industry (no layout shared between the two) ----------
FONTS_P = ["Arial", "Courier New", "Tahoma", "Times New Roman", "Verdana", "Consolas", "Arial Narrow", "Lucida Console", "Calibri", "Segoe UI",
           "Georgia", "Trebuchet MS", "Cambria", "Franklin Gothic Medium", "Century Gothic"]
FONTS_C = ["Segoe UI", "Georgia", "Calibri", "Trebuchet MS", "Arial Narrow", "Cambria", "Tahoma", "Century Gothic", "Verdana", "Times New Roman",
           "Lucida Console", "Arial", "Franklin Gothic Medium", "Consolas", "Courier New"]
COLS_P = [  # medicine invoice columns
    ["n", "code", "des", "lot", "ddp", "qte", "pu", "ppa", "mt"], ["des", "lot", "ddp", "qte", "ug", "pu", "rem", "mt"],
    ["code", "des", "qte", "pu", "tva", "mt"], ["n", "des", "lot", "qte", "pu", "shp", "ppa", "mt"], ["des", "ddp", "lot", "pu", "qte", "mt"],
    ["n", "des", "qte", "ug", "pu", "ppa", "mt"], ["code", "des", "lot", "ddp", "qte", "pu", "rem", "tva", "mt"], ["des", "qte", "pu", "mt"],
    ["n", "code", "des", "qte", "pu", "ppa", "shp", "mt"], ["des", "lot", "ddp", "ppa", "qte", "pu", "mt"], ["n", "des", "lot", "qte", "pu", "rem", "mt"],
    ["code", "des", "ddp", "qte", "ug", "pu", "mt"], ["n", "des", "qte", "pu", "ppa", "tva", "mt"], ["des", "lot", "qte", "pu", "mt"],
    ["n", "code", "des", "lot", "ddp", "qte", "ug", "pu", "rem", "ppa", "mt"]]
COLS_C = [  # cosmetics/parapharmacy invoice columns
    ["ref", "des", "colis", "qte", "pu", "mt"], ["n", "ean", "des", "qte", "pu", "rem", "mt"], ["des", "qte", "pu", "tva", "mt"],
    ["ref", "des", "qte", "pu", "rem", "tva", "mt"], ["n", "des", "colis", "uc", "qte", "pu", "mt"], ["ean", "des", "qte", "pu", "mt"],
    ["ref", "des", "pu", "qte", "mt"], ["n", "ref", "des", "qte", "pu", "tva", "mt"], ["des", "colis", "qte", "pu", "rem", "mt"],
    ["n", "des", "qte", "pu", "mt"], ["ref", "ean", "des", "qte", "pu", "mt"], ["des", "uc", "colis", "qte", "pu", "tva", "mt"],
    ["n", "ref", "des", "colis", "qte", "pu", "rem", "mt"], ["ean", "des", "pu", "qte", "tva", "mt"], ["ref", "des", "qte", "pu", "rem", "mt"]]
HEAD = {"n": "N°", "code": "Code", "des": "Désignation", "lot": "Lot", "ddp": "DDP", "qte": "Qté", "ug": "UG", "pu": "P.U HT", "ppa": "PPA",
        "shp": "SHP", "rem": "Rem %", "tva": "TVA", "mt": "Montant HT", "ref": "Réf", "ean": "Code barre", "colis": "Colis", "uc": "U/C"}
HEAD_ALT = {"des": ["Désignation", "DESIGNATION", "Produit", "Libellé", "Article", "Désignation du produit"], "qte": ["Qté", "QTE", "Quantité", "Qte livrée"],
            "pu": ["P.U HT", "PU", "Prix U.", "P.U", "Prix HT", "PRIX"], "mt": ["Montant HT", "MONTANT", "Total HT", "Montant", "Total"]}
TITLES_P = ["FACTURE", "FACTURE N°", "FACTURE DE VENTE", "BON DE LIVRAISON / FACTURE", "FACTURE", "FACTURE COMMERCIALE", "F A C T U R E", "FACTURE",
            "FACTURE N°", "FACTURE DE VENTE EN GROS", "FACTURE", "FACTURE / BL", "FACTURE N°", "FACTURE", "FACTURE ORIGINALE"]
TITLES_C = ["Facture", "FACTURE CLIENT", "Facture de vente", "FACTURE", "BL - Facture", "Facture N°", "FACTURE", "Facture commerciale", "FACTURE",
            "Facture de livraison", "FACTURE", "Facture", "FACTURE DE VENTE", "Facture N°", "FACTURE"]

def layouts(kind):
    F, C, TT = (FONTS_P, COLS_P, TITLES_P) if kind == "pharmacy" else (FONTS_C, COLS_C, TITLES_C)
    header = (["left-right", "centered", "logo-left", "dotmatrix", "boxed-grid"] if kind == "pharmacy" else ["banner", "right-block", "two-boxes", "plain-lines", "logo-right"])
    border = (["grid", "hlines", "dashes", "outer", "grid-thick"] if kind == "pharmacy" else ["zebra", "underline", "grid-thin", "none", "double"])
    headrow = (["black", "bold-under", "boxed"] if kind == "pharmacy" else ["grey", "caps-line", "black-round"])
    totals = (["box-right", "table", "lines", "box-wide", "inline"] if kind == "pharmacy" else ["stack", "box-left", "bar", "two-col", "big-ttc"])
    numfmt = ["fr", "plain", "en"]
    out = []
    for i in range(15):
        out.append(dict(id=f"{'P' if kind == 'pharmacy' else 'C'}{i + 1:02}", font=F[i], size=[9.5, 10, 9, 10.5, 9.2][i % 5] + (0.4 if F[i] in ("Arial Narrow",) else 0),
                        cols=C[i], title=TT[i], header=header[i % 5], border=border[(i * 2) % 5], headrow=headrow[i % 3],
                        totals=totals[(i * 3) % 5], num=numfmt[i % 3], words=i % 2 == 0, upper=i % 4 != 1, date=["%d/%m/%Y", "%d-%m-%Y", "%d.%m.%Y"][i % 3],
                        hdr={k: v[(i + len(k)) % len(v)] for k, v in HEAD_ALT.items()}, sign=["cachet", "two", "none", "client"][i % 4]))
    return out

def fmt(x, style, dec=2):
    s = f"{x:,.{dec}f}"
    if style == "fr": return s.replace(",", " ").replace(".", ",")
    if style == "plain": return s.replace(",", "")
    return s

def invoice(kind, k, lay, cat, al, ppa):
    sup = SUP[kind][k % 15] if k < 15 else R.choice(SUP[kind])
    ids, cli = fake_ids(), fake_ids()
    n_lines = R.randint(15, 22 if lay["size"] >= 10 else 26)
    prods = R.sample(list(cat), n_lines)
    day = f"{R.randint(1, 28):02}"; month = R.choice(["08", "09", "10"])
    date = lay["date"].replace("%d", day).replace("%m", month).replace("%Y", "2026")
    lines = []
    for j, pid in enumerate(prods, 1):
        p = cat[pid]; opts = al.get(pid, [])
        realo = [a for a, s in opts if s == "real"]; pato = [a for a, s in opts if s != "real"]
        r = R.random()
        if not opts:  # no alias: write one the way pharmacies do (brand at the end, size left out, words cut)
            t = p["name"].split(); b = p.get("brand") or t[0]
            v = R.choice([lambda: [w for w in t if w != b] + [b], lambda: [w for w in t if not re.search(r"\d", w) and w not in ("ML", "GR", "G", "MG")],
                          lambda: [w[:5] if len(w) > 6 and not re.search(r"\d", w) else w for w in t]])()
            text, how = (" ".join(v) if v and " ".join(v) != p["name"] else p["name"]), "invented alias"
        elif r < 0.07: text, how = p["name"], "catalogue name"
        elif realo and (r < 0.85 or not pato): text, how = R.choice(realo[:10]), "real alias"
        else: text, how = R.choice(pato or realo), "pattern alias"
        if not lay["upper"]: text = text.title() if R.random() < 0.5 else text
        if kind == "pharmacy":
            base = ppa.get(p["name"]) or R.uniform(120, 2400)
            pu = round(base * R.uniform(0.70, 0.80), 2)
            qte = R.choice([10, 20, 30, 40, 50, 60, 80, 100, 120, 150, 200, 240, 300, 400, 500, 600])
            ppa_v = round(base, 2)
        else:
            pu = round(R.choice([R.uniform(90, 400), R.uniform(300, 1200), R.uniform(900, 3600)]), 2); ppa_v = 0
            qte = R.choice([12, 24, 36, 48, 60, 72, 96, 120, 144, 180, 240, 288, 360, 480])
        rem = R.choice([0, 0, 0, 2, 3, 5, 5, 7, 10]) if "rem" in lay["cols"] else 0
        ug = R.choice([0, 0, 0, qte // 10, qte // 20]) if "ug" in lay["cols"] else 0
        tva = 19 if kind == "cosmetics" else (0 if R.random() < 0.9 else 9)
        mt = round(qte * pu * (1 - rem / 100), 2)
        colis = R.choice([6, 12, 24, 36, 48]) if kind == "cosmetics" else 0
        lines.append(dict(no=j, product_id=pid, product=p["name"], printed=text, how=how, qte=qte, ug=ug, pu=pu, rem=rem, tva=tva, mt=mt, ppa=ppa_v,
                          lot=f"{R.choice('ABCDEFGHKLMNPRST')}{R.randint(100, 999)}{R.choice(['', '-', ''])}{R.randint(10, 99)}",
                          ddp=f"{R.randint(1, 12):02}/{R.randint(2027, 2029)}", code=f"{R.randint(100, 99999):05}", ref=f"{R.choice(['RF', 'AR', 'P', 'CO'])}{R.randint(1000, 9999)}",
                          ean=f"6{R.randint(10**11, 10**12 - 1)}", colis=colis, uc=colis, shp=round(ppa_v * 0.04, 2) if ppa_v else 0))
    ht = round(sum(l["mt"] for l in lines), 2)
    tva_amt = round(sum(l["mt"] * l["tva"] / 100 for l in lines), 2)
    timbre = round(min(2500, (ht + tva_amt) * 0.01), 2) if R.random() < 0.4 else 0
    ttc = round(ht + tva_amt + timbre, 2)
    no = f"{R.choice(['FA', 'F', 'FV', 'FC', ''])}{R.choice(['', '-', '/'])}{R.randint(100, 9999):04}/{R.choice(['26', '2026'])}"
    return dict(id=f"{'MED' if kind == 'pharmacy' else 'COS'}-{k + 1:02}", layout=lay["id"], supplier=sup, sup_ids=ids, city=R.choice(CITIES),
                client=CLIENT[kind], cli_ids=cli, number=no, date=date, lines=lines, ht=ht, tva=tva_amt, timbre=timbre, ttc=ttc)

# ---------- HTML for one invoice ----------
def render(inv, lay, kind):
    e = html.escape; nf = lambda x, d=2: fmt(x, lay["num"], d)
    s = inv["sup_ids"]; c = inv["cli_ids"]
    head = lay["hdr"]
    def h(col): return e(head.get(col, HEAD[col]))
    sup_block = f"<div class=sup><div class=sname>{e(inv['supplier'])}</div><div>{e(inv['city'])}</div><div>Vente en gros</div></div>"
    cli_block = f"<div class=cli><div class=lab>Client :</div><div class=cname>{e(inv['client'])}</div><div>Alger</div></div>"
    meta = f"<div class=meta><div class=ttl>{e(lay['title'])} {e(inv['number']) if 'N°' in lay['title'] else ''}</div>{'' if 'N°' in lay['title'] else '<div>N° : ' + e(inv['number']) + '</div>'}<div>Date : {e(inv['date'])}</div></div>"
    H = lay["header"]
    if H in ("left-right", "two-boxes"): header = f"<div class='hrow'>{sup_block}{meta}</div><div class='hrow'><div></div>{cli_block}</div>"
    elif H in ("centered", "banner"): header = f"<div class=center>{sup_block}</div><div class=hrow>{meta}{cli_block}</div>"
    elif H in ("logo-left", "logo-right"):
        logo = f"<div class=logo>{e(''.join(w[0] for w in inv['supplier'].split()[1:4]))}</div>"
        header = f"<div class=hrow>{logo + sup_block if H == 'logo-left' else sup_block + logo}</div><div class=hrow>{cli_block}{meta}</div>"
    elif H in ("dotmatrix", "plain-lines"): header = f"<div class=dm>{sup_block}<hr>{meta}{cli_block}<hr></div>"
    else: header = f"<table class=hgrid><tr><td>{sup_block}</td><td>{meta}</td></tr><tr><td colspan=2>{cli_block}</td></tr></table>"
    rows = []
    for l in inv["lines"]:
        cell = {"n": str(l["no"]), "code": l["code"], "des": e(l["printed"]), "lot": l["lot"], "ddp": l["ddp"], "qte": str(l["qte"]), "ug": str(l["ug"]) if l["ug"] else "",
                "pu": nf(l["pu"]), "ppa": nf(l["ppa"]), "shp": nf(l["shp"]), "rem": (str(l["rem"]) if l["rem"] else ""), "tva": f"{l['tva']}%", "mt": nf(l["mt"]),
                "ref": l["ref"], "ean": l["ean"], "colis": str(max(1, l["qte"] // (l["colis"] or 1))), "uc": str(l["uc"])}
        rows.append("<tr>" + "".join(f"<td class='{col}'>{cell[col]}</td>" for col in lay["cols"]) + "</tr>")
    table = f"<table class=lines><thead><tr>{''.join(f'<th class={col}>{h(col)}</th>' for col in lay['cols'])}</tr></thead><tbody>{''.join(rows)}</tbody></table>"
    tot = [("Total HT", inv["ht"]), ("TVA", inv["tva"])] + ([("Droit de timbre", inv["timbre"])] if inv["timbre"] else []) + [("Net à payer TTC", inv["ttc"])]
    T = lay["totals"]
    if T in ("table", "two-col", "box-wide"):
        totals = "<table class=tot>" + "".join(f"<tr><td>{a}</td><td class=num>{nf(b)}</td></tr>" for a, b in tot) + "</table>"
    elif T in ("lines", "stack", "inline"):
        totals = "<div class=totl>" + (" &nbsp;|&nbsp; " if T == "inline" else "<br>").join(f"{a} : <b>{nf(b)}</b>" for a, b in tot) + "</div>"
    else:
        totals = "<div class=totbox>" + "".join(f"<div><span>{a}</span><span>{nf(b)}</span></div>" for a, b in tot) + "</div>"
    words = f"<p class=words>Arrêtée la présente facture à la somme de : {words_da(inv['ttc'])}</p>" if lay["words"] else ""
    sign = {"cachet": "<div class=sign><div>Cachet et signature</div></div>", "two": "<div class=sign2><div>Le fournisseur</div><div>Le client</div></div>",
            "client": "<div class=sign><div>Signature client</div></div>", "none": ""}[lay["sign"]]
    foot = "<div class=foot>Document de test — jeu de factures Order Desk (fournisseurs fictifs)</div>"
    return f"<section class='inv {lay['id']} b-{lay['border']} hr-{lay['headrow']} t-{T} h-{H}'>{header}{table}<div class=after>{totals}{words}</div>{sign}{foot}</section>"

CSS = """
@page { size: A4; margin: 11mm 10mm; }
body { margin:0; color:#000; background:#fff; }
section.inv { page-break-after: always; position: relative; min-height: 270mm; }
.hrow { display:flex; justify-content:space-between; gap:12px; margin-bottom:6px; }
.center { text-align:center; } .center .sup { margin: 0 auto 6px; }
.sname { font-weight:700; font-size:1.35em; } .cname { font-weight:700; } .lab { font-style:italic; }
.ttl { font-size:1.5em; font-weight:800; letter-spacing:.5px; }
.cli { border:1px solid #000; padding:4px 8px; min-width:42%; }
.logo { width:70px; height:70px; border:3px solid #000; border-radius:50%; display:flex; align-items:center; justify-content:center; font-weight:900; font-size:22px; }
.dm hr { border:0; border-top:1px dashed #000; } .dm .meta { margin:6px 0; }
table.hgrid { width:100%; border-collapse:collapse; margin-bottom:8px; } table.hgrid td { border:1.5px solid #000; padding:5px; vertical-align:top; }
table.lines { width:100%; border-collapse:collapse; margin-top:6px; }
table.lines th, table.lines td { padding:2.5px 4px; }
td.qte, td.pu, td.mt, td.ppa, td.shp, td.ug, td.rem, td.tva, td.colis, td.uc, th.qte, th.pu, th.mt { text-align:right; white-space:nowrap; }
td.n, td.code, td.lot, td.ddp, td.ref, td.ean { white-space:nowrap; }
.b-grid td, .b-grid th, .b-grid-thin td, .b-grid-thin th { border:1px solid #000; } .b-grid-thin td { border-width:.5px; }
.b-grid-thick td, .b-grid-thick th { border:1px solid #000; } .b-grid-thick table.lines { border:2.5px solid #000; }
.b-hlines td { border-bottom:1px solid #000; } .b-dashes td { border-bottom:1px dashed #000; } .b-underline td { border-bottom:.6px solid #000; }
.b-outer table.lines { border:2px solid #000; } .b-double table.lines { border:4px double #000; } .b-double td { border-bottom:.5px dotted #000; }
.b-zebra tbody tr:nth-child(even) td { background:#e6e6e6; }
.hr-black th { background:#000; color:#fff; } .hr-bold-under th { border-bottom:2px solid #000; text-transform:uppercase; }
.hr-boxed th { border:2px solid #000; } .hr-grey th { background:#cfcfcf; } .hr-caps-line th { text-transform:uppercase; border-top:2px solid #000; border-bottom:2px solid #000; }
.hr-black-round th { background:#000; color:#fff; } .hr-black-round th:first-child { border-radius:8px 0 0 0; } .hr-black-round th:last-child { border-radius:0 8px 0 0; }
.after { margin-top:10px; display:flex; flex-direction:column; align-items:flex-end; }
.t-box-left .after, .t-two-col .after { align-items:flex-start; }
table.tot { border-collapse:collapse; min-width:45%; } table.tot td { border:1px solid #000; padding:3px 8px; } table.tot tr:last-child td { font-weight:800; }
.t-box-wide table.tot { width:100%; }
.totbox { border:2px solid #000; padding:6px 10px; min-width:42%; } .totbox div { display:flex; justify-content:space-between; gap:20px; } .totbox div:last-child { font-weight:800; border-top:1px solid #000; margin-top:3px; }
.totbox { box-sizing:border-box; } .totbox span + span { margin-left:10px; }
.t-big-ttc .totbox div:last-child { font-size:1.4em; } .t-bar .totbox { width:100%; display:flex; gap:20px; justify-content:space-between; } .t-bar .totbox div { display:block; border:0; margin:0; }
.totl { text-align:right; line-height:1.6; }
.words { font-style:italic; width:100%; margin-top:8px; }
.sign, .sign2 { margin-top:18px; display:flex; justify-content:flex-end; gap:80px; } .sign div, .sign2 div { border-top:1px solid #000; padding-top:3px; width:180px; text-align:center; }
.foot { position:absolute; bottom:0; left:0; right:0; text-align:center; font-size:7px; color:#000; }
"""

def build(kind):
    cat, al = load(kind)
    ppa = pharmacy_ppa() if kind == "pharmacy" else {}
    lays = layouts(kind)
    invs, secs, styles = [], [], []
    for lay in lays:
        styles.append(f".{lay['id']} {{ font-family:'{lay['font']}', Arial, sans-serif; font-size:{lay['size']}pt; }}")
    for k in range(40):
        lay = lays[k % 15]
        inv = invoice(kind, k, lay, cat, al, ppa); invs.append(inv)
        secs.append(render(inv, lay, kind))
    page = f"<!doctype html><html><head><meta charset=utf-8><style>{CSS}{''.join(styles)}</style></head><body>{''.join(secs)}</body></html>"
    hp = HERE / f"invoices-{kind}.html"; hp.write_text(page, encoding="utf-8")
    pdf = HERE / f"invoices-{kind}.pdf"
    subprocess.run([EDGE, "--headless=new", "--disable-gpu", "--no-pdf-header-footer", f"--print-to-pdf={pdf}", hp.as_uri()], check=True, capture_output=True, timeout=180)
    json.dump([{k: v for k, v in i.items() if k not in ('sup_ids', 'cli_ids')} for i in invs], open(HERE / f"truth-{kind}.json", "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    from openpyxl import Workbook
    from openpyxl.styles import Font
    wb = Workbook(); ws = wb.active; ws.title = "Answer key"
    ws.append(["invoice", "layout", "supplier", "line", "printed on the invoice", "how written", "catalogue id", "catalogue product", "qty", "UG", "unit price HT", "discount %", "TVA %", "amount HT"])
    for inv in invs:
        for l in inv["lines"]:
            ws.append([inv["id"], inv["layout"], inv["supplier"], l["no"], l["printed"], l["how"], l["product_id"], l["product"], l["qte"], l["ug"], l["pu"], l["rem"], l["tva"], l["mt"]])
    for c in ws[1]: c.font = Font(bold=True)
    ws2 = wb.create_sheet("Invoices")
    ws2.append(["invoice", "layout", "supplier", "number", "date", "lines", "total HT", "TVA", "timbre", "TTC"])
    for inv in invs: ws2.append([inv["id"], inv["layout"], inv["supplier"], inv["number"], inv["date"], len(inv["lines"]), inv["ht"], inv["tva"], inv["timbre"], inv["ttc"]])
    wb.save(HERE / f"truth-{kind}.xlsx")
    hw = collections.Counter(l["how"] for i in invs for l in i["lines"])
    print(kind, len(invs), "invoices,", sum(len(i["lines"]) for i in invs), "lines", dict(hw))

if __name__ == "__main__":
    for kind in ("pharmacy", "cosmetics"): build(kind)
