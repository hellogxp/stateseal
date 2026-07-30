# Observation and attribution model

StateSeal separates data origin from analytical confidence.

## Sources

1. **Skill files**: declared name, description, scripts, references, assets.
2. **Agent transcripts**: session, message, tool call, tool result, timing.
3. **Local artifacts**: file and Git metadata when explicitly available.
4. **Correlation**: relationships derived from IDs, paths, timestamps, and
   deterministic rules.

## Evidence grades

- **Observed**: copied from a supported source event.
- **Derived**: reproducible correlation over observed data.
- **Inferred**: a model or heuristic diagnosis.

The UI must never silently upgrade Derived or Inferred information to Observed.

## Claim limits

StateSeal can say that a command was called or reported exit code zero. It
cannot conclude that the model understood an instruction, that an unobserved
Skill was considered, or that a change is semantically correct.

## Non-intervention guarantee

The supported product has no Agent launcher, lifecycle-hook installer, prompt
rewriter, deny/block response, branch writer, approval handler, or Apply
endpoint. Compatibility commands left by older releases return neutral results.
