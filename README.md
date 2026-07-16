# StateSeal

StateSeal is a local-first, Git-native admission layer for changes produced by coding agents. It lets an agent propose code in an isolated worktree, binds verifier evidence to the exact code state, preserves the last verified checkpoint, and requires fresh completion recertification before a change can be applied.

It is not another coding agent and it does not replace your tests. It wraps the agent and verification commands you already use.

> The model proposes. Evidence informs. The broker decides. Git remembers. The user applies.

## Why

A test pass is only meaningful for the state that was tested. Long-running coding agents can pass a suite, continue editing, regress the code, and still report success. A receipt can also be stale, copied from another tree, or edited after it was produced.

StateSeal makes these transitions explicit:

```text
WORKING → CANDIDATE → VERIFYING → VERIFIED → RECERTIFYING → ADMITTED
                           ↘ REJECTED                 ↘ STALE / REJECTED / ABSTAINED
```

The broker, not the agent, owns admission. Failed proposals do not overwrite a verified checkpoint.

## Quick start

Requirements: Git and Go 1.24 or newer.

```bash
go install github.com/hellogxp/stateseal/cmd/seal@latest

cd your-project
seal init
# Review seal.yaml before enforcing it.
git add seal.yaml && git commit -m "Configure StateSeal policy"

seal verify -- go test ./...
seal run -- codex exec "Fix the duplicate payment bug"
seal diff
seal apply --branch fix/payment-idempotency
```

`seal run` starts the agent in a managed proposal worktree. Your current branch is not changed until `seal apply` succeeds.

## Policy

Commands are argv arrays and are executed directly, without an implicit shell:

```yaml
version: v0alpha1

task:
  id: payment-idempotency
  goal: Repeated callbacks must not create duplicate charges.

state:
  include: ["src/**", "tests/**", "go.mod", "go.sum"]
  protected: [".github/workflows/**", ".stateseal/**"]

admission:
  checks:
    - id: targeted-tests
      command: ["go", "test", "./internal/payment/..."]
      timeout_seconds: 300

completion:
  checks:
    - id: full-suite
      command: ["go", "test", "./..."]
      timeout_seconds: 900
  recertify_latest_checkpoint: true
  on_missing_evidence: abstain
  on_stale_evidence: reject

execution:
  clean_worktree: true
  network: inherit

budget:
  max_candidates: 8
  max_wall_seconds: 1800

residual_risks:
  - Integration environment was not evaluated.
```

Admission checks decide whether a candidate is worth preserving. Completion checks certify the selected checkpoint again in a fresh detached worktree.

## Commands

| Command | Purpose |
| --- | --- |
| `seal init` | Detect a starter verifier and write `seal.yaml` |
| `seal verify -- …` | Verify the current tree and issue state-bound evidence |
| `seal run -- …` | Run an agent in a proposal worktree and broker admission |
| `seal submit` | Request an intermediate candidate boundary during `seal run` |
| `seal status` | Show the task, checkpoint, verdict, and coverage |
| `seal diff` | Compare the verified checkpoint with its trusted base |
| `seal apply` | Apply an admitted checkpoint to the user branch |
| `seal explain` | Explain the latest verdict and next action |
| `seal restore` | Restore the managed proposal to the verified checkpoint |
| `seal inspect` | Check a receipt's structural integrity |
| `seal doctor` | Validate local prerequisites and policy |

Modes support gradual adoption:

- `shadow`: collect and report without blocking;
- `warn`: report a blocking verdict but return success;
- `enforce`: return a stable non-zero exit code unless admitted.

## Trust boundary

StateSeal v0alpha1 is designed for honest-but-fallible coding agents. A Git worktree is isolation from accidental edits, not a security sandbox. A malicious process running as the same OS user may alter local state. Protected CI must perform a clean checkout and fresh verification; it must not trust a receipt committed in a pull request.

An `ADMITTED` verdict means that the exact checkpoint satisfied the configured policy. It does not prove that the specification or test suite is complete, that every environment is safe, or that the host is uncompromised. Receipts therefore always include residual risks.

## Architecture

```text
existing agent → proposal worktree → candidate
                                      ↓
                               external broker
                              ↙       ↓       ↘
                   clean evaluator  ledger  checkpoint
                              ↘       ↓       ↙
                         fresh recertification
                                   ↓
                         receipt → user apply
```

Authoritative state and the hash-chained event ledger live outside the repository under the platform state directory. Exported `.stateseal/receipts/*.json` files are shareable records, not admission authority.

## Development

```bash
make test
make build
make sealbench
```

For networks that block the Git transport but allow the GitHub API, maintainers can publish a committed tree without storing credentials:

```bash
GH_TOKEN=... scripts/publish-via-github-api.sh owner/repository main
```

Every change to the core admission path should include a failure-injection test. See [docs/threat-model.md](docs/threat-model.md) and [sealbench/README.md](sealbench/README.md).

## Status

StateSeal is pre-release software. The `v0alpha1` protocol may change with migration notes. Core state semantics will be frozen at beta.

## License

Apache License 2.0.
