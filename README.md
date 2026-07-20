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
```

From a source checkout, experience verified-checkpoint recovery end to end:

```bash
make experience
```

Then use StateSeal in your own repository:

```bash
seal integrate codex-desktop   # once for this Agent on this computer
cd your-project
seal run "修复重复回调导致的重复扣款，并保持现有接口兼容"
```

`seal integrate` non-destructively merges user-level lifecycle hooks and keeps a
safety backup of the original configuration. Codex, Claude Code, and Qoder
integrations can be inspected with `seal integrate status` and removed without
touching unrelated hooks. User-level Desktop integration is currently
experimental; the authoritative delivery path is still the managed `seal run`
process.

On the first managed run in a repository, StateSeal shows the detected project
type, exact admission and completion commands, and protected configuration. The
user confirms that verification contract once before StateSeal commits it as
auditable project infrastructure. StateSeal then runs the Agent loop in an
isolated proposal, evaluates the exact candidate in clean worktrees, and asks
one final delivery question.
StateSeal follows the operating system message locale automatically and falls
back to English. Use `--agent`, `--apply`, or `--no-apply` when explicit control
is needed. The default view reports real lifecycle events, check results, and
durations; use `--verbose` for the Agent stream, `--quiet` for the final status
only, or `--json` for automation. Your branch is unchanged until apply succeeds.

See [Getting started](docs/getting-started.md) for the complete installation,
integration, first-project confirmation, delivery, and uninstall flow. A
[Simplified Chinese guide](docs/getting-started.zh-CN.md) is also available.

## Policy

Commands are argv arrays and are executed directly, without an implicit shell:

```yaml
version: v0alpha1

task:
  id: payment-service
  goal: Runtime goals are supplied by seal run.

state:
  include: ["src/**", "tests/**", "go.mod", "go.sum"]
  protected:
    - ".stateseal/**"
    - ".codex/**"
    - ".claude/**"
    - ".qoder/**"
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
| `seal integrate <agent>` | Install a user-level lifecycle integration once per Agent (`codex-desktop`, `claude-code`, or `qoder`) |
| `seal integrate status` | Show installed integrations and their honest support level |
| `seal integrate doctor <agent>` | Validate the configured lifecycle boundaries |
| `seal integrate uninstall <agent>` | Remove only StateSeal-owned user hooks |
| `seal run "…"` | Detect or reuse an Agent, show trustworthy progress, independently verify the development loop, and offer delivery (`--verbose`, `--quiet`, and `--json` supported) |
| `seal setup --agent …` | Explicitly configure a repository Agent integration |
| `seal init` | Legacy policy-only initialization |
| `seal verify -- …` | Verify the current tree and issue state-bound evidence |
| `seal run -- <command>` | Advanced compatibility mode for an arbitrary terminal Agent command |
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

The core protocol is agent-agnostic. Codex, Claude Code, Qoder, Gemini CLI,
Cursor Agent, GitHub Copilot CLI, and OpenCode integrations are thin lifecycle
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

StateSeal does not ship a separate Desktop application. Existing Agent Desktop
and IDE products integrate through their native lifecycle configuration; the
same Go broker, evaluator, checkpoint, and receipt protocol remains
authoritative. See the [current product map](docs/product-map.md) and
[compatibility evidence](docs/compatibility.md). The product map is also
available in [Simplified Chinese](docs/product-map.zh-CN.md).

## Development

```bash
make test
make build
make sealbench
make stackbench
```

Live Agent compatibility checks are opt-in because they invoke an authenticated
Agent and are not deterministic:

```bash
make live-codex
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
