# How Order Desk works

The short version is in the [README](../README.md#how-it-works). This page covers each part in full.

- [How a line is resolved](#how-a-line-is-resolved)
- [Messages, voice notes and photos](#messages-voice-notes-and-photos)
- [Supplier invoices](#supplier-invoices)
- [Learning](#learning)
- [Adaptive matching: a pipeline Muse Spark designs for each industry](#adaptive-matching-a-pipeline-muse-spark-designs-for-each-industry)
- [Meta models: each one's job, and what checks it](#meta-models-each-ones-job-and-what-checks-it)
- [The rule language](#the-rule-language)

## How a line is resolved

| Step | What happens | Cost |
|---|---|---|
| −1 · codes | a printed barcode or reference, or a supplier's article code confirmed before | free |
| 0 · normalise | lower case, accents, Arabic digits, sizes (`1,5 L` → `1500ml`), the desk's Matching Profile, live rewrite rules | free |
| 1 · alias | a trusted wording for this shop or supplier | free |
| 2 · rules | synonyms, packs, defaults, guards, attributes, then an exact catalogue match | free |
| 3 · shortlist | the 8 most likely products the code would accept (same attributes, the brand the line names) | free |
| 4 · Muse Spark | picks one id **from the shortlist only**, one call for all the open lines of a message | paid |
| 5 · person | one precise question with buttons | — |

Every line is checked against the ERP's own prices and stock. Without a Meta key, steps −1 to 3 still answer
and anything else becomes a question.

## Messages, voice notes and photos

Messages are cut into items first. Plain lists use a deterministic parser, with no model call; chatty text
goes to Muse Spark, and without a key (or when Muse Spark fails) a lenient parse takes over and anything it
cannot place becomes a question, so a chatty or Arabic message never fails. "Same as last time" is copied
from the ERP by code.

**Voice notes.** Muse Voice Transcribe writes the text, then Muse Spark extracts the items. Voice Transcribe
is given the catalogue's brands as keywords, so it writes "Choufan" and "Kool" instead of "chou fin" and
"cool"; it gets only the brand names, never the catalogue, and it quotes the shop's words without choosing
products. The transcript is shown back to the shop with every line.

**Photos.** Muse Spark first tells a written list, which it copies line by line, from a photo of the products
themselves. For products:

1. **SAM 3.1** finds every bottle, can or carton, with one call per kind of packaging.
2. Duplicate and weak boxes are dropped.
3. Muse Spark reads all the crops in one call, the way a shop would type them ("Selecto 2L").
4. Identical products are counted, and a caption like "3 cartons" applies to a single product.

The shop gets its photo back with every product boxed in a colour that matches the reply ("🟩 2 × Selecto
2L"). A photo costs about $0.015.

## Supplier invoices

The owner sends a photo or PDF of a supplier invoice, in the owner's chat or on the **Invoices** screen:

1. Muse Spark reads the supplier, the number, the date and every line with its quantity, price and total.
2. **Muse Wonder reads the same paper on its own.** A line the two readers do not agree on (words, quantity
   or unit price) is never booked automatically: it is shown to the person with its product proposed.
3. Each line is resolved in that supplier's scope, with its own wordings and rules.
4. The quantities are checked against the line totals (packs vs units).
5. Once approved, the invoice is booked in the ERP as a goods receipt that moves the stock.

Try it with [samples/invoice-specimen.png](../samples/invoice-specimen.png), a fictional invoice marked as a
specimen; `python tools/make_invoice.py out.png` draws it again.

## Learning

Every confirmed line becomes ground truth and a witness for its wording, but **the desk learns only from
lines it can trust**: a product a person chose or corrected, or a trusted alias or rule. A wording becomes a
trusted alias after two confirmations, and a supplier's article code after one. A person's correction
blocks the wrong wording, retires a code that named two products, and counts against the rule that gave the
wrong answer; a rule goes back after repeated corrections.

Confirming is idempotent: where the ERP can be searched by external reference (Odoo), the desk looks the
order up before creating it, so a double tap never books it twice. The shop's words go into the ERP's notes;
no person's name does.

## Adaptive matching: a pipeline Muse Spark designs for each industry

Every industry writes its products its own way: strengths and forms, dimensions and inch sizes, flavours and
pack counts. Order Desk does not ship that knowledge. It learns it per desk (`internal/profile`):

- **Bootstrap.** Muse Spark reads a sample of the catalogue and designs a first Matching Profile: which
  first-pass blocks the industry needs (`strength`, `dimensions`, `inch`, `units_glue`, `pack_count`, `shade`,
  `split_glued`, `memory`), the vocabulary its suppliers use, the attributes that tell products apart, and a
  search method. It may also write attribute rules while designing (author `muse-spark-profile`).
- **Proof.** The code measures every proposal in a scratch lab, on confirmed lines and on lines Spark never
  saw. A proposal is kept only with zero wrong, zero lost and no products made to read alike. Vocabulary
  proposed before any evidence waits in shadow. A word that would drop or change digits is refused, because
  codes go to the code table.
- **Search.** BM25, character trigrams and word similarity are raced on real lines (`internal/search`). The
  winner is kept only if it holds on unseen lines.
- **Codes.** A printed barcode or manufacturer reference resolves a line at once. A supplier's article code is
  learned from one confirmation and retired if it ever names two products. Until a second confirmation,
  Spark still checks it.
- **Earned autonomy.** Muse Spark checks every answer the code gives until the code's precision clears a 99 %
  lower bound (about 270 confirmed answers). After that it spot-checks risky answers, and a correction sends
  that supplier back under supervision (`internal/desk/autonomy.go`).
- **The rule engineer and the breaker.** The rule engineer, a Muse Spark agent with tools, studies what the
  desk could not resolve and writes rules; a code gate admits a rule only with zero wrong matches. Before a
  rule goes live, a second Muse Spark call, the breaker, writes look-alike lines for neighbouring products to
  try to make the rule book the wrong one.

## Meta models: each one's job, and what checks it

Every model Order Desk calls is a Meta model, through the Meta Model API, and the chat channel is Meta's own
WhatsApp Cloud API. No model's answer is used unchecked.

| Model | Its jobs | What checks it |
|---|---|---|
| **Muse Spark 1.3** (`muse-spark-1.3`) | reads list photos, product crops and supplier invoices; cuts chatty messages and voice transcripts into items; picks a product from a shortlist; designs and calibrates each desk's Matching Profile; the **rule engineer**, a tool-using agent (`list_residue`, `search_catalogue`, `list_rules`, `run_benchmark`, `submit_rule`); the **breaker** that attacks every admitted rule; checks the free steps' answers until they earn autonomy | its picks only come from a shortlist the code built, and a pick under 0.9 confidence goes to a person; its profile and rules are measured on known and unseen lines and admitted only with 0 wrong; a person confirms every new wording |
| **Muse Wonder 1.0** (`muse-wonder-1.0`) | reads every supplier invoice a second time, on its own; takes over the rule engineer when a Muse Spark session finds no rule the gate admits | the two invoice readings must agree line by line; its rules go through the same gate and breaker |
| **Muse Voice Transcribe** (`muse-voice-transcribe-1.0`) | Darja, French and Arabic voice notes to text, with the catalogue's brands as keywords | the transcript is shown back to the shop with every line, and goes through the same matching as typed text |
| **SAM 3.1** (`sam-3.1`) | finds every bottle, can or carton on a photo of the products | weak and duplicate boxes are dropped; Muse Spark reads each box, and the shop gets its photo back boxed to confirm |

Every call is logged with its model, job, tokens and cost (**Learning & cost**). A daily budget (**Settings**,
$2 by default) stops model calls while aliases and rules keep working.

## The rule language

Eight rule types, each shown in the panel as a sentence generated from its structured form:

| Type | Example sentence |
|---|---|
| **synonym** | `"slkt"` means `"selecto"` |
| **rewrite** | `"1l5"` is written as `"1.5l"` |
| **pack** | a trailing `"x{n}"` means a carton of `{n}` |
| **default** | for Épicerie Nour, `"ifri"` with no size means Ifri 1.5 L |
| **guard** | `"citron"` only matches products whose name contains "citron" |
| **attribute** | `"zero"`, `"light"` each name a variant, in orders, invoices and catalogue names alike |
| **word** | `"pet"` describes a product but never tells two products apart |
| **policy** | Two texts whose colour differs are never the same product. |

Patterns use literal words, `a|b`, `{n}`, `{size}`, `{word}`, `^` and `$`. Scopes are global, tenant,
supplier or customer.

A rule's life is proposed → benchmark → **gate** → shadow → live: the benchmark runs the rule on every
ground-truth line through the free steps only; the gate needs 0 wrong, 0 conflicts and a minimum gain; in
shadow the rule watches live lines; it goes live after N agreements. A live rule is rolled back
automatically after repeated corrections by people. Every step is an event, shown as the rule's birth
timeline in the panel.
