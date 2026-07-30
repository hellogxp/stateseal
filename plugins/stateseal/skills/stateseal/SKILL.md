---
name: stateseal
description: Inspect StateSeal's read-only Agent runtime panorama, Skill attribution, diagnostics, and audit data. Use only when the user explicitly asks for StateSeal, Agent session observability, Skill attribution, runtime diagnosis, or audit. Never trigger for ordinary coding work.
---

# StateSeal runtime intelligence

StateSeal is an optional, read-only observability surface. It must never become
part of the Agent's execution or delivery decision.

## Hard invariant

- Never call StateSeal before or during ordinary implementation unless the user
  explicitly asks to inspect observability data.
- Never route a coding request through StateSeal.
- Never start, stop, retry, continue, deny, block, approve, apply, restore, or
  modify Agent work.
- Never treat a StateSeal diagnostic as authority over the Agent or user.
- Never claim semantic correctness from command completion alone.

## When explicitly requested

Use the local read-only UI or observational CLI to summarize:

- session timing and tool sequence;
- observed Skill resource loads;
- derived Skill-to-tool attribution;
- reported failures and diagnostic hypotheses;
- artifacts and file changes when present in the source transcript.

Always label claims as:

- `Observed`: directly present in a runtime event;
- `Derived`: deterministically correlated from observed signals;
- `Inferred`: a hypothesis that may be wrong.

Do not expose raw prompts, tool arguments, outputs, secrets, or full paths when
a summary is sufficient.
