// Order Desk admin panel — "Carbon Copy". A small single-page app over the JSON API (no build step).
// Vocabulary: the shop's words are the carbon (.ev), the product is the print (.prod), decisions are stamps,
// AI cost is a coin, free is a dashed circle, and every line shows its ladder: alias → rules → AI pick → person.
"use strict";

const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];
const main = $("#main");
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const money = (c) => (Math.round((c || 0) / 100)).toString().replace(/\B(?=(\d{3})+(?!\d))/g, " ") + " DA";
const usd = (v) => "$" + (v || 0).toFixed(v >= 1 ? 2 : v >= 0.01 ? 3 : 4);
const cents = (v) => ((v || 0) * 100).toFixed(2) + "¢";
const pct = (v) => (isFinite(v) ? Math.round(v * 1000) / 10 : 0) + "%";
const isAr = (s) => /[؀-ۿ]/.test(s || "");
const hm = (s) => (s || "").slice(11, 16);
function short(s) {
  if (!s) return "";
  const d = new Date(s.replace(" ", "T"));
  if (isNaN(d)) return s;
  const now = new Date(), y = new Date(now); y.setDate(now.getDate() - 1);
  const t = d.toTimeString().slice(0, 5);
  if (d.toDateString() === now.toDateString()) return "today " + t;
  if (d.toDateString() === y.toDateString()) return "yesterday " + t;
  return d.toLocaleDateString("en-GB", { day: "numeric", month: "short" }) + " " + t;
}
const dayLabel = (d) => new Date(d + "T12:00").toLocaleDateString("en-GB", { day: "numeric", month: "short" });

async function api(path, opts = {}) {
  const res = await fetch("/api" + path, { headers: { "Content-Type": "application/json" }, ...opts,
    body: opts.body && typeof opts.body !== "string" ? JSON.stringify(opts.body) : opts.body });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "HTTP " + res.status);
  return data;
}
function toast(msg, err) {
  const t = $("#toast");
  t.textContent = msg; t.style.background = err ? "#C0262D" : "#0F1E3D"; t.hidden = false;
  clearTimeout(toast._t); toast._t = setTimeout(() => (t.hidden = true), 4200);
}

// ---- vocabulary -----------------------------------------------------------------------------
const ev = (s, cls = "") => `<span class="ev q ${isAr(s) ? "ar" : ""} ${cls}" dir="auto">${esc(s)}</span>`;
const evRaw = (s, cls = "") => `<span class="ev ${isAr(s) ? "ar" : ""} ${cls}" dir="auto">${esc(s)}</span>`;
const STAMP = { live: "● Live", shadow: "◐ Shadow", proposed: "○ Proposed", rejected: "✕ Refused", rolled_back: "↺ Rolled back", disabled: "Disabled",
  trusted: "✓ Trusted", candidate: "○ Candidate", blocked: "✕ Blocked", open: "Open", confirmed: "✓ In ERP", cancelled: "Cancelled", removed: "Removed" };
const stamp = (st, label, cls = "") => `<span class="stamp ${esc(st)} ${cls}">${esc(label || STAMP[st] || st)}</span>`;
const free = () => `<span class="free">free</span>`;
const coin = (v) => `<span class="coin">${(v || 0) >= 0.1 ? usd(v) : cents(v)}</span>`;
const RUNGS = [["A", "alias"], ["R", "rules"], ["AI", "ai"], ["P", "person"]];
function ladder(level) {
  const at = { alias: 0, rules: 1, history: 1, ai: 2, person: 3, none: 3 }[level] ?? 3;
  const label = { alias: "a trusted wording", rules: "rules", history: "copied from the last order", ai: "AI pick", person: "a person", none: "a person" }[level] || level;
  return `<span class="ladder" aria-label="Stopped at ${esc(label)}">${RUNGS.map(([t, k], i) => {
    if (i < at) return `<span class="rung past">${t}</span>`;
    if (i === at) return `<span class="rung at ${level === "history" ? "history" : k}">${level === "history" ? "H" : t}</span>`;
    return `<span class="rung">${t}</span>`;
  }).join("")}</span>`;
}
function lvlText(l) {
  const u = l.used || [];
  const rules = u.filter((x) => x.startsWith("R-"));
  if (l.status !== "ok") return l.options && l.options.length ? "No sure match. Rules don't guess; the AI won't pick alone." : "Not in the catalogue.";
  switch (l.level) {
    case "alias": return "alias · a trusted wording" + (rules.length ? " · " + rules.join(", ") : "");
    case "history": return "copied by code from " + esc((u[0] || "").replace("order:", "order "));
    case "rules":
      if (u.includes("habit")) return "rules · habit: this shop always takes this one";
      return "rules · " + (rules.length ? rules.join(", ") + (u.includes("match") ? " + brand & size match" : "") : "brand + size match");
    case "ai": return u.includes("hint") ? "AI · read from the message" : `AI pick from the shortlist · conf. ${(l.confidence || 0).toFixed(2)}`;
    case "person": return "answered by a person";
  }
  return l.level;
}
const sentenceHTML = (s) => esc(s).replace(/&quot;(.+?)&quot;/g, (_, w) => `<span class="w">“${w}”</span>`);
const SVG = {
  text: `<svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true"><path d="M2 3h10v7H6l-3 2v-2H2z" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/></svg>`,
  voice: `<svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true"><path d="M2 6v2M5 4v6M8 2v10M11 5v4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>`,
  photo: `<svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true"><rect x="1.5" y="3" width="11" height="8.5" rx="1.5" fill="none" stroke="currentColor" stroke-width="1.4"/><circle cx="7" cy="7.2" r="2" fill="none" stroke="currentColor" stroke-width="1.4"/></svg>`,
  invoice: `<svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true"><path d="M3 1.5h8v11l-2-1-2 1-2-1-2 1z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/><path d="M5 5h4M5 7.5h4" stroke="currentColor" stroke-width="1.2"/></svg>`,
  play: `<svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true"><path d="M3 1.5v9l7-4.5z" fill="currentColor"/></svg>`,
  pause: `<svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true"><path d="M3 1.5h2v9H3zM7 1.5h2v9H7z" fill="currentColor"/></svg>`,
  up: `<svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true"><path d="M8 11V3M4.5 6.5 8 3l3.5 3.5M3 13h10" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>`,
};
const srcTag = (kind, label) => `<span class="src">${SVG[kind] || (kind === "products" ? SVG.photo : SVG.text)}${esc(label || { voice: "voice", photo: "photo", products: "products", text: "text", invoice: "invoice" }[kind] || kind)}</span>`;
function packText(l) {
  if (!l.pack_factor) return `${l.qty} u.`;
  const f = l.pack_factor / 1000, name = (l.pack_name || "pack").toLowerCase();
  const shown = /[x×]\s*\d+/.test(name) ? name.replace(/\s*x\s*(\d+)/, " ×$1") : `${name} ×${f}`;
  return `${l.qty} × ${esc(shown)} <span class="muted">= ${Math.round(l.qty_milli / 1000)} u.</span>`;
}
function bars(seed) {
  let h = 0; for (const c of seed || "x") h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return `<span class="bars" aria-hidden="true">${Array.from({ length: 22 }, () => { h = (h * 1103515245 + 12345) >>> 0; return `<i style="height:${6 + (h >> 16) % 21}px"></i>`; }).join("")}</span>`;
}

// ---- shell ----------------------------------------------------------------------------------
let overview = {}, settingsCache = {};
async function refreshShell() {
  try { overview = await api("/overview"); } catch { return; }
  const e = overview.erp || {};
  $("#tenant").textContent = e.connected ? (e.name || "ERP") + " · " + (e.user || "") : "ERP not connected";
  const ni = $("#n-inbox"); ni.hidden = !overview.pending_lines; ni.textContent = overview.pending_lines;
  api("/drafts?kind=invoice&status=open").then((x) => { const n = $("#n-inv"); n.hidden = !x.items.length; n.textContent = x.items.length; }).catch(() => {});
  const spent = overview.spend_today || 0, budget = overview.budget_day || 0;
  const row = (on, name, r, cls) => `<div class="st"><span class="dot ${cls || (on ? "" : "off")}"></span>${name}<span class="r">${esc(r)}</span></div>`;
  $("#status").innerHTML =
    row(e.connected, "ERP", e.connected ? "connected" : "offline") +
    row(!!overview.telegram, "Telegram", overview.telegram ? "@" + overview.telegram : "off") +
    row(overview.meta, "Meta models", overview.meta ? "on" : "off") +
    row(true, "Rule engineer", overview.engineer_running ? "running" : "idle", overview.engineer_running ? "run" : "") +
    `<div class="col" style="gap:6px;margin-top:4px"><div class="st">AI today<span class="r" style="color:#F2CB6B">${usd(spent)} / ${usd(budget)}</span></div>
      <div style="height:6px;border-radius:3px;background:#253A66;overflow:hidden"><div style="width:${Math.min(100, budget ? spent / budget * 100 : 0)}%;height:100%;background:#E3B341"></div></div></div>`;
}

