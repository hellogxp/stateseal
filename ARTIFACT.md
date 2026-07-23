# Artifact evaluation

This guide verifies StateSeal from a source snapshot without relying on a
public repository identity or GitHub Release assets. It is intended for
double-anonymous review and also works with the **Full repo ZIP** provided by
Anonymous GitHub.

## Scope

The artifact provides implementation-conformance evidence for the executable
contract. It does not reproduce the paper's empirical results and does not
establish that StateSeal improves repair competence, verifier quality, or
general deployment reliability.

The deterministic checks cover:

- exact state and policy binding;
- stale, copied, or edited evidence;
- checkpoint preservation and fresh completion recertification;
- typed outcomes, rejection, abstention, and escalation;
- false-rejection and weak-gate visibility;
- bounded loop and process-lifecycle behavior;
- managed Go, Python, and Node project fixtures.

## Requirements

- Git
- Go 1.24 or newer
- Bash
- `jq`
- Python 3
- Node.js

The core build and Go test suite need only Git and Go. Python, Node.js, and
`jq` are required by the complete conformance matrix.

## Quick verification

Download and extract the anonymous **Full repo ZIP**, then run:

```sh
make build
make test
make lint
make sealbench
make stackbench
```

Expected terminal summaries include:

```text
SealBench passed 35/35 deterministic failure-injection cases.
Stack matrix passed 3/3 ecosystems.
```

`make test` runs the Go suite with the race detector. `make sealbench` executes
the SB001--SB035 catalog documented in
[`sealbench/README.md`](sealbench/README.md). `make stackbench` exercises
managed Go, Python, and Node fixtures.

## Evidence boundary

These checks demonstrate that the implementation enforces the tested contract
obligations under the declared environment. They do not prove:

- completeness or semantic correctness of a project's verifier suite;
- independence of multiple verifiers;
- trustworthiness of the host or runner;
- improved useful-delivery rate on unseen projects;
- absence of false rejection or abstention costs.

See [`docs/threat-model.md`](docs/threat-model.md),
[`docs/verification-model.md`](docs/verification-model.md), and
[`docs/research-evidence.md`](docs/research-evidence.md) for the complete trust
and claim boundaries.

## Snapshot identity

Record the evaluated source identity with:

```sh
git rev-parse HEAD 2>/dev/null || true
./bin/seal version --json
```

An Anonymous GitHub ZIP may omit Git history, in which case the anonymous
artifact page and the paper's artifact statement identify the frozen snapshot.
