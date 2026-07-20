# Changelog

All notable changes to StateSeal are documented in this file. The project uses
semantic versioning while protocol compatibility is tracked independently by
the version fields in its JSON schemas.

## Unreleased

### Added

- Transactional admission CLI and external broker.
- State-bound evidence, verified checkpoints, and fresh completion recertification.
- Recovery of the last verified checkpoint after terminal regression.
- Native lifecycle adapters for Codex, Claude Code, Gemini CLI, Cursor Agent,
  GitHub Copilot CLI, and OpenCode.
- Deterministic SealBench failure-injection and stack compatibility suites.
- Cross-platform release artifacts with embedded build identity and checksums.
- Actionable initialization output, explicit task goals, and repository-safe
  task identifiers.
- A deterministic, disposable regression-recovery experience that exercises
  checkpoint preservation and fresh terminal recertification end to end.
- Managed language-tool caches for sandboxed Agents, keeping generated build
  state outside candidate identity while preserving writable test execution.
- Live Codex lifecycle validation and a versioned compatibility matrix that
  distinguishes runtime evidence from generated adapter contracts.
- A one-command `seal run "goal"` experience with Agent detection, confirmed
  first-run setup, external project preferences, Chinese output, quiet progress,
  one-step delivery, and professional feature-branch commits.
- Layered verifier discovery for tests, build, lint, type checks, and static
  analysis, with explicit coverage gaps and actionable failure output.
- Repeated-state and oscillation detection (`LC003`) so unproductive Agent
  loops stop with a bounded, auditable result.
- Event-driven run progress with elapsed time, candidate boundaries, verifier
  evidence, final recertification, and explicit delivery-basis reporting.
- Automatic system-locale matching with English fallback, plus dedicated
  default, verbose, quiet, and stable JSON output modes.
- User-level `seal integrate` management for Codex, Claude Code, and Qoder,
  including non-destructive merge, safety backup, static doctor, status, and
  selective uninstall.
- Qoder CLI/IDE/JetBrains lifecycle support based on its shared hook contract,
  including the blocking Stop exit code and retry-loop guard.
- A first-project confirmation view that shows the exact admission and
  completion commands before creating auditable project configuration.
- Codex Desktop controlled delegation using official session, prompt, tool, and
  stop hooks; goals remain isolated, direct parent-session edits are denied,
  and verified checkpoints require explicit conversational acceptance.
- Durable Desktop session authority state, per-repository concurrency control,
  parent-turn recovery, and deterministic end-to-end coverage in SealBench.

### Security

- Authority state is stored outside the repository and protected paths are
  rejected before admission.
- Verifier processes do not inherit Agent-only secrets and timed-out process
  groups are terminated.
