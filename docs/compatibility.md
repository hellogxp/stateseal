# Agent compatibility

StateSeal separates generated adapter-contract tests from live Agent validation.
An adapter is listed as `lifecycle` only after its public hook surface captures
an intermediate candidate and its outer managed run certifies the terminal
result. A missing hook always degrades to terminal recertification; it never
grants admission.

Support levels are evidence-based:

- `supported`: static conformance, live lifecycle, recovery, and a maintained version range;
- `verified`: a pinned version passed live lifecycle and recovery validation;
- `experimental`: configuration and deterministic contract tests pass, but live evidence is incomplete;
- `contract-only`: a generated mapping exists but has not passed live Agent validation.

| Agent surface | Validated version | Coverage | Level | Notes |
| --- | --- | --- | --- | --- |
| Codex CLI | 0.144.5 | `intermediate + terminal` | verified | `PostToolUse` preserved a passing checkpoint; terminal regression recovered under `CP001` |
| Codex Desktop | Pending | lifecycle configuration | experimental | User-level integration exists; isolated Desktop session routing is not yet certified |
| Claude Code CLI / IDE | Pending | contract only | experimental | Non-destructive user/project configuration and static doctor pass; live validation required |
| Qoder CLI / IDE / JetBrains | Pending | contract only | experimental | Official shared hook configuration, blocking `Stop`, and retry guard are implemented; live validation required |
| Gemini CLI | Pending | contract only | contract-only | Live validation required |
| Cursor Agent | Pending | contract only | contract-only | Live validation required |
| GitHub Copilot CLI | Pending | contract only | contract-only | Live validation required |
| OpenCode | Pending | terminal enforcement | contract-only | `session.idle` is observational; outer managed run remains authoritative |

Run the opt-in Codex validation from a machine with an authenticated Codex CLI:

```bash
make live-codex
```

The test invokes a real model and therefore is intentionally excluded from the
deterministic default CI suite. It creates a temporary repository, installs the
generated project hook, bypasses interactive hook trust only for that vetted
temporary hook, and removes the repository when complete.

Static user-integration checks do not invoke a model:

```bash
seal integrate status
seal integrate doctor codex-desktop
seal integrate doctor qoder
```

They validate configuration shape and required lifecycle entries only. They do
not promote an integration to `verified`.
