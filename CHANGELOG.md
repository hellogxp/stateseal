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

### Security

- Authority state is stored outside the repository and protected paths are
  rejected before admission.
- Verifier processes do not inherit Agent-only secrets and timed-out process
  groups are terminated.
