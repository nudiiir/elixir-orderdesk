"""Build the benchmark report (HTML, then PDF with Edge) from results-final.json and results-stream.json.

    python -I report.py
"""
import sys, json, html, subprocess
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).parent
F = json.load(open(HERE / "results-final.json", encoding="utf-8"))
S = json.load(open(HERE / "results-stream.json", encoding="utf-8"))
O = json.load(open(HERE / "results-orders.json", encoding="utf-8"))
INDS = ["supermarket", "pharmacy", "electronics", "cosmetics"]
LABEL = {"supermarket": "Supermarket", "pharmacy": "Pharmacy", "electronics": "Electronics", "cosmetics": "Cosmetics & parapharmacy"}
# Every wrong booking, read by hand: "desk" = Order Desk booked a product the paper does not name;
# "key" = the paper names the product the desk booked, the answer key says another.
ADJ = {
    "SOSEMIE PATES COURTES": ("key", "the paper prints the exact catalogue name of another product (SOSEMIE PATES COURTES); the key wants the TLITLI variant"),
    "Betabio Sol Buv Gttes": ("key", "the paper says oral drops; the key (a real pharmacy's mapping, seen once) says tablets. The desk booked the drops"),
    "BPN MAGNESIUM 500 G BISGLYCINATE": ("desk", "500 G is a typo for 500 MG; the desk ruled out the 500 MG box on size and booked the only box left, which states no size. It should have asked"),
    "رقائق NESQUIK": ("key", "the catalogue lists the product twice (NESQUIK CEREALES / NESQUIK CEREALE)"),
    "TETINE SILICONE SANSIRO": ("desk", "the line does not say the shade; the desk booked N°002 when the key wants N°003 (mid-run only, gone at the end)"),
    "حليب ACTIVIA": ("desk", "'Activia milk' (the official GS1 Arabic name) fits two Activia products; Muse Spark picked the milk, the key wants the dairy speciality"),
}
e = html.escape
pct = lambda a, b: f"{100 * a / b:.0f}%" if b else "—"

def last(o): return o["curve"][-1]
def first(o): return o["curve"][0]

def adjudicate(errs):
    seen, desk, key, rows = set(), 0, 0, []
    for x in errs:
        k = (x["invoice"], x["said"])
        kind, why = ADJ.get(x["said"], ("desk", "not reviewed"))
        if x["after"] == errs[-1]["after"] or True:
            pass
        if k in seen: continue
        seen.add(k); rows.append((x, kind, why))
    return rows

def final_wrong(o, adj):
    """wrong bookings at the last measure: strict, and after review"""
    lastph = f"test after {last(o)['after']}"
    errs = [x for x in o["errors"] if x["after"] == lastph]
    desk = sum(1 for x in errs if ADJ.get(x["said"], ("desk",))[0] == "desk")
    return len(errs), desk

def curve_table(o):
    rows = "".join(f"<tr><td>{c['after']}</td><td>{c['lines']}</td><td>{pct(c['free'] - c.get('checked', 0), c['lines'])}</td><td>{pct(c.get('checked', 0), c['lines'])}</td><td>{pct(c['spark'], c['lines'])}</td>"
                   f"<td>{pct(c['asked'], c['lines'])}</td><td>{c['wrong']}</td><td><b>{pct(c['auto'], c['lines'])}</b></td><td>${c['cost']:.3f}</td></tr>" for c in o["curve"])
    return f"<table><tr><th>invoices learned</th><th>locked lines</th><th>free (no model call)</th><th>code, checked by Spark</th><th>picked by Spark</th><th>asked</th><th>wrong</th><th>automatic</th><th>Meta cost</th></tr>{rows}</table>"

def alone_row(o):
    a = o["alone"]
    if not a: return ""
    return f"{a['RightAt']} right, <b>{a['WrongAt']} wrong</b>, {a['AskedAt']} asked of {a['Lines']} (automatic {pct(a['RightAt'] + a['WrongAt'], a['Lines'])}) · ${a['cost']:.3f}"

