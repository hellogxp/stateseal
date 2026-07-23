# Research and evidence boundary

StateSeal is informed by empirical studies of iterative code repair and by
adjacent open-source systems. Those inputs shape the problem decomposition;
repository tests and compatibility evidence determine what this implementation
can claim.

## What the research supports

The current evidence supports treating the following as explicit engineering
obligations:

- evidence must be bound to the exact code, policy and verifier suite;
- stale evidence must not authorize a changed state;
- the last verified checkpoint must survive a later regression;
- terminal completion requires fresh recertification;
- the Agent must not own its own final admission verdict;
- coverage and residual risk must be disclosed;
- safety and delivery liveness must be evaluated separately.

The research also provides failure mechanisms and experimental designs that
inform SealBench, including stale-vs-current evidence, verify-then-edit,
checkpoint regression, gate errors, false rejection, and loop-without-progress.

## What the research does not prove

StateSeal does not claim that:

- more loop depth improves task success;
- the bundled verifier policy is universally complete;
- blocking more candidates necessarily improves useful reliability;
- StateSeal makes an Agent more capable or intelligent;
- the paper is an end-to-end effectiveness evaluation of this product.

The paper's orchestration contract is best read as an auditable specification
and source of testable obligations. Product effectiveness must be established
separately through repository regression tests, SealBench failure injection,
pinned Agent conformance, and real project evaluation.

## Evidence levels used in project documentation

| Level | Meaning |
| --- | --- |
| Protocol obligation | A required invariant or failure boundary |
| Deterministic engineering test | Reproducible repository or SealBench evidence |
| Live compatibility evidence | A pinned external Agent version passed a maintained scenario |
| Product outcome evidence | Comparative data about useful delivery, false rejection, overhead and operator outcomes |

Only the first three are currently mature. Product outcome evidence remains a
roadmap item; until then, documentation must not turn mechanistic evidence into
a broad success-rate claim.
