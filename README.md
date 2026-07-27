# StateSeal

**English** · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) ·
[한국어](README.ko.md) · [Español](README.es.md) ·
[Português do Brasil](README.pt-BR.md) · [Deutsch](README.de.md) ·
[Français](README.fr.md)

StateSeal turns coding-agent candidates into state-bound, independently
verified, recoverable, recertifiable, and auditable delivery results—with
explicit verification coverage and residual risk.

It is not another coding agent and it does not replace your tests. It wraps the
agent and verification commands you already use. Git is one implementation
mechanism for reproducible state and delivery transactions; it is not the
entire trust model.

> The model proposes. Evidence informs. The broker decides. Git remembers. The user applies.

StateSeal does not make Agents smarter. It makes their delivery verifiable.

![StateSeal trusted delivery pipeline](docs/assets/stateseal-trust-pipeline.svg)

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

Install the latest release binary built from a Git tag by GitHub Actions. The
installer selects the current macOS/Linux architecture, verifies the published
SHA-256 checksum, and installs to `~/.local/bin`:

```sh
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version
```

To inspect the installer first, replace `| sh` with `| less`. Anonymous artifact
reviewers and source users can install the exact checkout with Git and Go 1.24
or newer:

```sh
make install
export PATH="$HOME/.local/bin:$PATH"
```

From the source checkout, experience verified-checkpoint recovery end to end:

```bash
make experience
```

For Codex, install the repo-packaged StateSeal Plugin. The Plugin
bundles the StateSeal Skill and MCP registration; it starts the MCP server from
the installed StateSeal Core binary, so there is no separate MCP product to
install:

```bash
codex plugin marketplace add /path/to/stateseal
codex plugin add stateseal@stateseal
```

The first command registers this source checkout as a local Plugin marketplace;
it does not publish code, create an online account, or install another MCP
service. A packaged marketplace release can hide this developer-only source
registration step.

Start a new Codex task after installation. Ordinary code-changing prompts
activate StateSeal automatically; `@stateseal` and `/seal status`, `/seal on`,
`/seal off`, `/seal run`, and `/seal exclude` remain optional explicit controls.
The one-time project contract and final Apply remain explicit native
confirmations.

When Plugins are unavailable, use the compatibility installer or invoke the
portable CLI boundary directly:

```bash
seal install codex-desktop
# From a repository root:
seal run "Fix duplicate callbacks that cause duplicate charges while preserving API compatibility"
# From a non-Git workspace:
seal run --repo /path/to/repository "Fix duplicate callbacks while preserving API compatibility"
```

Open the local runs console from any directory:

```bash
seal ui
```

The console discovers every local StateSeal run across repositories. The Runs
view answers what is active, admitted, blocked, or applied; each run opens into
an interactive state-provenance DAG with candidate transitions, verifier
evidence, checkpoint recovery, the integrity-checked event timeline, and the
completion receipt. It updates live, remains read-only, and listens only on a
loopback address. Its interface is available in English, Simplified Chinese,
Japanese, Korean, Spanish, Brazilian Portuguese, German, and French.

![StateSeal Runs Console](docs/assets/stateseal-runs-console.svg)

Stay inside Codex Desktop after Plugin installation, open a repository or a
workspace containing repositories, and describe the development goal normally:

```text
Fix duplicate callbacks that cause duplicate charges while preserving API compatibility
```

The Plugin routes code-changing requests through StateSeal's MCP
contract while leaving read-only explanation, search, and planning alone. On
first use it shows the selected workspace, repository, same-type worker Agent,
execution mode, and detected verification contract. The host's important-tool
approval is the single confirmation for `enable_project`; StateSeal does not
open a second nested elicitation. It then delegates implementation to an
isolated worker that cannot recursively invoke StateSeal, rejects an empty
development candidate even when existing tests pass, and returns a compact
timeline plus structured evidence. A separate native acceptance is still
required before `apply_verified` can change the user's branch.

`seal install` / `seal integrate` remain compatibility and repair surfaces.
They non-destructively add a user-level MCP server for Codex
Desktop, keep a safety backup, and remove only obsolete StateSeal Desktop
hooks. Existing MCP servers, settings, and unrelated hooks are preserved. No
`/hooks` command or `StateSeal:` prompt prefix is required. Codex Desktop
controlled delivery passes deterministic MCP contract tests but remains
experimental until the repaired path passes a pinned live Desktop conformance
run. CLI
adapters continue to use lifecycle hooks for intermediate candidate coverage.

