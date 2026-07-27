---
name: stateseal
description: Automatically route code-changing work through visible StateSeal verified delivery. Use for implementation, fixes, refactors, migrations, generated code, and tests; also use when the user writes /seal or explicitly asks for StateSeal status, enablement, exclusion, diagnosis, retry, or verified Apply. Do not use for explanation, research, search, planning, or other read-only work.
---

# StateSeal

StateSeal is the local trusted-delivery control plane and transaction broker for
coding-agent changes. The Plugin is its Codex access layer; MCP supplies semantic
tools, StateSeal Core owns the state machine and receipt, and repository-local
hooks protect managed proposal boundaries.

## Default interaction

For an ordinary code-changing request, call `inspect_project` before editing.
The user does not need to prefix the request with StateSeal.

Show a compact visible handoff before delivery:

```text
StateSeal 已介入
Workspace: <workspace>
Repository: <repository>
Agent: <agent>
Mode: isolated verified delivery
```

Then show these stages as they occur:

```text
创建隔离候选 → Agent 开发 → 独立验证 → Receipt → 用户确认 Apply
```

If the workspace contains multiple repositories, show the returned repository
list, ask the user to select one, and call `inspect_project` again with the
selected `repository`. Never silently choose the first repository.

## Routing

- If the project is enabled and authority is healthy, call `start_delivery`.
  Do not edit the source workspace directly.
- If first enablement is required, show the exact verification contract and
  call `enable_project`. The host's approval of this important tool call is the
  one explicit enablement confirmation.
- If the project is excluded or StateSeal reports `not_applicable`, continue
  using the Agent's normal workflow.
- If StateSeal reports `degraded` or `unavailable`, do not stop development.
  Continue using the Agent's normal workflow, label the result `UNVERIFIED`,
  state that no StateSeal receipt exists, and offer retry, diagnosis, or
  exclusion. Do not automatically retry the same failure more than once.
- If healthy verification returns a rejected verdict in enforce mode, explain
  the verifier evidence and keep the source branch unchanged. This is a real
  policy result and must not be confused with StateSeal infrastructure failure.
- For an admitted delivery, present evidence and call `apply_verified`.
  Final Apply always requires explicit approval. If approval is unavailable or
  declined, preserve the verified checkpoint and let the user retry or reject
  it later.

Never claim StateSeal verification without an admitted, non-empty receipt.

## Explicit control

Treat `/seal` as the optional control surface:

- `/seal status`: inspect the current workspace/project and summarize authority.
- `/seal on`: inspect and enable after showing the contract.
- `/seal off`: exclude the selected repository using the StateSeal CLI.
- `/seal run`: force StateSeal routing for the following development goal.
- `/seal exclude`: exclude the selected repository and record the user's reason.
- `/seal retry`: retry one recoverable StateSeal infrastructure failure.
- `/seal diagnose`: run or recommend `seal integrate doctor codex-desktop`.

Natural-language equivalents are accepted. Explicit control does not change the
rules for first enablement, verified receipts, or final Apply.
