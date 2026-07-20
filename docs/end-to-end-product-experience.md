# End-to-end product experience: verified Codex delivery

This report records a complete StateSeal product journey from an unmet
requirement to a verified checkpoint, user-controlled application, independent
revalidation, and a successful remote push. It is an execution record, not a
scripted mock.

Internal repository addresses and personal identity values are redacted. The
commit, checkpoint, tree, receipt, and StateSeal source identifiers are retained
so that the state transitions remain auditable.

## Executive result

The core delivery path completed successfully:

```text
unmet requirement
  -> isolated Codex proposal
  -> protected acceptance tests
  -> clean verification
  -> admitted checkpoint
  -> user inspection
  -> exact checkpoint apply
  -> tests, race detector, and vet
  -> remote push
```

The implementation result was correct and pushable. Two product-state defects
remain visible after successful delivery:

1. the same candidate was finalized three times, producing three receipts; and
2. `seal status` reported `STALE` after the verified checkpoint was applied.

The experience therefore passes the functional delivery gate but does not yet
pass the release-quality UX gate.

## Test identity

| Item | Value |
| --- | --- |
| Date | 2026-07-17 |
| StateSeal source | `db72814ae1f092966a4fa50fec95517c5a5aa5fa` |
| StateSeal version | `v0.0.0-20260717090553-db72814ae1f0` |
| Platform | `darwin/arm64` |
| Go | `go1.26.3` |
| Agent | Codex CLI `0.144.5` |
| Model | `gpt-5.5`, medium reasoning |
| Upstream project | `github.com/google/uuid` |
| Upstream commit | `2d3c2a9` |
| Acceptance baseline | `1653da9` |
| Policy commit | `d061a9e` |
| Verified checkpoint commit | `b0ed47114fcaac416bd7b3d1fb2b950284c58066` |
| StateSeal checkpoint | `cp_d4bb816bc74b5b779a9b05c1` |
| StateSeal tree | `b90c221f8c2a` |
| Selected receipt | `rcpt_46485983ae98cbaa9f630ba2` |
| Delivery branch | `feature/parse-canonical-product-test` |

The observed Agent run took approximately 94 seconds from process start to the
last admission event and used 29,479 Agent tokens. This is an environment record,
not a performance benchmark.

## Scenario

The upstream `uuid.Parse` function intentionally accepts several UUID text
representations. The requested feature was a separate strict API for systems
that use UUID text as a stable database or cache key.

The user requirement was:

> Add `ParseCanonical(string) (UUID, error)`. Accept only lowercase, hyphenated
> RFC 9562 UUID text. Reject uppercase, compact, URN, braced, and
> whitespace-padded representations. Preserve the existing permissive `Parse`
> behavior.

Acceptance criteria:

| Criterion | Expected |
| --- | --- |
| Lowercase hyphenated UUID | accepted |
| Uppercase UUID | rejected |
| Compact UUID | rejected |
| `urn:uuid:` representation | rejected |
| Braced representation | rejected |
| Leading or trailing whitespace | rejected |
| Existing permissive `Parse` behavior | unchanged |
| Existing and new tests | pass |
| Requirement, tests, policy, and hooks | not modified by the Agent |

## 1. Start from an unmet requirement

The test began from a fresh clone of the prepared acceptance baseline:

```sh
git clone --branch experience/uuid-canonical-baseline \
  --single-branch <repository-url> uuid
cd uuid
go test ./...
```

Observed output:

```text
# github.com/google/uuid [github.com/google/uuid.test]
./canonical_acceptance_test.go:12:14: undefined: ParseCanonical
./canonical_acceptance_test.go:36:17: undefined: ParseCanonical
FAIL    github.com/google/uuid [build failed]
FAIL
```

This established that the requested API did not exist before the Agent run.
The unmodified upstream suite had passed before the acceptance test was added.

## 2. Confirm the StateSeal build

Input:

```sh
seal version
```

Observed output:

```text
StateSeal v0.0.0-20260717090553-db72814ae1f0
Commit:    db72814ae1f092966a4fa50fec95517c5a5aa5fa
Built:     2026-07-17T09:05:53Z
Go:        go1.26.3
Platform:  darwin/arm64
```

The target repository already had a valid repository-scoped Git identity. The
identity values are redacted here; the before/after values were compared during
the test.

## 3. Declare the goal and install the Codex adapter

Input:

```sh
seal init --goal \
  "Add ParseCanonical to accept only lowercase hyphenated RFC 9562 UUIDs while preserving Parse compatibility."

seal adapter codex install
```

Observed output:

```text
StateSeal initialized

Policy:    <repository>/seal.yaml
Task:      uuid
Verifier:  go test
Mode:      enforce (default)

Codex adapter installed at <repository>/.codex/hooks.json
Commit the file as protected infrastructure before the first managed run.
```

