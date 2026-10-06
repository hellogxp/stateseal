const app = document.querySelector("#app");
const connection = document.querySelector("#connection");
const refreshButton = document.querySelector("#refresh");
const languagePicker = document.querySelector("#language");

const supportedLocales = ["en", "zh-CN", "ja", "ko", "es", "pt-BR", "de", "fr"];
const state = { runs: [], filter: "all", query: "", locale: "en", messages: {}, fallbackMessages: {} };

const normalizeLocale = value => {
  const locale = String(value || "").replace("_", "-").toLowerCase();
  if (locale === "zh" || locale.startsWith("zh-cn") || locale.startsWith("zh-sg")) return "zh-CN";
  if (locale.startsWith("pt")) return "pt-BR";
  return supportedLocales.find(item => locale === item.toLowerCase() || locale.startsWith(`${item.toLowerCase()}-`)) || "en";
};
const interpolate = (value, variables = {}) => String(value).replace(/\{(\w+)\}/g, (_, key) => variables[key] ?? `{${key}}`);
const t = (key, variables = {}, fallback = key) => interpolate(state.messages[key] ?? state.fallbackMessages[key] ?? fallback, variables);
const esc = value => String(value ?? "").replace(/[&<>"']/g, char => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;" })[char]);
const humanize = value => String(value || "").replaceAll("_", " ").replace(/\b\w/g, char => char.toUpperCase());
const short = (value, size = 18) => {
  const text = String(value || "");
  return text.length <= size ? text || "—" : `${text.slice(0, 10)}…${text.slice(-5)}`;
};

async function fetchJSON(path) {
  const response = await fetch(path, { headers: { Accept: "application/json" } });
  if (!response.ok) throw new Error((await response.json().catch(() => ({}))).error || `HTTP ${response.status}`);
  return response.json();
}

async function setLocale(locale, persist = false) {
  const next = normalizeLocale(locale);
  if (!Object.keys(state.fallbackMessages).length) state.fallbackMessages = await fetchJSON("/locales/en.json");
  state.messages = next === "en" ? state.fallbackMessages : await fetchJSON(`/locales/${next}.json`);
  state.locale = next;
  document.documentElement.lang = next;
  languagePicker.value = next;
  languagePicker.setAttribute("aria-label", t("common.language"));
  const loading = document.querySelector("#loading-copy");
  if (loading) loading.textContent = t("common.loading");
  if (persist) localStorage.setItem("stateseal.locale", next);
}

async function initLocale() {
  const saved = localStorage.getItem("stateseal.locale");
  const preferred = saved || navigator.languages?.find(locale => normalizeLocale(locale) !== "en") || navigator.language || "en";
  await setLocale(preferred);
}

