// Order Desk admin panel: a small single-page app over the JSON API (no build step).
"use strict";

const $ = (s, el = document) => el.querySelector(s);
const main = $("#main");
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const money = (c) => (Math.round((c || 0) / 100)).toString().replace(/\B(?=(\d{3})+(?!\d))/g, " ") + " DA";
const usd = (v) => "$" + (v || 0).toFixed(v >= 10 ? 2 : v >= 1 ? 2 : 4);
const pct = (v) => (isFinite(v) ? (v * 100).toFixed(1) : "0.0") + " %";
const day = (s) => (s || "").slice(0, 16).replace("T", " ");
const short = (s) => {
  if (!s) return "";
  const d = new Date(s.replace(" ", "T"));
  if (isNaN(d)) return s;
  const today = new Date();
  const hm = d.toTimeString().slice(0, 5);
  if (d.toDateString() === today.toDateString()) return "today " + hm;
  const y = new Date(today); y.setDate(today.getDate() - 1);
  if (d.toDateString() === y.toDateString()) return "yesterday " + hm;
  return d.toLocaleDateString("en-GB", { day: "numeric", month: "short" }) + " " + hm;
};

async function api(path, opts = {}) {
  const res = await fetch("/api" + path, { headers: { "Content-Type": "application/json" }, ...opts,
    body: opts.body && typeof opts.body !== "string" ? JSON.stringify(opts.body) : opts.body });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "HTTP " + res.status);
  return data;
}
function toast(msg, err) {
  const t = $("#toast");
  t.textContent = msg; t.className = "toast" + (err ? " err" : ""); t.hidden = false;
  clearTimeout(toast._t); toast._t = setTimeout(() => (t.hidden = true), 3800);
}
const chip = (status, label) => `<span class="chip c-${esc(status)}">${esc(label || status.replace("_", " "))}</span>`;
const lvl = (l) => `<span class="lv lv-${esc(l)}">${esc({ alias: "alias", rules: "rules", history: "history", ai: "AI", person: "person", none: "—" }[l] || l)}</span>`;
const scopeChip = (s) => `<span class="chip c-scope">${esc(s.kind === "customer" || s.kind === "supplier" ? (s.name || s.kind) : s.kind === "global" ? "Global" : s.kind)}</span>`;

// ---- shell -------------------------------------------------------------------------------
let overview = {};
async function refreshShell() {
  try { overview = await api("/overview"); } catch { return; }
  const e = overview.erp || {};
  $("#tenant").textContent = e.connected ? (e.name || "ERP") + " · " + (e.user || "") : "ERP not connected";
  const ib = $("#cnt-inbox");
  const n = overview.pending_lines + overview.open_drafts;
  ib.hidden = !n; ib.textContent = overview.open_drafts;
  api("/drafts?kind=invoice&status=open").then((x) => { const c = $("#cnt-inv"); c.hidden = !x.items.length; c.textContent = x.items.length; }).catch(() => {});
  const rl = $("#cnt-rules"), live = (overview.rules || {}).live || 0;
  rl.hidden = !live; rl.textContent = live;
  const dot = (ok) => `<span class="dot" style="background:${ok ? "#22c55e" : "#f87171"}"></span>`;
  $("#foot").innerHTML =
    `${dot(e.connected)}ERP ${e.connected ? "connected" : "offline"}<br>` +
    `${dot(!!overview.telegram)}Telegram ${overview.telegram ? "@" + esc(overview.telegram) : "off"}<br>` +
    `${dot(overview.meta)}Meta models ${overview.meta ? "on" : "off"}<br>` +
    `AI today ${usd(overview.spend_today)} of ${usd(overview.budget_day)}` +
    (overview.engineer_running ? `<br><span class="pulse">⚙ Rule engineer running…</span>` : "");
}