The user reviewed the generated policy and added the requirement and acceptance
test to the protected paths:

```yaml
task:
  id: uuid
  goal: Add ParseCanonical to accept only lowercase hyphenated RFC 9562 UUIDs while preserving Parse compatibility.

state:
  include:
    - "**"
  protected:
    - seal.yaml
    - REQUIREMENT.md
    - canonical_acceptance_test.go
    - .git/**
    - .codex/**

admission:
  checks:
    - id: tests
      command: [go, test, ./...]

completion:
  checks:
    - id: tests
      command: [go, test, ./...]
  recertify_latest_checkpoint: true
  on_missing_evidence: abstain
  on_stale_evidence: reject
```

Input:

```sh
seal doctor
git add seal.yaml .codex/hooks.json
git commit -m "Configure StateSeal admission policy"
```

Observed output:

```text
✓ Git repository
✓ seal.yaml
✓ git executable
Platform: darwin/arm64

[experience/uuid-canonical-baseline d061a9e] Configure StateSeal admission policy
 2 files changed, 79 insertions(+)
 create mode 100644 .codex/hooks.json
 create mode 100644 seal.yaml
```

## 4. Trust the project hooks

A normal interactive user starts Codex once, runs `/hooks`, reviews the generated
project hooks, and grants trust.

This test was unattended. It used Codex's
`--dangerously-bypass-hook-trust` flag only to replace that interactive click.
It did not bypass StateSeal policy, protected paths, verification, completion
recertification, or the final user-controlled apply step. Users should prefer
the interactive `/hooks` review.

## 5. Run the Agent inside StateSeal

Exact command:

```sh
seal run -- codex exec \
  --ephemeral \
  --dangerously-bypass-hook-trust \
  -s workspace-write \
  "Implement the requirement in REQUIREMENT.md. Add exported ParseCanonical(string) (UUID, error). It must accept only lowercase hyphenated canonical RFC 9562 UUID text, reject all non-canonical representations covered by canonical_acceptance_test.go, preserve the existing permissive Parse behavior, and run go test ./.... Do not modify REQUIREMENT.md, any test file, seal.yaml, or agent hook configuration."
```

StateSeal created an isolated proposal worktree:

```text
Proposal worktree: <user-cache>/stateseal/f4b9e984eefaf906/uuid/proposal-d061a9e489f9
```

Representative Agent output:

```text
I’ll inspect the repo shape and the requirement/tests first, then patch only
the implementation files needed. After that I’ll run the full Go test suite.

The implementation is scoped to uuid.go and util.go. I’m running gofmt and
then the full suite requested by the requirement.

ok      github.com/google/uuid  0.997s

Verification: go test ./... passes.
```

The Agent added `ParseCanonical` in `uuid.go` and a lowercase hexadecimal helper
in `util.go`. It did not modify the protected requirement, acceptance test,
policy, or hook configuration.

## 6. Observe StateSeal admission

Terminal output:

```text
StateSeal — ADMITTED
Receipt: rcpt_46485983ae98cbaa9f630ba2
Tree:    b90c221f8c2a
Mode:    enforce (ALLOWED)
Residual risk:
  - Only configured checks were evaluated.
  - The execution host was not independently attested.
```

Input:

```sh
seal status
seal explain
```

Observed output before apply:

```text
Task:       uuid
Status:     ADMITTED
Freshness:  CURRENT
Mode:       enforce
Coverage:   intermediate + terminal
Checkpoint: cp_d4bb816bc74b5b779a9b05c1
Tree:       b90c221f8c2a
Receipt:    rcpt_46485983ae98cbaa9f630ba2

Verdict: ADMITTED
Next: inspect with `seal diff`, then use `seal apply`
```

`seal diff` showed changes only in:

```text
M util.go
M uuid.go
```

The checkpoint commit was:

```text
commit=b0ed47114fcaac416bd7b3d1fb2b950284c58066
tree=373059cdb18e53ef45e68c9845936c4b8e708978
author=<repository-configured identity>
committer=<repository-configured identity>
subject=Checkpoint candidate state
```

The repository identity was unchanged after checkpoint creation. This validates
the checkpoint identity fix in StateSeal source commit `db72814`.

## 7. Inspect receipt integrity

The run produced three receipts for the same StateSeal tree:

```text
rcpt_277766154836e91df3817c54
rcpt_f6ef1b319b21a399cc4a364a
rcpt_46485983ae98cbaa9f630ba2
```

Each receipt passed structural integrity inspection:

```text
Receipt <id> is structurally intact.
Verdict: ADMITTED
Tree:    b90c221f8c2a
Note: local receipt integrity does not establish CI authority.
```

