# Reliability rule taxonomy

StateSeal receipts and ledger decisions use stable rule IDs. The free-form
`reason` remains diagnostic text; integrations should key policy and reporting
on `rule_id`.

| Family | Rule | Meaning |
| --- | --- | --- |
| State binding | `ST001` | Candidate changed while it was being verified |
| State binding | `ST002` | Trusted base changed during or after admission |
| State binding | `ST003` | Candidate escaped the repository boundary |
| Verification | `VR001` | A configured verifier failed |
| Verification | `VR002` | A configured verifier timed out |
| Verification | `VR003` | Verifier execution could not complete |
| Protected state | `PV001` | Protected path changed without an active exception |
| Protected state | `PV002` | Policy changed after admission |
| Protected state | `PV003` | Receipt integrity validation failed |
| Checkpoint | `CP001` | Terminal regression was replaced by a freshly recertified checkpoint |
| Checkpoint | `CP002` | No usable verified checkpoint was available |
| Checkpoint | `CP003` | Checkpoint state could not be reproduced |
| Checkpoint | `CP004` | Fresh checkpoint recertification failed |
| Lifecycle | `LC001` | Agent wall-time budget was exhausted |
| Lifecycle | `LC002` | Candidate budget was exhausted |
| Lifecycle | `LC003` | Agent repeated or oscillated between rejected states without progress |
| Lifecycle | `LC004` | A code-changing task produced no deliverable change |
| Execution | `EX001` | Agent process exited with an error |

An admitted terminal candidate normally has no rule ID. An admitted recovery
does have a rule ID because it records why StateSeal selected an earlier
checkpoint instead of the terminal candidate.

Mode disposition is orthogonal to the verdict:

| Mode | Failed verdict disposition | Process behavior |
| --- | --- | --- |
| `shadow` | `OBSERVED` | Report the counterfactual decision and return success |
| `warn` | `OVERRIDDEN` | Warn, record the override, and return success |
| `enforce` | `BLOCKED` | Return the verdict's stable non-zero exit code |

Every admitted verdict has disposition `ALLOWED`.
