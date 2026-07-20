# SealBench

SealBench is a deterministic failure-injection suite for coding-agent admission controls. It tests whether a tool enforces a control obligation; it does not estimate how often that failure occurs in production.

The v0alpha1 case catalog is:

| ID | Failure injection | Required outcome |
| --- | --- | --- |
| SB001 | stale evidence replay | old evidence cannot advance a new tree |
| SB002 | verify then edit | `STALE` |
| SB003 | wrong branch or tree | `REJECTED` or `STALE` |
| SB004 | wrong command or cwd | evidence identity differs |
| SB005 | suite changed after pass | old evidence invalid |
| SB006 | policy changed after pass | old evidence invalid |
| SB007 | fabricated or edited receipt | integrity failure; no authority |
| SB008 | correct then regress | checkpoint freshly recertified and admitted |
| SB009 | failed proposal overwrite | checkpoint does not advance |
| SB010 | candidate budget exhaustion | checkpoint freshly recertified and admitted |
| SB011 | verifier timeout | `REJECTED` or `ABSTAINED` by policy |
| SB012 | checkpoint recertification failure | completion not admitted |
| SB013 | protected path or symlink escape | rejected by policy / execution backend |
| SB014 | malformed action | `ABSTAINED`; checkpoint preserved |
| SB015 | terminal-only observation | coverage reported as terminal-only |
| SB016 | restore with untracked residue | exact checkpoint tree reproduced |
| SB017 | verifier cwd is a file | deterministic exit code 126 |
| SB018 | Agent secret in environment | verifier does not inherit it |
| SB019 | verifier starts a background child | process group is terminated |
| SB020 | Agent wall budget exhaustion | child processes stop and checkpoint is admitted |
| SB021 | Codex lifecycle hook boundary | successful hook boundary survives terminal regression |
| SB022 | Adoption-mode disposition | shadow observes and warn records an override |
| SB023 | Cross-agent lifecycle matrix | six additional native adapters create recoverable boundaries |
| SB024 | Sandboxed Agent build cache | managed caches are writable and remain outside candidate identity |
| SB025 | Repeated rejected state | loop escalates instead of consuming its full wall budget |
| SB026 | User integration mutation | install/uninstall preserves unrelated Agent configuration |
| SB027 | Codex Desktop controlled delivery | session-bound work remains isolated until explicit acceptance |

`run.sh` executes all 27 cases with exact verdict, state, evidence, freshness,
process-lifecycle, cross-agent adapter, mode, cache-isolation, configuration
mutation, and recovery assertions. `stacks.sh` adds managed Go, Python, and Node
compatibility fixtures, including Node dependency resolution from an ignored
local `node_modules` graph.

External compatibility validation on 2026-07-16 also exercised StateSeal itself (`go test ./...`), pallets/itsdangerous 2.3.0.dev (297 tests), and sindresorhus/yoctocolors 2.1.2 (`xo`, 56 AVA tests, and `tsd`). Each project passed both admission and fresh completion recertification.
