# StateSeal

**English** · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) ·
[한국어](README.ko.md) · [Español](README.es.md) ·
[Português do Brasil](README.pt-BR.md) · [Deutsch](README.de.md) ·
[Français](README.fr.md)

StateSeal is a **read-only delivery intelligence layer for coding agents**. It
reconstructs the delivery chain—request, observed code states, checks,
artifacts, failures, and the Agent's completion claim—without participating in
Agent execution.

> Observe the execution. Explain the chain. Never become the execution.

![StateSeal Runs Console](docs/assets/stateseal-runs-console.svg)

## Why

Agent transcripts contain useful delivery evidence, but it is fragmented
across messages, file edits, Git state, checks, tool results, and artifacts.
Developers need answers to practical questions:

- What code state did the Agent actually leave behind?
- Which files changed, and which artifacts were produced?
- Which checks ran on the current state, and which became stale after later edits?
- Did a current-state check fail, or did the Agent finish without observable checks?
- Which facts are observed, which attributions are derived, and which
  conclusions are only inferred?
- Where did time go, and where should a failed run be debugged?

StateSeal turns those signals into a searchable delivery list, evidence graph,
diagnostic findings, and an audit timeline.

## Product invariant

StateSeal is observational infrastructure:

```text
Agent-owned session logs + Git metadata + local artifacts
                         ↓  read only
           normalization and state correlation
                         ↓
     panorama · evidence · diagnosis · audit
```

It does **not** install lifecycle hooks, proxy model traffic, launch an Agent,
modify prompts, return deny/block signals, approve delivery, change branches,
or apply code. Agent execution and outcomes remain owned by the Agent and user.

Every analytical statement has an evidence grade:

| Grade | Meaning |
| --- | --- |
| **Observed** | Present directly in an Agent transcript or local runtime event |
| **Derived** | Deterministically correlated from observed IDs, paths, and time |
| **Inferred** | A diagnostic hypothesis; useful, but not asserted as fact |

## Quick start

Install from a source checkout with Go 1.24 or newer:

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

Or install a published release:

```bash
curl --proto '=https' --tlsv1.2 -fsSL \
  https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

The UI listens only on a loopback address and passively discovers supported
local Codex session transcripts. It refreshes while the Agent runs. StateSeal
does not install an integration. Raw generic tool arguments are hidden and
common secret patterns are redacted.

Useful options:

```bash
seal ui --no-open
seal ui --address 127.0.0.1:9137
seal workspace list /path/to/workspace
```

## Current scope

- passive Codex JSONL session discovery;
- live all-session delivery overview;
- request → observed code states → checks/artifacts → completion-claim DAG;
- state-aware check freshness and stale-evidence detection;
- observed change and artifact inventories;
- reproducible diagnostic findings with evidence basis;
- explicit Observed / Derived / Inferred semantics;
- privacy-preserving summaries and secret redaction;
- historical StateSeal evidence shown as a read-only archive;
- English, Simplified Chinese, Japanese, Korean, Spanish, Brazilian
  Portuguese, German, and French UI locales.

Claude Code and other Agent transcript adapters are planned. Compatibility is
reported per source format; StateSeal never claims semantic correctness merely
because a command completed.

## Commands

The default command surface is intentionally observational:

| Command | Purpose |
| --- | --- |
| `seal ui` | Open the local live panorama |
| `seal workspace list [path]` | Discover repositories without modifying them |
| `seal version` | Show build identity |

Managed execution, verification gates, admission, Apply, restore, lifecycle
hook installation, and delivery-control MCP tools are outside the supported
product and are not registered by the CLI.

## Documentation

- [Documentation index](docs/index.md)
- [Getting started](docs/getting-started.md)
- [Runtime panorama](docs/runs-console.md)
- [Product map](docs/product-map.md)
- [Privacy and evidence semantics](docs/observation-model.md)

## Development

```bash
go test ./...
go vet ./...
```

Licensed under Apache-2.0.
