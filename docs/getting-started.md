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

## 4. Wait for the delivery result

StateSeal creates an isolated proposal, starts the Agent, independently checks
candidate boundaries, preserves the last verified checkpoint, and performs a
fresh terminal recertification. The user decides whether to apply an admitted
checkpoint:

```text
StateSeal · Delivery result
✓ Ready to deliver

Delivery evidence
  ✓ candidate produced in an isolated workspace
  ✓ checks executed independently by StateSeal
  ✓ delivered code exactly matches verified state
  ✓ final state recertified in a fresh evaluator

Accept and apply this verified change? [y/N]
```

An `ADMITTED` result covers only the checks listed in `seal.yaml`. Host trust,
specification completeness, and remote CI remain explicit residual risks.

## Codex Desktop path

After `seal integrate codex-desktop`, restart Codex Desktop, choose the local
environment, open the Git repository, and describe the goal normally:

```text
Add input validation, preserve compatibility, and include tests
```

The MCP server first inspects the project without changing it. On first use,
Codex shows the detected admission checks, completion checks, protected paths,
and residual risks, then opens its native approval UI for `enable_project`.
StateSeal commits the approved contract as `seal.yaml`; it does not ask again
unless that policy changes.

The isolated child Agent then runs the same authoritative workflow as
`seal run --no-apply --json`, while the source workspace remains unchanged.
When verification succeeds, the conversation reports the exact receipt,
coverage, checks, and residual risks. A second native approval gates
`apply_verified`; the supplied session and receipt must exactly match the
admitted checkpoint. Rejecting it leaves the branch unchanged. The user never
needs to type a `StateSeal:` prefix or internal `seal desktop` command.
