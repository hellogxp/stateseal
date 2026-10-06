# Observation and evidence model

StateSeal separates source facts, reproducible correlations, and diagnostic
hypotheses.

## Sources

1. **Agent transcripts** — session, request, tool call/result, timing, and completion events.
2. **Observed mutations** — supported patch/write events and their affected paths.
3. **Recognized checks** — test, build, lint, typecheck, and static-analysis commands.
4. **Git workspace metadata** — branch, commit, dirty-state digest, and diff statistics.
5. **Artifacts** — supported reports, documents, images, and archives created in-session.

StateSeal does not rely on model self-report to establish the code state or a
check result. It uses recorded events and local read-only metadata.

## Evidence grades

| Grade | Requirement | Example |
| --- | --- | --- |
| Observed | Directly present in a supported event | A command reported exit code 1 |
| Derived | Deterministically reproducible | A passing check predates a later edit |
| Inferred | Plausible but not proven | Current Git changes likely belong to this session |

The UI never silently upgrades Derived or Inferred information to Observed.

## State and freshness

An ordered mutation event advances the observed code revision from `S0` to
`S1`, `S2`, and so on. A check is attached to the revision at which its command
started. If the transcript later advances to another revision, that check is
marked stale.

This sequence-based model is intentionally conservative. It can detect
verify-then-edit patterns but cannot prove that unobserved filesystem activity
did or did not occur. Git metadata is therefore shown separately from
transcript attribution.

## Claim limits

StateSeal can state that a recognized command ran, what exit code was observed,
which code revision preceded it, and whether later edits occurred. It cannot
conclude that:

- all requirements were covered;
- a passing test proves semantic correctness;
- the Agent understood the request;
- a completion claim is an approval;
- an inferred causal explanation is certain.

## Privacy and integrity

The supported UI is local and GET-only. Generic tool arguments are summarized,
displayed outputs are bounded, common secret patterns are redacted, and parse
gaps are surfaced as integrity warnings. StateSeal never sends a response,
decision, prompt, or action back to the Agent.