StateSeal distinguishes a healthy verification rejection from failure of its
own authority path. A rejected enforce-mode candidate remains blocked. Missing
confirmation capability, MCP startup failure, store failure, or worker startup
failure returns a visible structured degraded result; ordinary Agent
development continues and the outcome is labelled `UNVERIFIED` with no
StateSeal receipt. Final Apply never fails open: a verified checkpoint remains
pending and the source branch stays unchanged until explicit acceptance.

On the first managed run in a repository, StateSeal shows the detected project
type, exact admission and completion commands, and protected configuration. The
user confirms that verification contract once before StateSeal commits it as
auditable project infrastructure. StateSeal then runs the Agent loop in an
isolated proposal, evaluates the exact candidate in clean worktrees, and asks
one final delivery question.
StateSeal follows the operating system message locale automatically and falls
back to English. Use `--agent`, `--apply`, or `--no-apply` when explicit control
is needed. The default view reports meaningful phase changes, changed files,
candidate attempts, independent check results and durations. The delivery card
adds the receipt, exact code state, coverage and residual risks; use `--verbose`
for the Agent stream, `--quiet` for the final status only, or `--json` for
automation. Your branch is unchanged until apply succeeds.

See the [documentation index](docs/index.md) or
[Getting started](docs/getting-started.md) for the complete installation,
integration, first-project confirmation, delivery, and uninstall flow. A
[Simplified Chinese guide](docs/zh-CN/getting-started.md) and localized
documentation in seven additional languages are also available. Reviewers
working from an anonymous source snapshot should use the self-contained
[artifact evaluation guide](ARTIFACT.md).

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
      layer: L1
      origin: project-policy

completion:
  timeout_seconds: 900
  checks:
    - id: full-suite
      command: ["go", "test", "./..."]
      timeout_seconds: 900
      layer: L1
      origin: project-policy
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

Admission checks decide whether a candidate is worth preserving. Completion
checks certify the selected checkpoint again in a fresh detached worktree.
Each check declares a verification layer and provenance, so a receipt can
distinguish an auto-discovered project test from domain acceptance or an
external protected gate. See [Verification model](docs/verification-model.md).

## Commands

| Command | Purpose |
| --- | --- |
| `seal install [agent]` | One-step compatibility installation; defaults to Codex Desktop and includes MCP registration |
| `seal integrate <agent>` | Install a user-level Agent integration once per computer (`codex-desktop`, `codex-cli`, `claude-code`, or `qoder`) |
| `seal workspace list [path]` | Discover Git repositories under a non-Git workspace |
| `seal integrate status` | Show installed integrations and their honest support level |
| `seal integrate doctor <agent>` | Validate configuration plus the MCP or lifecycle handshake |
| `seal integrate uninstall <agent>` | Remove only StateSeal-owned integration entries |
| `seal desktop status --session …` | Inspect durable Desktop session authority state (normally invoked by the adapter) |
| `seal desktop recover --session …` | Recover an interrupted Desktop parent turn from durable task evidence |
| `seal run "…"` | Detect or reuse an Agent, show trustworthy progress, independently verify the development loop, and offer delivery (`--repo` selects a repository from a non-Git workspace) |
| `seal setup --agent …` | Explicitly configure a repository Agent integration |
| `seal init` | Legacy policy-only initialization |
| `seal verify -- …` | Verify the current tree and issue state-bound evidence |
| `seal run -- <command>` | Advanced compatibility mode for an arbitrary terminal Agent command |
| `seal submit` | Request an intermediate candidate boundary during `seal run` |
| `seal status` | Show the task, checkpoint, verdict, and coverage |
| `seal timeline` | Show the integrity-verified event timeline (`--json` supported) |
| `seal ui` | Open the live, read-only Runs console and inspect state provenance DAGs, evidence, and receipts |
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

An `ADMITTED` verdict means that the exact checkpoint satisfied the configured
policy. It does not prove that the specification or test suite is complete,
that every environment is safe, or that the host is uncompromised. Receipts
therefore include verification coverage, verifier provenance, delivery impact,
and residual risks. The [research evidence note](docs/research-evidence.md)
states which design obligations are supported and which product claims are not.

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

`seal ui` is an embedded local observability surface, not a separate authority
or Desktop application. It has no write API and cannot admit or apply code.
Existing Agent Desktop products integrate through MCP; CLI agents use native
lifecycle hooks when available. All surfaces delegate to the same Go broker,
evaluator, checkpoint, and receipt protocol. See the
[Runs console guide](docs/runs-console.md), [current product map](docs/product-map.md),
and [compatibility evidence](docs/compatibility.md). The product map is also
available in [Simplified Chinese](docs/zh-CN/product-map.md).

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