The shared tree proves the duplicated receipts refer to the same candidate. The
duplication is a lifecycle coordination defect, not three different admitted
implementations.

## 8. Apply the exact verified checkpoint

Input:

```sh
seal apply --branch feature/parse-canonical-product-test
```

Observed output:

```text
Verified checkpoint applied.
```

The applied HEAD was exactly the verified checkpoint:

```text
b0ed471 Checkpoint candidate state
d061a9e Configure StateSeal admission policy
1653da9 Add canonical UUID acceptance specification
2d3c2a9 feat: Generate V6 from custom time (#172)
```

No amend, squash, or identity rewrite occurred between verification and apply.

## 9. Revalidate the applied branch

Input:

```sh
go test ./...
go test -race ./...
go vet ./...
git diff --check d061a9e..HEAD
```

Observed output:

```text
ok      github.com/google/uuid  1.478s
ok      github.com/google/uuid  6.274s
```

`go vet` and `git diff --check` completed without output or error. The final
branch remained clean.

The receipt binds the configured `go test ./...` completion check. The race and
vet commands above were additional post-apply validation and are not claimed as
receipt-bound evidence.

## 10. Push the delivered branch

Input:

```sh
git push -u origin feature/parse-canonical-product-test
```

Observed result:

```text
To <repository-url>
 * [new branch] feature/parse-canonical-product-test
     -> feature/parse-canonical-product-test
branch 'feature/parse-canonical-product-test' set up to track
  'origin/feature/parse-canonical-product-test'.
```

Remote verification returned the same commit:

```text
b0ed47114fcaac416bd7b3d1fb2b950284c58066
refs/heads/feature/parse-canonical-product-test
```

This closes the functional path from unmet requirement to pushable verified
state.

## Acceptance result

| Check | Result | Evidence |
| --- | --- | --- |
| Started from unmet requirement | PASS | baseline compile failure |
| Agent ran outside the user's branch | PASS | managed proposal worktree |
| Requirement and acceptance test protected | PASS | final changed-path check |
| Existing permissive behavior preserved | PASS | acceptance and upstream tests |
| Configured verifier passed | PASS | `go test ./...` |
| Fresh completion recertification | PASS | `COMPLETION_RECERTIFIED` events |
| Exact state bound to checkpoint | PASS | checkpoint `b0ed471`, StateSeal tree `b90c221f8c2a` |
| Receipt integrity | PASS | three successful `seal inspect` results |
| Git identity preserved | PASS | repository identity unchanged before/after |
| User controlled apply | PASS | explicit `seal apply` |
| Post-apply test, race, vet | PASS | commands completed successfully |
| Remote push | PASS | remote branch points to `b0ed471` |
| Single terminal decision | FAIL | three equivalent admission cycles |
| Correct post-apply status | FAIL | status reported `STALE` after exact apply |

## Observed product defects

### P0: applied checkpoint is reported as stale

Immediately after successful apply, `seal status` returned:

```text
Status:     ADMITTED
Freshness:  STALE
Stale:      trusted base changed after admission
Checkpoint: cp_d4bb816bc74b5b779a9b05c1
```

This is misleading. HEAD was the exact checkpoint commit. The product should
recognize this transition and report an applied state, for example:

```text
Status:     APPLIED
Freshness:  CURRENT
```

### P1: duplicate terminal admission

The integrity-verified timeline contained 18 events: the same six-event
admission sequence was emitted three times. `PostToolUse`, `Stop`, and the outer
`seal run` fallback all finalized the same tree.

The target behavior is one intermediate verified boundary followed by one fresh
terminal completion decision, with same-tree submissions deduplicated.

### P2: Agent output obscures the decision

Codex environment warnings and repeated lifecycle hook messages dominate the
terminal. StateSeal should visually separate:

```text
Agent activity
Verification activity
Final decision
Next user action
```

## Trust interpretation

The `ADMITTED` verdict supports this precise statement:

> The checkpoint represented by StateSeal tree `b90c221f8c2a` satisfied the
> configured protected-path and `go test ./...` policy in fresh evaluation, and
> the exact checkpoint commit was applied and pushed without identity rewrite.

It does not prove that the requirement or tests are complete, that the host is
uncompromised, or that a local receipt carries CI authority. Those residual
risks were preserved in the receipt and terminal output.

## Product verdict

StateSeal demonstrated its intended core value on a real open-source codebase:
the Agent proposed a change, external evidence decided admission, Git preserved
the exact state, and the user explicitly applied and pushed that state.

The core workflow is operational. The post-apply state model and lifecycle
deduplication should be fixed before presenting the CLI as a polished alpha
experience to ordinary users.
