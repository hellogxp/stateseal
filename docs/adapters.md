# Agent adapters

StateSeal's command adapter works with any agent that can be launched from a terminal:

```bash
seal run -- codex exec "Implement the requested change"
seal run -- claude -p "Implement the requested change"
seal run -- opencode run "Implement the requested change"
```

The adapter is intentionally thin. It selects a source identity, sets these environment variables, and starts the agent in the proposal worktree:

- `STATESEAL_TASK_ID`
- `STATESEAL_PROPOSAL_ROOT`
- `STATESEAL_ORIGINAL_ROOT`

The terminal state is always submitted as a candidate, including when the agent exits non-zero. An agent may also request an intermediate boundary:

```bash
seal submit
```

The request blocks while the external broker commits and evaluates the candidate. The agent receives the verdict but cannot issue it. A later failed candidate does not replace the checkpoint created by a successful intermediate submission. StateSeal reports whether coverage was `terminal-only` or `intermediate + terminal`.
