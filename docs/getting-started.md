# Getting started

The normal StateSeal experience has three setup scopes and one recurring task
step:

```text
computer   install StateSeal once
Agent      integrate each Agent once
project    confirm its verification contract once
task       describe the development goal
```

## 1. Install StateSeal

Install the latest GitHub Actions-built release. The installer detects
macOS/Linux and amd64/arm64, downloads the matching tagged archive, verifies its
published SHA-256 checksum, and installs it to `~/.local/bin`:

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version
```

To inspect the installer before running it, replace `| sh` with `| less`.
From an anonymous artifact or source checkout, install that exact revision with
`make install`; source installation requires Git and Go 1.24 or newer.

## 2. Install the Plugin or integrate an Agent

For Codex, install the StateSeal Plugin. It packages automatic intent routing
and MCP registration; StateSeal Core supplies the server, so there is no
separate MCP installation:

```bash
codex plugin marketplace add /path/to/stateseal
codex plugin add stateseal@stateseal
```

`marketplace add` only registers the local source checkout as a Plugin catalog
for this Codex installation. It does not publish the Plugin, upload the
repository, create an account, or install a second MCP server. A packaged
marketplace release can remove this source-development step.

Start a new Codex task after installation. Ordinary code-changing requests
activate StateSeal automatically and visibly. `@stateseal` and `/seal status`,
`/seal on`, `/seal off`, `/seal run`, and `/seal exclude` are optional controls.

When Plugins are unavailable, or for CLI/headless and repair workflows, use:

```bash
seal install codex-desktop
# or: seal integrate claude-code
# or: seal integrate qoder
```

The Codex compatibility installer registers a local MCP server in
`~/.codex/config.toml`, writes a one-time safety backup, and performs static
validation plus a live MCP handshake. Existing models, projects, MCP servers,
and unrelated hooks are preserved. No `/hooks` command is needed.

Inspect or undo the integration at any time:

```bash
seal integrate status
seal integrate doctor codex-desktop
seal integrate uninstall codex-desktop
```

Codex Desktop controlled delivery passes deterministic end-to-end tests but is
still experimental until a pinned Desktop build passes live conformance.
Claude Code and Qoder installation currently proves the lifecycle foundation,
not a complete managed Desktop delivery. `seal run` remains the portable,
live-validated workflow.

## 3. Portable CLI path

When a Plugin is unavailable, in CI/headless use, or when an explicit
authoritative boundary is required, select the repository directly:

```bash
seal run --repo /path/to/your-project \
  "Add input validation, preserve compatibility, and include tests"
```

From a repository root, `--repo` can be omitted. From a non-Git parent
workspace, StateSeal discovers child repositories. One unambiguous child can be
selected automatically; multiple repositories require explicit selection:

```bash
seal workspace list /path/to/workspace
seal run --repo /path/to/workspace/service-a "Add validation and tests"
```

On first use, StateSeal shows the exact project contract before changing the
repository:

```text
StateSeal · First project setup

  Project: service
  Agent: Codex
  Admission: go test ./...
  Completion: go test ./...; go vet ./...
  Protected: seal.yaml, .codex/hooks.json

Continue? [Y/n]
```

Confirm only when those commands are an appropriate minimum delivery gate. Use
`seal setup --agent <agent>` or edit and review `seal.yaml` when the project
requires private tests, remote CI, generated-code checks, or domain-specific
acceptance commands.

Auto-discovered checks are marked `L1 / auto-discovered`. A project can promote
reviewed checks to `project-policy`, add L2 domain acceptance, or connect L3
external gates. StateSeal never treats auto-discovery as a complete
specification.

## 4. Wait for the delivery result

StateSeal creates an isolated proposal, starts the Agent, independently checks
candidate boundaries, preserves the last verified checkpoint, and performs a
fresh terminal recertification. The user decides whether to apply an admitted
checkpoint:

```text
StateSeal · Controlled delivery

[00:00] ✓ Project verification policy loaded
[00:01] ✓ Isolated candidate workspace created
[00:15] ● Agent is analyzing the project · no code change yet
[00:45] ● Agent is implementing · 2 changed files
[01:14] ◆ Candidate #1 received · 2 changed files
[01:18] ✓ go test ./... · passed · 2.1s
[01:19] ✓ go vet ./... · passed · 0.6s
[01:23] ✓ Final code state matches verified evidence

