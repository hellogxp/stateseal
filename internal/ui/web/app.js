const app = document.querySelector("#app");
const connection = document.querySelector("#connection");
const refreshButton = document.querySelector("#refresh");
const languagePicker = document.querySelector("#language");

const supportedLocales = ["en", "zh-CN", "ja", "ko", "es", "pt-BR", "de", "fr"];

const state = {
  runs: [],
  filter: "all",
  query: "",
  selectedNode: null,
  locale: "en",
  messages: {},
  fallbackMessages: {},
};

const normalizeLocale = (value) => {
  const locale = String(value || "").replace("_", "-").toLowerCase();
  if (locale === "zh" || locale.startsWith("zh-cn") || locale.startsWith("zh-sg")) return "zh-CN";
  if (locale.startsWith("pt")) return "pt-BR";
  return supportedLocales.find(item => locale === item.toLowerCase() || locale.startsWith(`${item.toLowerCase()}-`)) || "en";
};

const interpolate = (value, variables = {}) => String(value).replace(/\{(\w+)\}/g, (_, key) => variables[key] ?? `{${key}}`);

const t = (key, variables = {}, fallback = key) => interpolate(
  state.messages[key] ?? state.fallbackMessages[key] ?? fallback,
  variables
);

async function setLocale(locale, persist = false) {
  const next = normalizeLocale(locale);
  if (!Object.keys(state.fallbackMessages).length) {
    state.fallbackMessages = await fetchJSON("/locales/en.json");
  }
  state.messages = next === "en" ? state.fallbackMessages : await fetchJSON(`/locales/${next}.json`);
  state.locale = next;
  document.documentElement.lang = next;
  languagePicker.value = next;
  languagePicker.setAttribute("aria-label", t("common.language", {}, "Language"));
  const loadingCopy = document.querySelector("#loading-copy");
  if (loadingCopy) loadingCopy.textContent = t("common.loading");
  if (persist) localStorage.setItem("stateseal.locale", next);
}

async function initLocale() {
  const saved = localStorage.getItem("stateseal.locale");
  const preferred = saved || navigator.languages?.find(locale => normalizeLocale(locale) !== "en") || navigator.language || "en";
  await setLocale(preferred);
}

const statusColor = (status) => {
  const value = String(status || "").toUpperCase();
  if (["ADMITTED", "APPLIED", "PASSED", "VERIFIED"].includes(value)) return "var(--green)";
  if (["REJECTED", "FAILED", "ESCALATED"].includes(value)) return "var(--red)";
  if (["ABSTAINED", "STALE", "WARNING"].includes(value)) return "var(--amber)";
  return "var(--blue)";
};

const graphColor = (status) => ({
  passed: "var(--green)", failed: "var(--red)", warning: "var(--amber)", running: "var(--blue)"
}[status] || "var(--muted)");

const esc = (value) => String(value ?? "").replace(/[&<>"']/g, (char) => ({
  "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;"
}[char]));

const formatTime = (value) => {
  if (!value || String(value).startsWith("0001-")) return "—";
  const date = new Date(value);
  const delta = Date.now() - date.getTime();
  if (delta >= 0 && delta < 60_000) return t("time.justNow");
  const relative = new Intl.RelativeTimeFormat(state.locale, { numeric: "auto", style: "narrow" });
  if (delta >= 0 && delta < 3_600_000) return relative.format(-Math.floor(delta / 60_000), "minute");
  if (delta >= 0 && delta < 86_400_000) return relative.format(-Math.floor(delta / 3_600_000), "hour");
  return date.toLocaleDateString(state.locale, { month: "short", day: "numeric" });
};

const clock = (value) => {
  if (!value || String(value).startsWith("0001-")) return "—";
  return new Date(value).toLocaleTimeString(state.locale, { hour: "2-digit", minute: "2-digit", second: "2-digit" });
};