head = []
for ind in INDS:
    if ind not in S: continue
    o = S[ind]; c0, c = first(o), last(o); strict, desk = final_wrong(o, ADJ)
    per_inv = c["cost"] / 30
    alone = o["alone"]
    head.append(f"<tr><td>{LABEL[ind]}</td><td>{pct(c0['free'] - c0.get('checked', 0), c0['lines'])} → <b>{pct(c['free'] - c.get('checked', 0), c['lines'])}</b></td><td><b>{pct(c['auto'], c['lines'])}</b></td>"
                f"<td>{pct(c['asked'], c['lines'])}</td><td>{strict} ({desk} after review)</td><td>${per_inv:.4f}</td>"
                f"<td>{pct(alone['RightAt'] + alone['WrongAt'], alone['Lines'])} auto, {alone['WrongAt']} wrong, ${alone['cost'] / 30:.4f}</td></tr>")

tot = {k: sum(last(S[i]).get(k, 0) for i in INDS if i in S) for k in ("lines", "auto", "free", "wrong", "checked")}
strict_all = sum(final_wrong(S[i], ADJ)[0] for i in INDS if i in S); desk_all = sum(final_wrong(S[i], ADJ)[1] for i in INDS if i in S)
AUTO, FREE = pct(tot["auto"], tot["lines"]), pct(tot["free"] - tot["checked"], tot["lines"])
every = [x for i in INDS if i in S for x in S[i]["errors"]]
WRONGS = (f"Wrong bookings at the end: <b>{strict_all} of {tot['lines']:,}</b> lines. Over every measurement of both benchmarks, "
          f"{len(every)} wrong bookings appeared in the stream, read one by one below with the photographed set's.")

def section(ind, o, kind):
    rows = adjudicate(o["errors"])
    errs = "".join(f"<tr><td>{e(x['invoice'])}</td><td>{e(x['said'])}</td><td>{e(x['got'])}</td><td>{e(x['want'])}</td><td>{'Order Desk' if k == 'desk' else 'answer key'}</td><td>{e(w)}</td></tr>" for x, k, w in rows)
    errs = f"<table class=s><tr><th>invoice</th><th>printed</th><th>booked</th><th>answer key</th><th>fault</th><th>why</th></tr>{errs}</table>" if errs else "<p>No wrong booking on the locked invoices.</p>"
    rules = "".join(f"<li>{e(r['sentence'] or '')}</li>" for r in o["rules"][:10])
    lex = ", ".join(f"<code>{e(x['from'])}</code> → <code>{e(x['to'] or '∅')}</code>" for x in o["lexicon"] if x["status"] == "live")
    return f"""<h3>{LABEL[ind]} <span class=muted>— {e(o['industry'] or '')}, run cost ${o['spent']:.2f}</span></h3>
{curve_table(o)}
<p class=muted>Muse Spark alone, same locked lines, same 0.9 bar: {alone_row(o)}</p>
<h4>Wrong bookings on the locked invoices</h4>{errs}
<h4>What it learned</h4><p><b>Vocabulary (live):</b> {lex or '—'}</p><ul>{rules or '<li>—</li>'}</ul>"""

