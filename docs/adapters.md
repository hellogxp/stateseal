# Agent adapters

StateSeal is agent-agnostic. The command adapter works with any coding agent
that can be launched from a terminal:

```bash
seal run "Implement the requested change"

# Advanced compatibility mode:
seal run -- codex exec "Implement the requested change"
seal run -- claude -p "Implement the requested change"
seal run -- qodercli --prompt "Implement the requested change"
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

StateSeal has two integration scopes:

```text
user scope       seal integrate <agent>       once per Agent and computer
project scope    first seal run / seal setup  once per repository
```

User scope preserves existing configuration. Desktop surfaces use MCP when the
host provides a stable MCP client; CLI surfaces use lifecycle entry points for
intermediate candidate coverage:

```bash
seal integrate codex-desktop
seal integrate claude-code
seal integrate qoder

seal integrate status
seal integrate doctor qoder
seal integrate uninstall qoder
```

Installation alone is not proof of end-to-end compatibility. Codex Desktop now
uses a local stdio MCP server with explicit tool schemas, routing instructions,
server-initiated native confirmation for project enablement and final apply,
durable session state, isolated Agent execution, exact receipt binding, and
recovery surfaces. The managed child drops outer user configuration and the MCP
server exposes no recursive tools inside a child run. Development tasks reject
empty candidates under `LC004`. It
remains experimental until a pinned Desktop release passes a live compatibility
run. Claude Code and Qoder still require equivalent Desktop MCP conformance
before promotion.

For Codex Desktop, ordinary code-changing prompts are routed through
`inspect_project` and `start_delivery`; no prefix is required. `enable_project`
binds the user's native approval to the exact displayed policy digest.
`apply_verified` accepts only the session-bound admitted receipt. The MCP server
returns structured progress and evidence, while internal `seal desktop`
commands remain recovery surfaces rather than user workflow.

MCP routing is host-Agent mediated: the instructions distinguish code-changing
work from explanation, search, planning, and review-only work, but MCP alone is
not a universal filesystem interception boundary. Once `start_delivery` begins,
StateSeal owns the hard boundaries: isolated execution, non-empty delivery,
external verification, exact receipt matching, native acceptance, and apply.
Use `seal run` when the invocation itself must be an authoritative boundary.

## Exclude a project from automatic integration

Projects such as StateSeal itself can opt out of automatic MCP routing without
disabling the user-level Agent integration:

```bash
cd /path/to/project
seal integrate exclude --reason "StateSeal self-development"
seal integrate exclusions

# Allow automatic routing again; fresh project approval is still required.
seal integrate include
```

The exclusion applies to the exact Git root and is stored in the user's
StateSeal state directory, outside the repository. Project code cannot silently
remove it. Excluding a project clears prior Desktop authorization and trusted
hook automation. It does not block an explicit `seal run` command, and it is
not inherited by CI or other computers.

Use `seal adapter list` to inspect the built-in matrix and install only the
agents used by the repository:

```bash
seal adapter codex install
seal adapter claude install
seal adapter qoder install
seal adapter gemini install
seal adapter cursor install
seal adapter copilot install
seal adapter opencode install
```

| Agent | Automatic verifier boundary | Automatic terminal boundary | Generated file |
| --- | --- | --- | --- |
| Codex | `PostToolUse` | `Stop` | `.codex/hooks.json` |
| Claude Code | `PostToolUse` | `Stop` | `.claude/settings.json` |
| Qoder | `PostToolUse` | `Stop` | `.qoder/settings.json` |
| Gemini CLI | `AfterTool` | `AfterAgent` | `.gemini/settings.json` |
| Cursor Agent | `afterShellExecution` | `stop` | `.cursor/hooks.json` |
| GitHub Copilot CLI | `postToolUse` | `agentStop` | `.github/hooks/stateseal.json` |
| OpenCode | `tool.execute.after` | `session.idle` | `.opencode/plugins/stateseal.js` |

Generated configuration is not treated as proof of runtime compatibility. See
[the live compatibility matrix](compatibility.md) for validated Agent versions,
coverage, and known lifecycle limits.

The mappings follow each product's public extension surface: [Codex
MCP](https://learn.chatgpt.com/docs/extend/mcp), [Codex
hooks](https://learn.chatgpt.com/docs/hooks), [Claude Code
hooks](https://code.claude.com/docs/en/hooks), [Gemini CLI
hooks](https://geminicli.com/docs/hooks/reference/), [Cursor
hooks](https://cursor.com/docs/hooks), [GitHub Copilot
hooks](https://docs.github.com/en/copilot/reference/hooks-reference), and
[OpenCode plugins](https://opencode.ai/docs/plugins/). Qoder's
[official hooks documentation](https://docs.qoder.com/extensions/hooks)
explicitly shares user and project configuration across CLI, IDE, and its
JetBrains plugin.

Commit generated adapter files as protected infrastructure. The goal-driven
workflow does this during its confirmed first-run setup. Codex requires trust
for project hooks; StateSeal records explicit user authorization outside the
repository and enables only the generated, marker-checked hook during managed
automation. It does not bypass the Agent sandbox. `--force`
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

Retry capability is reported honestly: Codex, Claude Code, Gemini CLI, Qoder,
Cursor Agent, and Copilot CLI expose a stop decision or follow-up mechanism.
Qoder requires exit code `2` for a blocking `Stop` and marks the retry with
`stop_hook_active`; StateSeal releases that retry to prevent an infinite hook
loop, while the outer managed run still performs authoritative terminal
recertification.
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
