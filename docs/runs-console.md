# Delivery panorama

![StateSeal Delivery Panorama](assets/stateseal-runs-console.svg)

`seal ui` is a read-only projection of supported local Agent delivery sessions.

## Questions it answers

| Question | Surface |
| --- | --- |
| What did the Agent leave behind? | Current state and code-delta cards |
| Which state did each check evaluate? | Delivery evidence graph |
| Did later edits make earlier results stale? | Check freshness and diagnostic findings |
| What reports or documents were produced? | Artifact inventory |
| What failed, and what is the evidence? | Diagnostic banner and finding basis |
| What is fact versus analysis? | Observed / Derived / Inferred labels |

## Information hierarchy

The home view is outcome-first: session status, evidence posture, changed files,
checks, and findings. Tool-level activity is deliberately omitted from the main
graph and remains available in the audit timeline.

The detail view contains:

1. current state and code delta;
2. a primary diagnostic insight;
3. the delivery evidence graph;
4. findings and observed changes;
5. check freshness and bounded output;
6. artifacts and the sanitized audit timeline.

## Status versus evidence posture

- **Observing**: the supported transcript has not emitted a completion event.
- **Agent ended**: the Agent reported completion; this is not StateSeal approval.
- **Needs attention**: an evidence failure, gap, or stale result was detected.

Evidence posture is separate:

- **Current**: a recognized passing check follows the latest observed edit.
- **Failure**: a current-state recognized check reported failure.
- **Stale**: checks passed, but later edits created a newer observed state.
- **Gap**: the Agent ended without a recognized current-state check.
- **Unknown**: available signals are insufficient for a stronger statement.

## Evidence grades

- **Observed** — directly present in a supported source event.
- **Derived** — reproducible correlation over event order, IDs, or paths.
- **Inferred** — a diagnostic hypothesis, never presented as fact.

There is no action arrow back to the Agent. The console contains no block,
approve, apply, restore, or prompt-control action.
