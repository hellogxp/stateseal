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

StateSeal merges its `PostToolUse` and `Stop` entries into the Agent's existing
user configuration, writes a one-time safety backup, and performs a static
conformance check. It does not replace unrelated hooks.

Inspect or undo the integration at any time:

```bash
seal integrate status
seal integrate doctor codex-desktop
seal integrate uninstall codex-desktop
```

Desktop and IDE integrations are currently experimental. Installation proves
that the lifecycle configuration is structurally present; it does not prove a
live Agent version or a complete managed Desktop delivery. Use `seal run` for
the authoritative workflow.

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
