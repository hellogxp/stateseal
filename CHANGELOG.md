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

### Security

- Authority state is stored outside the repository and protected paths are
  rejected before admission.
- Verifier processes do not inherit Agent-only secrets and timed-out process
  groups are terminated.
