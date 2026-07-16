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
- `STATESEAL_SUBMIT_DIR`
- `STATESEAL_MODE`
- `STATESEAL_ADAPTER_CHECKS`

The terminal state is always submitted as a candidate, including when the agent exits non-zero. An agent may also request an intermediate boundary:

```bash
seal submit
```

The request blocks while the external broker commits and evaluates the candidate. The agent receives the verdict but cannot issue it. A later failed candidate does not replace the checkpoint created by a successful intermediate submission. StateSeal reports whether coverage was `terminal-only` or `intermediate + terminal`.

## Codex lifecycle adapter

Codex exposes `PostToolUse` and `Stop` lifecycle hooks. StateSeal uses those
official boundaries instead of parsing transcripts or injecting instructions
into the model prompt.

Install the repository-local hooks once:

```bash
seal adapter codex install
git add .codex/hooks.json seal.yaml
git commit -m "Configure StateSeal Codex hooks"
```

Open `/hooks` in Codex and review and trust the exact definitions before the
first run. Project hooks only load for a trusted project. The generated file
preserves unrelated hook groups; `--force` replaces only existing StateSeal
groups.

During `seal run -- codex ...`:

1. `PostToolUse` recognizes an exact configured verifier command and requests
   an intermediate candidate boundary.
2. The external broker reruns the policy checks in clean evaluators; hook output
   is never treated as verification evidence.
3. `Stop` submits the terminal candidate. In enforce mode, a failed terminal
   verdict asks Codex to continue rather than claim completion.
4. The outer `seal run` still performs terminal evaluation and checkpoint
   recovery, so disabled or unavailable hooks reduce coverage but cannot forge
   admission.

The hook command inherits only the opaque broker request directory. It cannot
write a verdict or choose a checkpoint.
