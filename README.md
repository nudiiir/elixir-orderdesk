# Elixir Order Desk

**Orders from any chat into any ERP, on a catalogue that writes its own matching rules.**

Small shops order from their distributor the way they talk: Darja voice notes, photos of handwritten lists,
"same as last time, plus two cartons of Ifri". Order Desk turns every line of those messages into one precise
product from the distributor's catalogue, checks it with the ERP's own prices and stock, asks only what it must,
and books a confirmed order in the ERP.

The matching is not an AI call on every line forever. Order Desk keeps a memory of the catalogue (every
wording people use for every product) and a set of **rules**. A background agent, the **rule engineer**
(Muse Wonder), studies the lines the rules could not resolve, writes a new rule in a small rule language,
tests it on every line whose answer is known, and the **code** lets it in only with zero wrong matches. Week
after week, more lines are resolved for free; the AI is called only for what is new.

Built during the Meta Global AI Developer Hackathon (October 2026). Everything in this repository is new.
Elixir Distribution, our existing ERP, is only one ERP it connects to, through its HTTP API.

## How a line is resolved

| Level | What happens | Cost |
|---|---|---|
| 0 · normalise | lower case, accents, Arabic digits, sizes (`1,5 L` → `1500ml`), live rewrite rules | free |
| 1 · alias | a trusted wording for this shop | free |
| 2 · rules | synonyms, pack patterns, defaults, guards, then an exact catalogue match | free |
| 3 · shortlist | the 8 most likely products, with the shop's habits | free |
| 4 · Muse Spark | picks one id **from the shortlist only**, one call per message | paid |
| 5 · person | one precise question with buttons | — |

Messages are cut into items first. Plain lists use a deterministic parser (no AI). Voice notes go to Muse Voice
Transcribe, then Muse Spark extracts the items, quoting the shop's words and never choosing products, with only
the brand names in the prompt, never the catalogue. "Same as last time" is copied from the ERP by code.

## The rule language

Five rule types, each shown as a sentence generated from its structured form:

- **synonym**: `"slkt"` means `"selecto"`
- **rewrite**: `"1l5"` is written as `"1.5l"`
- **pack**: a trailing `"x{n}"` means a carton of `{n}`
- **default**: for Épicerie Nour, `"ifri"` with no size means Ifri 1.5 L
- **guard**: `"citron"` only matches products whose name contains "citron"

Patterns: literal words, `a|b`, `{n}`, `{size}`, `^`, `$`. Scopes: global, tenant, supplier, customer.

A rule's life: proposed → benchmark (every ground-truth line, levels 0–2 only) → **gate** (0 wrong, 0 conflicts,
minimum gain) → shadow mode (watches live lines) → live after N agreements → rolled back automatically after
N corrections by people. Every step is an event, shown as the rule's birth timeline in the panel.

## Run

```bash
go build -o orderdesk.exe ./cmd/orderdesk
./orderdesk.exe -addr 127.0.0.1:8610 -data ./data -keys keys.env
```

`keys.env` holds `TELEGRAM_BOT_TOKEN=…` and `META_MODEL_API_KEY=…`. Open http://127.0.0.1:8610, then
**Settings → ERP connection** (address and a user of the ERP; only the session token is kept), then
**Customers → Pairing code**, and send the code to the bot on Telegram. `ffmpeg` must be on the PATH for voice notes.

## Layout

```
cmd/orderdesk/        the single program: API + admin panel + Telegram + workers
internal/catalog/     products mirror, packs, aliases, normalisation, shortlist
internal/rules/       the rule language: types, patterns, interpreter, sentences
internal/resolve/     the resolution levels
internal/engineer/    the rule engineer (Muse Wonder + tools), benchmark, gate
internal/desk/        the order agent: extraction, drafts, questions, confirmation, learning
internal/adapter/     the ERP contract; elixir/ = Elixir Distribution over its HTTP API
internal/meta/        Meta Model API client with a cost meter and a daily budget
internal/telegram/    Telegram Bot API (long polling: no public address needed)
internal/web/         panel and JSON API
```

## Meta models

- **Muse Voice Transcribe**: Darja, French and Arabic voice notes to text.
- **Muse Spark 1.3**: reads list photos, extracts items from conversations, picks among a shortlist.
- **Muse Wonder**: the rule engineer, a tool-using agent (`list_residue`, `search_catalogue`, `list_rules`,
  `run_benchmark`, `submit_rule`).

Every call is logged with its tokens and cost (Learning & cost screen); a daily budget stops AI calls while
aliases and rules keep working.

## License

Apache-2.0