doc = f"""<!doctype html><html><head><meta charset=utf-8><title>Order Desk benchmark</title><style>
@page {{ size: A4; margin: 14mm }}
body {{ font: 10.5pt/1.45 'Segoe UI', Arial, sans-serif; color: #1b1f24 }}
h1 {{ font-size: 21pt; margin: 0 0 4px }} h2 {{ font-size: 14pt; margin: 22px 0 6px; border-bottom: 2px solid #1b1f24; padding-bottom: 3px }}
h3 {{ font-size: 12pt; margin: 18px 0 6px }} h4 {{ font-size: 10.5pt; margin: 10px 0 4px }}
table {{ border-collapse: collapse; width: 100%; margin: 6px 0 }} th, td {{ border: 1px solid #c9ced6; padding: 3px 6px; text-align: left; vertical-align: top }}
th {{ background: #eef1f5 }} table.s td, table.s th {{ font-size: 8.5pt }} .muted {{ color: #5a6472; font-weight: normal }}
.big {{ font-size: 12pt }} code {{ background: #f1f3f6; padding: 0 3px; border-radius: 3px }} section {{ break-inside: avoid-page }}
</style></head><body>
<h1>Order Desk — matching benchmark</h1>
<p class=muted>4 industries · 4 real catalogues (524–796 products) · supplier invoices whose answers are known · locked invoices the desk never learns from</p>

<h2>Result</h2>
<p class=big>On invoices it never saw, after learning from 120 supplier invoices, Order Desk books <b>{AUTO} of lines automatically</b>, <b>{FREE} for free</b>
(no model call), and asks a person about the rest. {WRONGS}</p>
<table><tr><th>industry</th><th>free (no model call), before → after learning</th><th>automatic</th><th>asked</th><th>wrong (strict)</th><th>Meta cost / invoice</th><th>Muse Spark alone</th></tr>{''.join(head)}</table>
<p class=muted>Free = booked with no model call at all. A line the code answered while Muse Spark still checked it (until the desk earns autonomy: about 270 confirmed answers in a row, 99 % lower bound, and later on risky answers) counts as automatic, not free, and its check is in the cost. Stream benchmark, 30 locked invoices per industry (about 540 lines each), measured after 120 training invoices. Muse Spark alone: the whole catalogue and the invoice
in one prompt, booked when its confidence is 0.9 or more.</p>

<h2>How it was measured</h2>
<ul>
<li><b>The desk is trained like a real clerk would train it.</b> On training invoices, a clerk who knows the right product answers every question and corrects every wrong line;
the desk learns aliases, and Muse Spark recalibrates the desk's Matching Profile every 10 or 30 invoices (vocabulary, first-pass blocks, search method, the rule engineer's
rules). A change is kept only if it resolves more lines with <b>0 wrong and 0 lost</b>, also on lines it was not chosen on.</li>
<li><b>Locked invoices are never learned from.</b> They are scored before training and after every round, with nothing answered or corrected.</li>
<li><b>Two benchmarks.</b> <b>Photographed set:</b> the 160 invoices we printed and photographed (40 per industry, 15 layouts each), 30 training + 10 locked. Each invoice
draws its products at random from the whole catalogue, so a supplier almost never prints the same product twice: this is cold-start matching.
<b>Stream:</b> 150 invoices per industry, generated the way wholesale works: 15 suppliers, each with a range of 100 products, each printing a product with <i>one fixed
designation</i> (as supplier software does), popular products reordered more often. 120 training, 30 locked; about 80 % of locked lines are products that supplier
already sold. Designations come from the same alias lists (real pharmacy wordings seen at least twice, written and pattern variants).</li>
<li>Matching only: lines go in as printed text. Reading the paper (photo or PDF → lines) is a separate step and is not scored here.</li>
</ul>

<h2>Stream benchmark — learning curve on locked invoices</h2>
{''.join(f'<section>{section(ind, S[ind], "stream")}</section>' for ind in INDS if ind in S)}

<h2>Shop orders — 200 orders, five channels</h2>
<p>50 orders per industry, as shops send them on WhatsApp: 10 voice notes (8 recorded by us, 32 synthesised in Algerian Darja),
10 typed in French, 10 in Darja in Latin letters, 10 in Darja mixed with French in Arabic script, and 10 screenshots. Each went
through the desk trained on the supplier invoices (stream benchmark), with nothing learned from the orders.</p>
{{ORDERS_TABLE}}
<p class=muted>Booked right: the right product booked automatically; quantity and unit are scored on those lines. Missed: the reader
(transcription, photo reading or extraction) never produced the line. The 3 wrong bookings: a catalogue entry where two medicines
were merged by our PDF parse, a screenshot line read as two items (one product booked twice), and Muse Spark picking a tablet variant
the shop did not state.</p>
<h2>Photographed set — cold start (10 locked invoices per industry)</h2>
<p>Here a supplier rarely repeats a product, so the desk has little to learn per supplier; the curve is flat and Muse Spark alone books more lines on its own.
What holds: the desk's wrong bookings stay at 0–1 per industry.</p>
{''.join(f'<section>{section(ind, F[ind], "final")}</section>' for ind in INDS if ind in F)}

<h2>Fixes this benchmark found (committed)</h2>
<ul>
<li><b>Sizes glued to a pack count were not read</b> — <code>SPRITE LIMO 1LX12</code>, <code>PUREE 240GX12</code> (b30a392).</li>
<li><b>Model codes were read as pack counts</b> — <code>CABLE HOCO X21 IPHONE</code> lost <code>X21</code> and matched the X21PLUS cable; the only wrong booking in electronics, gone after the fix (e564806).</li>
<li><b>Phone memory written many ways</b> — <code>8+256</code>, <code>6-128</code>, <code>8GB/256GB</code> vs the catalogue's <code>8/256GB</code>: new first-pass block <code>memory</code> that Muse Spark can switch on (8e253b3).</li>
</ul>

<h2>Limits</h2>
<ul>
<li>The invoices are generated from real catalogues and real wordings, not received from real suppliers. The stream's fixed designation per supplier is how supplier
software prints, but it is our model of it.</li>
<li>The answer key carries noise from real data: a wording a pharmacy linked to the wrong product once (Betabio drops → tablets) is scored as the truth.</li>
<li>Known desk weakness: a stated size that no product has (500 G for 500 MG) should lead to a question; today the desk books the only product without a size.</li>
<li>On small catalogues (≈600 products) Muse Spark alone can read the whole catalogue in one prompt and books more lines on cold invoices. It cannot do that on a
distributor's real catalogue of thousands of products, and it books some wrong lines; the desk sends Spark a shortlist of 8 and books nothing it cannot defend.</li>
</ul>
</body></html>"""
def orders_table():
    keys = [("french", "Typed in French"), ("darja_latin", "Darja, Latin letters"), ("mixed", "Darja + French, Arabic script"), ("screenshot", "Screenshots"),
            ("voice", "Voice notes"), ("voice: human", "· recorded by us"), ("voice: generated (Gemini TTS)", "· Gemini TTS"), ("ALL", "All orders")]
    rows = ""
    for k, label in keys:
        c = O[k]; L = c["lines"]
        rows += (f"<tr><td>{label}</td><td>{c['orders']}</td><td>{L}</td><td><b>{pct(c.get('right', 0), L)}</b></td><td>{pct(c.get('qty', 0), c.get('right', 0))}</td>"
                 f"<td>{pct(c.get('asked', 0), L)}</td><td>{c.get('wrong', 0)}</td><td>{pct(c.get('missed', 0), L)}</td><td>${c['cost'] / c['orders']:.4f}</td></tr>")
    return f"<table><tr><th>channel</th><th>orders</th><th>lines</th><th>booked right</th><th>right qty + unit</th><th>asked</th><th>wrong</th><th>missed by reader</th><th>Meta cost / order</th></tr>{rows}</table>"
doc = doc.replace("{ORDERS_TABLE}", orders_table())
out = HERE / "orderdesk-benchmark.html"
out.write_text(doc, encoding="utf-8")
pdf = HERE / "orderdesk-benchmark.pdf"
edge = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
subprocess.run([edge, "--headless", "--disable-gpu", "--no-pdf-header-footer", f"--print-to-pdf={pdf}", out.as_uri()], check=False, capture_output=True)
print(out, pdf, pdf.exists())