const routes = {};
function route(name, fn) { routes[name] = fn; }
let timer = null;
async function go() {
  clearInterval(timer);
  const [name, arg] = (location.hash.replace(/^#\/?/, "") || "inbox").split("/");
  document.querySelectorAll("#nav a").forEach((a) => a.classList.toggle("on", a.dataset.r === ({ draft: "inbox", product: "catalogue", rule: "rules", invoice: "invoices" }[name] || name)));
  const fn = routes[name] || routes.inbox;
  try { await fn(arg ? decodeURIComponent(arg) : ""); } catch (e) { main.innerHTML = `<div class="card warn">${esc(e.message)}</div>`; }
  refreshShell();
}
window.addEventListener("hashchange", go);

// ---- inbox & orders -----------------------------------------------------------------------
route("inbox", async (sel) => {
  await draftsScreen("open", sel, "Inbox", "Orders being built in the chats. People only see what needs them.");
  timer = setInterval(() => { if (!document.activeElement || document.activeElement.tagName !== "TEXTAREA") draftsScreen("open", currentDraft, "Inbox", "", true); }, 6000);
});
route("orders", (sel) => draftsScreen("", sel, "Orders", "Every order Order Desk handled, with its evidence."));
route("draft", (id) => draftsScreen("", id, "Order", ""));
let currentDraft = "";

async function draftsScreen(status, sel, title, sub, quiet) {
  const [list, custs] = await Promise.all([api("/drafts?kind=order" + (status ? "&status=" + status : "")), quiet ? null : api("/customers")]);
  const items = list.items;
  if (!sel && items.length) sel = items[0].id;
  currentDraft = sel;
  if (quiet && $("#drafts-list")) {
    $("#drafts-list").innerHTML = draftItems(items, sel);
    if (sel) renderDraft(sel, true);
    return;
  }
  main.innerHTML = `
    <div class="ttl"><div><h1>${esc(title)}</h1><div class="s">${esc(sub)}</div></div></div>
    <div class="grid g12">
      <div>
        ${status === "open" ? tryBox(custs.items) : ""}
        <div class="list" id="drafts-list">${draftItems(items, sel)}</div>
      </div>
      <div id="draft-view">${items.length ? "" : `<div class="card empty">No order here yet. Send a message to the bot on Telegram, or try one above.</div>`}</div>
    </div>`;
  bindTry();
  if (sel) renderDraft(sel);
}

function draftItems(items, sel) {
  if (!items.length) return "";
  return items.map((d) => `
    <a class="it ${d.id === sel ? "on" : ""}" href="#/${location.hash.includes("orders") ? "orders" : location.hash.includes("draft") ? "draft" : "inbox"}/${d.id}">
      <div class="top"><span>${esc(d.customer || "—")}</span>${d.status === "confirmed" ? chip("confirmed", d.erp_order_number) : d.pending ? chip("ask", d.pending + " to check") : chip(d.status)}</div>
      <div class="small muted">${d.lines} line(s) · ${esc((d.kinds || "text").replace(/,/g, " + "))} · ${esc(d.chat_key.split(":")[0])} · ${short(d.updated_at)}</div>
    </a>`).join("");
}

function tryBox(custs) {
  return `<div class="card" style="margin-bottom:12px">
    <h5>Try a message</h5>
    <select id="try-cust" style="width:100%;margin-bottom:6px">${custs.map((c) => `<option value="${esc(c.id)}">${esc(c.name)}</option>`).join("")}</select>
    <textarea id="try-text" rows="3" placeholder="3 cartons dove shampooing 250 ml&#10;2 fardou nivea creme"></textarea>
    <div class="row" style="margin-top:6px"><span class="tiny muted">Runs the real pipeline. Lists use the free parser; chatty text uses the AI.</span><span class="sp"></span>
    <button class="btn p sm" id="try-go">Run</button></div></div>`;
}
function bindTry() {
  const b = $("#try-go");
  if (!b) return;
  b.onclick = async () => {
    const text = $("#try-text").value.trim();
    if (!text) return;
    b.disabled = true; b.textContent = "Running…";
    try {
      const r = await api("/try", { method: "POST", body: { customer_id: $("#try-cust").value, text } });
      if (!r.draft) toast(r.extraction.reply || "No product found in this message.");
      else { $("#try-text").value = ""; location.hash = "#/inbox/" + r.draft; go(); }
    } catch (e) { toast(e.message, true); }
    b.disabled = false; b.textContent = "Run";
  };
}

const levelsOrder = ["alias", "rules", "ai", "person"];
function traceOf(l) {
  if (l.level === "history") return `<div class="trace"><span class="hit">copied from the last order</span></div>`;
  const hitAt = { alias: 0, rules: 1, ai: 2, person: 3, none: 3 }[l.level] ?? 3;
  const names = ["Alias", "Rules", "AI pick", "Person"];
  return `<div class="trace">${names.map((n, i) => `<span class="${i < hitAt ? "" : i === hitAt ? (i === 2 ? "ai" : i === 3 ? "ask" : "hit") : ""}">${i < hitAt ? "✕ " : i === hitAt ? "✓ " : ""}${n}</span>`).join("")}</div>`;
}

async function renderDraft(id, quiet) {
  const v = $("#draft-view");
  if (!v) return;
  const d = await api("/drafts/" + id);
  const dr = d.draft;
  const msgs = d.messages.map((m) => `
    <div class="msg"><div class="k">${{ voice: "🎤 Voice note", photo: "📷 Photo of a list", text: "💬 Message" }[m.kind] || m.kind} · ${short(m.created_at)} ·
      read by ${m.extraction === "parser" ? "the free parser" : "Muse Spark"} · AI ${usd(m.ai_cost)}</div>
      ${m.kind === "voice" && m.media ? `<audio controls src="${esc(m.media)}"></audio>` : ""}
      ${m.kind === "photo" && m.media ? `<img src="${esc(m.media)}" alt="list">` : ""}
      <div class="tx">${m.kind === "voice" ? "« " + esc(m.text) + " »" : esc(m.text)}</div></div>`).join("");
  const lines = d.lines.map((l) => lineCard(l, dr.status)).join("");
  const open = dr.status === "open";
  v.innerHTML = `<div class="card">
      <div class="row"><div><div class="crumb">${esc(dr.chat_key)} · started ${short(dr.created_at)}</div>
        <h2 style="font-size:19px;margin:0">${esc(dr.customer)}</h2></div><span class="sp"></span>
        ${dr.status === "confirmed" ? chip("confirmed", "Order " + dr.erp_order_number + " in the ERP") : chip(dr.status)}</div>
      ${dr.delivery ? `<div class="small muted" style="margin-top:4px">🚚 ${esc(dr.delivery)}</div>` : ""}
    </div>
    <div class="card"><h5>What the shop sent</h5>${msgs || `<div class="muted small">—</div>`}</div>
    <div class="card"><h5>Lines · how each one was resolved</h5>${lines}
      <div class="total"><span>Total TTC</span><span>${money(d.total)}</span></div>
      ${open ? `<div class="row" style="margin-top:12px"><span class="small muted">The shop confirms in the chat; you can also confirm here.</span><span class="sp"></span>
        <button class="btn p" id="confirm">Confirm → create the order in the ERP</button></div>` : ""}
    </div>`;
  v.querySelectorAll("[data-pick]").forEach((b) => (b.onclick = async () => {
    try { await api(`/lines/${b.dataset.line}/choose`, { method: "POST", body: { product_id: b.dataset.pick, correction: b.dataset.fix === "1" } }); renderDraft(id); }
    catch (e) { toast(e.message, true); }
  }));
  v.querySelectorAll("[data-remove]").forEach((b) => (b.onclick = async () => { await api(`/lines/${b.dataset.remove}/remove`, { method: "POST" }); renderDraft(id); }));
  v.querySelectorAll("[data-alts]").forEach((b) => (b.onclick = () => { b.parentElement.querySelector(".alts").hidden = false; b.remove(); }));
  const c = $("#confirm");
  if (c) c.onclick = async () => {
    c.disabled = true;
    try { const r = await api(`/drafts/${id}/confirm`, { method: "POST" }); toast("Order " + r.number + " created in the ERP"); go(); }
    catch (e) { toast(e.message, true); c.disabled = false; }
  };
}

function lineCard(l, draftStatus) {
  const ckeys = Object.keys(l.checks || {}).filter((k) => !["corrected_from", "pack_rule", "history"].includes(k));
  const checks = ckeys.map((k) => `<span class="chk ${k === "offer" ? "g" : ""}">${esc(l.checks[k])}</span>`).join("");
  const used = (l.used || []).map((u) => u.startsWith("R-") ? `<a class="used" href="#/rule/${esc(u)}" title="${esc((l.rule_sentences || {})[u] || "")}">${esc(u)}</a>`
    : `<span class="used">${esc(u)}</span>`).join("");
  const qty = l.pack_factor ? `${l.qty} × ${esc((l.pack_name || "").toLowerCase())} (${l.pack_factor / 1000})` : `${l.qty} u.`;
  let body;
  if (l.status === "ask" || l.status === "unknown") {
    body = `<div class="prod">❓ ${esc(l.question || "Which product?")}</div>
      <div class="opts">${(l.options || []).map((o) => `<button data-line="${l.id}" data-pick="${esc(o)}">${esc(l.option_names[o] || o)}</button>`).join("")}
        <button data-remove="${l.id}" style="border-color:#cbd5e1;color:#64748b">Remove</button></div>`;
  } else {
    body = `<div class="prod">${esc(l.product_name || l.product_id)}</div>`;
  }
  const corrected = l.checks && l.checks.corrected_from ? `<span class="chk">corrected (was ${esc(l.option_names[l.checks.corrected_from] || "")})</span>` : "";
  return `<div class="ln ${esc(l.status)}">
    <div class="head"><div style="flex:1;min-width:0">
      <div class="said">${l.said ? "« " + esc(l.said) + " »" : "—"}${l.hint ? ` <span class="muted tiny">(reader: ${esc(l.hint)})</span>` : ""}</div>
      ${body}
      <div class="meta">${lvl(l.level)} <span>${qty}</span> ${used} ${l.confidence && l.level === "ai" ? `<span>conf. ${l.confidence.toFixed(2)}</span>` : ""} ${checks} ${corrected}
        ${l.status === "removed" ? chip("removed") : ""}</div>
      ${traceOf(l)}
      ${l.status === "ok" && l.said ? `<button class="btn sm" style="margin-top:6px" data-alts="1">${draftStatus === "open" ? "Correct" : "Correct what was learned"}</button>
        <div class="alts opts" hidden>${altButtons(l)}</div>` : ""}
    </div><div class="price">${l.status === "ok" ? money(l.line_ttc) : ""}</div></div></div>`;
}
function altButtons(l) {
  // alternatives come from the line's options when it was asked; else a search link
  const opts = [...new Set([...(l.options || []), ...(l.alts || [])])].filter((o) => o !== l.product_id);
  return opts.map((o) => `<button data-line="${l.id}" data-fix="1" data-pick="${esc(o)}">${esc(l.option_names[o] || o)}</button>`).join("") +
    `<button data-remove="${l.id}" style="border-color:#cbd5e1;color:#64748b">Remove</button>`;
}

// ---- catalogue ---------------------------------------------------------------------------
route("catalogue", async () => {
  const d = await api("/products");
  const tot = d.items.reduce((a, p) => a + p.aliases, 0), lines = d.items.reduce((a, p) => a + p.lines, 0), free = d.items.reduce((a, p) => a + p.free_lines, 0);
  main.innerHTML = `
    <div class="ttl"><div><h1>Catalogue</h1><div class="s">The ERP knows what the products are. Order Desk knows how people refer to them.</div></div>
      <div class="row"><input id="q" placeholder="Search products…" style="width:260px"><button class="btn" id="sync">Sync from ERP</button></div></div>
    <div class="kpis" style="grid-template-columns:repeat(4,1fr)">
      <div class="kpi"><div class="l">Products</div><div class="v">${d.items.length}</div><div class="d n">mirrored from the ERP</div></div>
      <div class="kpi"><div class="l">Wordings known</div><div class="v">${tot}</div><div class="d n">${overview.trusted_aliases || 0} trusted</div></div>
      <div class="kpi"><div class="l">Lines seen</div><div class="v">${lines}</div></div>
      <div class="kpi"><div class="l">Resolved without AI</div><div class="v">${lines ? pct(free / lines) : "—"}</div></div>
    </div>
    <div class="card" style="padding:0"><table class="t" id="pt"></table></div>`;
  const draw = (q) => {
    q = (q || "").toLowerCase();
    const rows = d.items.filter((p) => !q || (p.name + " " + p.brand).toLowerCase().includes(q));
    $("#pt").innerHTML = `<tr><th>Product</th><th>Brand</th><th>Packs</th><th class="num">Wordings</th><th class="num">Lines</th><th class="num">Without AI</th></tr>` +
      rows.map((p) => `<tr class="click" onclick="location.hash='#/product/${encodeURIComponent(p.id)}'">
        <td class="sent">${esc(p.name)}</td><td class="muted">${esc(p.brand)}</td>
        <td class="small muted">${(p.packs || []).map((k) => esc(k.name) + " ×" + k.factor_milli / 1000).join(" · ") || "unit"}</td>
        <td class="num">${p.aliases ? p.aliases + (p.trusted ? ` <span class="muted small">(${p.trusted} trusted)</span>` : "") : "—"}</td>
        <td class="num">${p.lines || "—"}</td><td class="num">${p.lines ? pct(p.free_lines / p.lines) : "—"}</td></tr>`).join("");
  };
  draw();
  $("#q").oninput = (e) => draw(e.target.value);
  $("#sync").onclick = async () => { try { const r = await api("/erp/sync", { method: "POST" }); toast(r.products + " products synced"); go(); } catch (e) { toast(e.message, true); } };
});

route("product", async (id) => {
  const d = await api("/products/" + encodeURIComponent(id));
  const p = d.product;
  const src = { text: "Text", voice: "Voice", photo: "List photo", clerk: "Clerk", invoice: "Invoice" };
  main.innerHTML = `
    <div class="ttl"><div><div class="crumb"><a href="#/catalogue">Catalogue</a> › ${esc(p.brand || "—")}</div><h1>${esc(p.name)}</h1>
      <div class="s">ERP id ${esc(p.id)} · packs: ${(p.packs || []).map((k) => esc(k.name) + " ×" + k.factor_milli / 1000).join(" · ") || "sold by unit"}
      ${p.size ? " · size " + esc(p.size) : ""} · catalogue words: <code>${esc((p.words || []).join(" "))}</code></div></div></div>
    <div class="kpis" style="grid-template-columns:repeat(4,1fr)">
      <div class="kpi"><div class="l">Wordings known</div><div class="v">${d.aliases.length}</div></div>
      <div class="kpi"><div class="l">Lines seen</div><div class="v">${d.lines}</div></div>
      <div class="kpi"><div class="l">Resolved without AI</div><div class="v">${d.lines ? pct(d.free_lines / d.lines) : "—"}</div></div>
      <div class="kpi"><div class="l">Rules touching it</div><div class="v">${(d.rules || []).length}</div></div>
    </div>
    <div class="card" style="padding:0">
      <table class="t"><tr><th>Wording</th><th>Learned from</th><th>Valid for</th><th class="num">Confirmations</th><th>First seen</th><th>State</th><th></th></tr>
      ${d.aliases.map((a) => `<tr><td class="id" dir="auto">${esc(a.text)}</td><td><span class="chip c-src">${esc(src[a.source] || a.source)}</span></td>
        <td><span class="chip c-scope">${esc(a.scope_name || a.scope_kind)}</span></td><td class="num">${a.witnesses}</td><td class="small muted">${short(a.first_seen)}</td>
        <td>${chip(a.state)}</td><td class="num">${a.state !== "trusted" ? `<button class="btn sm" data-al="${a.id}" data-st="trusted">Trust</button>` : ""}
        ${a.state !== "blocked" ? `<button class="btn sm r" data-al="${a.id}" data-st="blocked">Block</button>` : ""}</td></tr>`).join("") ||
        `<tr><td colspan="7" class="empty">No wording learned yet. Every confirmed order teaches one.</td></tr>`}
      </table>
      <div class="row" style="padding:12px 14px;border-top:1px solid var(--line)"><input id="na" placeholder="Add a wording people use, e.g. « shampoing hns »" style="flex:1">
        <button class="btn p" id="add">Add as trusted</button></div>
    </div>
    <div class="card"><h5>Rules that touch this product</h5>
      ${(d.rules || []).map((r) => `<div style="margin-bottom:6px"><a class="id" href="#/rule/${esc(r.id)}">${esc(r.id)}</a> ${chip(r.status)} ${esc(r.sentence)}</div>`).join("") || `<div class="muted small">None yet.</div>`}</div>`;
  main.querySelectorAll("[data-al]").forEach((b) => (b.onclick = async () => { await api(`/aliases/${b.dataset.al}/state`, { method: "POST", body: { state: b.dataset.st } }); go(); }));
  $("#add").onclick = async () => {
    const text = $("#na").value.trim(); if (!text) return;
    try { await api("/aliases", { method: "POST", body: { text, product_id: p.id } }); toast("Wording added"); go(); } catch (e) { toast(e.message, true); }
  };
});

// ---- rules -------------------------------------------------------------------------------
let ruleFilter = "all";
route("rules", async () => {
  const [d, eng] = await Promise.all([api("/rules"), api("/engineer")]);
  const by = {}; d.items.forEach((r) => (by[r.status] = (by[r.status] || 0) + 1));
  const last = eng.last || {};
  main.innerHTML = `
    <div class="ttl"><div><h1>Rules</h1><div class="s">${by.live || 0} live · ${by.shadow || 0} in shadow · ${by.rejected || 0} rejected · ${by.rolled_back || 0} rolled back
      · written by the rule engineer (Muse Wonder) and by people · the code decides</div></div>
      <div class="row"><button class="btn" id="run">${eng.running ? "Engineer running…" : "Run engineer now"}</button><button class="btn p" id="write">+ Write a rule</button></div></div>
    <div class="grid g3" style="margin-bottom:14px">
      <div class="kpi"><div class="l">Benchmark (known lines)</div><div class="v">${eng.benchmark}</div><div class="d n">ground truth from confirmed orders</div></div>
      <div class="kpi"><div class="l">Residue</div><div class="v">${eng.residue}</div><div class="d n">known lines rules can't resolve alone yet</div></div>
      <div class="kpi"><div class="l">Last engineer run</div><div class="v" style="font-size:16px">${eng.last_run ? short(eng.last_run) : "never"}</div>
        <div class="d n">${last.summary ? esc(last.summary).slice(0, 140) : last.error ? esc(last.error) : ""}${last.cost ? " · " + usd(last.cost) : ""}</div></div>
    </div>
    <div id="form"></div>
    <div class="filters">${["all", "live", "shadow", "proposed", "rejected", "rolled_back", "disabled"].map((f) =>
      `<button class="${f === ruleFilter ? "on" : ""}" data-f="${f}">${f === "all" ? "All" : f.replace("_", " ")}${by[f] ? " · " + by[f] : ""}</button>`).join("")}</div>
    <div class="card" style="padding:0"><table class="t">
      <tr><th>Rule</th><th>What it says</th><th>Scope</th><th>Born</th><th class="num">Lines</th><th>Status</th></tr>
      ${d.items.filter((r) => ruleFilter === "all" || r.status === ruleFilter).map((r) => {
        const b = r.bench || {};
        const why = r.status === "rejected" || r.status === "rolled_back" ? r.reason
          : (r.author === "rule-engineer" ? "rule engineer" : "written by " + r.author) + (b.lines ? ` · benchmark +${b.newly_resolved}, ${b.wrong} wrong` : "");
        return `<tr class="click" onclick="location.hash='#/rule/${r.id}'"><td class="id">${esc(r.id)} v${r.version}<br><span class="chip c-type">${esc(r.type)}</span></td>
        <td class="sent">${esc(r.sentence)}<span class="why">${r.scope.kind === "supplier" ? "🧾 supplier invoices · " : ""}${esc(why)}</span></td><td>${scopeChip(r.scope)}</td><td class="small">${short(r.created_at)}</td>
        <td class="num">${r.lines_resolved || "—"}</td><td>${r.status === "shadow" ? chip("shadow", `shadow ${r.shadow_agree}/${overviewSetting("shadow_agreements")}`) : chip(r.status)}</td></tr>`;
      }).join("") || `<tr><td colspan="6" class="empty">No rule yet. Rules are born from the lines that needed the AI or a person.</td></tr>`}
    </table></div>`;
  main.querySelectorAll("[data-f]").forEach((b) => (b.onclick = () => { ruleFilter = b.dataset.f; go(); }));
  $("#write").onclick = () => ruleForm();
  $("#run").onclick = async () => {
    try { await api("/engineer/run", { method: "POST" }); toast("Rule engineer started"); pollEngineer(); } catch (e) { toast(e.message, true); }
  };
  if (eng.running) pollEngineer();
});
let settingsCache = {};
const overviewSetting = (k) => settingsCache[k] || "5";
function pollEngineer() {
  clearInterval(timer);
  timer = setInterval(async () => { const e = await api("/engineer"); refreshShell(); if (!e.running) { clearInterval(timer); toast("Rule engineer finished: " + ((e.last || {}).summary || "").slice(0, 160)); go(); } }, 3000);
}

function ruleForm() {
  const f = $("#form");
  f.innerHTML = `<div class="card" style="margin-bottom:14px"><h5>Write a rule · it goes through the same benchmark gate as the engineer's</h5>
    <div class="grid g3">
      <div><label class="f">Type</label><select id="rt">${["synonym", "rewrite", "pack", "default", "guard"].map((t) => `<option>${t}</option>`).join("")}</select></div>
      <div><label class="f">Scope</label><select id="rs"><option value="global">Global</option><option value="tenant">This distributor</option></select></div>
      <div><label class="f">When the line contains (pattern)</label><input id="rp" placeholder="hns|h&s  ·  x{n}$  ·  fardou" style="width:100%"></div>
    </div>
    <div class="grid g3">
      <div><label class="f" id="ral">Then</label><input id="ra" placeholder="head shoulders" style="width:100%"></div>
      <div><label class="f">Pack unit (pack rules)</label><select id="ru"><option value="">—</option><option>carton</option><option>pack</option><option>unit</option></select></div>
      <div><label class="f">Pack quantity (pack rules)</label><input id="rq" placeholder="{n} or 6" style="width:100%"></div>
    </div>
    <div id="pv" style="margin-top:12px"></div>
    <div class="row" style="margin-top:12px"><label class="small"><input type="checkbox" id="rk"> go live now (skip shadow mode)</label><span class="sp"></span>
      <button class="btn" id="rtst">Test on the benchmark</button><button class="btn p" id="rsub">Submit to the gate</button></div></div>`;
  const body = () => {
    const t = $("#rt").value, a = $("#ra").value.trim();
    const action = {};
    if (t === "synonym" || t === "rewrite") action.replace = a;
    if (t === "default") action.product = a;
    if (t === "guard") { if (/^[A-Za-z0-9_-]{6,}$/.test(a) && a.includes("-")) action.block = a; else action.block_word = a; }
    if (t === "pack") { action.pack_unit = $("#ru").value || "carton"; action.pack_qty = $("#rq").value.trim(); }
    return { type: t, scope: { kind: $("#rs").value }, pattern: $("#rp").value.trim(), action, skip_shadow: $("#rk").checked };
  };
  const label = () => ($("#ral").textContent = { synonym: "Then it means (words)", rewrite: "Then write it as", default: "Then it is (product id)", guard: "Only products containing (word)", pack: "Then (see pack fields)" }[$("#rt").value]);
  $("#rt").onchange = label; label();
  $("#rtst").onclick = async () => {
    try {
      const r = await api("/rules/preview", { method: "POST", body: body() });
      $("#pv").innerHTML = benchBox(r.bench, r.passes, r.verdict, r.sentence);
    } catch (e) { $("#pv").innerHTML = `<div class="warn">${esc(e.message)}</div>`; }
  };
  $("#rsub").onclick = async () => {
    try { const r = await api("/rules", { method: "POST", body: body() }); toast(r.rule.id + ": " + r.rule.status.replace("_", " ")); location.hash = "#/rule/" + r.rule.id; }
    catch (e) { toast(e.message, true); }
  };
}
function benchBox(b, passes, verdict, sentence) {
  b = b || {};
  return `${sentence ? `<div class="bigsent" style="font-size:16px">${esc(sentence)}</div>` : ""}
    <div class="bench"><div><span class="l">Known lines</span><b>${b.lines || 0}</b></div><div><span class="l">Newly resolved</span><b style="color:var(--green)">+${b.newly_resolved || 0}</b></div>
    <div><span class="l">Wrong</span><b style="color:${b.wrong ? "var(--red)" : "var(--green)"}">${b.wrong || 0}</b></div><div><span class="l">Conflicts</span><b>${b.conflicts || 0}</b></div></div>
    ${(b.examples_new || []).length ? `<div class="small" style="margin-top:8px">Resolves: ${b.examples_new.map((x) => `« ${esc(x.said)} » → ${esc(x.expected)}`).join(" · ")}</div>` : ""}
    ${(b.examples_wrong || []).length ? `<div class="small" style="margin-top:4px;color:var(--red)">Wrong: ${b.examples_wrong.map((x) => `« ${esc(x.said)} » → ${esc(x.got)} (expected ${esc(x.expected)})`).join(" · ")}</div>` : ""}
    ${verdict ? `<div class="${passes ? "okb" : "warn"}" style="margin-top:8px">${passes ? "✓ Gate passed: " : "✕ Gate refused: "}${esc(verdict)}</div>` : ""}`;
}

route("rule", async (id) => {
  const d = await api("/rules/" + encodeURIComponent(id));
  const r = d.rule, b = r.bench || {};
  const icon = { "rule.proposed": "v", "rule.benchmark": "g", "rule.shadow": "a", "rule.live": "g", "rule.rejected": "r", "rule.rolled_back": "r", "rule.strike": "r", "rule.disabled": "" };
  const what = (e) => {
    const x = e.details || {};
    switch (e.kind) {
      case "rule.proposed": return ["Proposed by " + (e.actor === "rule-engineer" ? "the rule engineer (Muse Wonder)" : e.actor), x.rationale || ""];
      case "rule.benchmark": return ["Tested on the benchmark", `${x.lines} known lines · +${x.newly_resolved} resolved · ${x.wrong} wrong · ${x.conflicts} conflicts`];
      case "rule.shadow": return ["Gate passed → shadow mode", x.gate || ""];
      case "rule.live": return [e.actor === "clerk" ? "Set live by a person" : "Agreed with real lines → live", x.agreements ? x.agreements + " agreements" : x.gate || ""];
      case "rule.rejected": return ["Rejected", x.gate || (x.said ? "disagreed in shadow on « " + x.said + " »" : "")];
      case "rule.strike": return ["Corrected by a person", "« " + (x.said || "") + " »"];
      case "rule.rolled_back": return ["Rolled back", x.strikes ? x.strikes + " corrections" : "by a person"];
      default: return [e.kind.replace("rule.", ""), ""];
    }
  };
  const inspired = d.inspired.length ? `<div class="e a"><div class="when">${short(d.inspired[0].at)}${d.inspired.length > 1 ? " → " + short(d.inspired[d.inspired.length - 1].at) : ""}</div>
      <div class="what">${d.inspired.length} line(s) needed help</div><div class="det">${d.inspired.slice(0, 4).map((x) => `« ${esc(x.said)} » → ${esc(x.product)}`).join(" · ")}</div></div>` : "";
  main.innerHTML = `
    <div class="ttl" style="align-items:flex-start"><div style="flex:1"><div class="crumb"><a href="#/rules">Rules</a> › ${esc(r.id)} · version ${r.version}
      <span class="chip c-type">${esc(r.type)}</span> ${scopeChip(r.scope)} ${chip(r.status)}</div>
      <div class="bigsent">${esc(r.sentence)}</div>
      <div class="s">${r.author === "rule-engineer" ? "Proposed by the rule engineer (Muse Wonder)" : "Written by " + esc(r.author)} on ${short(r.created_at)}${r.live_at ? " · live since " + short(r.live_at) : ""}
      ${r.reason && r.status !== "live" ? " · " + esc(r.reason) : ""}</div></div>
      <div class="row">${r.status !== "live" ? `<button class="btn" data-st="live">Set live</button>` : ""}${r.status !== "disabled" ? `<button class="btn" data-st="disabled">Disable</button>` : ""}
        ${r.status === "live" ? `<button class="btn r" data-st="rolled_back">Roll back</button>` : ""}</div></div>
    <div class="grid g21">
      <div class="card"><h5>Birth of this rule</h5><div class="tl">${inspired}
        ${d.events.map((e) => { const [w, det] = what(e); return `<div class="e ${icon[e.kind] || ""}"><div class="when">${short(e.at)}</div><div class="what">${esc(w)}</div><div class="det">${esc(det)}</div></div>`; }).join("")}
      </div></div>
      <div>
        <div class="card"><h5>Impact</h5><div class="grid g2">
          <div><div class="small muted">Lines resolved</div><div style="font-size:22px;font-weight:750">${r.lines_resolved}</div></div>
          <div><div class="small muted">Corrections</div><div style="font-size:22px;font-weight:750;color:${r.strikes ? "var(--red)" : "var(--green)"}">${r.strikes}</div></div>
          <div><div class="small muted">AI calls avoided</div><div style="font-size:22px;font-weight:750">${r.lines_resolved}</div></div>
          <div><div class="small muted">AI cost saved (est.)</div><div style="font-size:22px;font-weight:750">${usd(d.saved)}</div></div></div>
          <div class="small muted" style="margin-top:8px">Cost of writing it: ${usd(r.ai_cost)}${r.status === "shadow" ? ` · shadow agreements ${r.shadow_agree}` : ""}</div></div>
        <div class="card"><h5>Stored form</h5><div class="codeb"><span class="k">type</span>: ${esc(r.type)}
<span class="k">scope</span>: ${esc(r.scope.kind)}${r.scope.name ? " = <span class='s'>" + esc(r.scope.name) + "</span>" : ""}
<span class="k">when</span>: <span class="s">"${esc(r.pattern)}"</span>
<span class="k">then</span>: ${esc(JSON.stringify(r.action))}${d.product_name ? "\n<span class='k'>product</span>: " + esc(d.product_name) : ""}</div></div>
        <div class="card"><h5>Benchmark when it was born</h5>${benchBox(b)}</div>
      </div>
    </div>
    <div class="card"><h5>Recent lines it resolved</h5><table class="t"><tr><th>Line as said</th><th>Result</th><th>When</th></tr>
      ${d.resolved.map((x) => `<tr><td class="id">« ${esc(x.said)} »</td><td>${esc(x.product)}</td><td class="small muted">${short(x.at)}</td></tr>`).join("") ||
        `<tr><td colspan="3" class="empty">None yet${r.status === "shadow" ? " (shadow rules only watch)" : ""}.</td></tr>`}</table></div>`;
  main.querySelectorAll("[data-st]").forEach((bt) => (bt.onclick = async () => { await api(`/rules/${r.id}/status`, { method: "POST", body: { status: bt.dataset.st } }); toast(r.id + " → " + bt.dataset.st.replace("_", " ")); go(); }));
});

// ---- learning & cost -----------------------------------------------------------------------
let learnKind = "";
route("learning", async () => {
  const d = await api("/learning?days=30&kind=" + learnKind);
  const t = d.totals || {};
  const ov = { lines: t.lines, free: t.free, auto: t.auto, wrong_auto: t.wrong_auto, resolve_calls: t.resolve_calls, orders: t.docs };
  const days = d.days || [];
  const lines = ov.lines || 0;
  const aiLines = days.reduce((a, x) => a + x.ai + x.person, 0);
  const cost = days.reduce((a, x) => a + x.cost, 0), dl = days.reduce((a, x) => a + x.lines, 0);
  const lastDay = days[days.length - 1] || {}, firstDay = days[0] || {};
  const freeShare = (x) => x.lines ? (x.alias + x.rules) / x.lines : 0;
  main.innerHTML = `
    <div class="ttl"><div><h1>Learning &amp; cost</h1><div class="s">Every AI call counted${learnKind ? "" : ", the rule engineer included"}. Last 30 days.</div></div>
      <div class="tabs">${[["", "All"], ["order", "💬 Shop messages"], ["invoice", "🧾 Supplier invoices"]].map(([k, n]) =>
        `<button class="${k === learnKind ? "on" : ""}" data-lk="${k}">${n}</button>`).join("")}</div></div>
    <div class="kpis">
      <div class="kpi"><div class="l">Lines handled</div><div class="v">${lines}</div><div class="d n">${ov.orders} ${learnKind === "invoice" ? "receipts" : learnKind === "order" ? "orders" : "orders + receipts"} confirmed</div></div>
      <div class="kpi"><div class="l">Resolved without AI</div><div class="v">${lines ? pct(ov.free / lines) : "—"}</div>
        <div class="d">${days.length > 1 ? `${pct(freeShare(firstDay))} → ${pct(freeShare(lastDay))}` : ""}</div></div>
      <div class="kpi"><div class="l">Wrong automatic</div><div class="v">${ov.auto ? pct(ov.wrong_auto / ov.auto) : "—"}</div><div class="d n">${ov.wrong_auto} of ${ov.auto} automatic lines</div></div>
      <div class="kpi"><div class="l">AI cost / 1,000 lines</div><div class="v">${dl ? usd(cost / dl * 1000) : "—"}</div><div class="d n">reading + resolving</div></div>
      <div class="kpi"><div class="l">Model calls / line</div><div class="v">${lines ? (ov.resolve_calls / lines).toFixed(2) : "—"}</div><div class="d n">to identify a product</div></div>
    </div>
    <div class="grid g21">
      <div class="card"><h5>How lines were resolved, by day</h5>${stackChart(days)}
        <div class="legend"><span><i style="background:#0f766e"></i>Alias</span><span><i style="background:#5eead4"></i>Rules</span><span><i style="background:#f59e0b"></i>AI (Spark)</span><span><i style="background:#f87171"></i>Person</span></div></div>
      <div class="card"><h5>AI cost per 1,000 lines</h5>${costChart(days)}<div class="small muted" style="margin-top:4px">Dots mark rules going live.</div></div>
    </div>
    <div class="grid g21" style="margin-top:14px">
      <div class="card"><h5>AI spend by job</h5><table class="t"><tr><th>Model</th><th>Job</th><th class="num">Calls</th><th class="num">Cost</th></tr>
        ${(d.jobs || []).map((j) => `<tr><td>${esc(j.model)}</td><td>${esc({ transcribe: "Voice orders", read: "Reading list photos", extract: "Reading chatty messages", resolve: "Resolving the residue", engineer: "Rule engineer", invoice: "Reading supplier invoices" }[j.purpose] || j.purpose)}</td>
          <td class="num">${j.calls}</td><td class="num">${usd(j.cost)}</td></tr>`).join("") || `<tr><td colspan="4" class="empty">No AI call yet.</td></tr>`}
        <tr><td colspan="3"><b>Total</b></td><td class="num"><b>${usd((d.jobs || []).reduce((a, j) => a + j.cost, 0))}</b></td></tr></table></div>
      <div class="card"><h5>Saved by aliases and rules</h5><div style="font-size:30px;font-weight:800;color:var(--brand)">${usd(d.saved)}</div>
        <div class="small muted">${d.free_lines} lines resolved for free × ${usd(d.cost_per_ai_line)}, the average AI cost of a line that needed the AI. ${aiLines} line(s) needed the AI or a person.</div></div>
    </div>`;
  main.querySelectorAll("[data-lk]").forEach((b) => (b.onclick = () => { learnKind = b.dataset.lk; go(); }));
});

function stackChart(days) {
  if (!days.length) return `<div class="empty">No line yet.</div>`;
  const W = 620, H = 220, L = 34, B = 22, bw = Math.max(6, Math.min(34, (W - L - 10) / days.length - 6));
  const x = (i) => L + 6 + i * ((W - L - 10) / days.length);
  const y = (v) => 8 + (H - B - 8) * (1 - v);
  let out = "";
  [0, 0.5, 1].forEach((v) => (out += `<line x1="${L}" x2="${W}" y1="${y(v)}" y2="${y(v)}" stroke="#e2e8f0"/><text x="${L - 5}" y="${y(v) + 4}" font-size="10" fill="#64748b" text-anchor="end">${v * 100}%</text>`));
  days.forEach((dd, i) => {
    const tot = dd.lines || 1; let base = 0;
    [["alias", "#0f766e"], ["rules", "#5eead4"], ["ai", "#f59e0b"], ["person", "#f87171"]].forEach(([k, c]) => {
      const v = dd[k] / tot; if (!v) return;
      out += `<rect x="${x(i)}" y="${y(base + v)}" width="${bw}" height="${(H - B - 8) * v}" fill="${c}" rx="2"><title>${dd.day} ${k}: ${dd[k]}</title></rect>`;
      base += v;
    });
    if (days.length <= 12 || i % Math.ceil(days.length / 8) === 0) out += `<text x="${x(i) + bw / 2}" y="${H - 6}" font-size="10" fill="#64748b" text-anchor="middle">${dd.day.slice(5)}</text>`;
  });
  return `<svg viewBox="0 0 ${W} ${H}" width="100%">${out}</svg>`;
}
function costChart(days) {
  const pts = days.filter((d) => d.lines).map((d) => ({ d: d.day, v: d.cost / d.lines * 1000, live: d.rules_live }));
  if (!pts.length) return `<div class="empty">No cost yet.</div>`;
  const W = 420, H = 220, L = 44, B = 22, max = Math.max(...pts.map((p) => p.v)) * 1.15 || 1;
  const x = (i) => L + (pts.length === 1 ? (W - L) / 2 : i * (W - L - 12) / (pts.length - 1));
  const y = (v) => 10 + (H - B - 10) * (1 - v / max);
  let g = "";
  [0, 0.5, 1].forEach((f) => (g += `<line x1="${L}" x2="${W}" y1="${y(max * f)}" y2="${y(max * f)}" stroke="#e2e8f0"/><text x="${L - 5}" y="${y(max * f) + 4}" font-size="10" fill="#64748b" text-anchor="end">$${(max * f).toFixed(2)}</text>`));
  const line = pts.map((p, i) => `${x(i)},${y(p.v)}`).join(" ");
  const area = line + ` ${x(pts.length - 1)},${y(0)} ${x(0)},${y(0)}`;
  const dots = pts.map((p, i) => (p.live || []).length ? `<circle cx="${x(i)}" cy="${y(p.v)}" r="4.5" fill="#fff" stroke="#0f766e" stroke-width="2"/><text x="${x(i) + 6}" y="${y(p.v) - 7}" font-size="9.5" fill="#0f766e" font-family="Consolas">${esc(p.live.join(", "))}</text>`
    : `<circle cx="${x(i)}" cy="${y(p.v)}" r="2.5" fill="#0f766e"/>`).join("");
  const xl = pts.map((p, i) => (pts.length <= 8 || i % Math.ceil(pts.length / 6) === 0) ? `<text x="${x(i)}" y="${H - 6}" font-size="10" fill="#64748b" text-anchor="middle">${p.d.slice(5)}</text>` : "").join("");
  return `<svg viewBox="0 0 ${W} ${H}" width="100%">${g}<polygon points="${area}" fill="#ccfbf1" opacity=".7"/><polyline points="${line}" fill="none" stroke="#0f766e" stroke-width="2.2"/>${dots}${xl}</svg>`;
}

// ---- invoices ----------------------------------------------------------------------------
route("invoices", (sel) => invoicesScreen(sel));
route("invoice", (sel) => invoicesScreen(sel));
async function invoicesScreen(sel) {
  const [list, own] = await Promise.all([api("/drafts?kind=invoice"), api("/owner")]);
  const items = list.items;
  if (!sel && items.length) sel = items[0].id;
  main.innerHTML = `
    <div class="ttl"><div><h1>Supplier invoices</h1><div class="s">A photo or PDF of a supplier invoice becomes a goods receipt in the ERP. Each supplier's wordings are learned.</div></div></div>
    <div class="grid g12">
      <div>
        <label class="drop" id="drop"><input type="file" id="file" accept="image/*,application/pdf" hidden>
          <div style="font-weight:700">⬆ Drop an invoice here</div><div class="small">photo or PDF · or click to choose</div></label>
        <div class="card" style="margin:12px 0">
          <h5>Send invoices from Telegram</h5>
          ${own.chats.length ? `<div class="small">Linked as owner: ${own.chats.map((c) => esc(c.name || c.chat)).join(", ")}. Send photos or PDFs to
            <a target="_blank" href="https://t.me/${esc(own.bot)}">@${esc(own.bot)}</a>.</div>` : `<div class="small muted">Link your own Telegram once, then send invoices to the bot.</div>`}
          <div class="row" style="margin-top:8px"><span id="owncode"></span><span class="sp"></span><button class="btn sm" id="ownpair">Owner pairing code</button></div>
        </div>
        <div class="list">${items.map((d) => `
          <a class="it ${d.id === sel ? "on" : ""}" href="#/invoices/${d.id}">
            <div class="top"><span>${esc(d.customer || "—")}</span>${d.status === "confirmed" ? chip("confirmed", d.erp_order_number) : d.pending ? chip("ask", d.pending + " to check") : chip(d.status)}</div>
            <div class="small muted">${d.lines} line(s) · ${esc(d.chat_key.split(":")[0])} · ${short(d.updated_at)}</div></a>`).join("")}</div>
      </div>
      <div id="inv-view">${items.length ? "" : `<div class="card empty">No invoice yet. Drop one on the left, or send it to the bot from your owner chat.</div>`}</div>
    </div>`;
  const file = $("#file"), drop = $("#drop");
  const upload = async (f) => {
    if (!f) return;
    drop.innerHTML = `<div class="pulse" style="font-weight:700">Reading the invoice with Muse Spark…</div><div class="small">${esc(f.name)}</div>`;
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
  drop.ondrop = (e) => { e.preventDefault(); upload(e.dataTransfer.files[0]); };
  $("#ownpair").onclick = async () => {
    const r = await api("/owner/pair-code", { method: "POST" });
    $("#owncode").innerHTML = `<span class="codebox" style="font-size:18px;padding:4px 10px">${esc(r.code)}</span>
      <a class="small" target="_blank" href="https://t.me/${esc(r.bot)}?start=${esc(r.code)}">open in Telegram</a>`;
  };
  if (sel) renderInvoice(sel);
}

async function renderInvoice(id) {
  const v = $("#inv-view");
  const d = await api("/drafts/" + id);
  const dr = d.draft, inv = d.invoice || {}, open = dr.status === "open";
  const msg = (d.messages || [])[0] || {};
  const paper = msg.media ? (msg.media.endsWith(".pdf") ? `<embed src="${esc(msg.media)}" type="application/pdf">` : `<img src="${esc(msg.media)}" alt="invoice">`) : "";
  const checks = Object.values(inv.checks || {}).map((c) => `<div class="warn" style="margin-top:6px">${esc(c)}</div>`).join("");
  let sumLines = 0;
  const rows = d.lines.map((l) => { sumLines += l.line_ttc || 0; return invLine(l, open); }).join("");
  const counts = { auto: 0, ai: 0, open: 0 };
  d.lines.forEach((l) => { if (l.status !== "ok") counts.open++; else if (l.level === "alias" || l.level === "rules") counts.auto++; else counts.ai++; });
  v.innerHTML = `
    <div class="card"><div class="row" style="align-items:flex-start"><div>
      <div class="crumb">Invoice n° ${esc(inv.doc_number || "—")} · ${esc(inv.doc_date || "")} · read ${short(dr.created_at)} · AI ${usd(msg.ai_cost)}</div>
      <h2 style="font-size:19px;margin:2px 0 0">${esc(inv.supplier || inv.supplier_said || "Unknown supplier")}</h2>
      ${inv.supplier_said && inv.supplier !== inv.supplier_said ? `<div class="small muted">printed: « ${esc(inv.supplier_said)} »</div>` : ""}</div>
      <span class="sp"></span>${dr.status === "confirmed" ? chip("confirmed", "Receipt " + dr.erp_order_number + " in the ERP") : chip(dr.status)}</div>
      ${open ? `<div class="row" style="margin-top:10px"><label class="small muted">Supplier</label>
        <select id="sup">${["<option value=''>— choose —</option>"].concat((d.suppliers || []).map((s) => `<option value="${esc(s.id)}" ${s.id === inv.supplier_id ? "selected" : ""}>${esc(s.name)}</option>`)).join("")}</select></div>` : ""}
      ${checks}
      <div class="row small" style="margin-top:10px;gap:14px"><span>${lvl("rules")} ${counts.auto} by aliases &amp; rules</span><span>${lvl("ai")} ${counts.ai} by the AI or a person</span>
        <span>${counts.open ? chip("ask", counts.open + " to check") : chip("ok", "all lines matched")}</span></div>
    </div>
    <div class="inv" style="margin-top:14px">
      <div class="paper">${paper ? `<div class="card" style="padding:8px">${paper}</div>` : ""}</div>
      <div class="card"><h5>Lines · printed → matched</h5>${rows}
        <div class="total"><span>Total HT (lines)</span><span>${money(sumLines)}${inv.doc_total ? ` <span class="small muted">· printed ${money(inv.doc_total)}</span>` : ""}</span></div>
        ${open ? `<div class="row" style="margin-top:12px"><span class="small muted">Approving books a goods receipt (BR) in the ERP and teaches every line for this supplier.</span><span class="sp"></span>
          <button class="btn p" id="approve" ${counts.open || !inv.supplier_id ? "disabled" : ""}>Approve → goods receipt in the ERP</button></div>` : ""}
      </div>
    </div>`;
  const sup = $("#sup");
  if (sup) sup.onchange = async () => { if (!sup.value) return; await api(`/drafts/${id}/supplier`, { method: "POST", body: { supplier_id: sup.value } }); toast("Supplier set; lines matched again in its scope"); renderInvoice(id); };
  v.querySelectorAll("[data-pick]").forEach((b) => (b.onclick = async () => {
    try { await api(`/lines/${b.dataset.line}/choose`, { method: "POST", body: { product_id: b.dataset.pick, correction: b.dataset.fix === "1" } }); renderInvoice(id); }
    catch (e) { toast(e.message, true); }
  }));
  v.querySelectorAll("[data-remove]").forEach((b) => (b.onclick = async () => { await api(`/lines/${b.dataset.remove}/remove`, { method: "POST" }); renderInvoice(id); }));
  v.querySelectorAll("[data-alts]").forEach((b) => (b.onclick = () => { b.parentElement.querySelector(".alts").hidden = false; b.remove(); }));
  const a = $("#approve");
  if (a) a.onclick = async () => {
    a.disabled = true; a.textContent = "Booking…";
    try { const r = await api(`/drafts/${id}/confirm`, { method: "POST" }); toast("Goods receipt " + r.number + " created in the ERP"); go(); }
    catch (e) { toast(e.message, true); a.disabled = false; a.textContent = "Approve → goods receipt in the ERP"; }
  };
}

function invLine(l, open) {
  const ckeys = Object.keys(l.checks || {}).filter((k) => !["corrected_from", "pack_rule", "pack_qty", "history"].includes(k));
  const checks = ckeys.map((k) => `<span class="chk">${esc(l.checks[k])}</span>`).join("");
  const used = (l.used || []).map((u) => u.startsWith("R-") ? `<a class="used" href="#/rule/${esc(u)}" title="${esc((l.rule_sentences || {})[u] || "")}">${esc(u)}</a>` : `<span class="used">${esc(u)}</span>`).join("");
  const qty = l.pack_factor ? `${l.qty} × ${esc((l.pack_name || "").toLowerCase())} (${l.pack_factor / 1000}) = ${l.qty_milli / 1000} u.` : `${l.qty} u.`;
  const unit = l.qty_milli ? Math.round((l.line_ttc || 0) / (l.qty_milli / 1000)) : 0;
  const body = l.status === "ask" || l.status === "unknown"
    ? `<div class="prod">❓ ${esc(l.status === "unknown" && !(l.options || []).length ? "Not in the catalogue" : (l.question || "Which product?"))}</div>
       <div class="opts">${(l.options || []).map((o) => `<button data-line="${l.id}" data-pick="${esc(o)}">${esc(l.option_names[o] || o)}</button>`).join("")}
       <button data-remove="${l.id}" style="border-color:#cbd5e1;color:#64748b">Skip line</button></div>`
    : `<div class="prod">→ ${esc(l.product_name || l.product_id)}</div>`;
  return `<div class="ln ${esc(l.status)}"><div class="head"><div style="flex:1;min-width:0">
      <span class="printed">${esc(l.said)}</span>
      ${body}
      <div class="meta">${lvl(l.level)} <span>${qty}</span> ${used} ${l.confidence && l.level === "ai" ? `<span>conf. ${l.confidence.toFixed(2)}</span>` : ""} ${checks}
        ${l.status === "removed" ? chip("removed", "skipped") : ""}</div>
      ${traceOf(l)}
      ${l.status === "ok" ? `<button class="btn sm" style="margin-top:6px" data-alts="1">${open ? "Correct" : "Correct what was learned"}</button><div class="alts opts" hidden>${altButtons(l)}</div>` : ""}
    </div><div class="price">${money(l.line_ttc)}<div class="amt">${unit ? money(unit * 100 / 100) + " / u." : ""}</div></div></div></div>`;
}

// ---- customers ---------------------------------------------------------------------------
route("customers", async () => {
  const d = await api("/customers");
  main.innerHTML = `
    <div class="ttl"><div><h1>Customers</h1><div class="s">Customers come from the ERP. Give a shop its code once; it sends the code to
      ${d.bot ? `<a href="https://t.me/${esc(d.bot)}" target="_blank">@${esc(d.bot)}</a>` : "the bot"} and its chat is linked.</div></div></div>
    <div id="code"></div>
    <div class="card" style="padding:0"><table class="t"><tr><th>Customer</th><th>Phone</th><th>Linked chats</th><th class="num">Orders</th><th></th></tr>
    ${d.items.map((c) => `<tr><td class="sent">${esc(c.name)}</td><td class="muted small">${esc(c.phone)}</td>
      <td class="small">${c.chats.length ? c.chats.map(esc).join("<br>") : `<span class="muted">—</span>`}</td><td class="num">${c.orders || "—"}</td>
      <td class="num"><button class="btn sm" data-c="${esc(c.id)}" data-n="${esc(c.name)}">${c.pair_code ? "Code " + esc(c.pair_code) : "Pairing code"}</button></td></tr>`).join("") ||
      `<tr><td colspan="5" class="empty">No customer yet: connect the ERP in Settings.</td></tr>`}</table></div>`;
  main.querySelectorAll("[data-c]").forEach((b) => (b.onclick = async () => {
    const r = await api(`/customers/${encodeURIComponent(b.dataset.c)}/pair-code`, { method: "POST" });
    $("#code").innerHTML = `<div class="card" style="margin-bottom:14px"><h5>Pairing code for ${esc(b.dataset.n)}</h5>
      <div class="row"><span class="codebox">${esc(r.code)}</span><div class="small">Send this code to <b>@${esc(r.bot || "the bot")}</b> on Telegram, or open
      <a target="_blank" href="https://t.me/${esc(r.bot)}?start=${esc(r.code)}">t.me/${esc(r.bot)}?start=${esc(r.code)}</a>. Valid 7 days, once.</div></div></div>`;
    window.scrollTo(0, 0);
  }));
});

// ---- settings ----------------------------------------------------------------------------
route("settings", async () => {
  const [ov, st] = await Promise.all([api("/overview"), api("/settings")]);
  settingsCache = st;
  const e = ov.erp || {};
  const field = (k, label, help) => `<div class="row" style="padding:8px 0;border-bottom:1px solid #f1f5f9"><div style="flex:1"><div>${label}</div><div class="tiny muted">${help}</div></div>
    <input data-k="${k}" value="${esc(st[k])}" style="width:150px"></div>`;
  main.innerHTML = `
    <div class="ttl"><div><h1>Settings</h1><div class="s">Every number that governs the agent's behaviour, in plain words.</div></div><button class="btn p" id="save">Save</button></div>
    <div class="grid g2">
      <div class="card"><h5>ERP connection</h5>
        <div class="${e.connected ? "okb" : "warn"}">${e.connected ? `Connected to <b>${esc(e.name)}</b> at ${esc(e.url)} as ${esc(e.user)} · ${ov.products} products · synced ${short(e.synced_at)}`
          : `Not connected${e.error ? ": " + esc(e.error) : ""}. Order Desk talks to the ERP only through its API, with a user session.`}</div>
        <label class="f">ERP address</label><input id="eu" value="${esc(e.url || "http://127.0.0.1:8543")}" style="width:100%">
        <div class="grid g2"><div><label class="f">Username</label><input id="en" style="width:100%" autocomplete="off"></div>
        <div><label class="f">Password</label><input id="ep" type="password" style="width:100%" autocomplete="new-password"></div></div>
        <div class="tiny muted" style="margin-top:6px">The password is used once to open a session; only the session token is kept.</div>
        <div class="row" style="margin-top:10px"><span class="sp"></span>${e.connected ? `<button class="btn" id="es">Sync now</button>` : ""}<button class="btn p" id="ec">Connect</button></div>
      </div>
      <div class="card"><h5>Channels</h5>
        <div class="row" style="padding:8px 0"><span class="dot" style="width:9px;height:9px;border-radius:50%;background:${ov.telegram ? "#22c55e" : "#f87171"}"></span>
          <div><b>Telegram</b> ${ov.telegram ? `· <a target="_blank" href="https://t.me/${esc(ov.telegram)}">@${esc(ov.telegram)}</a>` : "· set TELEGRAM_BOT_TOKEN"}</div></div>
        <div class="row" style="padding:8px 0"><span class="dot" style="width:9px;height:9px;border-radius:50%;background:#cbd5e1"></span><div><b>WhatsApp</b> · same connector interface, next</div></div>
        <div class="row" style="padding:8px 0"><span class="dot" style="width:9px;height:9px;border-radius:50%;background:${ov.meta ? "#22c55e" : "#f87171"}"></span>
          <div><b>Meta models</b> · Voice Transcribe, Muse Spark 1.3, Muse Wonder ${ov.meta ? "" : "· set META_MODEL_API_KEY"}</div></div>
      </div>
      <div class="card"><h5>Matching policy</h5>
        ${field("alias_trust_after", "A wording becomes trusted after", "independent confirmations; before that it is only a hint")}
        ${field("shadow_agreements", "Shadow: agreements before a rule goes live", "0 = straight to live after the gate")}
        ${field("rollback_strikes", "Roll a live rule back after", "corrections by people")}
        ${field("gate_min_gain", "Gate: minimum new lines a rule must resolve", "and always 0 wrong, 0 conflicts")}
      </div>
      <div class="card"><h5>Rule engineer &amp; budget</h5>
        ${field("engineer_auto", "Run automatically (1 = yes)", "after confirmed orders, when enough residue has accumulated")}
        ${field("engineer_every", "Run after every", "new residue lines (lines that needed the AI or a person)")}
        ${field("engineer_model", "Model", "muse-wonder-1.0 (tool use); muse-spark-1.3 is cheaper")}
        ${field("engineer_rounds", "Max tool rounds per run", "caps the cost of a run")}
        ${field("ai_budget_day", "AI budget per day (USD)", "when reached, rules and aliases keep working; new lines wait for a person")}
      </div>
    </div>`;
  $("#save").onclick = async () => {
    const body = {}; main.querySelectorAll("[data-k]").forEach((i) => (body[i.dataset.k] = i.value));
    try { await api("/settings", { method: "PUT", body }); toast("Settings saved"); } catch (er) { toast(er.message, true); }
  };
  $("#ec").onclick = async () => {
    const b = $("#ec"); b.disabled = true; b.textContent = "Connecting…";
    try { await api("/erp/connect", { method: "POST", body: { url: $("#eu").value.trim(), username: $("#en").value.trim(), password: $("#ep").value } }); toast("ERP connected and synced"); go(); }
    catch (er) { toast(er.message, true); b.disabled = false; b.textContent = "Connect"; }
  };
  const es = $("#es");
  if (es) es.onclick = async () => { try { const r = await api("/erp/sync", { method: "POST" }); toast(r.products + " products synced"); go(); } catch (er) { toast(er.message, true); } };
});

api("/settings").then((s) => (settingsCache = s)).catch(() => {});
go();
