-- Order Desk's own memory. Products are a mirror of the ERP (refreshed by the adapter);
-- everything else (aliases, rules, ground truth, lines, events, AI usage) belongs to Order Desk.

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS products (
	id        TEXT PRIMARY KEY,           -- the ERP id
	name      TEXT NOT NULL,
	name_ar   TEXT NOT NULL DEFAULT '',
	brand     TEXT NOT NULL DEFAULT '',
	barcode   TEXT NOT NULL DEFAULT '',
	unit      TEXT NOT NULL DEFAULT '',
	active    INTEGER NOT NULL DEFAULT 1,
	synced_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS packs (
	product_id   TEXT NOT NULL,
	name         TEXT NOT NULL,           -- carton, fardeau, pack...
	factor_milli INTEGER NOT NULL,        -- base units × 1000 in one pack
	PRIMARY KEY (product_id, name)
);

CREATE TABLE IF NOT EXISTS customers (
	id       TEXT PRIMARY KEY,            -- the ERP id
	name     TEXT NOT NULL,
	phone    TEXT NOT NULL DEFAULT '',
	route_id TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS suppliers (
	id    TEXT PRIMARY KEY,            -- the ERP id
	name  TEXT NOT NULL,
	phone TEXT NOT NULL DEFAULT ''
);

-- A confirmed wording. scope_kind: global | tenant | supplier | customer.
-- state: candidate (hint only) | trusted (resolves alone) | blocked.
CREATE TABLE IF NOT EXISTS aliases (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	text       TEXT NOT NULL,             -- normalised wording
	product_id TEXT NOT NULL,
	scope_kind TEXT NOT NULL,
	scope_id   TEXT NOT NULL DEFAULT '',
	source     TEXT NOT NULL,             -- text | voice | photo | clerk | invoice
	witnesses  INTEGER NOT NULL DEFAULT 1,
	state      TEXT NOT NULL DEFAULT 'candidate',
	first_seen TEXT NOT NULL,
	last_seen  TEXT NOT NULL,
	UNIQUE (text, scope_kind, scope_id, product_id)
);

-- Every version of every rule, rejected ones included (so they are not proposed again).
-- status: proposed | shadow | live | rejected | rolled_back | disabled
CREATE TABLE IF NOT EXISTS rules (
	id              TEXT NOT NULL,
	version         INTEGER NOT NULL,
	type            TEXT NOT NULL,
	scope_kind      TEXT NOT NULL,
	scope_id        TEXT NOT NULL DEFAULT '',
	scope_name      TEXT NOT NULL DEFAULT '',
	pattern         TEXT NOT NULL,
	action          TEXT NOT NULL,        -- JSON
	sentence        TEXT NOT NULL,
	author          TEXT NOT NULL,        -- rule-engineer | person name
	rationale       TEXT NOT NULL DEFAULT '',
	inspired_by     TEXT NOT NULL DEFAULT '[]', -- JSON line ids
	status          TEXT NOT NULL,
	reason          TEXT NOT NULL DEFAULT '',
	bench           TEXT NOT NULL DEFAULT '{}', -- JSON benchmark result
	strikes         INTEGER NOT NULL DEFAULT 0,
	shadow_agree    INTEGER NOT NULL DEFAULT 0,
	lines_resolved  INTEGER NOT NULL DEFAULT 0,
	ai_cost         REAL NOT NULL DEFAULT 0,  -- cost of proposing it
	created_at      TEXT NOT NULL,
	live_at         TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (id, version)
);

CREATE TABLE IF NOT EXISTS chats (
	chat_key    TEXT PRIMARY KEY,          -- channel:chat id, e.g. tg:123456
	customer_id TEXT NOT NULL,
	name        TEXT NOT NULL DEFAULT '',
	linked_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pair_codes (
	code        TEXT PRIMARY KEY,
	customer_id TEXT NOT NULL,
	created_at  TEXT NOT NULL,
	used_at     TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS messages (
	id          TEXT PRIMARY KEY,
	chat_key    TEXT NOT NULL,
	customer_id TEXT NOT NULL DEFAULT '',
	kind        TEXT NOT NULL,             -- text | voice | photo
	text        TEXT NOT NULL DEFAULT '',  -- text, transcript or lines read from the photo
	media_path  TEXT NOT NULL DEFAULT '',
	extraction  TEXT NOT NULL DEFAULT '',  -- parser | ai
	created_at  TEXT NOT NULL
);

-- One order being built in a conversation. status: open | confirmed | cancelled
CREATE TABLE IF NOT EXISTS drafts (
	id               TEXT PRIMARY KEY,
	chat_key         TEXT NOT NULL,
	customer_id      TEXT NOT NULL,
	status           TEXT NOT NULL,
	summary_msg_id   INTEGER NOT NULL DEFAULT 0,
	erp_order_id     TEXT NOT NULL DEFAULT '',
	erp_order_number TEXT NOT NULL DEFAULT '',
	total_ttc        INTEGER NOT NULL DEFAULT 0,
	delivery         TEXT NOT NULL DEFAULT '',
	created_at       TEXT NOT NULL,
	updated_at       TEXT NOT NULL
);

-- Every line ever seen, with the full story of its resolution.
-- level: alias | rules | history | ai | person | none ; status: ok | ask | unknown | removed
CREATE TABLE IF NOT EXISTS lines (
	id           TEXT PRIMARY KEY,
	draft_id     TEXT NOT NULL,
	message_id   TEXT NOT NULL DEFAULT '',
	position     INTEGER NOT NULL,
	said         TEXT NOT NULL,
	norm         TEXT NOT NULL,
	hint         TEXT NOT NULL DEFAULT '',
	qty          REAL NOT NULL,
	unit         TEXT NOT NULL DEFAULT '',
	product_id   TEXT NOT NULL DEFAULT '',
	pack_name    TEXT NOT NULL DEFAULT '',
	pack_factor  INTEGER NOT NULL DEFAULT 0,
	qty_milli    INTEGER NOT NULL DEFAULT 0,
	level        TEXT NOT NULL,
	used         TEXT NOT NULL DEFAULT '[]',
	confidence   REAL NOT NULL DEFAULT 0,
	options      TEXT NOT NULL DEFAULT '[]',
	question     TEXT NOT NULL DEFAULT '',
	status       TEXT NOT NULL,
	checks       TEXT NOT NULL DEFAULT '{}',
	unit_price   INTEGER NOT NULL DEFAULT 0,
	line_ttc     INTEGER NOT NULL DEFAULT 0,
	shadow       TEXT NOT NULL DEFAULT '{}', -- rule id -> product id proposed by shadow rules
	outcome      TEXT NOT NULL DEFAULT '',   -- auto | confirmed | corrected
	created_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS lines_draft ON lines (draft_id, position);

-- Lines whose correct answer is known. origin: shop | clerk | history
CREATE TABLE IF NOT EXISTS ground_truth (
	line_id     TEXT PRIMARY KEY,
	said        TEXT NOT NULL,
	norm        TEXT NOT NULL,
	unit        TEXT NOT NULL DEFAULT '',
	customer_id TEXT NOT NULL DEFAULT '',
	product_id  TEXT NOT NULL,
	level       TEXT NOT NULL,              -- how it was resolved before the answer was known
	origin      TEXT NOT NULL,
	created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS events (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	at      TEXT NOT NULL,
	kind    TEXT NOT NULL,
	object  TEXT NOT NULL DEFAULT '',
	actor   TEXT NOT NULL DEFAULT '',
	details TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS events_object ON events (object, id);

CREATE TABLE IF NOT EXISTS ai_usage (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	at         TEXT NOT NULL,
	model      TEXT NOT NULL,
	purpose    TEXT NOT NULL,               -- transcribe | read | extract | resolve | engineer
	tokens_in  INTEGER NOT NULL DEFAULT 0,
	tokens_out INTEGER NOT NULL DEFAULT 0,
	audio_sec  REAL NOT NULL DEFAULT 0,
	cost       REAL NOT NULL,
	ref        TEXT NOT NULL DEFAULT ''
);
