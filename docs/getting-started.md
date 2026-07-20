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

StateSeal merges lifecycle entries into the Agent's existing user
configuration, writes a one-time safety backup, and performs a static
conformance check. It does not replace unrelated hooks. Codex Desktop uses
`SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, and `Stop` to
bind the conversation, prevent direct source-worktree editing during a managed
turn, and return the delivery decision to the same conversation.

Codex requires a native trust review for non-managed command hooks. Open a new
Codex session, run `/hooks`, inspect the exact StateSeal definition, and trust
it once. StateSeal does not bypass this review for ordinary Desktop use.

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

After `seal integrate codex-desktop`, open the Git repository in Codex Desktop
and use the explicit marker once per managed task:

```text
StateSeal: Add input validation, preserve compatibility, and include tests
```

StateSeal binds the Codex `session_id`, repository, goal, and generated task id
in authority state outside the repository. On the first project task, Codex
shows the detected admission and completion commands and asks for confirmation.
After confirmation, the parent Desktop turn can invoke only the exact StateSeal
delegation command; direct editing tools are denied. The isolated child Agent
runs the same workflow as `seal run --no-apply --json`.

When verification succeeds, the Desktop conversation reports the receipt and
residual risks and asks whether to apply. A plain `yes` applies the exact
checkpoint to a feature branch; `no` leaves the user branch unchanged. Internal
`seal desktop` commands are adapter-facing recovery surfaces and normally do
not need to be typed by the user.
