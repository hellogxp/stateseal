const app = document.querySelector("#app");
const connection = document.querySelector("#connection");
const refreshButton = document.querySelector("#refresh");

const state = {
  runs: [],
  filter: "ALL",
  query: "",
  selectedNode: null,
};

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
  if (delta >= 0 && delta < 60_000) return "just now";
  if (delta >= 0 && delta < 3_600_000) return `${Math.floor(delta / 60_000)}m ago`;
  if (delta >= 0 && delta < 86_400_000) return `${Math.floor(delta / 3_600_000)}h ago`;
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
};

const clock = (value) => {
  if (!value || String(value).startsWith("0001-")) return "—";
  return new Date(value).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
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
    connection.lastChild.textContent = " Live";
    route();
  } catch (error) {
    connection.classList.add("offline");
    connection.lastChild.textContent = " Offline";
    showError(error);
  }
}

function showError(error) {
  app.innerHTML = `<section class="panel error-state"><p class="eyebrow">LOCAL STATE UNAVAILABLE</p><h2>Could not load StateSeal runs</h2><p>${esc(error.message)}</p></section>`;
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
    app.replaceChildren(document.querySelector("#empty-template").content.cloneNode(true));
    return;
  }
  const metrics = summaryMetrics(runs);
  app.innerHTML = `
    <section class="hero">
      <div>
        <p class="eyebrow">VERIFIED AGENT WORKFLOWS</p>
        <h1>Runs</h1>
        <p class="hero-copy">Every candidate, verifier decision, recovery, and explicit apply — from trusted local state.</p>
      </div>
      <div class="metrics">
        <div class="metric"><strong>${runs.length}</strong><span>Total runs</span></div>
        <div class="metric"><strong>${metrics.active}</strong><span>Active</span></div>
        <div class="metric"><strong>${metrics.admitted}</strong><span>Admitted</span></div>
        <div class="metric"><strong>${metrics.blocked}</strong><span>Not admitted</span></div>
      </div>
    </section>
    <section class="toolbar">
      <label class="search">
        <svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/></svg>
        <input id="run-search" type="search" placeholder="Search goals, tasks, repositories…" value="${esc(state.query)}">
      </label>
      <div class="filters">
        ${["ALL", "ACTIVE", "ADMITTED", "NOT ADMITTED"].map(filter =>
          `<button class="filter ${state.filter === filter ? "active" : ""}" data-filter="${filter}">${filter}</button>`
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
    if (state.filter === "ACTIVE") return ["WORKING", "VERIFYING", "VERIFIED"].includes(run.status);
    if (state.filter === "ADMITTED") return ["ADMITTED", "APPLIED"].includes(run.status);
    if (state.filter === "NOT ADMITTED") return ["REJECTED", "ABSTAINED", "STALE", "ESCALATED"].includes(run.status);
    return true;
  });
}

function renderRunRows() {
  const target = document.querySelector("#run-list");
  if (!target) return;
  const runs = filterRuns();
  if (!runs.length) {
    target.innerHTML = `<div class="error-state"><p>No runs match this view.</p></div>`;
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
          ${run.integrity_error ? `<div class="integrity-note">Ledger integrity warning</div>` : ""}
        </div>
        <div class="run-status">
          <span class="status-pill ${["WORKING","VERIFYING","VERIFIED"].includes(run.status) ? "running" : ""}"><i></i>${esc(run.status)}</span>
          <div class="progress-rail" title="intent → candidate → checkpoint → decision → apply">
            ${[0,1,2,3,4].map((step, i) => `${i ? "<span></span>" : ""}<i class="${step <= phase ? "done" : ""}"></i>`).join("")}
          </div>
        </div>
        <div class="run-stats">
          <div><strong>${run.candidates_evaluated}</strong><span>Candidates</span></div>
          <div><strong>${run.evidence_count}</strong><span>Evidence</span></div>
          <div><strong>${run.checkpoints_verified}</strong><span>Checkpoints</span></div>
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
  app.innerHTML = `<section class="loading-state"><span class="loader"></span><p>Projecting state provenance…</p></section>`;
  try {
    const snapshot = await fetchJSON(`/api/runs/${encodeURIComponent(id)}`);
    const run = snapshot.summary;
    const color = statusColor(run.status);
    app.innerHTML = `
      <a class="back-link" href="#/"><svg viewBox="0 0 24 24"><path d="m15 18-6-6 6-6"/></svg>All runs</a>
      <section class="detail-head">
        <div class="detail-title">
          <p class="eyebrow">${esc(run.task_id)}</p>
          <h1>${esc(run.goal)}</h1>
          <div class="subline"><span>${esc(run.repository)}</span><span>${esc(run.repository_root)}</span><span>Updated ${formatTime(run.updated_at)}</span></div>
        </div>
        <div class="detail-badge" style="--status-color:${color}">
          <span class="status-pill ${["WORKING","VERIFYING","VERIFIED"].includes(run.status) ? "running" : ""}"><i></i>${esc(run.status)}</span>
        </div>
      </section>
      <section class="overview-grid">
        ${overviewCard("Disposition", run.disposition || "Pending", run.mode ? `${run.mode} mode` : "Execution in progress")}
        ${overviewCard("Duration", duration(run.started_at, isTerminal(run.status) ? run.updated_at : null), run.started_at ? `Started ${clock(run.started_at)}` : "Awaiting first event")}
        ${overviewCard("Candidates", run.candidates_evaluated, `${run.candidates_rejected} rejected · ${run.checkpoints_verified} checkpointed`)}
        ${overviewCard("Evidence", run.evidence_count, run.recovered ? "Recovered checkpoint recertified" : "State-bound executions")}
      </section>
      ${run.integrity_error ? `<section class="panel reason" style="margin:12px 0 0;border-color:rgba(251,113,133,.35);color:var(--red)">Ledger integrity warning: ${esc(run.integrity_error)}</section>` : ""}
      <section class="panel section">
        <div class="section-head">
          <div><h2>State provenance DAG</h2><p>Click any node to inspect the bound state and evidence.</p></div>
          <div class="legend">
            <span style="--legend:var(--blue)"><i></i>Active</span>
            <span style="--legend:var(--green)"><i></i>Passed</span>
            <span style="--legend:var(--amber)"><i></i>Recovered</span>
            <span style="--legend:var(--red)"><i></i>Blocked</span>
          </div>
        </div>
        <div class="dag-shell">
          <div class="dag-scroll"><svg id="dag" class="dag" role="img" aria-label="Run state provenance graph"></svg></div>
          <aside id="inspector" class="inspector"></aside>
        </div>
      </section>
      <section class="detail-columns">
        <div class="panel content-panel">
          <div class="section-head"><div><h2>Trusted event timeline</h2><p>Append-only ledger with hash continuity.</p></div><span class="tiny-label">${snapshot.events.length} EVENTS</span></div>
          <div class="timeline">${renderEvents(snapshot.events)}</div>
        </div>
        <div>
          <section class="panel content-panel">
            <div class="section-head"><div><h2>Verifier evidence</h2><p>Fresh executions bound to exact code states.</p></div><span class="tiny-label">${snapshot.evidence.length} RECORDS</span></div>
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
  if (!events.length) return `<div class="error-state"><p>No trusted events recorded yet.</p></div>`;
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
  if (item.timed_out) return { label: "TIMED OUT", value: "warning" };
  if (item.exit_code !== 0) return { label: `FAILED · ${item.exit_code}`, value: "failed" };
  return { label: "PASSED", value: "passed" };
}