const duration = (start, end) => {
  if (!start || String(start).startsWith("0001-")) return "—";
  const milliseconds = Math.max(0, new Date(end || Date.now()).getTime() - new Date(start).getTime());
  if (milliseconds < 1000) return `${milliseconds}ms`;
  const seconds = Math.floor(milliseconds / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${seconds % 60}s`;
};

const short = (value, size = 12) => {
  value = String(value || "");
  return value.length <= size ? value || "—" : `${value.slice(0, 8)}…${value.slice(-4)}`;
};

const humanize = (value) => String(value || "").replaceAll("_", " ").replace(/\b\w/g, c => c.toUpperCase());

async function fetchJSON(path) {
  const response = await fetch(path, { headers: { Accept: "application/json" } });
  if (!response.ok) throw new Error((await response.json().catch(() => ({}))).error || `HTTP ${response.status}`);
  return response.json();
}

async function loadRuns() {
  try {
    state.runs = await fetchJSON("/api/runs");
    connection.classList.remove("offline");
    connection.lastChild.textContent = ` ${t("connection.live")}`;
    route();
  } catch (error) {
    connection.classList.add("offline");
    connection.lastChild.textContent = ` ${t("connection.offline")}`;
    showError(error);
  }
}

function showError(error) {
  app.innerHTML = `<section class="panel error-state"><p class="eyebrow">${esc(t("error.eyebrow"))}</p><h2>${esc(t("error.title"))}</h2><p>${esc(error.message)}</p></section>`;
}

function summaryMetrics(runs) {
  const admitted = runs.filter(run => ["ADMITTED", "APPLIED"].includes(run.status)).length;
  const active = runs.filter(run => ["WORKING", "VERIFYING", "VERIFIED"].includes(run.status)).length;
  const blocked = runs.filter(run => ["REJECTED", "ABSTAINED", "STALE", "ESCALATED"].includes(run.status)).length;
  return { admitted, active, blocked };
}

function renderRuns() {
  const runs = state.runs;
  if (!runs.length) {
    const empty = document.querySelector("#empty-template").content.cloneNode(true);
    empty.querySelectorAll("[data-i18n]").forEach(element => {
      element.textContent = t(element.dataset.i18n);
    });
    app.replaceChildren(empty);
    return;
  }
  const metrics = summaryMetrics(runs);
  app.innerHTML = `
    <section class="hero">
      <div>
        <p class="eyebrow">${esc(t("runs.eyebrow"))}</p>
        <h1>${esc(t("runs.title"))}</h1>
        <p class="hero-copy">${esc(t("runs.description"))}</p>
      </div>
      <div class="metrics">
        <div class="metric"><strong>${runs.length}</strong><span>${esc(t("runs.total"))}</span></div>
        <div class="metric"><strong>${metrics.active}</strong><span>${esc(t("status.active"))}</span></div>
        <div class="metric"><strong>${metrics.admitted}</strong><span>${esc(t("status.admitted"))}</span></div>
        <div class="metric"><strong>${metrics.blocked}</strong><span>${esc(t("status.notAdmitted"))}</span></div>
      </div>
    </section>
    <section class="toolbar">
      <label class="search">
        <svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/></svg>
        <input id="run-search" type="search" placeholder="${esc(t("runs.search"))}" value="${esc(state.query)}">
      </label>
      <div class="filters">
        ${["all", "active", "admitted", "notAdmitted"].map(filter =>
          `<button class="filter ${state.filter === filter ? "active" : ""}" data-filter="${filter}">${esc(t(`filter.${filter}`))}</button>`
        ).join("")}
      </div>
    </section>
    <section id="run-list" class="run-list panel"></section>`;
  document.querySelector("#run-search").addEventListener("input", event => {
    state.query = event.target.value;
    renderRunRows();
  });
  document.querySelectorAll(".filter").forEach(button => button.addEventListener("click", () => {
    state.filter = button.dataset.filter;
    document.querySelectorAll(".filter").forEach(item => item.classList.toggle("active", item === button));
    renderRunRows();
  }));
  renderRunRows();
}

function filterRuns() {
  const query = state.query.trim().toLowerCase();
  return state.runs.filter(run => {
    const haystack = `${run.goal} ${run.task_id} ${run.repository} ${run.status} ${run.rule_id}`.toLowerCase();
    if (query && !haystack.includes(query)) return false;
    if (state.filter === "active") return ["WORKING", "VERIFYING", "VERIFIED"].includes(run.status);
    if (state.filter === "admitted") return ["ADMITTED", "APPLIED"].includes(run.status);
    if (state.filter === "notAdmitted") return ["REJECTED", "ABSTAINED", "STALE", "ESCALATED"].includes(run.status);
    return true;
  });
}

function renderRunRows() {
  const target = document.querySelector("#run-list");
  if (!target) return;
  const runs = filterRuns();
  if (!runs.length) {
    target.innerHTML = `<div class="error-state"><p>${esc(t("runs.noMatch"))}</p></div>`;
    return;
  }
  target.innerHTML = runs.map((run, index) => {
    const complete = ["ADMITTED", "APPLIED", "REJECTED", "ABSTAINED", "STALE", "ESCALATED"].includes(run.status);
    const phase = run.applied ? 4 : complete ? 3 : run.checkpoints_verified ? 2 : run.candidates_evaluated ? 1 : 0;
    const color = statusColor(run.status);
    return `
      <article class="run-row" data-run="${esc(run.id)}" tabindex="0" style="--status-color:${color};animation-delay:${Math.min(index * 35, 280)}ms">
        <div class="run-name">
          <span class="run-meta">${esc(run.task_id)}</span>
          <h2>${esc(run.goal)}</h2>
          <span class="run-repo">${esc(run.repository)}</span>
          ${run.integrity_error ? `<div class="integrity-note">${esc(t("integrity.warning"))}</div>` : ""}
        </div>
        <div class="run-status">
          <span class="status-pill ${["WORKING","VERIFYING","VERIFIED"].includes(run.status) ? "running" : ""}"><i></i>${esc(run.status)}</span>
          <div class="progress-rail" title="${esc(t("runs.progress"))}">
            ${[0,1,2,3,4].map((step, i) => `${i ? "<span></span>" : ""}<i class="${step <= phase ? "done" : ""}"></i>`).join("")}
          </div>
        </div>
        <div class="run-stats">
          <div><strong>${run.candidates_evaluated}</strong><span>${esc(t("common.candidates"))}</span></div>
          <div><strong>${run.evidence_count}</strong><span>${esc(t("common.evidence"))}</span></div>
          <div><strong>${run.checkpoints_verified}</strong><span>${esc(t("common.checkpoints"))}</span></div>
        </div>
        <time class="run-time">${formatTime(run.updated_at)}</time>
        <svg class="row-arrow" viewBox="0 0 24 24"><path d="m9 18 6-6-6-6"/></svg>
      </article>`;
  }).join("");
  target.querySelectorAll(".run-row").forEach(row => {
    const open = () => { location.hash = `#/runs/${encodeURIComponent(row.dataset.run)}`; };
    row.addEventListener("click", open);
    row.addEventListener("keydown", event => { if (event.key === "Enter" || event.key === " ") open(); });
  });
}

