# Product verification

This guide verifies the supported StateSeal product: a local, read-only Agent
delivery-intelligence layer.

## Product invariants

The supported CLI must:

- register only observational commands;
- never start, steer, block, approve, restore, or apply Agent work;
- keep legacy hook compatibility responses neutral;
- expose local UI data through GET-only APIs;
- distinguish Observed, Derived, and Inferred statements;
- bind recognized checks to observed code revisions and mark stale evidence;
- redact common secret patterns from displayed data.

## Requirements

- Git
- Go 1.24 or newer
- Node.js (JavaScript syntax validation)
- `jq` (locale catalog validation)
- Python 3 (documentation-language validation)

## Verify

```sh
go mod verify
go test -race ./...
go vet ./...
go build -trimpath ./cmd/seal
node --check internal/ui/web/app.js
sh scripts/check-i18n.sh
```

The Go tests include command-surface, inert-compatibility, state-freshness,
secret-redaction, local-server security-header, and API projection checks.

## Manual UI check

```sh
go run ./cmd/seal ui --no-open --address 127.0.0.1:9137
```

Open `http://127.0.0.1:9137/` and confirm that the home view shows delivery
sessions and the detail view shows current state, code delta, evidence graph,
diagnostic findings, changes, checks, artifacts, and audit timeline. There must
be no Agent-control action.

## Claim boundary

Passing these checks establishes implementation conformance to the tested
read-only behavior. It does not establish semantic correctness of an Agent's
code, completeness of its test suite, or correctness of every diagnostic
inference.