const formatTime = value => {
  if (!value || String(value).startsWith("0001-")) return "—";
  const date = new Date(value);
  const delta = Date.now() - date.getTime();
  if (delta >= 0 && delta < 60_000) return t("time.justNow");
  const relative = new Intl.RelativeTimeFormat(state.locale, { numeric: "auto", style: "narrow" });
  if (delta >= 0 && delta < 3_600_000) return relative.format(-Math.floor(delta / 60_000), "minute");
  if (delta >= 0 && delta < 86_400_000) return relative.format(-Math.floor(delta / 3_600_000), "hour");
  return date.toLocaleDateString(state.locale, { month: "short", day: "numeric" });
};
const clock = value => !value || String(value).startsWith("0001-") ? "—" : new Date(value).toLocaleTimeString(state.locale, { hour: "2-digit", minute: "2-digit", second: "2-digit" });
const duration = (start, end) => {
  if (!start || String(start).startsWith("0001-")) return "—";
  const ms = Math.max(0, new Date(end || Date.now()).getTime() - new Date(start).getTime());
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${seconds % 60}s`;
};
const msDuration = value => value >= 60_000 ? `${Math.floor(value / 60_000)}m ${Math.floor((value % 60_000) / 1000)}s` : value >= 1000 ? `${(value / 1000).toFixed(1)}s` : `${value || 0}ms`;

const statusColor = status => ({
  OBSERVING: "var(--blue)", AGENT_ENDED: "var(--green)", NEEDS_ATTENTION: "var(--amber)", ARCHIVED: "var(--muted)"
})[String(status || "").toUpperCase()] || "var(--muted)";
const postureColor = posture => ({
  CURRENT: "var(--green)", FAILURE: "var(--red)", GAP: "var(--amber)", STALE: "var(--amber)", OBSERVING: "var(--blue)", HISTORICAL: "var(--muted)"
})[String(posture || "").toUpperCase()] || "var(--muted)";
const graphColor = value => ({ observed: "var(--green)", passed: "var(--green)", failed: "var(--red)", warning: "var(--amber)", running: "var(--blue)" })[value] || "var(--muted)";
const statusLabel = status => t(`status.${String(status || "unknown").toLowerCase()}`, {}, humanize(status));
const postureLabel = posture => t(`posture.${String(posture || "unknown").toLowerCase()}`, {}, humanize(posture));

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
  return {
    observing: runs.filter(run => run.status === "OBSERVING").length,
    current: runs.filter(run => run.evidence_posture === "CURRENT").length,
    attention: runs.filter(run => ["FAILURE", "GAP", "STALE"].includes(run.evidence_posture)).length,
  };
}

function renderRuns() {
  if (!state.runs.length) {
    const empty = document.querySelector("#empty-template").content.cloneNode(true);
    empty.querySelectorAll("[data-i18n]").forEach(element => { element.textContent = t(element.dataset.i18n); });
    app.replaceChildren(empty);
    return;
  }
  const metrics = summaryMetrics(state.runs);
  app.innerHTML = `
    <section class="hero">
      <div><p class="eyebrow">${esc(t("runs.eyebrow"))}</p><h1>${esc(t("runs.title"))}</h1><p class="hero-copy">${esc(t("runs.description"))}</p></div>
      <div class="metrics">
        <div class="metric"><strong>${state.runs.length}</strong><span>${esc(t("metrics.sessions"))}</span></div>
        <div class="metric"><strong>${metrics.observing}</strong><span>${esc(t("metrics.observing"))}</span></div>
        <div class="metric"><strong>${metrics.current}</strong><span>${esc(t("metrics.current"))}</span></div>
        <div class="metric"><strong>${metrics.attention}</strong><span>${esc(t("metrics.attention"))}</span></div>
      </div>
    </section>
    <section class="toolbar">
      <label class="search"><svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/></svg><input id="run-search" type="search" placeholder="${esc(t("runs.search"))}" value="${esc(state.query)}"></label>
      <div class="filters">${["all", "observing", "current", "attention"].map(filter => `<button class="filter ${state.filter === filter ? "active" : ""}" data-filter="${filter}">${esc(t(`filter.${filter}`))}</button>`).join("")}</div>
    </section>
    <section id="run-list" class="run-list panel"></section>`;
  document.querySelector("#run-search").addEventListener("input", event => { state.query = event.target.value; renderRunRows(); });
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
    const haystack = `${run.goal} ${run.task_id} ${run.repository} ${run.agent} ${run.status} ${run.evidence_posture}`.toLowerCase();
    if (query && !haystack.includes(query)) return false;
    if (state.filter === "observing") return run.status === "OBSERVING";
    if (state.filter === "current") return run.evidence_posture === "CURRENT";
    if (state.filter === "attention") return ["FAILURE", "GAP", "STALE"].includes(run.evidence_posture);
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
  target.innerHTML = runs.map((run, index) => `
    <article class="run-row delivery-row" data-run="${esc(run.id)}" tabindex="0" style="--status-color:${postureColor(run.evidence_posture)};animation-delay:${Math.min(index * 35, 280)}ms">
      <div class="run-name">
        <span class="run-meta">${esc(run.agent || "Agent")} · ${esc(run.task_id)}</span>
        <h2>${esc(run.goal)}</h2>
        <span class="run-repo">${esc(run.repository)}${run.branch ? ` · ${esc(run.branch)}` : ""}${run.current_state ? ` · ${esc(short(run.current_state))}` : ""}</span>
        ${run.integrity_error ? `<div class="integrity-note">${esc(t("integrity.warning"))}</div>` : ""}
      </div>
      <div class="run-status">
        <span class="status-pill ${run.status === "OBSERVING" ? "running" : ""}" style="--status-color:${statusColor(run.status)}"><i></i>${esc(statusLabel(run.status))}</span>
        <span class="posture-label" style="--posture-color:${postureColor(run.evidence_posture)}">${esc(postureLabel(run.evidence_posture))}</span>
      </div>
      <div class="run-stats delivery-stats">
        <div><strong>${run.changed_files || 0}</strong><span>${esc(t("common.files"))}</span></div>
        <div><strong>${run.checks_passed || 0}/${run.checks_total || 0}</strong><span>${esc(t("common.checks"))}</span></div>
        <div><strong>${run.findings_count || 0}</strong><span>${esc(t("common.findings"))}</span></div>
      </div>
      <time class="run-time">${formatTime(run.updated_at)}</time>
      <svg class="row-arrow" viewBox="0 0 24 24"><path d="m9 18 6-6-6-6"/></svg>
    </article>`).join("");
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
    const primary = snapshot.findings?.find(item => item.severity === "high") || snapshot.findings?.[0];
    app.innerHTML = `
      <a class="back-link" href="#/"><svg viewBox="0 0 24 24"><path d="m15 18-6-6 6-6"/></svg>${esc(t("detail.allRuns"))}</a>
      <section class="detail-head">
        <div class="detail-title"><p class="eyebrow">${esc(run.agent || "Agent")} · ${esc(run.task_id)}</p><h1>${esc(run.goal)}</h1><div class="subline"><span>${esc(run.repository)}</span>${run.branch ? `<span>${esc(run.branch)}</span>` : ""}<span>${esc(t("detail.updated", { time: formatTime(run.updated_at) }))}</span></div></div>
        <div class="detail-badges"><span class="status-pill ${run.status === "OBSERVING" ? "running" : ""}" style="--status-color:${statusColor(run.status)}"><i></i>${esc(statusLabel(run.status))}</span><span class="status-pill" style="--status-color:${postureColor(run.evidence_posture)}"><i></i>${esc(postureLabel(run.evidence_posture))}</span></div>
      </section>
      <section class="overview-grid">
        ${overviewCard(t("detail.currentState"), short(run.current_state || t("common.unknown"), 24), run.workspace_dirty ? t("detail.dirtyWorkspace") : t("detail.cleanOrUnknown"))}
        ${overviewCard(t("detail.codeDelta"), `${run.changed_files || 0} ${t("common.files")}`, `+${run.additions || 0} / −${run.deletions || 0}`)}
        ${overviewCard(t("detail.checks"), `${run.checks_passed || 0}/${run.checks_total || 0}`, t("detail.checkBreakdown", { failed: run.checks_failed || 0, stale: run.checks_stale || 0 }))}
        ${overviewCard(t("detail.duration"), duration(run.started_at, run.status === "OBSERVING" ? null : run.updated_at), t("detail.findingCount", { count: run.findings_count || 0 }))}
      </section>
      <section class="panel insight-banner ${primary ? `severity-${esc(primary.severity)}` : "severity-none"}">
        <div><span class="insight-grade">${esc(primary ? primary.evidence_grade : "Observed")}</span><h2>${esc(primary ? primary.title : t("insight.noCriticalTitle"))}</h2><p>${esc(primary ? primary.detail : t("insight.noCriticalBody"))}</p></div>
        <span class="read-only-mark">${esc(t("common.readOnly"))}</span>
      </section>
      <section class="panel section">
        <div class="section-head"><div><h2>${esc(t("graph.title"))}</h2><p>${esc(t("graph.help"))}</p></div><div class="legend"><span style="--legend:var(--green)"><i></i>${esc(t("grade.observed"))}</span><span style="--legend:var(--amber)"><i></i>${esc(t("grade.derived"))}</span><span style="--legend:var(--red)"><i></i>${esc(t("common.failure"))}</span></div></div>
        <div class="dag-shell"><div class="dag-scroll"><svg id="dag" class="dag" role="img" aria-label="${esc(t("graph.aria"))}"></svg></div><aside id="inspector" class="inspector"></aside></div>
      </section>
      <section class="detail-columns intelligence-columns">
        <section class="panel content-panel"><div class="section-head"><div><h2>${esc(t("findings.title"))}</h2><p>${esc(t("findings.help"))}</p></div><span class="tiny-label">${snapshot.findings?.length || 0}</span></div><div class="finding-list">${renderFindings(snapshot.findings || [])}</div></section>
        <section class="panel content-panel"><div class="section-head"><div><h2>${esc(t("changes.title"))}</h2><p>${esc(t("changes.help"))}</p></div><span class="tiny-label">${snapshot.changes?.length || 0}</span></div><div class="change-list">${renderChanges(snapshot.changes || [])}</div></section>
      </section>
      <section class="panel content-panel section"><div class="section-head"><div><h2>${esc(t("checks.title"))}</h2><p>${esc(t("checks.help"))}</p></div><span class="tiny-label">${snapshot.checks?.length || 0}</span></div><div class="check-list">${renderChecks(snapshot.checks || [])}</div></section>
      <section class="detail-columns">
        <section class="panel content-panel"><div class="section-head"><div><h2>${esc(t("artifacts.title"))}</h2><p>${esc(t("artifacts.help"))}</p></div><span class="tiny-label">${snapshot.artifacts?.length || 0}</span></div><div class="artifact-list">${renderArtifacts(snapshot.artifacts || [])}</div></section>
        <section class="panel content-panel"><div class="section-head"><div><h2>${esc(t("timeline.title"))}</h2><p>${esc(t("timeline.help"))}</p></div><span class="tiny-label">${snapshot.events?.length || 0}</span></div><div class="timeline">${renderEvents(snapshot.events || [])}</div></section>
      </section>`;
    renderDAG(snapshot.nodes || [], snapshot.edges || []);
    bindExpanders();
  } catch (error) {
    showError(error);
  }
}

function overviewCard(label, value, helper) {
  return `<article class="panel overview-card"><span class="label">${esc(label)}</span><strong>${esc(value)}</strong><p>${esc(helper)}</p></article>`;
}

function renderFindings(findings) {
  if (!findings.length) return emptyBlock(t("findings.empty"));
  return findings.map(item => `<article class="finding-item severity-${esc(item.severity)}"><div class="finding-top"><span class="finding-severity">${esc(t(`severity.${item.severity}`, {}, item.severity))}</span><span class="evidence-grade">${esc(t(`grade.${item.evidence_grade.toLowerCase()}`, {}, item.evidence_grade))}</span></div><h3>${esc(item.title)}</h3><p>${esc(item.detail)}</p><small>${esc(t("findings.basis"))}: ${esc(item.basis)}</small></article>`).join("");
}

function renderChanges(changes) {
  if (!changes.length) return emptyBlock(t("changes.empty"));
  const render = items => items.map(item => `<div class="change-item"><span class="change-op op-${esc(item.operation)}">${esc(t(`operation.${item.operation}`, {}, item.operation))}</span><div><strong>${esc(item.path)}</strong><small>S${item.state_revision} · ${esc(item.source)} · ${esc(t(`grade.${item.evidence_grade.toLowerCase()}`, {}, item.evidence_grade))}</small></div><time>${clock(item.timestamp)}</time></div>`).join("");
  const split = Math.max(0, changes.length - 30);
  return `${split ? historyFold(split, render(changes.slice(0, split))) : ""}${render(changes.slice(split))}`;
}

function renderChecks(checks) {
  if (!checks.length) return emptyBlock(t("checks.empty"));
  const render = (items, offset = 0) => items.map((item, index) => {
    const color = item.status === "FAILED" ? "var(--red)" : item.freshness === "STALE" ? "var(--amber)" : item.status === "PASSED" ? "var(--green)" : "var(--muted)";
    const id = index + offset;
    return `<article class="check-item"><button class="check-head expander" data-target="check-output-${id}"><span class="check-name"><strong>${esc(item.name)}</strong><span>${esc(item.category)} · S${item.state_revision} · ${msDuration(item.duration_ms)}</span></span><span class="check-tags"><i style="--check-color:${color}">${esc(t(`checkStatus.${item.status.toLowerCase()}`, {}, item.status))}</i><i style="--check-color:${item.freshness === "STALE" ? "var(--amber)" : "var(--green)"}">${esc(t(`freshness.${item.freshness.toLowerCase()}`, {}, item.freshness))}</i></span></button><div id="check-output-${id}" class="check-detail"><dl class="kv"><dt>${esc(t("checks.command"))}</dt><dd>${esc(item.command || "—")}</dd><dt>${esc(t("checks.directory"))}</dt><dd>${esc(item.working_directory || "—")}</dd><dt>${esc(t("checks.source"))}</dt><dd>${esc(item.source)} · ${esc(item.evidence_grade)}</dd></dl>${item.output ? `<pre>${esc(item.output)}</pre>` : ""}</div></article>`;
  }).join("");
  const split = Math.max(0, checks.length - 12);
  return `${split ? historyFold(split, render(checks.slice(0, split))) : ""}${render(checks.slice(split), split)}`;
}

function renderArtifacts(artifacts) {
  if (!artifacts.length) return emptyBlock(t("artifacts.empty"));
  const render = items => items.map(item => `<div class="artifact-item"><span class="artifact-icon">${esc(item.kind.slice(0, 1).toUpperCase())}</span><div><strong>${esc(item.path)}</strong><small>${esc(item.kind)} · ${esc(item.source)} · ${esc(item.evidence_grade)}</small></div></div>`).join("");
  const split = Math.max(0, artifacts.length - 16);
  return `${split ? historyFold(split, render(artifacts.slice(0, split))) : ""}${render(artifacts.slice(split))}`;
}

function renderEvents(events) {
  if (!events.length) return emptyBlock(t("timeline.empty"));
  const render = items => items.map(event => {
    const failed = String(event.data?.result || "").includes("failure");
    const grade = event.data?.evidence_grade || "Observed";
    const details = Object.entries(event.data || {}).filter(([key]) => key !== "evidence_grade").slice(0, 3).map(([key, value]) => `${humanize(key)}: ${typeof value === "object" ? JSON.stringify(value) : value}`).join(" · ");
    return `<div class="event" style="--event-color:${failed ? "var(--red)" : grade === "Derived" ? "var(--amber)" : "var(--green)"}"><time>${clock(event.timestamp)}</time><i class="event-dot"></i><div class="event-main"><strong>${esc(t(`event.${event.type.toLowerCase()}`, {}, humanize(event.type)))}</strong><span>${esc(grade)}${details ? ` · ${esc(details)}` : ""}</span></div></div>`;
  }).join("");
  const split = Math.max(0, events.length - 30);
  return `${split ? historyFold(split, render(events.slice(0, split))) : ""}${render(events.slice(split))}`;
}

function historyFold(count, content) {
  return `<details class="history-fold"><summary>${esc(t("common.showEarlier", { count }))}</summary><div>${content}</div></details>`;
}

function emptyBlock(message) {
  return `<div class="section-empty">${esc(message)}</div>`;
}

function bindExpanders() {
  document.querySelectorAll(".expander").forEach(button => button.addEventListener("click", () => {
    document.getElementById(button.dataset.target)?.classList.toggle("open");
  }));
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
    const source = queue.shift(); processed++;
    outgoing.get(source).forEach(target => {
      ranks.set(target, Math.max(ranks.get(target), ranks.get(source) + 1));
      indegree.set(target, indegree.get(target) - 1);
      if (indegree.get(target) === 0) queue.push(target);
    });
  }
  if (processed !== nodes.length) nodes.forEach((node, index) => ranks.set(node.id, index));
  return ranks;
}

function truncateVisual(value, maxUnits) {
  let units = 0, output = "";
  for (const character of Array.from(String(value || ""))) {
    const weight = /[\u2E80-\u9FFF\uAC00-\uD7AF\u3040-\u30FF]/.test(character) ? 1.75 : 1;
    if (units + weight > maxUnits) return `${output.trimEnd()}…`;
    output += character; units += weight;
  }
  return output;
}

function renderDAG(nodes, edges) {
  const svg = document.querySelector("#dag");
  if (!svg || !nodes.length) return;
  const width = 180, height = 72, xGap = 32, yGap = 24, pad = 34;
  const ranks = topologicalRanks(nodes, edges);
  const columns = new Map();
  nodes.forEach(node => { const rank = ranks.get(node.id) || 0; if (!columns.has(rank)) columns.set(rank, []); columns.get(rank).push(node); });
  const positions = new Map();
  let maxRank = 0, maxRows = 1;
  for (const [rank, items] of columns) { maxRank = Math.max(maxRank, rank); maxRows = Math.max(maxRows, items.length); }
  const canvasHeight = Math.max(330, pad * 2 + maxRows * height + (maxRows - 1) * yGap);
  const naturalWidth = pad * 2 + (maxRank + 1) * width + maxRank * xGap;
  const canvasWidth = Math.max(780, svg.parentElement?.clientWidth || 0, naturalWidth);
  const xOffset = Math.max(0, (canvasWidth - naturalWidth) / 2);
  for (const [rank, items] of columns) {
    const contentHeight = items.length * height + (items.length - 1) * yGap;
    const startY = Math.max(pad, (canvasHeight - contentHeight) / 2);
    items.forEach((node, index) => positions.set(node.id, { x: xOffset + pad + rank * (width + xGap), y: startY + index * (height + yGap) }));
  }
  svg.setAttribute("viewBox", `0 0 ${canvasWidth} ${canvasHeight}`);
  svg.setAttribute("width", canvasWidth); svg.setAttribute("height", canvasHeight);
  const edgeMarkup = edges.map(edge => {
    const source = positions.get(edge.source), target = positions.get(edge.target);
    if (!source || !target) return "";
    const x1 = source.x + width, y1 = source.y + height / 2, x2 = target.x, y2 = target.y + height / 2;
    const bend = Math.max(30, (x2 - x1) * .46);
    return `<path class="dag-edge ${esc(edge.status)}" d="M${x1} ${y1} C${x1 + bend} ${y1},${x2 - bend} ${y2},${x2} ${y2}" marker-end="url(#dag-arrow)"/>`;
  }).join("");
  const nodeMarkup = nodes.map((node, index) => {
    const point = positions.get(node.id), color = graphColor(node.status);
    return `<g class="dag-node ${esc(node.status)}" data-node="${esc(node.id)}" tabindex="0" transform="translate(${point.x} ${point.y})" style="--node-color:${color}"><title>${esc(`${node.label} — ${node.subtitle || ""}`)}</title><rect width="${width}" height="${height}" rx="10"/><rect class="node-accent" width="3" height="${height - 20}" y="10" rx="1"/><rect class="node-icon-bg" x="14" y="17" width="32" height="32" rx="9"/>${nodeIcon(node.kind)}<clipPath id="node-copy-${index}"><rect x="54" y="10" width="${width - 64}" height="42"/></clipPath><g clip-path="url(#node-copy-${index})"><text class="node-label" x="56" y="28">${esc(truncateVisual(node.label, 21))}</text><text class="node-subtitle" x="56" y="45">${esc(truncateVisual(node.subtitle || t(`node.${node.kind}`, {}, humanize(node.kind)), 29))}</text></g><text class="node-time" x="${width - 12}" y="${height - 10}" text-anchor="end">${esc(clock(node.timestamp))}</text></g>`;
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
    request: `<path d="M22 23h12v12H22zM25 27h6M25 31h4"/>`,
    state: `<path d="M28 20 36 24.5V33L28 38l-8-5v-8.5zM24 28h8M24 32h5"/>`,
    check: `<path d="m22 30 4 4 9-11M28 19l8 4v6c0 6-3 9-8 11-5-2-8-5-8-11v-6z"/>`,
    artifact: `<path d="M22 20h9l5 5v13H22zM31 20v6h5M25 31h8"/>`,
    claim: `<path d="M22 20h12v18l-3-2-3 2-3-2-3 2zM25 25h6M25 29h6"/>`,
  };
  return `<g class="node-icon">${icons[kind] || icons.state}</g>`;
}

function openInspector(node) {
  if (!node) return;
  const inspector = document.querySelector("#inspector");
  document.querySelectorAll(".dag-node").forEach(element => element.classList.toggle("selected", element.dataset.node === node.id));
  const details = Object.entries(node.details || {}).filter(([, value]) => value !== "" && value != null);
  inspector.style.setProperty("--node-color", graphColor(node.status));
  inspector.innerHTML = `<button class="inspector-close" aria-label="${esc(t("inspector.close"))}">×</button><span class="kind">${esc(t(`node.${node.kind}`, {}, humanize(node.kind)))}</span><h3>${esc(node.label)}</h3><p class="subtitle">${esc(node.subtitle || "")}</p><dl class="kv"><dt>${esc(t("inspector.status"))}</dt><dd>${esc(humanize(node.status))}</dd>${node.timestamp && !String(node.timestamp).startsWith("0001-") ? `<dt>${esc(t("inspector.timestamp"))}</dt><dd>${esc(new Date(node.timestamp).toLocaleString(state.locale))}</dd>` : ""}${details.map(([key, value]) => `<dt>${esc(humanize(key))}</dt><dd>${esc(typeof value === "object" ? JSON.stringify(value) : value)}</dd>`).join("")}</dl>`;
  inspector.classList.add("open");
  inspector.querySelector(".inspector-close").addEventListener("click", () => { inspector.classList.remove("open"); document.querySelectorAll(".dag-node").forEach(element => element.classList.remove("selected")); });
}

function route() {
  const match = location.hash.match(/^#\/runs\/([^/]+)$/);
  if (match) renderDetail(decodeURIComponent(match[1])); else renderRuns();
}

function connectStream() {
  const stream = new EventSource("/api/stream");
  stream.addEventListener("runs", event => {
    const snapshots = JSON.parse(event.data);
    state.runs = snapshots.map(snapshot => snapshot.summary);
    connection.classList.remove("offline"); connection.lastChild.textContent = ` ${t("connection.live")}`;
    if (!location.hash.match(/^#\/runs\/([^/]+)$/)) renderRuns();
  });
  stream.onerror = () => { connection.classList.add("offline"); connection.lastChild.textContent = ` ${t("connection.reconnecting")}`; };
}

window.addEventListener("hashchange", route);
refreshButton.addEventListener("click", loadRuns);
languagePicker.addEventListener("change", async event => { await setLocale(event.target.value, true); connection.lastChild.textContent = ` ${t("connection.live")}`; route(); });
initLocale().then(loadRuns).then(connectStream);