async function renderDetail(id) {
  app.innerHTML = `<section class="loading-state"><span class="loader"></span><p>${esc(t("detail.projecting"))}</p></section>`;
  try {
    const snapshot = await fetchJSON(`/api/runs/${encodeURIComponent(id)}`);
    const run = snapshot.summary;
    const color = statusColor(run.status);
    app.innerHTML = `
      <a class="back-link" href="#/"><svg viewBox="0 0 24 24"><path d="m15 18-6-6 6-6"/></svg>${esc(t("detail.allRuns"))}</a>
      <section class="detail-head">
        <div class="detail-title">
          <p class="eyebrow">${esc(run.task_id)}</p>
          <h1>${esc(run.goal)}</h1>
          <div class="subline"><span>${esc(run.repository)}</span><span>${esc(run.repository_root)}</span><span>${esc(t("detail.updated", {time: formatTime(run.updated_at)}))}</span></div>
        </div>
        <div class="detail-badge" style="--status-color:${color}">
          <span class="status-pill ${["WORKING","VERIFYING","VERIFIED"].includes(run.status) ? "running" : ""}"><i></i>${esc(run.status)}</span>
        </div>
      </section>
      <section class="overview-grid">
        ${overviewCard(t("detail.disposition"), run.disposition || t("detail.pending"), run.mode ? t("detail.mode", {mode: run.mode}) : t("detail.inProgress"))}
        ${overviewCard(t("detail.duration"), duration(run.started_at, isTerminal(run.status) ? run.updated_at : null), run.started_at ? t("detail.started", {time: clock(run.started_at)}) : t("detail.awaitingEvent"))}
        ${overviewCard(t("common.candidates"), run.candidates_evaluated, t("detail.candidateStats", {rejected: run.candidates_rejected, checkpoints: run.checkpoints_verified}))}
        ${overviewCard(t("common.evidence"), run.evidence_count, run.recovered ? t("detail.recovered") : t("detail.stateBound"))}
      </section>
      ${run.integrity_error ? `<section class="panel reason" style="margin:12px 0 0;border-color:rgba(251,113,133,.35);color:var(--red)">${esc(t("integrity.detail", {error: run.integrity_error}))}</section>` : ""}
      <section class="panel section">
        <div class="section-head">
          <div><h2>${esc(t("dag.title"))}</h2><p>${esc(t("dag.help"))}</p></div>
          <div class="legend">
            <span style="--legend:var(--blue)"><i></i>${esc(t("status.active"))}</span>
            <span style="--legend:var(--green)"><i></i>${esc(t("status.passed"))}</span>
            <span style="--legend:var(--amber)"><i></i>${esc(t("status.recovered"))}</span>
            <span style="--legend:var(--red)"><i></i>${esc(t("status.blocked"))}</span>
          </div>
        </div>
        <div class="dag-shell">
          <div class="dag-scroll"><svg id="dag" class="dag" role="img" aria-label="${esc(t("dag.aria"))}"></svg></div>
          <aside id="inspector" class="inspector"></aside>
        </div>
      </section>
      <section class="detail-columns">
        <div class="panel content-panel">
          <div class="section-head"><div><h2>${esc(t("timeline.title"))}</h2><p>${esc(t("timeline.help"))}</p></div><span class="tiny-label">${snapshot.events.length} ${esc(t("timeline.events"))}</span></div>
          <div class="timeline">${renderEvents(snapshot.events)}</div>
        </div>
        <div>
          <section class="panel content-panel">
            <div class="section-head"><div><h2>${esc(t("evidence.title"))}</h2><p>${esc(t("evidence.help"))}</p></div><span class="tiny-label">${snapshot.evidence.length} ${esc(t("evidence.records"))}</span></div>
            <div class="evidence-list">${renderEvidence(snapshot.evidence)}</div>
          </section>
          ${renderReceipt(snapshot.receipt)}
        </div>
      </section>`;
    renderDAG(snapshot.nodes, snapshot.edges);
    bindEvidence();
  } catch (error) {
    showError(error);
  }
}

