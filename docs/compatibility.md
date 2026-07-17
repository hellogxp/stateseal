# Agent compatibility

StateSeal separates generated adapter-contract tests from live Agent validation.
An adapter is listed as `lifecycle` only after its public hook surface captures
an intermediate candidate and its outer managed run certifies the terminal
result. A missing hook always degrades to terminal recertification; it never
grants admission.

| Agent | Validated version | Coverage | Live result | Notes |
| --- | --- | --- | --- | --- |
| Codex CLI | 0.144.5 | `intermediate + terminal` | PASS | `PostToolUse` preserved a passing checkpoint; terminal regression recovered under rule `CP001` |
| Claude Code | Pending | contract only | Pending | Live validation required |
| Gemini CLI | Pending | contract only | Pending | Live validation required |
| Cursor Agent | Pending | contract only | Pending | Live validation required |
| GitHub Copilot CLI | Pending | contract only | Pending | Live validation required |
| OpenCode | Pending | contract only | Pending | Live validation required |

Run the opt-in Codex validation from a machine with an authenticated Codex CLI:

```bash
make live-codex
```

The test invokes a real model and therefore is intentionally excluded from the
deterministic default CI suite. It creates a temporary repository, installs the
generated project hook, bypasses interactive hook trust only for that vetted
temporary hook, and removes the repository when complete.
