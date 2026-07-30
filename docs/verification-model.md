# Verification model

> **Historical research model.** This document is not a description of the
> current observation-only product surface.

StateSeal separates four questions that are often collapsed into “tests
passed”:

1. Which exact code state was evaluated?
2. Which checks ran, where did they come from, and did they pass?
3. What was not evaluated?
4. Did the gate preserve a useful delivery, or merely stop every candidate?

This distinction is part of the product contract, not an optional reporting
feature.

## Coverage layers

| Layer | Responsibility | Typical evidence | Authority |
| --- | --- | --- | --- |
| L0 | State and evidence integrity | tree, policy and suite binding; freshness; checkpoint and receipt integrity | StateSeal broker |
| L1 | Project engineering checks | tests, build, lint, type checks, static analysis | local project policy |
| L2 | Domain acceptance | business invariants, compatibility fixtures, private acceptance suites | project or domain owner |
| L3 | Protected environment gates | remote CI, protected runner, deployment environment, human approval | external authority |

StateSeal always supplies L0 controls. Automatic project discovery proposes a
minimum L1 contract; it never claims to discover a complete specification.
L2 and L3 checks must be declared by the project or connected to an external
authority.

## Verifier provenance

Every configured check has a layer and an origin:

- `auto-discovered`: proposed from portable project metadata;
- `project-policy`: reviewed and owned in `seal.yaml`;
- `external`: executed by an external or protected authority.

Existing v0alpha1 policies remain valid. A check without explicit metadata is
reported as `L1 / project-policy`. Newly discovered checks are written as
`L1 / auto-discovered`.

```yaml
completion:
  checks:
    - id: unit-tests
      command: ["go", "test", "./..."]
      layer: L1
      origin: auto-discovered
    - id: payment-contract
      command: ["./tools/payment-acceptance"]
      layer: L2
      origin: project-policy
```

## Safety and liveness

A system that rejects every candidate can appear perfectly safe while being
useless. StateSeal therefore reports both:

- safety evidence: verdict, exact state, verifier results, freshness and
  receipt integrity;
- delivery impact: candidates evaluated, candidates rejected, checkpoints
  verified, recovery, and checkpoint selection reason.

These fields do not claim that StateSeal caused an Agent to succeed. They make
abstention, false rejection, repeated non-progress, and recovery visible so
operators can evaluate the gate rather than infer quality from a blocked run.

## Receipt interpretation

An `ADMITTED` receipt establishes this bounded statement:

> The identified checkpoint passed the listed verifier evidence under the
> identified policy, and StateSeal reproduced that state at completion.

It does not establish specification completeness, uncompromised host
execution, remote CI success, or production safety unless those obligations
appear as authoritative L2/L3 evidence. The `verification_coverage.uncovered`
and `residual_risks` fields preserve that boundary explicitly.

Learned or LLM-based evaluators may contribute evidence, but they do not own
the admission verdict. The deterministic broker remains the authority that
binds evidence, state, policy and final disposition.