function overviewCard(label, value, helper) {
  return `<article class="panel overview-card"><span class="label">${esc(label)}</span><strong>${esc(value)}</strong><p>${esc(helper)}</p></article>`;
}

function isTerminal(status) {
  return ["ADMITTED", "APPLIED", "REJECTED", "ABSTAINED", "STALE", "ESCALATED"].includes(status);
}

function renderEvents(events) {
  if (!events.length) return `<div class="error-state"><p>${esc(t("timeline.empty"))}</p></div>`;
  return events.map(event => {
    const eventStatus = event.type.includes("REJECT") || event.type.includes("REGRESSION") ? "failed"
      : event.type.includes("ABSTAIN") || event.type.includes("STALE") || event.type.includes("SELECTED") ? "warning"
      : event.type.includes("VERIFIED") || event.type.includes("ADMITTED") || event.type.includes("RECERTIFIED") || event.type.includes("APPLIED") ? "passed" : "running";
    const detail = Object.entries(event.data || {}).slice(0, 2).map(([key, value]) => `${humanize(key)}: ${value}`).join(" · ");
    return `<div class="event" style="--event-color:${graphColor(eventStatus)}">
      <time>${clock(event.timestamp)}</time><i class="event-dot"></i>
      <div class="event-main"><strong>${esc(humanize(event.type))}</strong><span>#${event.sequence}${detail ? ` · ${esc(detail)}` : ""}</span></div>
    </div>`;
  }).join("");
}

function evidenceStatus(item) {
  if (item.timed_out) return { label: t("evidence.timedOut"), value: "warning" };
  if (item.exit_code !== 0) return { label: `${t("evidence.failed")} · ${item.exit_code}`, value: "failed" };
  return { label: t("status.passed"), value: "passed" };
}

