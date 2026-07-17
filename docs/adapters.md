# Agent adapters

StateSeal is agent-agnostic. The command adapter works with any coding agent
that can be launched from a terminal:

```bash
seal run -- codex exec "Implement the requested change"
seal run -- claude -p "Implement the requested change"
seal run -- gemini -p "Implement the requested change"
seal run -- cursor-agent -p "Implement the requested change"
seal run -- copilot -p "Implement the requested change"
seal run -- opencode run "Implement the requested change"
```

The core never imports an agent SDK. It starts the agent in a proposal
worktree, supplies an opaque submission channel, evaluates candidates in clean
worktrees, and always evaluates the terminal state. A missing or disabled
lifecycle integration therefore reduces checkpoint coverage to
`terminal-only`; it cannot forge an admission.

## Native lifecycle integrations

Use `seal adapter list` to inspect the built-in matrix and install only the
agents used by the repository:

```bash
seal adapter codex install
seal adapter claude install
seal adapter gemini install
seal adapter cursor install
seal adapter copilot install
seal adapter opencode install
```

| Agent | Automatic verifier boundary | Automatic terminal boundary | Generated file |
| --- | --- | --- | --- |
| Codex | `PostToolUse` | `Stop` | `.codex/hooks.json` |
| Claude Code | `PostToolUse` | `Stop` | `.claude/settings.json` |
| Gemini CLI | `AfterTool` | `AfterAgent` | `.gemini/settings.json` |
| Cursor Agent | `afterShellExecution` | `stop` | `.cursor/hooks.json` |
| GitHub Copilot CLI | `postToolUse` | `agentStop` | `.github/hooks/stateseal.json` |
| OpenCode | `tool.execute.after` | `session.idle` | `.opencode/plugins/stateseal.js` |

Generated configuration is not treated as proof of runtime compatibility. See
[the live compatibility matrix](compatibility.md) for validated Agent versions,
coverage, and known lifecycle limits.

The mappings follow each product's public extension surface: [Codex
hooks](https://learn.chatgpt.com/docs/hooks), [Claude Code
hooks](https://code.claude.com/docs/en/hooks), [Gemini CLI
hooks](https://geminicli.com/docs/hooks/reference/), [Cursor
hooks](https://cursor.com/docs/hooks), [GitHub Copilot
hooks](https://docs.github.com/en/copilot/reference/hooks-reference), and
[OpenCode plugins](https://opencode.ai/docs/plugins/).

Commit generated adapter files as protected infrastructure. Review or trust
them through the agent's native UI when that product requires it. `--force`
replaces StateSeal-owned entries while preserving unrelated configuration;
the Copilot and OpenCode integrations use dedicated StateSeal-owned files.

## Boundary behavior

For agents with command-level lifecycle events, StateSeal compares the shell
command with the admission and completion commands in `seal.yaml`. A match
requests an intermediate boundary. The command's own output is not trusted:
the external broker reruns the configured checks in clean evaluators.

At the agent's stop/idle boundary, StateSeal submits the terminal candidate.
Where the native hook contract supports retry, enforce mode feeds a rejected
verdict back to the agent. Shadow and warn modes never force another turn.
Regardless of native retry behavior, the outer `seal run` process makes the
final decision and can recover a previously verified checkpoint.

Retry capability is reported honestly: Codex, Claude Code, Gemini CLI,
Cursor Agent, and Copilot CLI expose a stop decision or follow-up mechanism.
OpenCode's current `session.idle` plugin event is observational and cannot
restart the agent loop, so its outer StateSeal process performs enforcement.
Cursor hooks remain a beta product surface; terminal recertification remains
authoritative if their event contract changes.

An agent or script may also request an explicit boundary:

```bash
seal submit
```

The request blocks while the external broker commits and evaluates the
candidate. The caller receives the verdict but cannot issue it. A later failed
candidate does not replace a successful checkpoint.

## Runtime contract

The managed agent receives:

- `STATESEAL_TASK_ID`
- `STATESEAL_PROPOSAL_ROOT`
- `STATESEAL_ORIGINAL_ROOT`
- `STATESEAL_SUBMIT_DIR`
- `STATESEAL_MODE`
- `STATESEAL_ADAPTER_CHECKS`
- `STATESEAL_RUNTIME_ROOT`

StateSeal also redirects common language build caches (`GOCACHE`, `GOTMPDIR`,
`PYTHONPYCACHEPREFIX`, `npm_config_cache`, and `CARGO_TARGET_DIR`) to a managed
temporary runtime directory. This keeps generated cache files out of candidate
identity and gives sandboxed Agents a writable location without weakening the
proposal worktree boundary.

The submission directory is an opaque request/response channel. The broker's
state, ledger, checkpoint selection, receipt digest, and verifier execution
remain outside the agent process.
