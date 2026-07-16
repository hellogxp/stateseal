# Threat model

StateSeal separates reliability claims from security claims.

## Covered in v0alpha1

The primary adversary is an honest-but-fallible coding agent. StateSeal is designed to prevent these failures from becoming an admitted result:

- evidence replayed against a different code state;
- code modified after a successful verification;
- failed or malformed candidates replacing the last verified checkpoint;
- protected policy paths modified by a proposal;
- completion reported without fresh checks of the selected checkpoint;
- an exported receipt edited after issuance;
- concurrent local brokers racing on the same task ledger.
- verifier or agent subprocesses surviving a timeout or normal parent exit.

The broker recomputes state identity, executes checks outside the agent process, terminates dedicated agent and verifier process groups, records a hash-chained append-only ledger, and stores authority outside the repository.

For Node projects, StateSeal may link an existing, Git-ignored `node_modules` directory into managed worktrees. The link target is broker-created, excluded from candidate identity, and its installed lock metadata contributes to the environment digest. This is dependency reuse, not dependency isolation or supply-chain attestation.

## Not covered in v0alpha1

- A malicious process with the same OS-user permissions can tamper with local files or broker state.
- Git worktrees do not isolate the network, processes, credentials, or kernel.
- A passing test suite does not establish specification completeness.
- Receipt integrity does not establish trusted execution or CI authority.
- `execution.network: inherit` is descriptive; StateSeal does not claim network isolation.

Run untrusted code in an appropriate sandbox. For protected branches, use a clean CI checkout, trusted workflow definitions, least-privilege credentials, fresh completion checks, and branch protection.

## Authority

Only the broker advances `VERIFIED` and issues `ADMITTED`. An agent can create a candidate but cannot supply the decisive verdict. Exported receipts are evidence artifacts. CI must recompute its own verdict and must not infer authority from a receipt included in a pull request.