const routes = {};
const route = (n, fn) => (routes[n] = fn);
const ALIAS = { draft: "orders", product: "catalogue", rule: "rules", invoice: "invoices" };
let timer = null;
async function go() {
  clearInterval(timer);
  const [name, arg] = (location.hash.replace(/^#\/?/, "") || "today").split("/");
  $$(".nav").forEach((a) => a.classList.toggle("on", a.dataset.r === (ALIAS[name] || name)));
  try { await (routes[name] || routes.today)(arg ? decodeURIComponent(arg) : ""); }
  catch (e) { main.innerHTML = `<div class="warnbox">${esc(e.message)}</div>`; }
  refreshShell();
}
window.addEventListener("hashchange", go);
const head = (title, sub, right = "") => `<header class="head"><div class="col" style="gap:6px"><h1 class="h1">${title}</h1>${sub ? `<p class="muted">${sub}</p>` : ""}</div>
  <div class="row">${right}</div></header>`;
const tile = (label, value, sub, tone) => `<div class="card tile"><span class="cap">${label}</span>
  <span class="big" style="font-size:36px;color:${{ free: "var(--free)", paid: "var(--paid)", human: "var(--human)" }[tone] || "var(--ink)"}">${value}</span>
  <span class="muted" style="font-size:12px">${sub || ""}</span></div>`;

// ---- today ----------------------------------------------------------------------------------
route("today", async () => {
  const [ov, open, inv, evs, learn] = await Promise.all([api("/overview"), api("/drafts?kind=order&status=open"), api("/drafts?kind=invoice&status=open"),
    api("/events?limit=40"), api("/learning?days=1")]);
  const d = (learn.days || [])[0] || { lines: 0, alias: 0, rules: 0 };
  const waiting = [...open.items.filter((x) => x.pending).map((x) => ({ ...x, kind: "order" })), ...inv.items.map((x) => ({ ...x, kind: "invoice" }))];
  main.innerHTML = head("Today", "What came in, what waits for you, and what the system learned.",
    stamp("sample", new Date().toLocaleDateString("en-GB", { weekday: "long", day: "numeric", month: "long" }))) + `
    <div class="tiles">
      ${tile("Lines today", d.lines, `${ov.orders} orders confirmed in total`)}
      ${tile("Resolved without AI today", d.lines ? pct((d.alias + d.rules) / d.lines) : "—", "by aliases and rules", "free")}
      ${tile("Waiting for you", waiting.length, `${ov.pending_lines} question(s) · ${inv.items.length} invoice(s)`, waiting.length ? "human" : "")}
      ${tile("AI today", usd(ov.spend_today), `of ${usd(ov.budget_day)} budget`, "paid")}
    </div>
    <div class="split">
      <section class="card" style="flex:1 1 420px;overflow:hidden">
        <div class="section-h"><h2 class="h2">Waiting for you</h2><span class="muted" style="font-size:12px">you only see what needs you</span></div>
        ${waiting.length ? waiting.map((w) => `<a class="rrow" href="#/${w.kind === "invoice" ? "invoices" : "inbox"}/${w.id}">
            ${srcTag(w.kind === "invoice" ? "invoice" : (w.kinds || "text").split(",")[0])}<b style="flex:1">${esc(w.customer || "—")}</b>
            ${w.kind === "invoice" ? stamp("open", "To approve") : stamp("ask", `? ${w.pending} to answer`)}<span class="mono muted" style="font-size:11px">${hm(w.updated_at)}</span></a>`).join("")
          : `<div class="empty">Nothing waits for you.</div>`}
      </section>
      <section class="card" style="flex:1.4 1 520px;overflow:hidden">
        <div class="section-h"><h2 class="h2">What happened</h2><span class="muted" style="font-size:12px">the model proposes · the code decides · people correct</span></div>
        ${evs.items.map(eventRow).filter(Boolean).slice(0, 18).join("") || `<div class="empty">Nothing yet.</div>`}
      </section>
    </div>`;
  timer = setInterval(() => { if (location.hash.startsWith("#/today") || !location.hash) go(); }, 15000);
});
function eventRow(e) {
  const x = e.details || {};
  const map = {
    "order.created": ["sys", `Order <b>${esc(x.number)}</b> created in the ERP`, `${x.lines} lines`, `#/orders/${e.object}`],
    "receipt.created": ["sys", `Goods receipt <b>${esc(x.number)}</b> posted`, esc(x.supplier || ""), `#/invoices/${e.object}`],
    "alias.trusted": ["person", `A wording became trusted: ${evRaw(x.text || "")}`, `${x.witnesses || ""} confirmations`, `#/product/${encodeURIComponent(e.object)}`],
    "alias.created": ["person", `New wording learned: ${evRaw(x.text || "")}`, "candidate", `#/product/${encodeURIComponent(e.object)}`],
    "alias.blocked": ["person", `A wording was blocked: ${evRaw(x.text || "")}`, "", ""],
    "rule.proposed": ["model", `Rule <b>${esc(e.object)}</b> proposed`, esc(x.sentence || ""), `#/rule/${e.object}`],
    "rule.shadow": ["sys", `<b>${esc(e.object)}</b> admitted by the gate → shadow`, esc(x.gate || ""), `#/rule/${e.object}`],
    "rule.live": ["sys", `<b>${esc(e.object)}</b> went live`, x.agreements ? `${x.agreements} agreements` : "", `#/rule/${e.object}`],
    "rule.rejected": ["sys", `<b>${esc(e.object)}</b> refused`, esc(x.gate || ""), `#/rule/${e.object}`],
    "rule.rolled_back": ["sys", `<b>${esc(e.object)}</b> rolled back`, x.strikes ? `${x.strikes} corrections` : "", `#/rule/${e.object}`],
    "rule.strike": ["person", `A person corrected <b>${esc(e.object)}</b>`, x.said ? ev(x.said) : "", `#/rule/${e.object}`],
    "line.corrected": ["person", "A line was corrected", x.said ? ev(x.said) : "", ""],
    "chat.linked": ["person", "A Telegram chat was linked", esc(x.customer || x.role || ""), "#/customers"],
    "engineer.finished": ["model", "Rule engineer run", x.error ? esc(x.error) : `${(x.proposed || []).length} proposed · ${(x.accepted || []).length} admitted · ${usd(x.cost)}`, "#/rules"],
  }[e.kind];
  if (!map) return "";
  const [who, what, det, href] = map;
  const i = { model: "W", sys: "{}", person: "P" }[who];
  return `<a class="rrow" style="padding:12px 20px" ${href ? `href="${href}"` : ""}><span class="who ${who}"><i>${i}</i></span>
    <span style="flex:1;min-width:0">${what} <span class="muted" style="font-size:12px">${det}</span></span><span class="mono muted" style="font-size:11px">${short(e.at)}</span></a>`;
}

// ---- inbox & orders -------------------------------------------------------------------------
route("inbox", (sel) => draftsScreen("open", sel));
route("orders", (sel) => draftsScreen("", sel));
route("draft", (sel) => draftsScreen("", sel));
let current = "";
async function draftsScreen(status, sel, quiet) {
  const inbox = status === "open";
  const [list, custs] = await Promise.all([api("/drafts?kind=order" + (status ? "&status=" + status : "")), quiet ? null : api("/customers")]);
  const items = list.items;
  if (!sel && items.length) sel = items[0].id;
  current = sel;
  const questions = items.reduce((a, d) => a + d.pending, 0), ready = items.filter((d) => !d.pending && d.lines).length;
  const listHTML = items.map((d) => qi(d, sel, inbox)).join("") || `<div class="card dashed empty">${inbox ? "No order is being built right now." : "No order yet."}</div>`;
  if (quiet && $("#qlist")) { $("#qlist").innerHTML = listHTML; if (sel) renderDraft(sel); return; }
  main.innerHTML = head(inbox ? "Inbox" : "Orders", inbox ? "Orders being built in the chats right now. You only see what needs you." : "Every order Order Desk handled, with its evidence.",
    inbox ? `${questions ? stamp("ask", `? ${questions} question${questions > 1 ? "s" : ""}`) : stamp("ready", "✓ Nothing to answer")}<span class="muted" style="font-size:13px">${items.length} chat(s) building · ${ready} ready to confirm</span>` : "") + `
    <div class="split">
      <aside aria-label="Chats"><div class="col" style="gap:10px" id="qlist">${listHTML}</div>${inbox ? tryBox(custs.items) : ""}</aside>
      <section class="card" style="overflow:hidden" id="dv">${items.length ? `<div class="reading"><div class="skel" style="width:60%"></div><div class="skel" style="width:40%"></div></div>` : `<div class="empty">Send a message to the bot on Telegram, or try one on the left.</div>`}</section>
    </div>`;
  bindTry();
  if (sel) renderDraft(sel);
  if (inbox) timer = setInterval(() => { if (!document.activeElement || !/TEXTAREA|INPUT|SELECT/.test(document.activeElement.tagName)) draftsScreen("open", current, true); }, 6000);
}
function qi(d, sel, inbox) {
  const kinds = (d.kinds || "text").split(",").filter(Boolean);
  const state = d.status === "confirmed" ? stamp("confirmed", "✓ " + d.erp_order_number, "sm") : d.pending ? stamp("ask", "? Asks you", "sm") : d.status === "open" ? stamp("ready", "✓ Ready", "sm") : stamp(d.status, "", "sm");
  return `<a class="qi ${d.id === sel ? "on" : ""}" href="#/${inbox ? "inbox" : "orders"}/${d.id}">
    <div class="row" style="justify-content:space-between"><b style="font-size:15px">${esc(d.customer || "—")}</b><span class="mono muted" style="font-size:11px">${hm(d.updated_at)}</span></div>
    <div class="row" style="gap:10px">${kinds.map((k) => srcTag(k)).join("")}<span class="muted" style="font-size:12px">${d.lines} line${d.lines === 1 ? "" : "s"}${d.copied ? ` · ${d.copied} copied from last order` : ""}</span></div>
    <div class="row" style="gap:8px">${state}${d.ai_cost > 0 ? coin(d.ai_cost) : free()}</div></a>`;
}
function tryBox(custs) {
  return `<div class="card dashed" style="padding:16px;display:flex;flex-direction:column;gap:10px;margin-top:8px">
    <span class="cap">Try a message · demo</span>
    <label class="l" for="t-shop">As shop</label><select id="t-shop" class="inp">${custs.map((c) => `<option value="${esc(c.id)}">${esc(c.name)}</option>`).join("")}</select>
    <label class="l" for="t-msg">Message</label><textarea id="t-msg" class="inp ev" rows="3" placeholder="3 cartons selecto 2l&#10;2 fardou ifri 1.5&#10;1 candia"></textarea>
    <label class="l" for="t-photo">Or a photo · a list, or the products</label><input id="t-photo" class="inp" type="file" accept="image/*">
    <div class="row" style="justify-content:space-between"><span class="muted" style="font-size:11px;max-width:200px">Runs the real pipeline. Lists use the free parser; chatty text uses the AI; photos of products use SAM 3.1.</span>
    <button class="btn primary sm" id="t-run" type="button">Run</button></div></div>`;
}
function bindTry() {
  const b = $("#t-run");
  if (!b) return;
  b.onclick = async () => {
    const text = $("#t-msg").value.trim(), file = $("#t-photo").files[0]; if (!text && !file) return;
    b.disabled = true; b.textContent = file ? "Looking…" : "Running…";
    try {
      const image = file ? await new Promise((ok, ko) => { const fr = new FileReader(); fr.onload = () => ok(fr.result); fr.onerror = ko; fr.readAsDataURL(file); }) : "";
      const r = await api("/try", { method: "POST", body: { customer_id: $("#t-shop").value, text, image } });
      if (!r.draft) toast(r.extraction.reply || "No product found in this message.");
      else { $("#t-msg").value = ""; $("#t-photo").value = ""; location.hash = "#/inbox/" + r.draft; go(); }
    } catch (e) { toast(e.message, true); }
    b.disabled = false; b.textContent = "Run";
  };
}

async function renderDraft(id) {
  const v = $("#dv"); if (!v) return;
  const d = await api("/drafts/" + id);
  const dr = d.draft, open = dr.status === "open";
  const lines = d.lines.filter((l) => l.status !== "removed");
  const msgCost = {}; d.messages.forEach((m) => (msgCost[m.id] = m.ai_cost));
  const paidIn = {}; lines.forEach((l) => { if (l.level === "ai" || l.level === "person" || l.status !== "ok") paidIn[l.message_id] = (paidIn[l.message_id] || 0) + 1; });
  const lineCost = (l) => (l.level === "ai" || l.level === "person") && msgCost[l.message_id] ? msgCost[l.message_id] / (paidIn[l.message_id] || 1) : 0;
  const waiting = lines.filter((l) => l.status === "ask" || l.status === "unknown").length;
  const freeN = lines.filter((l) => l.status === "ok" && ["alias", "rules", "history"].includes(l.level)).length;
  const aiN = lines.filter((l) => l.status === "ok" && l.level === "ai").length;
  const totalAI = d.messages.reduce((a, m) => a + m.ai_cost, 0);
  v.innerHTML = `
    <div class="row" style="justify-content:space-between;align-items:flex-start;padding:20px 20px 16px;gap:16px">
      <div class="col" style="gap:6px"><div class="row" style="gap:12px"><h2 class="disp" style="font-size:30px;font-weight:800;letter-spacing:-0.02em">${esc(dr.customer)}</h2>
        ${dr.status === "confirmed" ? stamp("confirmed", "✓ " + dr.erp_order_number + " in ERP") : stamp(dr.status)}</div>
        <span class="muted" style="font-size:12px">${dr.chat_key.startsWith("tg:") ? "Telegram chat" : "Panel"} · started ${short(dr.created_at)} · <span class="mono">${esc(dr.chat_key)}</span>${dr.delivery ? " · 🚚 " + esc(dr.delivery) : ""}</span></div>
      <div class="col" style="align-items:flex-end;gap:6px"><span class="big" style="font-size:28px">${money(d.total)}</span>
        <span class="muted row" style="font-size:12px;gap:8px">${waiting ? `+${waiting} line${waiting > 1 ? "s" : ""} waiting · ` : ""}AI on this order ${totalAI > 0 ? coin(totalAI) : free()}</span></div>
    </div>
    <div class="row" style="padding:0 20px 18px;gap:14px;align-items:stretch">${d.messages.map(slipOf).join("")}</div>
    <div class="band"><span class="cap">Lines · how each one was resolved</span>
      <span class="muted" style="font-size:12px">cheapest first: <span class="mono">A</span> alias → <span class="mono">R</span> rules → <span class="mono">AI</span> pick → <span class="mono">P</span> person</span></div>
    ${d.lines.map((l, i) => orderLine(l, i + 1, open, lineCost(l))).join("")}
    <div class="foot"><span class="row" style="gap:16px;font-size:13px"><span><b>${freeN}</b> of ${lines.length} free</span><span><b>${aiN}</b> AI pick${aiN === 1 ? "" : "s"}</span>
      ${waiting ? `<span style="color:var(--human)"><b>${waiting}</b> waiting for you</span>` : ""}</span>
      ${open ? `<span class="row"><span class="muted" style="font-size:12px">${waiting ? "Answer the open line" + (waiting > 1 ? "s" : "") + " to confirm" : "The shop confirms in the chat — or you can, here."}</span>
        <button class="btn go" id="confirm" ${waiting ? "disabled" : ""}>Confirm into ERP →</button></span>` : stamp("confirmed", "✓ " + dr.erp_order_number)}</div>`;
  bindLines(v, () => renderDraft(id));
  const c = $("#confirm");
  if (c) c.onclick = async () => {
    c.disabled = true;
    try { const r = await api(`/drafts/${id}/confirm`, { method: "POST" }); toast("Order " + r.number + " created in the ERP"); go(); }
    catch (e) { toast(e.message, true); c.disabled = false; }
  };
  $$(".vplay", v).forEach((b) => (b.onclick = () => {
    const a = b.parentElement.querySelector("audio");
    if (a.paused) { a.play(); b.innerHTML = SVG.pause; a.onended = () => (b.innerHTML = SVG.play); } else { a.pause(); b.innerHTML = SVG.play; }
  }));
}
function slipOf(m) {
  const top = (k, what) => `<div class="row" style="justify-content:space-between;gap:8px">${srcTag(k)}<span class="muted" style="font-size:12px;flex:1">${what}</span>${m.ai_cost > 0 ? coin(m.ai_cost) : free()}</div>`;
  const reader = m.extraction === "parser" ? "free parser" : "Muse Spark";
  if (m.kind === "voice") return `<div class="col" style="flex:1 1 280px;min-width:0;gap:6px">${top("voice", `${hm(m.created_at)} · Voice Transcribe → ${reader}`)}
    <div class="slip" style="gap:4px"><div class="row" style="gap:10px;padding:4px 0">${m.media ? `<button class="btn icon play vplay" type="button" aria-label="Play voice note">${SVG.play}</button><audio src="${esc(m.media)}" preload="none"></audio>` : ""}${bars(m.id)}</div>
    <bdi class="ev ${isAr(m.text) ? "ar" : ""}" dir="auto" style="display:block">${esc(m.text)}</bdi></div></div>`;
  if (m.kind === "photo" || m.kind === "products") return `<div class="col" style="flex:1 1 280px;min-width:0;gap:6px">${top(m.kind, `${hm(m.created_at)} · ${m.kind === "products" ? "photo of the products → SAM 3.1 finds them, Muse Spark reads them" : "photo of a list → " + reader}`)}
    <div class="slip">${m.media ? `<img src="${esc(m.media)}" alt="the photo the shop sent">` : ""}${m.text.split("\n").map((t) => evRaw(t)).join("")}</div></div>`;
  return `<div class="col" style="flex:1 1 280px;min-width:0;gap:6px">${top("text", `${hm(m.created_at)} · ${reader}`)}
    <div class="slip">${m.text.split("\n").filter((t) => t.trim()).map((t) => evRaw(t)).join("")}</div></div>`;
}
function checkChips(l) {
  const c = l.checks || {};
  const out = [];
  Object.entries(l.shadow || {}).forEach(([rid, pid]) => out.push(`<span class="chip shadow">◐ ${esc(rid)} in shadow ${pid === l.product_id ? "agrees" : "disagrees"}</span>`));
  if (c.stock) out.push(`<span class="chip warn">! ${esc(c.stock)}</span>`); else if (l.status === "ok" && l.line_ttc && !c.history && !l.pack_rule && l.unit_price) out.push(`<span class="chip good">✓ in stock</span>`);
  for (const k of ["usual", "pack", "unit", "price", "new"]) if (c[k]) out.push(`<span class="chip warn">! ${esc(c[k])}</span>`);
  if (c.offer) out.push(`<span class="chip gift">🎁 ${esc(c.offer)}</span>`);
  if (c.corrected_from) out.push(`<span class="chip" style="color:var(--human);border-color:#C9B6E3">corrected · was ${esc(l.option_names[c.corrected_from] || "")}</span>`);
  return out.join("");
}
function orderLine(l, n, open, cost) {
  const asks = l.status === "ask" || l.status === "unknown";
  const removed = l.status === "removed";
  const said = l.said ? ev(l.said) : `<span class="muted">${esc((l.checks || {}).history || "same as last time")}</span>`;
  if (asks) {
    const opts = l.options || [];
    return `<div class="line asks"><span class="mono" style="padding-top:2px;color:var(--human);font-weight:600">${n}</span><div>
      <div class="lr1">${said}${l.hint ? `<span class="muted" style="font-size:12px">reader: ${esc(l.hint)}</span>` : ""}<span class="muted">→</span>
        <span style="color:var(--human);font-weight:600">${esc(opts.length ? (l.question || "Which one?") : "Not in the catalogue")}</span><span class="qty">${l.qty} ${esc(l.unit || "")}</span></div>
      <div class="lr2">${ladder("none")}<span class="lvl">${lvlText(l)}</span>${checkChips(l)}</div>
      ${opts.length ? `<div class="lr3">${opts.map((o) => `<button class="btn opt" data-line="${l.id}" data-pick="${esc(o)}">${esc(l.option_names[o] || o)}</button>`).join("")}</div>` : ""}
      ${open ? `<div class="row" style="margin-top:10px;justify-content:space-between"><span class="muted" style="font-size:12px">The shop is asked in the chat too. Your answer becomes a candidate wording for this shop.</span>
        <button class="btn ghost sm" data-remove="${l.id}">Remove line</button></div>` : ""}</div></div>`;
  }
  const alts = [...new Set([...(l.options || []), ...(l.alts || [])])].filter((o) => o !== l.product_id);
  return `<div class="line ${removed ? "removed" : ""}"><span class="mono muted" style="padding-top:2px">${n}</span><div>
    <div class="lr1">${said}<span class="muted">→</span><span class="prod">${esc(l.product_name || l.product_id)}</span><span class="qty">${packText(l)}</span><span class="amt">${l.line_ttc ? money(l.line_ttc) : "—"}</span></div>
    <div class="lr2">${ladder(l.level)}<span class="lvl">${lvlText(l)}</span>${checkChips(l)}
      <span class="row" style="margin-left:auto;gap:8px">${cost > 0 ? coin(cost) : free()}${removed ? stamp("removed") : l.said ? `<button class="btn sm" data-alts="${l.id}">Correct</button>` : ""}
      ${open && !removed ? `<button class="btn icon ghost" data-remove="${l.id}" aria-label="Remove line ${n}">✕</button>` : ""}</span></div>
    <div class="lr3" hidden id="alts-${l.id}">${alts.map((o) => `<button class="btn opt" data-line="${l.id}" data-fix="1" data-pick="${esc(o)}">${esc(l.option_names[o] || o)}</button>`).join("") || `<span class="muted" style="font-size:12px">No close alternative in the catalogue.</span>`}
      <span class="muted" style="font-size:12px;align-self:center">${open ? "A correction blocks this wording and teaches the right one." : "Corrects what was learned (the ERP order itself is corrected in the ERP)."}</span></div>
  </div></div>`;
}
function bindLines(v, again) {
  $$("[data-pick]", v).forEach((b) => (b.onclick = async () => {
    b.disabled = true;
    try { await api(`/lines/${b.dataset.line}/choose`, { method: "POST", body: { product_id: b.dataset.pick, correction: b.dataset.fix === "1" } }); again(); }
    catch (e) { toast(e.message, true); b.disabled = false; }
  }));
  $$("[data-remove]", v).forEach((b) => (b.onclick = async () => { await api(`/lines/${b.dataset.remove}/remove`, { method: "POST" }); again(); }));
  $$("[data-alts]", v).forEach((b) => (b.onclick = () => { const a = $("#alts-" + b.dataset.alts); a.hidden = !a.hidden; }));
}

// ---- supplier invoices ----------------------------------------------------------------------
route("invoices", (sel) => invoicesScreen(sel));
route("invoice", (sel) => invoicesScreen(sel));
async function invoicesScreen(sel) {
  const [list, own] = await Promise.all([api("/drafts?kind=invoice"), api("/owner")]);
  const items = list.items;
  if (!sel && items.length) sel = items[0].id;
  const linked = own.chats.length;
  main.innerHTML = head("Supplier invoices", "A photo or PDF becomes a goods receipt in the ERP. Each supplier's wordings are learned.", `
      <label class="dropzone" id="drop"><input type="file" id="file" accept="image/*,application/pdf" hidden>${SVG.up} Drop a photo or PDF</label>
      <div class="card row" style="padding:6px 6px 6px 14px;gap:10px"><span class="dot ${linked ? "" : "off"}"></span>
        <span style="font-size:13px">${linked ? "Owner Telegram: " + own.chats.map((c) => esc(c.name || c.chat) + ` <button class="btn ghost sm" data-unlink="${esc(c.chat)}">Unlink</button>`).join(", ") : "Owner Telegram not linked"}</span>
        <span id="owncode"></span><button class="btn sm" id="ownpair">Pairing code</button></div>`) + `
    <div class="tabs">${items.map((d) => `<a class="tab ${d.id === sel ? "on" : ""}" href="#/invoices/${d.id}">
        <div class="row" style="justify-content:space-between;gap:10px"><b>${esc(d.customer || "Unknown supplier")}</b><span class="mono muted" style="font-size:11px">${hm(d.updated_at)}</span></div>
        <div class="row" style="gap:8px">${d.status === "confirmed" ? stamp("erp", "✓ In ERP", "sm") : d.status === "open" ? stamp("open", "To approve", "sm") : stamp(d.status, "", "sm")}
          <span class="mono muted" style="font-size:11px">${esc(d.status === "confirmed" ? d.erp_order_number : d.doc_number || "")}</span></div></a>`).join("")}</div>
    <div id="iv">${items.length ? "" : `<div class="card empty">No invoice yet. Drop one above, or send it to the bot from your owner chat.</div>`}</div>`;
  const file = $("#file"), drop = $("#drop");
  const upload = async (f) => {
    if (!f) return;
    $("#iv").innerHTML = `<div class="card reading"><span class="cap">Reading ${esc(f.name)} with Muse Spark…</span><div class="skel" style="width:70%"></div><div class="skel" style="width:52%"></div><div class="skel" style="width:61%"></div></div>`;
    const fd = new FormData(); fd.append("file", f);
    try {
      const res = await fetch("/api/invoices", { method: "POST", body: fd });
      const d = await res.json();
      if (!res.ok) throw new Error(d.error || "HTTP " + res.status);
      location.hash = "#/invoices/" + d.draft; go();
    } catch (e) { toast(e.message, true); go(); }
  };
  file.onchange = () => upload(file.files[0]);
  drop.ondragover = (e) => { e.preventDefault(); drop.classList.add("on"); };
  drop.ondragleave = () => drop.classList.remove("on");
  drop.ondrop = (e) => { e.preventDefault(); drop.classList.remove("on"); upload(e.dataTransfer.files[0]); };
  $("#ownpair").onclick = async () => {
    const r = await api("/owner/pair-code", { method: "POST" });
    $("#owncode").innerHTML = `<span class="stamp scope" style="font-size:14px;letter-spacing:.2em">${esc(r.code)}</span> <a style="font-size:12px" target="_blank" href="https://t.me/${esc(r.bot)}?start=${esc(r.code)}">open in Telegram</a>`;
  };
  $$("[data-unlink]").forEach((b) => (b.onclick = async (e) => { e.preventDefault(); await api("/chats/unlink", { method: "POST", body: { chat: b.dataset.unlink } }); toast("Owner chat unlinked"); go(); }));
  if (sel) renderInvoice(sel);
}
async function renderInvoice(id) {
  const v = $("#iv");
  const d = await api("/drafts/" + id);
  const dr = d.draft, inv = d.invoice || {}, open = dr.status === "open";
  const msg = d.messages[0] || {};
  const lines = d.lines, live = lines.filter((l) => l.status !== "removed");
  const sum = live.reduce((a, l) => a + (l.line_ttc || 0), 0);
  const units = live.filter((l) => l.status === "ok").reduce((a, l) => a + Math.round((l.qty_milli || 0) / 1000), 0);
  const byAlias = live.filter((l) => l.level === "alias").length, byRules = live.filter((l) => l.level === "rules").length;
  const byAI = live.filter((l) => l.status === "ok" && (l.level === "ai" || l.level === "person")).length;
  const asks = live.filter((l) => l.status !== "ok").length;
  const flagged = live.filter((l) => (l.checks || {}).price || (l.checks || {}).pack).length;
  const totalOK = !inv.doc_total || Math.abs(sum - inv.doc_total) <= Math.max(100, inv.doc_total * 0.005);
  const pdf = msg.media && msg.media.endsWith(".pdf");
  const paper = msg.media ? (pdf ? `<embed src="${esc(msg.media)}" type="application/pdf">` : `<img src="${esc(msg.media)}" alt="the invoice as received">`) : "";
  v.innerHTML = `<div class="split">
    <aside style="max-width:440px;flex:1 1 360px;gap:18px">
      ${paper ? `<div class="paper">${paper}</div>` : ""}
      <div class="card pad col" style="gap:10px"><div class="row" style="justify-content:space-between"><span class="cap">Totals checked against the print</span>${totalOK ? `<span class="ok">✓</span>` : `<span class="no">!</span>`}</div>
        <table class="tbl num" style="font-size:13px"><tr><th></th><th class="cap r">Lines add up to</th><th class="cap r">Printed</th></tr>
          <tr><td>Total HT</td><td class="r mono">${money(sum)}</td><td class="r mono">${inv.doc_total ? money(inv.doc_total) : "—"}</td></tr></table>
        ${totalOK ? "" : `<div class="warnbox">${esc((inv.checks || {}).total || "The lines don't add up to the printed total.")}</div>`}</div>
    </aside>
    <section class="card" style="overflow:hidden">
      <div class="col" style="padding:20px;gap:10px">
        <span class="mono muted" style="font-size:12px">Invoice n° ${esc(inv.doc_number || "—")} · ${esc(inv.doc_date || "")} · read ${short(dr.created_at)}</span>
        <div class="row" style="justify-content:space-between"><h2 class="disp" style="font-size:30px;font-weight:800;letter-spacing:-0.02em">${esc(inv.supplier || inv.supplier_said || "Unknown supplier")}</h2>
          ${dr.status === "confirmed" ? stamp("erp", "✓ " + dr.erp_order_number + " in ERP") : dr.status === "open" ? stamp("open", "To approve") : stamp(dr.status)}</div>
        ${inv.supplier ? `<span class="row" style="gap:8px;font-size:13px"><span class="ok">✓</span>Supplier recognised from the header ${ev(inv.supplier_said)}</span>`
          : `<div class="row"><span class="chip warn">! Supplier not recognised: ${ev(inv.supplier_said || "—")}</span>${open ? `<select id="sup" class="inp" style="width:auto">${["<option value=''>Choose the supplier…</option>"].concat((d.suppliers || []).map((s) => `<option value="${esc(s.id)}">${esc(s.name)}</option>`)).join("")}</select>` : ""}</div>`}
        ${inv.supplier && open ? `<details style="font-size:12px"><summary class="muted" style="cursor:pointer">Wrong supplier?</summary><select id="sup" class="inp" style="width:auto;margin-top:6px">${(d.suppliers || []).map((s) => `<option value="${esc(s.id)}" ${s.id === inv.supplier_id ? "selected" : ""}>${esc(s.name)}</option>`).join("")}</select></details>` : ""}
        <div class="row" style="gap:16px;font-size:13px">${byAlias ? `<span class="row" style="gap:6px"><span class="rung at alias">A</span><b>${byAlias} of ${live.length}</b> by aliases</span>` : ""}
          ${byRules ? `<span class="row" style="gap:6px"><span class="rung at rules">R</span><b>${byRules}</b> by rules</span>` : ""}
          ${byAI ? `<span class="row" style="gap:6px"><span class="rung at ai">AI</span><b>${byAI}</b> by the AI or a person</span>` : ""}
          ${asks ? `<span class="row" style="gap:6px"><span class="rung at person">P</span><b>${asks}</b> to check</span>` : ""}</div>
        <span class="row muted" style="gap:8px;font-size:12px">reading the ${pdf ? "PDF" : "photo"} ${coin(msg.ai_cost)} · matching ${byAI ? "AI on " + byAI + " line(s)" : free()}</span>
      </div>
      <div class="band"><span class="cap">Printed → matched</span><span class="cap">Packs · unit cost · line</span></div>
      ${lines.map((l, i) => invoiceLine(l, i + 1, open)).join("")}
      <div class="foot"><span class="col" style="gap:2px"><span style="font-size:14px"><b>${units} units</b> into stock · <b>${money(sum)}</b> HT</span>
        ${flagged ? `<span style="font-size:12px;color:var(--refused)">${flagged} price or pack to check before approving</span>` : asks ? `<span style="font-size:12px;color:var(--human)">${asks} line(s) to answer</span>` : `<span class="muted" style="font-size:12px">every line matched</span>`}</span>
        ${open ? `<button class="btn go" id="approve" ${asks || !inv.supplier_id ? "disabled" : ""}>Approve → goods receipt in ERP</button>` : dr.status === "confirmed" ? stamp("erp", "✓ " + dr.erp_order_number) : stamp(dr.status)}</div>
    </section></div>`;
  bindLines(v, () => renderInvoice(id));
  const sup = $("#sup");
  if (sup) sup.onchange = async () => { if (!sup.value) return; await api(`/drafts/${id}/supplier`, { method: "POST", body: { supplier_id: sup.value } }); toast("Supplier set — lines matched again in its scope"); renderInvoice(id); };
  const a = $("#approve");
  if (a) a.onclick = async () => {
    a.disabled = true; a.textContent = "Booking…";
    try { const r = await api(`/drafts/${id}/confirm`, { method: "POST" }); toast("Goods receipt " + r.number + " posted in the ERP"); go(); }
    catch (e) { toast(e.message, true); a.disabled = false; a.textContent = "Approve → goods receipt in ERP"; }
  };
}
function invoiceLine(l, n, open) {
  const c = l.checks || {};
  const asks = l.status === "ask" || l.status === "unknown";
  const flag = c.price || c.pack;
  const unit = l.qty_milli ? Math.round((l.line_ttc || 0) / (l.qty_milli / 1000)) : 0;
  const alts = [...new Set([...(l.options || []), ...(l.alts || [])])].filter((o) => o !== l.product_id);
  const body = asks
    ? `<div class="lr1">${evRaw(l.said)}<span class="muted">→</span><span style="color:var(--human);font-weight:600">${esc((l.options || []).length ? (l.question || "Which product?") : "Not in the catalogue")}</span><span class="amt" style="margin-left:auto">${money(l.line_ttc)}</span></div>
       <div class="lr2">${ladder("none")}<span class="lvl">${lvlText(l)}</span>${checkChips(l)}</div>
       <div class="lr3">${(l.options || []).map((o) => `<button class="btn opt" data-line="${l.id}" data-pick="${esc(o)}">${esc(l.option_names[o] || o)}</button>`).join("")}${open ? `<button class="btn ghost sm" data-remove="${l.id}">Skip line</button>` : ""}</div>`
    : `<div class="lr1">${evRaw(l.said)}<span class="muted">→</span><span class="prod">${esc(l.product_name || l.product_id)}</span><span class="qty">${packText(l)}</span><span class="amt">${money(l.line_ttc)}<div class="muted mono" style="font-size:11px;font-weight:400">${unit ? money(unit) + " / u." : ""}</div></span></div>
       <div class="lr2">${ladder(l.level)}<span class="lvl">${lvlText(l)}</span>${checkChips(l)}<span class="row" style="margin-left:auto;gap:8px">${l.level === "ai" || l.level === "person" ? `<span class="coin">AI</span>` : free()}
         ${l.status === "removed" ? stamp("removed", "skipped") : `<button class="btn sm" data-alts="${l.id}">Correct</button>`}</span></div>
       <div class="lr3" hidden id="alts-${l.id}">${alts.map((o) => `<button class="btn opt" data-line="${l.id}" data-fix="1" data-pick="${esc(o)}">${esc(l.option_names[o] || o)}</button>`).join("") || `<span class="muted" style="font-size:12px">No close alternative.</span>`}</div>`;
  return `<div class="line ${asks ? "asks" : flag ? "flag" : ""} ${l.status === "removed" ? "removed" : ""}">
    <span class="mk ${asks ? "ask" : flag ? "warn" : ""}">${n}</span><div>${body}</div></div>`;
}

// ---- catalogue ------------------------------------------------------------------------------
route("catalogue", async () => {
  const d = await api("/products");
  const tot = d.items.reduce((a, p) => a + p.aliases, 0), lines = d.items.reduce((a, p) => a + p.lines, 0), fr = d.items.reduce((a, p) => a + p.free_lines, 0);
  main.innerHTML = head("Catalogue", "The ERP knows what the products are. Order Desk knows how people refer to them.",
    `<input id="q" class="inp" placeholder="Search a product…" style="width:260px"><button class="btn" id="sync">Sync from ERP</button>`) + `
    <div class="tiles">${tile("Products", d.items.length, "mirrored from the ERP")}${tile("Wordings known", tot, `${overview.trusted_aliases || 0} trusted`)}
      ${tile("Lines seen", lines, "")}${tile("Resolved without AI", lines ? pct(fr / lines) : "—", "by aliases and rules", "free")}</div>
    <div class="card" style="overflow:hidden"><table class="tbl" id="pt"></table></div>`;
  const draw = (q) => {
    q = (q || "").toLowerCase();
    const rows = d.items.filter((p) => !q || (p.name + " " + p.brand).toLowerCase().includes(q));
    $("#pt").innerHTML = `<tr><th class="cap">Product</th><th class="cap">Brand</th><th class="cap">Packs</th><th class="cap r">Wordings</th><th class="cap r">Lines</th><th class="cap r">Without AI</th></tr>` +
      rows.map((p) => `<tr class="click" onclick="location.hash='#/product/${encodeURIComponent(p.id)}'"><td class="prod">${esc(p.name)}</td><td class="muted">${esc(p.brand)}</td>
        <td class="mono muted" style="font-size:12px">${(p.packs || []).map((k) => esc(k.name)).join(" · ") || "unit"}</td>
        <td class="r num">${p.aliases ? `${p.aliases}${p.trusted ? ` <span class="muted">(${p.trusted} ✓)</span>` : ""}` : "—"}</td><td class="r num">${p.lines || "—"}</td>
        <td class="r">${p.lines ? `<span style="color:var(--free);font-weight:600">${pct(p.free_lines / p.lines)}</span>` : "—"}</td></tr>`).join("");
  };
  draw();
  $("#q").oninput = (e) => draw(e.target.value);
  $("#sync").onclick = async () => { try { const r = await api("/erp/sync", { method: "POST" }); toast(r.products + " products synced"); go(); } catch (e) { toast(e.message, true); } };
});
route("product", async (id) => {
  const d = await api("/products/" + encodeURIComponent(id));
  const p = d.product;
  const SRC = { text: ["text", "Text"], voice: ["voice", "Voice note"], photo: ["photo", "List photo"], products: ["photo", "Product photo"], clerk: ["text", "Clerk"], invoice: ["invoice", "Invoice"] };
  main.innerHTML = `<div class="crumbs"><a href="#/catalogue">Catalogue</a> › ${esc(p.brand || "—")}</div>` +
    head(esc(p.name), `ERP id <span class="mono">${esc(p.id)}</span> · ${(p.packs || []).map((k) => esc(k.name)).join(" · ") || "sold by unit"}${p.size ? " · " + esc(p.size) : ""} · catalogue words <span class="ev">${esc((p.words || []).join(" "))}</span>`) + `
    <div class="tiles">${tile("Wordings known", d.aliases.length, "")}${tile("Lines seen", d.lines, "")}${tile("Resolved without AI", d.lines ? pct(d.free_lines / d.lines) : "—", "", "free")}${tile("Rules touching it", (d.rules || []).length, "")}</div>
    ${readAs(d.read_as)}
    <section class="card" style="overflow:hidden"><div class="section-h"><h2 class="h2">Every way people say it</h2><span class="muted" style="font-size:12px">the carbon is kept verbatim</span></div>
      <table class="tbl"><tr><th class="cap">Wording</th><th class="cap">Learned from</th><th class="cap">Valid for</th><th class="cap r">Confirmed</th><th class="cap">First seen</th><th class="cap">State</th><th></th></tr>
      ${d.aliases.map((a) => { const s = SRC[a.source] || ["text", a.source]; return `<tr><td>${evRaw(a.text.replace(/^supplier: /, ""))}</td><td>${srcTag(s[0], s[1])}</td>
        <td>${stamp("scope", a.scope_name || a.scope_kind)}</td><td class="r num">${a.witnesses}×</td><td class="muted" style="font-size:12px">${short(a.first_seen)}</td><td>${stamp(a.state)}</td>
        <td class="r">${a.state !== "trusted" ? `<button class="btn sm" data-al="${a.id}" data-st="trusted">Trust</button>` : ""} ${a.state !== "blocked" ? `<button class="btn sm danger" data-al="${a.id}" data-st="blocked">Block</button>` : ""}</td></tr>`; }).join("")
        || `<tr><td colspan="7" class="empty">No wording learned yet. Every confirmed order teaches one.</td></tr>`}</table>
      <div class="row" style="padding:14px 20px;border-top:1px solid var(--rule)"><input id="na" class="inp ev" placeholder="add a wording people use, e.g. shampoing hns" style="flex:1"><button class="btn primary" id="add">Add as trusted</button></div></section>
    <section class="card pad col" style="gap:10px"><h2 class="h2">Rules that touch it</h2>
      ${(d.rules || []).map((r) => `<a class="row" style="color:var(--ink);text-decoration:none" href="#/rule/${esc(r.id)}"><span class="mono muted" style="font-size:12px">${esc(r.id)}</span>${stamp(r.status, "", "sm")}<span class="rs" style="font-size:16px">${sentenceHTML(r.sentence)}</span></a>`).join("") || `<span class="muted">None yet.</span>`}</section>`;
  $$("[data-al]").forEach((b) => (b.onclick = async () => { await api(`/aliases/${b.dataset.al}/state`, { method: "POST", body: { state: b.dataset.st } }); go(); }));
  $("#add").onclick = async () => { const text = $("#na").value.trim(); if (!text) return; try { await api("/aliases", { method: "POST", body: { text, product_id: p.id } }); toast("Wording added"); go(); } catch (e) { toast(e.message, true); } };
});

// How the code reads a name: the words that must match, and the attributes that decide identity.
function readAs(r) {
  if (!r) return "";
  const attrs = Object.entries(r.attrs || {});
  const chip = (k, v) => `<span class="chip"><span class="muted">${esc(k)}</span> <b>${esc(v.v)}</b>${v.derived ? ` <span class="muted">(guess)</span>` : ""}</span>`;
  return `<section class="card pad col" style="gap:10px"><div class="row" style="justify-content:space-between"><h2 class="h2">How the code reads this name</h2><span class="who sys"><i>{}</i>same reading as every line</span></div>
    <div class="row" style="gap:6px;flex-wrap:wrap"><span class="cap" style="min-width:90px">Name words</span>${(r.words || []).map((w) => `<span class="chip">${esc(w)}</span>`).join("") || `<span class="muted">none</span>`}</div>
    <div class="row" style="gap:6px;flex-wrap:wrap"><span class="cap" style="min-width:90px">Attributes</span>${attrs.flatMap(([k, vs]) => vs.map((v) => chip(k, v))).join("") || `<span class="muted">none yet: the rule engineer writes the vocabulary</span>`}
      ${Object.entries(r.from || {}).flatMap(([k, ids]) => ids.map((id) => `<a class="mono muted" style="font-size:11px" href="#/rule/${esc(id)}">${esc(k)} ← ${esc(id)}</a>`)).join(" ")}</div>
    <span class="muted" style="font-size:12px">A line must carry every name word. Each attribute both sides state decides by its policy: a different size or flavour is another product; a marker such as Zéro must be on both sides or neither.</span></section>`;
}

// ---- rules ----------------------------------------------------------------------------------
let ruleFilter = "all";
route("rules", async () => {
  const [d, eng, prods, custs, st] = await Promise.all([api("/rules"), api("/engineer"), api("/products"), api("/customers"), api("/settings")]);
  settingsCache = st;
  const by = {}; d.items.forEach((r) => (by[r.status] = (by[r.status] || 0) + 1));
  const last = eng.last || {};
  const items = d.items.filter((r) => ruleFilter === "all" || r.status === ruleFilter);
  const group = (title, sub, list, cls = "") => list.length ? `<section class="card ${cls}" style="overflow:hidden"><div class="section-h"><h2 class="h2">${title}</h2><span class="muted" style="font-size:12px">${sub}</span></div>${list.map(ruleRow).join("")}</section>` : "";
  main.innerHTML = head("Rules", `Written by the rule engineer (Muse Spark, with Muse Wonder as a fallback) and by people. <b style="color:var(--ink)">The model proposes; the code decides.</b>`,
    `<button class="btn primary" id="run">${eng.running ? "Engineer running…" : "Run engineer now"}</button><a class="btn" href="#write" id="wlink">+ Write a rule</a>`) + `
    <div class="tiles">
      ${tile("Benchmark", eng.benchmark, "known lines · ground truth from confirmed orders and receipts")}
      ${tile("Residue", eng.residue, "known lines rules can't resolve alone yet", eng.residue ? "paid" : "free")}
      <div class="card tile"><span class="cap">Last engineer run</span><span class="big" style="font-size:36px">${eng.last_run ? hm(eng.last_run) : "never"}</span>
        <span class="row muted" style="font-size:12px;gap:6px">${eng.last_run ? short(eng.last_run).split(" ")[0] : ""} ${last.cost ? coin(last.cost) : ""}</span></div>
      <div class="card tile"><span class="cap">Status</span><div class="row" style="gap:6px">${stamp("live", `● ${by.live || 0} live`, "sm")}${stamp("shadow", `◐ ${by.shadow || 0} shadow`, "sm")}${stamp("rejected", `✕ ${by.rejected || 0} refused`, "sm")}</div>
        <span class="muted" style="font-size:12px">${by.rolled_back || 0} rolled back · ${by.disabled || 0} disabled</span></div>
    </div>
    <div class="row">${[["all", "All", d.items.length], ["live", "● Live"], ["shadow", "◐ Shadow"], ["proposed", "○ Proposed"], ["rejected", "✕ Refused"], ["rolled_back", "↺ Rolled back"], ["disabled", "Disabled"]]
      .map(([k, n, c]) => `<button class="ftab ${k === ruleFilter ? "on" : ""}" data-f="${k}">${n} <b>${c ?? by[k] ?? 0}</b></button>`).join("")}</div>
    <div class="split">
      <div class="col" style="flex:999 1 600px;min-width:0;gap:18px">
        ${group("Live", "answering before the AI is asked", items.filter((r) => r.status === "live"))}
        ${group("Learning · in shadow", "watching real lines, not acting yet", items.filter((r) => r.status === "shadow" || r.status === "proposed"))}
        ${group("What it refused to learn", "the code said no · kept on the record", items.filter((r) => r.status === "rejected" || r.status === "rolled_back"), "refusedbox")}
        ${group("Disabled", "switched off by a person", items.filter((r) => r.status === "disabled"))}
        ${items.length ? "" : `<div class="card empty">No rule here yet. Rules are born from the lines that needed the AI or a person.</div>`}
      </div>
      <aside style="flex:1 1 340px;max-width:400px" id="write">${writeForm(prods.items, custs.items)}</aside>
    </div>`;
  $$("[data-f]").forEach((b) => (b.onclick = () => { ruleFilter = b.dataset.f; go(); }));
  $("#wlink").onclick = (e) => { e.preventDefault(); $("#write").scrollIntoView({ behavior: "smooth" }); };
  $("#run").onclick = async () => { try { await api("/engineer/run", { method: "POST" }); toast("Rule engineer started"); pollEngineer(); } catch (e) { toast(e.message, true); } };
  bindWrite();
  if (eng.running) pollEngineer();
});
// rules written by a model: the rule engineer's sessions and the Matching Profile's lexicon
const byModel = (author) => author === "rule-engineer" || String(author || "").startsWith("muse-");

function ruleRow(r) {
  const b = r.bench || {};
  const need = +settingsCache.shadow_agreements || 5;
  const who = byModel(r.author) ? `<span class="who model"><i>AI</i>Rule engineer</span>` : `<span class="who person"><i>P</i>${esc(r.author)}</span>`;
  const pill = r.status === "rejected" || r.status === "rolled_back" ? `<span class="pill"><span>{}</span>${esc(r.reason)}</span>`
    : b.lines ? `<span class="pill"><span>{}</span>benchmark +${b.newly_resolved}${b.vocabulary ? ` · reads ${b.products_changed || 0} names` : ""} · ${b.wrong} wrong${r.lines_resolved ? ` · ${r.lines_resolved} lines since` : ""}</span>` : "";
  let track = "";
  if (r.status === "shadow") track = `<span class="track"><span class="d"></span><span class="l"></span><span class="d"></span><span class="l"></span>${Array.from({ length: need }, (_, i) => `<span class="o ${i < r.shadow_agree ? "on" : ""}"></span>`).join("")}<span class="lx"></span><span class="dx"></span></span>`;
  else if (r.status === "live") track = `<span class="track"><span class="d"></span><span class="l"></span><span class="d"></span><span class="l"></span><span class="d"></span><span class="l"></span><span class="d" style="background:var(--free)"></span></span>`;
  const right = r.status === "shadow" ? stamp("shadow", `◐ Shadow ${r.shadow_agree}/${need}`) : stamp(r.status);
  return `<a class="rrow" href="#/rule/${esc(r.id)}"><span class="id">${esc(r.id)}<br><span class="muted">v${r.version} · ${hm(r.created_at)}</span></span>
    <span class="body"><span class="rs">${sentenceHTML(r.sentence)}</span><span class="row" style="gap:8px">${stamp("tag", r.type)}${stamp("scope", r.scope.kind === "global" ? "Global" : r.scope.name || r.scope.kind)}${who}</span>${pill}</span>
    <span class="col" style="align-items:flex-end;gap:10px">${track}${right}</span></a>`;
}
function writeForm(prods, custs) {
  const opt = (v, t) => `<option value="${esc(v)}">${esc(t)}</option>`;
  return `<section class="card pad col" style="gap:14px"><div class="row" style="justify-content:space-between"><h2 class="h2">Write a rule</h2><span class="who person"><i>P</i>by you</span></div>
    <div class="field"><label for="rt">Type</label><select id="rt" class="inp">
      ${opt("synonym", "Synonym — a word means another word or a brand")}${opt("rewrite", "Rewrite — a way of writing becomes the standard one")}
      ${opt("attribute", "Attribute — words that tell products apart (flavour, variant, container…)")}${opt("word", "Word — a brand, or a word that never tells products apart")}${opt("policy", "Policy — how an attribute decides identity")}
      ${opt("pack", "Pack — a mark gives the pack (x6, fardou…)")}${opt("default", "Default — a wording with something missing")}${opt("guard", "Guard — forbid a wrong match")}</select></div>
    <div class="field"><label for="rp">When the shop writes</label><input id="rp" class="inp ev" placeholder="slkt · x{n}$ · fardou"></div>
    <div class="field" id="f-replace"><label for="ra">It means</label><input id="ra" class="inp ev" placeholder="selecto"></div>
    <div class="field" id="f-product" hidden><label for="rprod">It means</label><select id="rprod" class="inp">${prods.map((p) => opt(p.id, p.name)).join("")}</select></div>
    <div class="row" id="f-pack" hidden><div class="field" style="flex:1"><label for="ru">Pack</label><select id="ru" class="inp">${opt("carton", "carton")}${opt("pack", "pack (fardeau)")}${opt("unit", "units")}</select></div>
      <div class="field" style="flex:1"><label for="rq">Of</label><input id="rq" class="inp mono" placeholder="{n} or 6"></div></div>
    <div class="field" id="f-word" hidden><label for="rw">Only products whose name contains</label><input id="rw" class="inp" placeholder="citron"></div>
    <div class="row" id="f-attr" hidden><div class="field" style="flex:1"><label for="rattr">Attribute</label><input id="rattr" class="inp mono" placeholder="flavour"></div>
      <div class="field" style="flex:1" id="f-value"><label for="rval">Value</label><input id="rval" class="inp mono" placeholder="citron · {n} · {word}"></div>
      <div class="field" style="flex:1"><label for="rpol">Policy</label><select id="rpol" class="inp">${opt("", "default (strict)")}${opt("strict", "strict — different value, different product")}${opt("marker", "marker — one side only, different product")}${opt("soft", "soft — only scores")}${opt("ignore", "ignore")}</select></div></div>
    <div class="field" id="f-role" hidden><label for="rrole">It is</label><select id="rrole" class="inp">${opt("descriptor", "a descriptor — said or printed, never tells products apart")}${opt("stop", "chatter — never part of a product name")}${opt("brand", "a brand")}</select></div>
    <div class="field"><label for="rs">For</label><select id="rs" class="inp">${opt("global", "Every shop and supplier (global)")}${custs.map((c) => opt("customer:" + c.id, c.name)).join("")}</select></div>
    <pre class="code" id="rcode"></pre>
    <label class="row" style="gap:8px;font-size:12px"><input type="checkbox" id="rk"> go live now, skip shadow mode</label>
    <button class="btn" id="rtest" type="button">Test on the benchmark</button>
    <div id="rres"></div>
    <button class="btn go" id="rsub" type="button">Submit to the gate</button></section>`;
}
function ruleBody() {
  const t = $("#rt").value, sc = $("#rs").value;
  const action = {};
  if (t === "synonym" || t === "rewrite") action.replace = $("#ra").value.trim();
  if (t === "default") action.product = $("#rprod").value;
  if (t === "pack") { action.pack_unit = $("#ru").value; action.pack_qty = $("#rq").value.trim(); }
  if (t === "guard") action.block_word = $("#rw").value.trim();
  if (t === "attribute" || t === "policy") { action.attr = $("#rattr").value.trim(); action.policy = $("#rpol").value; }
  if (t === "attribute") action.value = $("#rval").value.trim();
  if (t === "word") action.role = $("#rrole").value;
  const scope = sc === "global" ? { kind: "global" } : { kind: "customer", id: sc.split(":")[1] };
  const pattern = t === "policy" ? $("#rattr").value.trim() : $("#rp").value.trim();
  return { type: t, scope, pattern, action, skip_shadow: $("#rk").checked };
}
function bindWrite() {
  const show = () => {
    const t = $("#rt").value;
    $("#f-replace").hidden = !(t === "synonym" || t === "rewrite"); $("#f-product").hidden = t !== "default"; $("#f-pack").hidden = t !== "pack"; $("#f-word").hidden = t !== "guard";
    $("#f-attr").hidden = !(t === "attribute" || t === "policy"); $("#f-value").hidden = t !== "attribute"; $("#f-role").hidden = t !== "word"; $("#rp").closest(".field").hidden = t === "policy";
    const b = ruleBody();
    $("#rcode").innerHTML = `<span class="k">type:</span>  <span class="v">${esc(b.type)}</span>\n<span class="k">scope:</span> <span class="v">${esc(b.scope.kind === "global" ? "global" : "customer · " + ($("#rs").selectedOptions[0] || {}).text)}</span>\n<span class="k">when:</span>  <span class="s">"${esc(b.pattern)}"</span>\n<span class="k">then:</span>  <span class="s">${esc(JSON.stringify(b.action))}</span>`;
  };
  ["rt", "rp", "ra", "rprod", "ru", "rq", "rw", "rs", "rattr", "rval", "rpol", "rrole"].forEach((id) => { const el = $("#" + id); if (el) el.oninput = el.onchange = show; });
  show();
  $("#rtest").onclick = async () => {
    try { const r = await api("/rules/preview", { method: "POST", body: ruleBody() }); $("#rres").innerHTML = benchBox(r.bench, r.passes, r.verdict, r.sentence); }
    catch (e) { $("#rres").innerHTML = `<div class="warnbox">${esc(e.message)}</div>`; }
  };
  $("#rsub").onclick = async () => {
    try { const r = await api("/rules", { method: "POST", body: ruleBody() }); toast(r.rule.id + ": " + (STAMP[r.rule.status] || r.rule.status)); location.hash = "#/rule/" + r.rule.id; }
    catch (e) { toast(e.message, true); }
  };
}
function benchBox(b, passes, verdict, sentence) {
  b = b || {};
  return `<div class="card col" style="gap:12px;padding:16px;background:var(--ground)">${sentence ? `<span class="rs" style="font-size:16px">${sentenceHTML(sentence)}</span>` : ""}
    <div class="row" style="justify-content:space-around;text-align:center">${[["known", b.lines], ["new", "+" + (b.newly_resolved || 0)], ["wrong", b.wrong || 0], ["conflicts", b.conflicts || 0], ["lost", b.lost || 0]].concat(b.vocabulary ? [["names read", b.products_changed || 0], ["twins", b.twins || 0]] : [])
      .map(([k, v]) => `<div class="col"><b class="big" style="font-size:22px;color:${(k === "new" || k === "names read") && v && v !== "+0" ? "var(--free)" : (k === "wrong" || k === "lost" || k === "twins" || k === "conflicts") && v ? "var(--refused)" : "var(--ink)"}">${v ?? 0}</b><span class="muted" style="font-size:11px">${k}</span></div>`).join("")}</div>
    ${(b.examples_products || []).slice(0, 3).map((x) => `<span style="font-size:12px"><b>${esc(x.said)}</b> <span class="mono muted">${esc(x.got)}</span></span>`).join("")}
    ${(b.examples_twins || []).slice(0, 3).map((x) => `<span style="font-size:12px;color:var(--refused)">would read the same: ${esc(x.said)}</span>`).join("")}
    ${(b.examples_new || []).slice(0, 3).map((x) => `<span style="font-size:12px">${ev(x.said)} → <b>${esc(x.expected)}</b></span>`).join("")}
    ${(b.examples_wrong || []).slice(0, 3).map((x) => `<span style="font-size:12px;color:var(--refused)">${ev(x.said)} → ${esc(x.got)} (expected ${esc(x.expected)})</span>`).join("")}
    ${verdict ? `<div class="gate ${passes ? "" : "no"}"><span class="who sys"><i>{}</i></span><span><b>${passes ? "The gate would admit it." : "The gate would refuse this."}</b> ${esc(verdict)}</span></div>` : ""}</div>`;
}
function pollEngineer() {
  clearInterval(timer);
  timer = setInterval(async () => {
    const e = await api("/engineer"); refreshShell();
    if (!e.running) { clearInterval(timer); const l = e.last || {}; toast(`Rule engineer finished · ${(l.accepted || []).length} admitted · ${(l.rejected || []).length} refused`); go(); }
  }, 3000);
}

// ---- rule detail: the birth of a rule ---------------------------------------------------------
route("rule", async (id) => {
  const d = await api("/rules/" + encodeURIComponent(id));
  const r = d.rule, b = r.bench || {}, E = d.events;
  const find = (k) => E.find((e) => e.kind === k);
  const proposed = find("rule.proposed"), bench = find("rule.benchmark"), shadowE = find("rule.shadow"), liveE = find("rule.live"),
    rejected = find("rule.rejected");
  const strikes = E.filter((e) => e.kind === "rule.strike");
  const need = d.shadow_needed, gateMin = d.gate_min_gain;
  const refusedAtGate = !!rejected && !shadowE && !liveE;
  const isLive = r.status === "live", isRolled = r.status === "rolled_back", isRefused = r.status === "rejected";
  const engineer = byModel(r.author);
  const perLine = d.cost_per_ai_line || 0;
  const breakEven = perLine > 0 && r.ai_cost ? Math.ceil(r.ai_cost / perLine) : 0;
  const pw = (r.pattern || "").replace(/[\^$]/g, "").split("|")[0];
  const rep = (r.action.replace || "").toLowerCase();
  const letters = r.type === "synonym" && rep && pw && pw.length < rep.length && [...pw].every((c) => rep.includes(c))
    ? `<div class="letters">${[...rep].map((c) => `<span class="${pw.includes(c) ? "" : "gone"}">${esc(c.toUpperCase())}</span>`).join("")}<br>${[...rep].map((c) => `<span class="kept">${pw.includes(c) ? esc(c) : "&nbsp;"}</span>`).join("")}</div>` : "";
  const st = (n, tone, title, time, body, cls = "", lastLine = true) => `<div class="stn"><div class="node"><b class="${tone}">${n}</b>${lastLine ? `<span class="nl ${tone === "dim" ? "dash" : ""}"></span>` : ""}</div>
    <div class="stbody ${cls}"><div class="t"><span class="cap" style="${tone === "green" ? "color:var(--free)" : tone === "red" ? "color:var(--refused)" : ""}">${title}</span><span class="mono muted" style="font-size:11px">${time || ""}</span></div>${body}</div></div>`;
  const total = Math.max(b.lines || 0, 1), nCells = Math.min(10, total);
  const nNew = b.newly_resolved ? Math.max(1, Math.round(nCells * b.newly_resolved / total)) : 0, nBad = b.wrong ? Math.max(1, Math.round(nCells * b.wrong / total)) : 0;
  const cells = `<div class="cells">${Array.from({ length: nCells }, (_, i) => i < nCells - nNew - nBad ? `<span class="cell same">=</span>` : i < nCells - nBad ? `<span class="cell new">+</span>` : `<span class="cell bad">✕</span>`).join("")}</div>`;
  const s1 = st(1, "", "Needed help", d.inspired.length ? hm(d.inspired[0].at) : "", d.inspired.length
    ? `<span style="font-size:13px">${d.inspired.length} line${d.inspired.length > 1 ? "s" : ""} that alias and rules couldn't read alone.</span>${d.inspired.slice(0, 5).map((x) => `<div class="col">${evRaw(x.said)}<span style="font-size:12px;font-weight:600">→ ${esc(x.product)}</span></div>`).join("")}
      <div class="row" style="border-top:1px solid var(--rule);padding-top:10px;margin-top:auto;gap:6px">${ladder("ai")}<span class="muted" style="font-size:12px">paid picks or people</span></div>`
    : `<span class="muted" style="font-size:13px">${engineer ? "The residue the engineer studied." : "Written by a person from experience."}</span>`);
  const why = (proposed && proposed.details.rationale) || r.rationale;
  const s2 = st(2, "", engineer ? "Pattern found" : "Written", proposed ? hm(proposed.at) : hm(r.created_at),
    `${letters}${why ? `<span style="font-size:13px">“${esc(why)}”</span>` : ""}
     <div class="col" style="gap:6px;border-top:1px solid var(--rule);padding-top:10px;margin-top:auto">${engineer ? `<span class="who model"><i>AI</i>The rule engineer proposes</span>` : `<span class="who person"><i>P</i>${esc(r.author)} writes</span>`}
      <span class="row muted" style="font-size:12px;justify-content:space-between">Cost to write ${r.ai_cost ? coin(r.ai_cost) : free()}</span></div>`);
  const s3 = st(3, refusedAtGate ? "red" : "", "Benchmark", bench ? hm(bench.at) : "",
    `<span style="font-size:13px">Run against every line whose answer is already known.</span>${cells}
     <div class="trio"><div><b style="color:var(--free)">+${b.newly_resolved || 0}</b><span>new</span></div><div><b style="color:${b.wrong ? "var(--refused)" : "var(--ink)"}">${b.wrong || 0}</b><span>wrong</span></div><div><b>${b.conflicts || 0}</b><span>conflicts</span></div></div>
     <div class="chk">${(b.newly_resolved || 0) >= (engineer ? gateMin : 0) ? `<span class="ok">✓</span>` : `<span class="no">✕</span>`}at least ${engineer ? gateMin : 0} new · got ${b.newly_resolved || 0}</div>
     <div class="chk">${!b.wrong && !b.conflicts ? `<span class="ok">✓</span>` : `<span class="no">✕</span>`}0 wrong · 0 conflicts</div>
     <div class="gate ${refusedAtGate ? "no" : ""}" style="margin-top:auto"><span class="who sys"><i>{}</i></span><b>${refusedAtGate ? "The gate refuses it" : "The gate admits it"}</b></div>`, refusedAtGate ? "red" : "");
  const s4 = st(4, refusedAtGate ? "dim" : "", "Shadow", shadowE ? hm(shadowE.at) + (liveE ? " → " + hm(liveE.at) : "") : "",
    refusedAtGate ? `<span class="muted" style="font-size:13px">Never reached: the code said no at the gate.</span>`
      : `<span style="font-size:13px">Watches real lines without acting. Does it agree with what really happened?</span>
         <div class="row" style="gap:8px"><span class="dots">${Array.from({ length: need }, (_, i) => `<i class="${i < r.shadow_agree || liveE ? "on" : ""}"></i>`).join("")}</span><span class="mono" style="font-size:12px">${liveE ? Math.max(need, r.shadow_agree) : r.shadow_agree} / ${need}</span></div>
         ${isRefused && shadowE ? `<div class="warnbox">${esc(r.reason)}</div>` : ""}<span class="muted" style="font-size:12px;margin-top:auto">${isRefused && shadowE ? "1 disagreement → refused" : "0 disagreements"}</span>`, refusedAtGate ? "off" : "");
  const s5 = st(5, isLive ? "green" : isRolled || isRefused ? "red" : "dim", isRolled ? "Rolled back" : isRefused ? "Refused" : "Live", liveE ? "since " + hm(liveE.at) : "",
    isLive ? `<span class="stamp live mid drop" style="align-self:flex-start">● Live</span><span style="font-size:13px">Now answers before the AI is asked. Every answer is still checked by people's corrections.</span>
        <div class="col" style="gap:6px;border-top:1px solid var(--rule);padding-top:10px;margin-top:auto"><div class="row" style="justify-content:space-between;font-size:12px"><span>Corrections</span><span class="mono">${r.strikes} of ${d.rollback_strikes}</span></div>
        <div class="row" style="gap:4px">${Array.from({ length: d.rollback_strikes }, (_, i) => `<span style="flex:1;height:8px;border-radius:2px;border:1.5px dashed ${i < r.strikes ? "var(--refused)" : "#F2B8BB"};background:${i < r.strikes ? "var(--refused)" : "transparent"}"></span>`).join("")}</div>
        <span class="muted" style="font-size:11px">${d.rollback_strikes} corrections → automatic rollback</span></div>`
      : isRolled ? `<span class="stamp rolled_back mid drop" style="align-self:flex-start">↺ Rolled back</span><span style="font-size:13px">${esc(r.reason)}</span>${strikes.map((s) => `<span style="font-size:12px">${ev(s.details.said || "")} corrected</span>`).join("")}`
      : isRefused ? `<span class="stamp rejected mid drop" style="align-self:flex-start">✕ Refused</span><span style="font-size:13px">${esc(r.reason)}</span><span class="muted" style="font-size:12px">Kept on the record so it isn't proposed again.</span>`
      : `<span class="muted" style="font-size:13px">Not live yet${r.status === "shadow" ? `: ${Math.max(0, need - r.shadow_agree)} more agreement(s) needed.` : "."}</span>`,
    isLive ? "on" : isRolled || isRefused ? "red" : "off", false);
  main.innerHTML = `<div class="crumbs"><a href="#/rules">Rules</a> › ${esc(r.id)} · v${r.version}</div>
    <div class="rhead"><div class="col" style="gap:14px;flex:1;min-width:300px">
        <div class="row" style="gap:8px">${stamp("tag", r.type)}${stamp("scope", r.scope.kind === "global" ? "Global · every shop" : r.scope.name || r.scope.kind)}</div>
        <h1 class="rtitle">${sentenceHTML(r.sentence)}</h1>
        <div class="row" style="gap:14px">${engineer ? `<span class="who model"><i>AI</i><b>Proposed</b>&nbsp;by the rule engineer · ${hm(r.created_at)}</span>` : `<span class="who person"><i>P</i><b>Written</b>&nbsp;by ${esc(r.author)} · ${hm(r.created_at)}</span>`}
          ${shadowE || liveE ? `<span class="gate"><span class="who sys"><i>{}</i></span><b>Admitted</b> by the gate · ${hm((shadowE || liveE).at)}</span>` : refusedAtGate ? `<span class="gate no"><span class="who sys"><i>{}</i></span><b>Refused</b> by the gate · ${hm(rejected.at)}</span>` : ""}
          ${liveE ? `<span class="gate"><span class="who sys"><i>{}</i></span><b>Set live</b> ${liveE.actor === "clerk" ? "by a person" : `after ${need} shadow agreements`} · ${hm(liveE.at)}</span>` : ""}
          <span class="who person"><i>P</i><b>${r.strikes ? r.strikes + " correction" + (r.strikes > 1 ? "s" : "") : "No corrections"}</b>&nbsp;by people${r.strikes ? "" : " yet"}</span></div></div>
      <div class="col" style="align-items:flex-end;gap:16px"><span class="stamp ${r.status} huge drop">${esc(STAMP[r.status] || r.status)}</span>
        <div class="row">${isLive ? `<button class="btn danger" data-st="rolled_back">↺ Roll back</button>` : ""}${!isLive && !isRefused ? `<button class="btn go" data-st="live">Set live</button>` : ""}
          ${r.status !== "disabled" ? `<button class="btn" data-st="disabled">Disable</button>` : ""}</div></div></div>
    <div class="row" style="justify-content:space-between"><h2 class="h2">Birth of this rule</h2><button class="btn ghost sm" id="replay">▷ Replay</button></div>
    <div class="birth" id="birth">${s1}${s2}${s3}${s4}${s5}</div>
    <div class="split">
      <section class="card pad col" style="flex:1.3 1 520px;gap:16px"><div class="row" style="justify-content:space-between"><h2 class="h2">Impact</h2><span class="muted" style="font-size:12px">since it ${isLive ? "went live" : "was written"}</span></div>
        <div class="row" style="gap:40px;justify-content:space-between">${[["lines resolved", r.lines_resolved, "var(--ink)"], ["AI calls avoided", r.lines_resolved, "var(--ink)"], ["corrections", r.strikes, r.strikes ? "var(--refused)" : "var(--ink)"], ["AI cost saved (est.)", usd(d.saved), "var(--free)"]]
          .map(([k, v, c]) => `<div class="col"><span class="big" style="font-size:46px;color:${c}">${v}</span><span class="muted" style="font-size:12px">${k}</span></div>`).join("")}</div>
        ${breakEven ? `<div class="col" style="gap:6px"><div class="row" style="justify-content:space-between;font-size:13px"><span><b>Pays for itself</b> after ≈${breakEven} lines · <span class="muted">${usd(r.ai_cost)} to write ÷ ${usd(perLine)} per AI line</span></span><span class="mono">${r.lines_resolved} / ${breakEven}</span></div>
          <div class="rail2"><i style="width:${Math.min(100, r.lines_resolved / breakEven * 100)}%"></i></div><div class="row muted" style="justify-content:space-between;font-size:11px"><span>written</span><span>break-even</span></div></div>` : ""}</section>
      <section class="col" style="flex:1 1 360px;gap:10px"><div class="row" style="justify-content:space-between"><h2 class="h2">Stored form</h2><span class="muted" style="font-size:12px">what the code runs</span></div>
        <pre class="code"><span class="k">type:</span>  <span class="v">${esc(r.type)}</span>\n<span class="k">scope:</span> <span class="v">${esc(r.scope.kind)}${r.scope.name ? " · " + esc(r.scope.name) : ""}</span>\n<span class="k">when:</span>  <span class="s">"${esc(r.pattern)}"</span>\n<span class="k">then:</span>  <span class="s">${esc(JSON.stringify(r.action))}</span>${d.product_name ? `\n<span class="k">product:</span> <span class="v">${esc(d.product_name)}</span>` : ""}</pre>
        <span class="muted" style="font-size:12px">Benchmark at birth: ${b.lines || 0} known · +${b.newly_resolved || 0} · ${b.wrong || 0} wrong · ${b.conflicts || 0} conflicts</span></section>
    </div>
    <section class="card" style="overflow:hidden"><div class="section-h"><h2 class="h2">Recent lines it resolved</h2><span class="muted" style="font-size:12px">each one free · would have been an AI pick</span></div>
      <table class="tbl"><tr><th class="cap">Shop said</th><th class="cap">Shop</th><th class="cap">Became</th><th class="cap">Path</th><th class="cap r">Cost</th><th class="cap r">When</th></tr>
      ${d.resolved.map((x) => `<tr><td>${ev(x.said)}</td><td>${esc(x.who)}</td><td class="prod">${esc(x.product)}</td><td>${ladder(x.level)}</td>
        <td class="r">${x.level === "ai" ? `<span class="coin">AI</span>` : `${free()} <span class="strike">${perLine ? cents(perLine) : ""}</span>`}</td><td class="r mono">${hm(x.at)}</td></tr>`).join("")
        || `<tr><td colspan="6" class="empty">None yet${r.status === "shadow" ? " — shadow rules only watch." : "."}</td></tr>`}</table></section>`;
  $$("[data-st]").forEach((bt) => (bt.onclick = async () => { await api(`/rules/${r.id}/status`, { method: "POST", body: { status: bt.dataset.st } }); toast(r.id + " → " + (STAMP[bt.dataset.st] || bt.dataset.st)); go(); }));
  $("#replay").onclick = () => { const el = $("#birth"); const h = el.innerHTML; el.innerHTML = ""; requestAnimationFrame(() => (el.innerHTML = h)); };
});

// ---- learning & cost ------------------------------------------------------------------------
let learnKind = "";
route("learning", async () => {
  const [d, rl] = await Promise.all([api("/learning?days=30&kind=" + learnKind), api("/rules")]);
  const rules = {}; rl.items.forEach((r) => (rules[r.id] = r));
  const t = d.totals || {}, days = d.days || [];
  const first = days.find((x) => x.lines) || {};
  const freeShare = (x) => (x && x.lines ? (x.alias + x.rules) / x.lines : 0);
  const costK = (x) => (x && x.lines ? x.cost / x.lines * 1000 : 0);
  const lines = t.lines || 0, dl = days.reduce((a, x) => a + x.lines, 0), cost = days.reduce((a, x) => a + x.cost, 0);
  const jobs = d.jobs || [], jobMax = Math.max(...jobs.map((j) => j.cost), 0.0001), total = jobs.reduce((a, j) => a + j.cost, 0);
  const eng = jobs.filter((j) => j.purpose === "engineer").reduce((a, j) => a + j.cost, 0);
  const JOB = { transcribe: "Transcribing voice notes", read: "Reading list photos", extract: "Reading chatty messages", resolve: "Resolving the residue", engineer: "Rule engineer", invoice: "Reading supplier invoices" };
  const marks = []; days.forEach((x) => { (x.rules_live || []).forEach((id) => marks.push({ id, day: x.day, back: false })); (x.rules_back || []).forEach((id) => marks.push({ id, day: x.day, back: true })); });
  const many = days.filter((x) => x.lines).length > 1;
  main.innerHTML = `<span class="cap">Learning &amp; cost · last 30 days</span>
    <h1 class="disp" style="font-size:64px;line-height:.98;font-weight:800;letter-spacing:-0.035em;max-width:900px">Every rule makes the next order <span style="color:var(--free)">cheaper.</span></h1>
    <p class="muted">Every AI call is counted${learnKind ? "" : ", the rule engineer's included"}. This is the proof, not a dashboard.</p>
    <div><span class="segs">${[["", "All"], ["order", "Shop messages"], ["invoice", "Supplier invoices"]].map(([k, n]) => `<button class="seg ${k === learnKind ? "on" : ""}" data-lk="${k}">${n}</button>`).join("")}</span></div>
    <div class="tiles">
      ${tile("Lines handled", lines.toLocaleString("fr-FR"), `${t.docs || 0} ${learnKind === "invoice" ? "receipts" : learnKind === "order" ? "orders" : "orders + receipts"} confirmed`)}
      ${tile("Resolved without AI", lines ? pct(t.free / lines) : "—", many ? `was ${pct(freeShare(first))} on ${dayLabel(first.day)}` : "by aliases and rules", "free")}
      ${tile("Wrong automatic", t.auto ? pct(t.wrong_auto / t.auto) : "—", `${t.wrong_auto || 0} of ${t.auto || 0} free lines, corrected by people`)}
      ${tile("AI cost / 1,000 lines", dl ? usd(cost / dl * 1000) : "—", many ? `was ${usd(costK(first))} on ${dayLabel(first.day)}` : "reading + resolving", "paid")}
      ${tile("Model calls / line", lines ? (t.resolve_calls / lines).toFixed(2) : "—", "to identify a product")}
    </div>
    <section class="card pad col" style="gap:14px"><div class="row" style="justify-content:space-between;align-items:flex-start"><div class="col" style="gap:4px"><h2 class="h2" style="font-size:26px">The learning curve</h2>
        <span class="muted" style="font-size:13px">Each stamp is a rule going live. Watch the free share step up and the cost step down at the same moment.</span></div>
        <div class="row" style="gap:14px"><span class="lg"><i style="background:#0E6B48"></i>Alias</span><span class="lg"><i style="background:#A0D6BC"></i>Rules</span><span class="lg"><i style="background:#E3B341"></i>AI pick</span><span class="lg"><i style="background:#673A9C"></i>Person</span><span class="lg"><i style="background:#946200;height:3px"></i>AI cost / 1,000 lines</span></div></div>
      ${curve(days, marks)}
      ${marks.length ? `<div class="row" style="gap:28px;align-items:flex-start;border-top:1px solid var(--rule);padding-top:14px">${marks.slice(-6).map((m) => { const r = rules[m.id] || {}; return `<a href="#/rule/${esc(m.id)}" class="col" style="gap:4px;max-width:220px;color:var(--ink);text-decoration:none">
        <span class="mono" style="font-size:12px;color:${m.back ? "var(--refused)" : "var(--free)"}">${esc(m.id)} · ${dayLabel(m.day)}${m.back ? " · ↺" : ""}</span><span style="font-size:13px">${sentenceHTML(r.sentence || "")}</span>
        <span class="muted" style="font-size:12px">${m.back ? esc(r.reason || "rolled back") : `+${r.lines_resolved || 0} lines free`}</span></a>`; }).join("")}</div>` : ""}
    </section>
    <div class="split">
      <section class="card" style="flex:1.5 1 560px;overflow:hidden"><div class="section-h"><h2 class="h2">AI spend by job</h2><span class="muted" style="font-size:12px">${learnKind ? "" : "all lines · "}30 days</span></div>
        <table class="tbl"><tr><th class="cap">Job</th><th class="cap">Model</th><th class="cap r">Calls</th><th class="cap" style="width:30%">Share</th><th class="cap r">Cost</th></tr>
        ${jobs.map((j) => `<tr><td><b>${esc(JOB[j.purpose] || j.purpose)}</b></td><td class="mono muted" style="font-size:12px;white-space:nowrap">${esc(j.model)}</td><td class="r num">${j.calls}</td>
          <td><div class="bar"><i style="width:${j.cost / jobMax * 100}%;background:var(--paid)"></i></div></td><td class="r">${coin(j.cost)}</td></tr>`).join("") || `<tr><td colspan="5" class="empty">No AI call yet.</td></tr>`}
        <tr><td colspan="4"><b>Total</b></td><td class="r mono" style="color:var(--paid);font-weight:600">${usd(total)}</td></tr></table></section>
      <section class="card pad col" style="flex:1 1 360px;gap:12px"><h2 class="h2">Saved by aliases &amp; rules</h2><span class="big" style="font-size:64px;color:var(--free)">${usd(d.saved)}</span>
        <span class="muted" style="font-size:13px">${d.free_lines} lines resolved for free × ${usd(d.cost_per_ai_line)}, the average AI cost of a line that needed the AI.</span>
        ${eng > 0 ? `<div class="col" style="gap:8px;border-top:1px solid var(--rule);padding-top:12px">
          <div class="row" style="justify-content:space-between;font-size:13px"><span>Saved</span><span class="mono" style="color:var(--free)">${usd(d.saved)}</span></div><div class="bar"><i style="width:${Math.min(100, d.saved / Math.max(d.saved, eng) * 100)}%;background:var(--free)"></i></div>
          <div class="row" style="justify-content:space-between;font-size:13px"><span>Rule engineer cost</span><span class="mono" style="color:var(--paid)">${usd(eng)}</span></div><div class="bar"><i style="width:${Math.min(100, eng / Math.max(d.saved, eng) * 100)}%;background:var(--paid)"></i></div>
          <span class="muted" style="font-size:12px">${d.saved >= eng ? `The rule engineer has earned back ${(d.saved / eng).toFixed(1)}× what it cost to run.` : `The rule engineer has earned back ${Math.round(d.saved / eng * 100)}% of what it cost so far.`}</span></div>` : ""}</section>
    </div>`;
  $$("[data-lk]").forEach((b) => (b.onclick = () => { learnKind = b.dataset.lk; go(); }));
});
function curve(days, marks) {
  const pts = days.filter((x) => x.lines);
  if (!pts.length) return `<div class="empty">No line yet. The curve starts with the first order.</div>`;
  const W = 1100, L = 56, R = 1000, T1 = 70, H1 = 230, T2 = 360, H2 = 150, Bt = T2 + H2;
  const n = pts.length, x = (i) => (n === 1 ? (L + R) / 2 : L + i * (R - L) / (n - 1));
  const share = pts.map((p) => { const t = p.lines || 1; return { a: p.alias / t, r: p.rules / t, ai: p.ai / t, p: p.person / t }; });
  const cum = (s, k) => ({ a: s.a, r: s.a + s.r, ai: s.a + s.r + s.ai, p: 1 })[k];
  const y1 = (v) => T1 + H1 * (1 - v);
  const band = (lo, hi, color) => {
    if (n === 1) { const w = 60; return `<rect x="${x(0) - w}" y="${y1(hi(share[0]))}" width="${2 * w}" height="${Math.max(0, H1 * (hi(share[0]) - lo(share[0])))}" fill="${color}"/>`; }
    const top = share.map((s, i) => `${x(i)},${y1(hi(s))}`), bot = share.map((s, i) => `${x(i)},${y1(lo(s))}`).reverse();
    return `<polygon points="${top.concat(bot).join(" ")}" fill="${color}"/>`;
  };
  let g = band(() => 0, (s) => cum(s, "a"), "#0E6B48") + band((s) => cum(s, "a"), (s) => cum(s, "r"), "#A0D6BC") +
    band((s) => cum(s, "r"), (s) => cum(s, "ai"), "#E3B341") + band((s) => cum(s, "ai"), () => 1, "#673A9C");
  if (n > 1) g += `<polyline points="${share.map((s, i) => `${x(i)},${y1(cum(s, "r"))}`).join(" ")}" fill="none" stroke="#111827" stroke-width="2" class="draw"/>`;
  [0, 0.5, 1].forEach((v) => (g += `<text x="${L - 10}" y="${y1(v) + 4}" font-size="11" fill="#5B6475" text-anchor="end" font-family="Consolas,monospace">${v * 100}%</text>`));
  const fs0 = cum(share[0], "r"), fsN = cum(share[n - 1], "r");
  if (n > 1) g += `<text x="${x(0) + 8}" y="${y1(fs0) - 10}" font-size="22" font-weight="800" fill="#111827" font-family="Bricolage Grotesque,sans-serif">${Math.round(fs0 * 100)}%</text>`;
  g += `<text x="${R + 14}" y="${y1(fsN) + 8}" font-size="30" font-weight="800" fill="#0E6B48" font-family="Bricolage Grotesque,sans-serif">${Math.round(fsN * 100)}%</text><text x="${R + 14}" y="${y1(fsN) + 26}" font-size="12" font-weight="600" fill="#0E6B48">free</text>`;
  const ck = pts.map((p) => p.cost / p.lines * 1000), mx = Math.max(...ck, 0.01) * 1.2;
  const y2 = (v) => T2 + H2 * (1 - v / mx);
  g += `<text x="${L}" y="${T2 - 14}" font-size="11" font-weight="600" letter-spacing="1" fill="#5B6475">AI COST PER 1,000 LINES</text>`;
  [0, 0.5, 1].forEach((f) => (g += `<line x1="${L}" x2="${R}" y1="${y2(mx * f)}" y2="${y2(mx * f)}" stroke="#E2E7EF"/><text x="${L - 10}" y="${y2(mx * f) + 4}" font-size="11" fill="#5B6475" text-anchor="end" font-family="Consolas,monospace">$${(mx * f).toFixed(2)}</text>`));
  const line = ck.map((v, i) => `${x(i)},${y2(v)}`).join(" ");
  g += n > 1 ? `<polygon points="${line} ${x(n - 1)},${y2(0)} ${x(0)},${y2(0)}" fill="#FFF3D1"/><polyline points="${line}" fill="none" stroke="#946200" stroke-width="2.4" class="draw"/>` : `<circle cx="${x(0)}" cy="${y2(ck[0])}" r="5" fill="#946200"/>`;
  if (n > 1) g += `<text x="${x(0) + 8}" y="${y2(ck[0]) - 10}" font-size="20" font-weight="800" fill="#946200" font-family="Bricolage Grotesque,sans-serif">$${ck[0].toFixed(2)}</text>`;
  g += `<text x="${R + 14}" y="${y2(ck[n - 1]) + 8}" font-size="28" font-weight="800" fill="#946200" font-family="Bricolage Grotesque,sans-serif">$${ck[n - 1].toFixed(2)}</text>`;
  const idx = {}; pts.forEach((p, i) => (idx[p.day] = i));
  const used = {};
  marks.forEach((m) => {
    const i = idx[m.day]; if (i === undefined) return;
    const c = m.back ? "#C0262D" : "#0E6B48", k = used[i] = (used[i] || 0) + 1, xx = x(i) + (k - 1) * 82, ly = 18;
    g += `<line x1="${xx}" x2="${xx}" y1="${ly + 26}" y2="${Bt}" stroke="${c}" stroke-width="${m.back ? 2 : 1.3}" ${m.back ? "" : `stroke-dasharray="3 3"`}/>`;
    g += `<rect x="${xx - 38}" y="${ly}" width="76" height="24" rx="3" fill="#fff" stroke="${c}" stroke-width="2" ${m.back ? `stroke-dasharray="4 3"` : ""}/><text x="${xx}" y="${ly + 16}" font-size="11" font-weight="600" fill="${c}" text-anchor="middle" font-family="Consolas,monospace">${m.back ? "↺ " : ""}${esc(m.id)}</text>`;
    g += `<circle cx="${x(i)}" cy="${y2(ck[i])}" r="5" fill="#fff" stroke="${m.back ? "#C0262D" : "#946200"}" stroke-width="2"/>`;
  });
  const step = Math.max(1, Math.ceil(n / 6));
  pts.forEach((p, i) => { if (i % step === 0 || i === n - 1) g += `<text x="${x(i)}" y="${Bt + 22}" font-size="11" fill="#5B6475" text-anchor="middle" font-family="Consolas,monospace">${dayLabel(p.day)}</text>`; });
  return `<svg viewBox="0 0 ${W} ${Bt + 32}" width="100%" role="img" aria-label="Share of lines resolved without AI and AI cost per 1,000 lines, by day">${g}</svg>`;
}

// ---- customers ------------------------------------------------------------------------------
route("customers", async () => {
  const d = await api("/customers");
  main.innerHTML = head("Customers", `Shops come from the ERP. A shop sends its one-time code to ${d.bot ? `<a href="https://t.me/${esc(d.bot)}" target="_blank">@${esc(d.bot)}</a>` : "the bot"} once, and its chat is linked.`) + `
    <div id="code"></div>
    <section class="card" style="overflow:hidden"><table class="tbl"><tr><th class="cap">Shop</th><th class="cap">Phone</th><th class="cap">Linked chats</th><th class="cap r">Orders</th><th></th></tr>
    ${d.items.map((c) => `<tr><td class="prod">${esc(c.name)}</td><td class="mono muted" style="font-size:12px">${esc(c.phone)}</td>
      <td>${c.chats.length ? c.chats.map((x) => { const [k, n] = x.split("|"); return `<span class="row" style="gap:8px;font-size:13px">${srcTag("text", n || k)}<span class="mono muted" style="font-size:11px">${esc(k)}</span><button class="btn ghost sm" data-unlink="${esc(k)}">Unlink</button></span>`; }).join("") : `<span class="muted">—</span>`}</td>
      <td class="r num">${c.orders || "—"}</td><td class="r"><button class="btn sm" data-c="${esc(c.id)}" data-n="${esc(c.name)}">${c.pair_code ? "Code " + esc(c.pair_code) : "Pairing code"}</button></td></tr>`).join("")
      || `<tr><td colspan="5" class="empty">No customer yet: connect the ERP in Settings.</td></tr>`}</table></section>`;
  $$("[data-unlink]").forEach((b) => (b.onclick = async () => { await api("/chats/unlink", { method: "POST", body: { chat: b.dataset.unlink } }); toast("Chat unlinked"); go(); }));
  $$("[data-c]").forEach((b) => (b.onclick = async () => {
    const r = await api(`/customers/${encodeURIComponent(b.dataset.c)}/pair-code`, { method: "POST" });
    $("#code").innerHTML = `<section class="card pad row" style="gap:20px"><span class="cap">Pairing code for ${esc(b.dataset.n)}</span>
      <span class="stamp scope" style="font-size:26px;letter-spacing:.25em;padding:8px 16px">${esc(r.code)}</span>
      <span style="font-size:13px">Send it to <b>@${esc(r.bot || "the bot")}</b> on Telegram, or open <a target="_blank" href="https://t.me/${esc(r.bot)}?start=${esc(r.code)}">t.me/${esc(r.bot)}?start=${esc(r.code)}</a>. One use, valid 7 days.</span></section>`;
    window.scrollTo({ top: 0, behavior: "smooth" });
  }));
});

// ---- settings -------------------------------------------------------------------------------
route("settings", async () => {
  const [ov, st] = await Promise.all([api("/overview"), api("/settings")]);
  settingsCache = st;
  const e = ov.erp || {};
  const field = (k, label, help) => `<div class="row" style="padding:10px 0;border-top:1px solid var(--rule)"><div style="flex:1"><div style="font-weight:600;font-size:13px">${label}</div><div class="muted" style="font-size:12px">${help}</div></div>
    <input data-k="${k}" class="inp mono" value="${esc(st[k])}" style="width:160px"></div>`;
  const ch = (on, name, det) => `<div class="row" style="padding:10px 0;border-top:1px solid var(--rule)"><span class="dot ${on ? "" : "off"}"></span><b>${name}</b><span class="muted" style="font-size:13px">${det}</span></div>`;
  main.innerHTML = head("Settings", "Every number that governs the agent, in plain words.", `<button class="btn primary" id="save">Save</button>`) + `
    <div class="split" style="align-items:stretch">
      <section class="card pad col" style="flex:1 1 440px;gap:12px"><h2 class="h2">ERP connection</h2>
        <div class="${e.connected ? "okbox" : "warnbox"}">${e.connected ? `Connected to <b>${esc(e.name)}</b> at <span class="mono">${esc(e.url)}</span> as ${esc(e.user)} · ${ov.products} products · synced ${short(e.synced_at)}`
          : `Not connected${e.error ? ": " + esc(e.error) : ""}. Order Desk talks to the ERP only through its API, with a user session.`}</div>
        <div class="row"><div class="field" style="flex:0 0 190px"><label for="ek">ERP</label><select id="ek" class="inp">
          ${[["elixir", "Elixir Gestion"], ["odoo", "Odoo"], ["erpnext", "ERPNext"], ["dolibarr", "Dolibarr"], ["inventree", "InvenTree"], ["tryton", "Tryton"]].map(([v, t]) => `<option value="${v}" ${(e.kind || "elixir") === v ? "selected" : ""}>${t}</option>`).join("")}</select></div>
          <div class="field" style="flex:1"><label for="eu">Address</label><input id="eu" class="inp mono" value="${esc(e.url || "http://127.0.0.1:8543")}"></div></div>
        <div class="field" id="f-edb"><label for="edb">Database (Odoo, Tryton)</label><input id="edb" class="inp mono" value="${esc(e.db || "")}" placeholder="orderdesk"></div>
        <div class="row"><div class="field" style="flex:1"><label for="en">Username</label><input id="en" class="inp" autocomplete="off" value="${esc(e.login || "")}"></div><div class="field" style="flex:1"><label for="ep" id="epl">Password</label><input id="ep" class="inp" type="password" autocomplete="new-password"></div></div>
        <span class="muted" style="font-size:12px" id="ehelp"></span>
        <div class="row" style="justify-content:flex-end">${e.connected ? `<button class="btn" id="es">Sync now</button>` : ""}<button class="btn primary" id="ec">Connect</button></div></section>
      <section class="card pad col" style="flex:1 1 380px;gap:4px"><h2 class="h2" style="margin-bottom:8px">Channels &amp; models</h2>
        ${ch(!!ov.telegram, "Telegram", ov.telegram ? `<a target="_blank" href="https://t.me/${esc(ov.telegram)}">@${esc(ov.telegram)}</a>` : "set TELEGRAM_BOT_TOKEN")}
        ${ch(!!ov.whatsapp, "WhatsApp", ov.whatsapp ? `Cloud API · ${esc(ov.whatsapp)} · webhook <span class="mono">/whatsapp/webhook</span>` : "set WHATSAPP_TOKEN and WHATSAPP_PHONE_ID")}
        ${ch(ov.meta, "Meta models", "Voice Transcribe · Muse Spark 1.3 · Muse Wonder")}</section>
    </div>
    <div class="split" style="align-items:stretch">
      <section class="card pad col" style="flex:1 1 440px"><h2 class="h2" style="margin-bottom:8px">Matching policy</h2>
        ${field("alias_trust_after", "A wording becomes trusted after", "independent confirmations; before that it is only a hint")}
        ${field("shadow_agreements", "Shadow: agreements before a rule goes live", "0 = straight to live after the gate")}
        ${field("rollback_strikes", "Roll a live rule back after", "corrections by people")}
        ${field("gate_min_gain", "Gate: new lines a rule must resolve", "and always 0 wrong, 0 conflicts")}
        ${field("receipt_post", "Post goods receipts at once (1 = yes)", "0 leaves them as drafts in the ERP")}</section>
      <section class="card pad col" style="flex:1 1 380px"><h2 class="h2" style="margin-bottom:8px">Rule engineer &amp; budget</h2>
        ${field("engineer_auto", "Run automatically (1 = yes)", "after confirmations, when enough residue has accumulated")}
        ${field("engineer_every", "Run after every", "new residue lines (lines that needed the AI or a person)")}
        ${field("engineer_model", "Model", "muse-spark-1.3 by default: the same rules as Wonder on the replays, at a quarter of the price")}
        ${field("engineer_fallback_model", "Fallback model", "muse-wonder-1.0 looks when Spark admitted nothing (empty: never)")}
        ${field("engineer_fallback_after", "Fallback after", "new residue lines since the fallback last looked")}
        ${field("engineer_rounds", "Max tool rounds per run", "caps the cost of a run")}
        ${field("ai_budget_day", "AI budget per day (USD)", "when reached, aliases and rules keep working; new lines wait for a person")}</section>
    </div>`;
  $("#save").onclick = async () => { const body = {}; $$("[data-k]").forEach((i) => (body[i.dataset.k] = i.value)); try { await api("/settings", { method: "PUT", body }); toast("Settings saved"); } catch (er) { toast(er.message, true); } };
  $("#ec").onclick = async () => {
    const b = $("#ec"); b.disabled = true; b.textContent = "Connecting…";
    try { await api("/erp/connect", { method: "POST", body: { kind: $("#ek").value, url: $("#eu").value.trim(), database: $("#edb").value.trim(), username: $("#en").value.trim(), password: $("#ep").value } }); toast("ERP connected and synced"); go(); }
    catch (er) { toast(er.message, true); b.disabled = false; b.textContent = "Connect"; }
  };
  const erpKind = () => {
    const k = $("#ek").value;
    $("#f-edb").hidden = k !== "odoo" && k !== "tryton";
    $("#epl").textContent = k === "odoo" ? "API key or password (kept)" : "Password (used once, not kept)";
    $("#ehelp").textContent = { elixir: "The password is used once to open a session; only the session token is kept.",
      odoo: "Odoo's API needs the key on every call, so it is kept: create an API key in Odoo (your profile → Account Security → New API key), it can be revoked there.",
      erpnext: "The password is used once to generate an API key and secret for this user; only those are kept.",
      dolibarr: "The password is used once to fetch this user's Dolibarr API key; only the key is kept. If Dolibarr's login API is off, paste the key from the user card instead.",
      inventree: "The password is used once to create a revocable API token named orderdesk; only the token is kept.",
      tryton: "The password is used once to open a Tryton session; only the session key is kept (it expires after 30 days, or when revoked in Tryton)." }[k];
    const port = { elixir: "8543", odoo: "8069", erpnext: "8090", dolibarr: "8081", inventree: "8083", tryton: "8000" }[k];
    if (!e.url || e.kind !== k) $("#eu").value = "http://127.0.0.1:" + port;
  };
  $("#ek").onchange = erpKind; erpKind();
  const es = $("#es"); if (es) es.onclick = async () => { try { const r = await api("/erp/sync", { method: "POST" }); toast(r.products + " products synced"); go(); } catch (er) { toast(er.message, true); } };
});

api("/settings").then((s) => (settingsCache = s)).catch(() => {});
go();