StateSeal · Delivery result
✓ Ready to deliver

  Changed files: 2
  Files: ulid.go, ulid_test.go
  Receipt: rcpt_…
  Code state: 230fe20ef22f

Delivery evidence
  ✓ candidate produced in an isolated workspace
  ✓ checks executed independently by StateSeal
  ✓ delivered code exactly matches verified state
  ✓ final state recertified in a fresh evaluator

Verification coverage
  L0 · state, policy, freshness, checkpoint, and receipt integrity
  L1 · auto-discovered · 2 checks
  Delivery impact: 2 evaluated, 0 rejected, 2 checkpoints verified

Accept and apply this verified change? [y/N]
```

An `ADMITTED` result covers only the checks listed in `seal.yaml`. Host trust,
specification completeness, and remote CI remain explicit residual risks.
Use `seal status`, `seal explain`, or `seal run --json` to inspect the same
coverage, provenance and delivery-impact fields after the interactive run.

## Inspect all runs visually

Start the local console from any directory:

```bash
seal ui
```

The browser opens a Runs overview across all repositories known to the current
OS user. Search or filter the list, then select a run to inspect:

- the state-provenance DAG from task intent through candidate, verification,
  checkpoint, recovery, final decision, and explicit apply;
- fresh verifier evidence and bounded command output;
- the hash-chain-validated event timeline;
- the completion receipt, rule, disposition, exact checkpoint, and residual
  reason.

The view refreshes from the append-only ledger while a run is active. It is
read-only and binds to a random `127.0.0.1` port by default. For terminal-only
or remote environments, use `seal ui --no-open`; the printed URL can be opened
on the same machine. StateSeal deliberately rejects non-loopback
`--address` values.

## Codex Plugin and Desktop path

The StateSeal Plugin packages its intent-routing Skill and MCP registration.
Install StateSeal Core first; no separate MCP installation is required. From a
source checkout, add the bundled marketplace and Plugin:

```bash
codex plugin marketplace add /path/to/stateseal
codex plugin add stateseal@stateseal
```

`marketplace add` registers the local source checkout as a Plugin catalog for
this Codex installation. It does not publish the Plugin, upload the repository,
create an account, or install a second MCP server. Installed marketplace
releases can remove this source-development step.

Use `seal install codex-desktop` when Plugins are unavailable or when repairing
an existing installation.

After Plugin installation (or `seal install codex-desktop`), restart Codex
Desktop, choose the local environment, open a Git repository or a workspace
containing Git repositories, and describe the goal normally:

```text
Add input validation, preserve compatibility, and include tests
```

The MCP server first inspects the project without changing it. Explanation,
search, and planning remain outside controlled delivery. On the first
code-changing task, Codex shows the detected admission checks, completion
checks, protected paths, and residual risks; StateSeal then opens native
confirmation for `enable_project`.
The same approved commit also installs the protected repository-local lifecycle
hooks. StateSeal does not ask again unless that policy changes.

The isolated child Agent then runs the same authoritative workflow as
`seal run --no-apply --json`, while the source workspace remains unchanged.
When verification succeeds, the conversation reports the phase timeline,
changed files, checks and durations, exact receipt and code state, coverage,
and residual risks. StateSeal opens a second native confirmation before
`apply_verified`; the supplied session and receipt must exactly match a
non-empty admitted checkpoint. Rejecting it leaves the branch unchanged. The
user never needs to type a `StateSeal:` prefix or internal `seal desktop`
command.

If the opened folder is not itself a Git repository, StateSeal discovers child
repositories. One unambiguous child is selected automatically; multiple
repositories are shown for explicit selection. CLI users can inspect them with
`seal workspace list` and select one with `seal run --repo <path> "…"`.

StateSeal infrastructure failures never masquerade as verifier rejections.
They return a visible degraded result and allow the Agent to continue its
native workflow with an `UNVERIFIED` label and no receipt. A healthy
enforce-mode rejection remains authoritative. Final Apply always requires
explicit approval; unavailable or declined approval preserves the checkpoint
and leaves the source branch unchanged.
