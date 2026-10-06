# Getting started

## Install

Requirements: macOS or Linux. Source builds require Go 1.24 or newer.

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal version
```

## Open the delivery panorama

```bash
seal ui
```

StateSeal opens a local browser view and passively discovers recent supported
Agent transcripts. There is no project initialization, Agent integration,
hook, proxy, policy, approval, or background daemon.

Use a fixed loopback port or avoid opening a browser:

```bash
seal ui --address 127.0.0.1:9137
seal ui --no-open
```

## Read a delivery session

```text
Delivery request
      ↓
Observed code states ──→ checks bound to each observed state
      ├────────────────→ created artifacts
      └────────────────→ Agent completion claim
```

Start with the diagnostic banner, then inspect:

1. **Current workspace state** — branch, commit/dirty digest, and code delta.
2. **Checks and evidence freshness** — whether a recognized check ran before
   or after the latest observed edit.
3. **Diagnostic findings** — the exact observed or derived basis for a warning.
4. **Audit timeline** — a sanitized sequence of source events.

`Evidence current` means at least one recognized passing check belongs to the
latest observed state. It does not mean all requirements were tested, the code
is semantically correct, or the delivery is approved.

## Privacy defaults

StateSeal listens on loopback, does not upload session data, hides raw generic
tool arguments, bounds displayed output, and redacts common token patterns.
Local Agent transcripts can still be sensitive; protect them as development
credentials and logs.

## Compatibility

The first supported source is Codex JSONL. StateSeal parses it defensively and
shows an integrity warning rather than inventing missing facts. Source adapters
for other Agents should remain read-only and versioned independently.
