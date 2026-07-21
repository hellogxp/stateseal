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

Install a release binary and confirm its build identity:

```bash
seal version
```

## 2. Integrate an Agent

Choose the Agent surface used on this computer:

```bash
seal integrate codex-desktop
# or: seal integrate claude-code
# or: seal integrate qoder
```

For Codex Desktop, StateSeal registers a local MCP server in
`~/.codex/config.toml`, writes a one-time safety backup, and performs both static
validation and a live MCP initialize/tool handshake. Existing models, projects,
MCP servers, and unrelated hooks are preserved. No `/hooks` command is needed.

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

## 3. Start the first task in a project

Run StateSeal from the Git repository root:

```bash
cd /path/to/your-project
seal run "Add input validation, preserve compatibility, and include tests"
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

## Codex Desktop path

After `seal integrate codex-desktop`, restart Codex Desktop, choose the local
environment, open the Git repository, and describe the goal normally:

```text
Add input validation, preserve compatibility, and include tests
```

The MCP server first inspects the project without changing it. Explanation,
search, and planning remain outside controlled delivery. On the first
code-changing task, Codex shows the detected admission checks, completion
checks, protected paths, and residual risks; StateSeal then opens native
confirmation for `enable_project`.
StateSeal commits the approved contract as `seal.yaml`; it does not ask again
unless that policy changes.

The isolated child Agent then runs the same authoritative workflow as
`seal run --no-apply --json`, while the source workspace remains unchanged.
When verification succeeds, the conversation reports the phase timeline,
changed files, checks and durations, exact receipt and code state, coverage,
and residual risks. StateSeal opens a second native confirmation before
`apply_verified`; the supplied session and receipt must exactly match a
non-empty admitted checkpoint. Rejecting it leaves the branch unchanged. The
user never needs to type a `StateSeal:` prefix or internal `seal desktop`
command.