function renderEvidence(evidence) {
  if (!evidence.length) return `<div class="error-state"><p>${esc(t("evidence.empty"))}</p></div>`;
  return evidence.map((item, index) => {
    const status = evidenceStatus(item);
    const name = String(item.verifier_identity || t("evidence.verifier")).replace(/^command\//, "").replace(/@v1$/, "");
    return `<article class="evidence-item">
      <button class="evidence-head" data-evidence="${index}">
        <span class="evidence-name"><strong>${esc(name)}</strong><span>${esc(humanize(item.verification_phase || "verification"))} · ${duration(item.started_at, item.finished_at)}</span></span>
        <span class="evidence-result" style="--evidence-color:${graphColor(status.value)}">${status.label}</span>
      </button>
      <pre class="evidence-output">${esc(item.output || t("evidence.noOutput"))}</pre>
    </article>`;
  }).join("");
}

function bindEvidence() {
  document.querySelectorAll(".evidence-head").forEach(button => button.addEventListener("click", () => {
    button.closest(".evidence-item").classList.toggle("open");
  }));
}

function renderReceipt(receipt) {
  if (!receipt) return "";
  return `<section class="panel content-panel section">
    <div class="section-head"><div><h2>${esc(t("receipt.title"))}</h2><p>${esc(t("receipt.help"))}</p></div></div>
    <div class="receipt">
      <div class="receipt-grid">
        <div><span>${esc(t("receipt.verdict"))}</span><strong>${esc(receipt.verdict)}</strong></div>
        <div><span>${esc(t("detail.disposition"))}</span><strong>${esc(receipt.disposition)}</strong></div>
        <div><span>${esc(t("receipt.rule"))}</span><strong>${esc(receipt.rule_id || "—")}</strong></div>
        <div><span>${esc(t("receipt.checkpoint"))}</span><strong>${esc(short(receipt.checkpoint_id))}</strong></div>
        <div><span>${esc(t("receipt.id"))}</span><strong>${esc(short(receipt.receipt_id))}</strong></div>
        <div><span>${esc(t("receipt.digest"))}</span><strong>${esc(short(receipt.receipt_digest))}</strong></div>
      </div>
      ${receipt.reason ? `<p class="reason">${esc(receipt.reason)}</p>` : ""}
    </div>
  </section>`;
}

function topologicalRanks(nodes, edges) {
  const ranks = new Map(nodes.map(node => [node.id, 0]));
  const indegree = new Map(nodes.map(node => [node.id, 0]));
  const outgoing = new Map(nodes.map(node => [node.id, []]));
  edges.forEach(edge => {
    if (!indegree.has(edge.source) || !indegree.has(edge.target)) return;
    indegree.set(edge.target, indegree.get(edge.target) + 1);
    outgoing.get(edge.source).push(edge.target);
  });
  const queue = nodes.filter(node => indegree.get(node.id) === 0).map(node => node.id);
  let processed = 0;
  while (queue.length) {
    const source = queue.shift();
    processed++;
    outgoing.get(source).forEach(target => {
      ranks.set(target, Math.max(ranks.get(target), ranks.get(source) + 1));
      indegree.set(target, indegree.get(target) - 1);
      if (indegree.get(target) === 0) queue.push(target);
    });
  }
  if (processed !== nodes.length) {
    // The protocol should always produce a DAG. Keep a deterministic fallback
    // for corrupt or future event projections rather than breaking the view.
    nodes.forEach((node, index) => ranks.set(node.id, index));
  }
  return ranks;
}

function truncateVisual(value, maxUnits) {
  const characters = Array.from(String(value || ""));
  let units = 0;
  let output = "";
  for (const character of characters) {
    const weight = /[\u2E80-\u9FFF\uAC00-\uD7AF\u3040-\u30FF]/.test(character) ? 1.75 : 1;
    if (units + weight > maxUnits) return `${output.trimEnd()}…`;
    output += character;
    units += weight;
  }
  return output;
}

function nodeDisplayLabel(node) {
  if (node.kind === "verifier" || node.kind === "receipt") return node.label;
  return t(`node.${node.kind}`, {}, node.label);
}

function renderDAG(nodes, edges) {
  const svg = document.querySelector("#dag");
  if (!svg) return;
  const width = 190, height = 68, xGap = 42, yGap = 24, pad = 34;
  const ranks = topologicalRanks(nodes, edges);
  const columns = new Map();
  nodes.forEach(node => {
    const rank = ranks.get(node.id) || 0;
    if (!columns.has(rank)) columns.set(rank, []);
    columns.get(rank).push(node);
  });
  const positions = new Map();
  let maxRank = 0, maxRows = 1;
  for (const [rank, items] of columns) {
    maxRank = Math.max(maxRank, rank);
    maxRows = Math.max(maxRows, items.length);
  }
  const canvasHeight = Math.max(300, pad * 2 + maxRows * height + (maxRows - 1) * yGap);
  const naturalWidth = pad * 2 + (maxRank + 1) * width + maxRank * xGap;
  const viewportWidth = svg.parentElement?.clientWidth || 0;
  const canvasWidth = Math.max(760, viewportWidth, naturalWidth);
  const xOffset = Math.max(0, (canvasWidth - naturalWidth) / 2);
  for (const [rank, items] of columns) {
    const contentHeight = items.length * height + (items.length - 1) * yGap;
    const startY = Math.max(pad, (canvasHeight - contentHeight) / 2);
    items.forEach((node, index) => positions.set(node.id, {
      x: xOffset + pad + rank * (width + xGap), y: startY + index * (height + yGap)
    }));
  }
  svg.setAttribute("viewBox", `0 0 ${canvasWidth} ${canvasHeight}`);
  svg.setAttribute("width", canvasWidth);
  svg.setAttribute("height", canvasHeight);

  const edgeMarkup = edges.map(edge => {
    const source = positions.get(edge.source), target = positions.get(edge.target);
    if (!source || !target) return "";
    const x1 = source.x + width, y1 = source.y + height / 2;
    const x2 = target.x, y2 = target.y + height / 2;
    const bend = Math.max(28, (x2 - x1) * .46);
    const path = `M${x1} ${y1} C${x1 + bend} ${y1},${x2 - bend} ${y2},${x2} ${y2}`;
    return `<path class="dag-edge ${esc(edge.status)}" d="${path}" marker-end="url(#dag-arrow)"/>`;
  }).join("");
  const nodeMarkup = nodes.map((node, index) => {
    const point = positions.get(node.id);
    const color = graphColor(node.status);
    const time = node.timestamp && !String(node.timestamp).startsWith("0001-") ? clock(node.timestamp) : "";
    const label = nodeDisplayLabel(node);
    const subtitle = node.subtitle || t(`node.${node.kind}`, {}, humanize(node.kind));
    return `<g class="dag-node ${esc(node.status)}" data-node="${esc(node.id)}" tabindex="0" transform="translate(${point.x} ${point.y})" style="--node-color:${color}">
      <title>${esc(`${label}${subtitle ? ` — ${subtitle}` : ""}`)}</title>
      <rect width="${width}" height="${height}" rx="9"/>
      <rect class="node-accent" width="2" height="${height - 20}" y="10" rx="1"/>
      <rect class="node-icon-bg" x="13" y="15" width="30" height="30" rx="8"/>
      ${nodeIcon(node.kind)}
      <clipPath id="node-copy-${index}"><rect x="51" y="10" width="${width - 61}" height="39"/></clipPath>
      <g clip-path="url(#node-copy-${index})">
        <text class="node-label" x="53" y="26">${esc(truncateVisual(label, 20))}</text>
        <text class="node-subtitle" x="53" y="42">${esc(truncateVisual(subtitle, 27))}</text>
      </g>
      <text class="node-time" x="${width - 11}" y="${height - 10}" text-anchor="end">${esc(time)}</text>
    </g>`;
  }).join("");
  svg.innerHTML = `<defs><marker id="dag-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="5" markerHeight="5" orient="auto"><path class="dag-arrow" d="M0 0 8 4 0 8z"/></marker></defs><g>${edgeMarkup}${nodeMarkup}</g>`;
  svg.querySelectorAll(".dag-node").forEach(element => {
    const open = () => openInspector(nodes.find(node => node.id === element.dataset.node));
    element.addEventListener("click", open);
    element.addEventListener("keydown", event => { if (event.key === "Enter" || event.key === " ") open(); });
  });
}

function nodeIcon(kind) {
  const icons = {
    goal: `<path d="M25 24h6M28 21v6"/><circle cx="28" cy="30" r="8"/>`,
    candidate: `<path d="m22 31 4 4 8-10M22 22h12v16H22z"/>`,
    verifier: `<path d="m21 29 5 5 9-11M28 19l8 4v6c0 6-3 9-8 11-5-2-8-5-8-11v-6z"/>`,
    checkpoint: `<path d="M28 20 36 24.5V33L28 38l-8-5v-8.5zM24 28l3 3 5-6"/>`,
    regression: `<path d="M21 35 35 21M22 22l13 13"/>`,
    rejection: `<path d="M21 35 35 21M22 22l13 13"/>`,
    selection: `<path d="M21 25h14M21 31h9M21 37h5"/>`,
    restore: `<path d="M22 25a9 9 0 1 1-1 9M21 20v6h6"/>`,
    recertification: `<circle cx="28" cy="29" r="9"/><path d="m24 29 3 3 5-6"/>`,
    receipt: `<path d="M22 20h12v18l-3-2-3 2-3-2-3 2zM25 25h6M25 29h6"/>`,
    apply: `<path d="M20 29h16M30 23l6 6-6 6M20 22v14"/>`,
    guard: `<path d="M28 19l8 4v6c0 6-3 9-8 11-5-2-8-5-8-11v-6z"/>`,
    integrity: `<path d="M28 20 37 37H19zM28 26v5M28 34v1"/>`
  };
  return `<g class="node-icon">${icons[kind] || icons.goal}</g>`;
}

function openInspector(node) {
  if (!node) return;
  const inspector = document.querySelector("#inspector");
  document.querySelectorAll(".dag-node").forEach(element => element.classList.toggle("selected", element.dataset.node === node.id));
  const details = Object.entries(node.details || {}).filter(([, value]) => value !== "" && value != null);
  inspector.style.setProperty("--node-color", graphColor(node.status));
  inspector.innerHTML = `
    <button class="inspector-close" aria-label="${esc(t("inspector.close"))}">×</button>
    <span class="kind">${esc(t(`node.${node.kind}`, {}, humanize(node.kind)))}</span>
    <h3>${esc(nodeDisplayLabel(node))}</h3>
    <p class="subtitle">${esc(node.subtitle || "")}</p>
    <dl class="kv">
      <dt>${esc(t("inspector.status"))}</dt><dd>${esc(humanize(node.status))}</dd>
      ${node.timestamp && !String(node.timestamp).startsWith("0001-") ? `<dt>${esc(t("inspector.timestamp"))}</dt><dd>${esc(new Date(node.timestamp).toLocaleString(state.locale))}</dd>` : ""}
      ${details.map(([key, value]) => `<dt>${esc(humanize(key))}</dt><dd>${esc(typeof value === "object" ? JSON.stringify(value) : value)}</dd>`).join("")}
    </dl>`;
  inspector.classList.add("open");
  inspector.querySelector(".inspector-close").addEventListener("click", () => {
    inspector.classList.remove("open");
    document.querySelectorAll(".dag-node").forEach(element => element.classList.remove("selected"));
  });
}

function route() {
  const match = location.hash.match(/^#\/runs\/([^/]+)$/);
  if (match) renderDetail(decodeURIComponent(match[1]));
  else renderRuns();
}

function connectStream() {
  const stream = new EventSource("/api/stream");
  stream.addEventListener("runs", event => {
    const snapshots = JSON.parse(event.data);
    state.runs = snapshots.map(snapshot => snapshot.summary);
    connection.classList.remove("offline");
    connection.lastChild.textContent = ` ${t("connection.live")}`;
    const match = location.hash.match(/^#\/runs\/([^/]+)$/);
    if (!match) renderRuns();
  });
  stream.onerror = () => {
    connection.classList.add("offline");
    connection.lastChild.textContent = ` ${t("connection.reconnecting")}`;
  };
}

window.addEventListener("hashchange", route);
refreshButton.addEventListener("click", loadRuns);
languagePicker.addEventListener("change", async event => {
  await setLocale(event.target.value, true);
  connection.lastChild.textContent = ` ${t("connection.live")}`;
  route();
});
initLocale().then(loadRuns).then(connectStream);