function renderEvidence(evidence) {
  if (!evidence.length) return `<div class="error-state"><p>No verifier evidence yet.</p></div>`;
  return evidence.map((item, index) => {
    const status = evidenceStatus(item);
    const name = String(item.verifier_identity || "Verifier").replace(/^command\//, "").replace(/@v1$/, "");
    return `<article class="evidence-item">
      <button class="evidence-head" data-evidence="${index}">
        <span class="evidence-name"><strong>${esc(name)}</strong><span>${esc(humanize(item.verification_phase || "verification"))} · ${duration(item.started_at, item.finished_at)}</span></span>
        <span class="evidence-result" style="--evidence-color:${graphColor(status.value)}">${status.label}</span>
      </button>
      <pre class="evidence-output">${esc(item.output || "No captured output.")}</pre>
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
    <div class="section-head"><div><h2>Completion receipt</h2><p>Cryptographic decision artifact.</p></div></div>
    <div class="receipt">
      <div class="receipt-grid">
        <div><span>Verdict</span><strong>${esc(receipt.verdict)}</strong></div>
        <div><span>Disposition</span><strong>${esc(receipt.disposition)}</strong></div>
        <div><span>Rule</span><strong>${esc(receipt.rule_id || "—")}</strong></div>
        <div><span>Checkpoint</span><strong>${esc(short(receipt.checkpoint_id))}</strong></div>
        <div><span>Receipt ID</span><strong>${esc(short(receipt.receipt_id))}</strong></div>
        <div><span>Digest</span><strong>${esc(short(receipt.receipt_digest))}</strong></div>
      </div>
      ${receipt.reason ? `<p class="reason">${esc(receipt.reason)}</p>` : ""}
    </div>
  </section>`;
}

const kindRank = {
  goal: 0, candidate: 1, guard: 2, verifier: 2, regression: 2, rejection: 2,
  checkpoint: 3, selection: 3, restore: 4, recertification: 5,
  receipt: 6, apply: 7, integrity: 2
};

function renderDAG(nodes, edges) {
  const svg = document.querySelector("#dag");
  if (!svg) return;
  const width = 190, height = 68, xGap = 56, yGap = 26, pad = 34;
  const columns = new Map();
  nodes.forEach(node => {
    const rank = kindRank[node.kind] ?? 2;
    if (!columns.has(rank)) columns.set(rank, []);
    columns.get(rank).push(node);
  });
  const positions = new Map();
  let maxRank = 0, maxRows = 1;
  for (const [rank, items] of columns) {
    maxRank = Math.max(maxRank, rank);
    maxRows = Math.max(maxRows, items.length);
  }
  const canvasHeight = Math.max(410, pad * 2 + maxRows * height + (maxRows - 1) * yGap);
  for (const [rank, items] of columns) {
    const contentHeight = items.length * height + (items.length - 1) * yGap;
    const startY = Math.max(pad, (canvasHeight - contentHeight) / 2);
    items.forEach((node, index) => positions.set(node.id, {
      x: pad + rank * (width + xGap), y: startY + index * (height + yGap)
    }));
  }
  const canvasWidth = Math.max(900, pad * 2 + (maxRank + 1) * width + maxRank * xGap);
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
    return `<path class="dag-edge ${esc(edge.status)}" d="${path}"/>`;
  }).join("");
  const nodeMarkup = nodes.map(node => {
    const point = positions.get(node.id);
    const color = graphColor(node.status);
    const time = node.timestamp && !String(node.timestamp).startsWith("0001-") ? clock(node.timestamp) : "";
    return `<g class="dag-node ${esc(node.status)}" data-node="${esc(node.id)}" tabindex="0" transform="translate(${point.x} ${point.y})" style="--node-color:${color}">
      <rect width="${width}" height="${height}" rx="9"/>
      <rect class="node-accent" width="2" height="${height - 20}" y="10" rx="1"/>
      <rect class="node-icon-bg" x="13" y="15" width="30" height="30" rx="8"/>
      ${nodeIcon(node.kind)}
      <text class="node-label" x="53" y="26">${esc(node.label)}</text>
      <text class="node-subtitle" x="53" y="42">${esc(node.subtitle || humanize(node.kind))}</text>
      <text class="node-time" x="${width - 11}" y="${height - 10}" text-anchor="end">${esc(time)}</text>
    </g>`;
  }).join("");
  svg.innerHTML = `<g>${edgeMarkup}${nodeMarkup}</g>`;
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
    <button class="inspector-close" aria-label="Close inspector">×</button>
    <span class="kind">${esc(humanize(node.kind))}</span>
    <h3>${esc(node.label)}</h3>
    <p class="subtitle">${esc(node.subtitle || "")}</p>
    <dl class="kv">
      <dt>Status</dt><dd>${esc(humanize(node.status))}</dd>
      ${node.timestamp && !String(node.timestamp).startsWith("0001-") ? `<dt>Timestamp</dt><dd>${esc(new Date(node.timestamp).toLocaleString())}</dd>` : ""}
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
    connection.lastChild.textContent = " Live";
    const match = location.hash.match(/^#\/runs\/([^/]+)$/);
    if (!match) renderRuns();
  });
  stream.onerror = () => {
    connection.classList.add("offline");
    connection.lastChild.textContent = " Reconnecting";
  };
}

window.addEventListener("hashchange", route);
refreshButton.addEventListener("click", loadRuns);
loadRuns().then(connectStream);
