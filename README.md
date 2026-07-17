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

When a terminal candidate regresses after an earlier checkpoint was verified, StateSeal selects that checkpoint, materializes it in a fresh evaluator, reruns completion checks, and records the recovered selection in the completion receipt.

## Quick start

Requirements: Git. Release binaries do not require Go.

Install the latest verified release on macOS or Linux:

```bash
curl -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh -o install-stateseal.sh
less install-stateseal.sh
sh install-stateseal.sh
rm install-stateseal.sh
```

The installer selects the current OS and architecture and verifies the archive
against the release checksum before installing `seal` to `~/.local/bin`.
Install a specific version with `--version v0.1.0-alpha.1`, or set
`STATESEAL_INSTALL_DIR` to choose another destination.

Until the first binary release is published, install from source with Go 1.24
or newer:

```bash
go install github.com/hellogxp/stateseal/cmd/seal@latest

cd your-project
seal init --goal "Repeated callbacks must create exactly one charge."
# Review seal.yaml before enforcing it.
seal adapter list
# Install the adapter for the agent you use, for example:
seal adapter claude install
git add seal.yaml .claude/settings.json && git commit -m "Configure StateSeal policy"

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
  protected:
    - ".stateseal/**"
    - ".codex/**"
    - ".claude/**"
    - ".gemini/**"
    - ".cursor/**"
    - ".opencode/**"
    - ".github/hooks/**"
    - ".github/workflows/**"

admission:
  timeout_seconds: 300
  checks:
    - id: targeted-tests
      command: ["go", "test", "./internal/payment/..."]
      timeout_seconds: 300

completion:
  timeout_seconds: 900
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
| `seal timeline` | Show the integrity-verified event timeline (`--json` supported) |
| `seal diff` | Compare the verified checkpoint with its trusted base |
| `seal apply` | Apply an admitted checkpoint to the user branch |
| `seal explain` | Explain the latest rule and next action (`--json` supported) |
| `seal restore` | Restore the managed proposal to the verified checkpoint |
| `seal inspect` | Check a receipt's structural integrity |
| `seal adapter list` | List native lifecycle integrations |
| `seal adapter <agent> install` | Install automatic boundaries for a supported agent |
| `seal doctor` | Validate local prerequisites and policy |

Modes support gradual adoption:

- `shadow`: record the counterfactual enforce verdict without blocking;
- `warn`: emit a warning, record an explicit override, and return success;
- `enforce`: return a stable non-zero exit code unless admitted.

Receipts keep the underlying verdict and add `enforcement_mode`, `disposition`,
and a stable `rule_id`. See [the reliability rule taxonomy](docs/rules.md).

## Trust boundary

StateSeal v0alpha1 is designed for honest-but-fallible coding agents. A Git worktree is isolation from accidental edits, not a security sandbox. A malicious process running as the same OS user may alter local state. Protected CI must perform a clean checkout and fresh verification; it must not trust a receipt committed in a pull request.

The core protocol is agent-agnostic. Codex, Claude Code, Gemini CLI, Cursor
Agent, GitHub Copilot CLI, and OpenCode integrations are thin lifecycle
adapters; every other terminal agent still works through `seal run -- <agent>`
with mandatory terminal recertification.

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
make stackbench
```

Build a local release snapshot with embedded version and commit identity:

```bash
make release-snapshot VERSION=v0.1.0-alpha.1
tar -xzf dist/stateseal_0.1.0-alpha.1_$(go env GOOS)_$(go env GOARCH).tar.gz -C dist
dist/stateseal_0.1.0-alpha.1_$(go env GOOS)_$(go env GOARCH)/seal version
```

Tagging a semantic version runs the full verification suite and publishes
macOS/Linux archives for AMD64/ARM64, `checksums.txt`, and the reviewed installer.
See [CHANGELOG.md](CHANGELOG.md) for release notes.

For networks that block the Git transport but allow the GitHub API, maintainers can publish a committed tree without storing credentials:

```bash
GH_TOKEN=... scripts/publish-via-github-api.sh owner/repository main
```

Every change to the core admission path should include a failure-injection test. See [docs/threat-model.md](docs/threat-model.md) and [sealbench/README.md](sealbench/README.md).

## Status

StateSeal is pre-release software. The `v0alpha1` protocol may change with migration notes. Core state semantics will be frozen at beta.

## License

Apache License 2.0.
