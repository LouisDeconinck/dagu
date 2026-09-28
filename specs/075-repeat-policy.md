# Spec 075: Repeat Policy

## Status

Implemented.

This spec defines conformance behavior for the step `repeat_policy` repeat
decision and its status effects.

## Scope

This spec covers:

- repeat modes `while` and `until`
- the repeat decision Dagu makes after each step attempt
- met, not-met, and evaluation-error outcomes of `repeat_policy.condition`
- the `limit` bound on the number of executions
- step status effects of the repeat decision

This spec does not define:

- `repeat_policy` field shapes and normalization, including the legacy
  `repeat` boolean and inferred modes
- `exit_code` repeat matching
- `interval_sec`, `backoff`, and `max_interval_sec` timing
- `retry_policy`, which re-runs a failed attempt and is decided before the
  repeat check
- scheduler, queue, API, UI, or distributed worker behavior

## Goal

Workflow authors can repeat a step with predictable loop semantics where a
broken condition cannot masquerade as a normal loop answer.

## Related Specs

- Preconditions: [Spec 023: Preconditions](023-preconditions.md), which defines
  the condition model a repeat condition uses
- Value resolution: [Spec 003: Value Resolution and Field Evaluation](003-value-resolution.md)

## Terms

A repeat condition is the `repeat_policy.condition` entry of a step.

The repeat decision is the check Dagu makes after a step attempt finishes to
decide whether the step runs again.

## Behavior

### Repeat Decision

Rules:

- Dagu checks the repeat policy after each step attempt finishes.
- `limit` bounds the total number of executions, counting the first attempt.
  Once the count reaches `limit`, the step does not repeat again regardless of
  the repeat decision.
- With `repeat: while`, the step repeats when the repeat condition is met and
  stops when it is not met.
- With `repeat: until`, the step repeats when the repeat condition is not met
  and stops when it is met.
- Without a condition or `exit_code`, `while` repeats while the attempt
  succeeded and `until` repeats until the attempt succeeds.
- Without `limit`, an `until` step whose condition stays not met repeats until
  the run is aborted or times out.

### Repeat Condition Checking

Rules:

- `repeat_policy.condition` follows the Spec 023 condition model: with
  `expected` it is a value-match condition, and without `expected` it is a
  command-check condition.
- The repeat condition is checked after each attempt, not before the first
  attempt.
- A met or not-met result is a normal loop answer.
- An evaluation error is not a loop answer: the step stops repeating and
  reaches terminal status `failed`.
- A repeat-condition evaluation error is a step failure for DAG-run status
  calculation, following normal step-failure rules for dependents and
  `continue_on`.
- If workflow abort or timeout interrupts the check, the step follows the
  abort or timeout outcome instead of treating the interrupted check as a loop
  answer.

## Examples

```yaml
steps:
  - id: poll
    run: ./check-ready.sh
    repeat_policy:
      repeat: until
      condition: "ready"
      expected: "ready"
      interval_sec: 5
```

Expected behavior:

- `./check-ready.sh` runs again while its output is not `ready`.
- When the output is `ready`, the step stops and completes.
- If the output can never be evaluated as written, such as a `num:` expected
  value compared against non-numeric output, the step fails instead of
  repeating.

## Conformance

`conformance/spec075_repeat_policy/` checks the status effects of the repeat
decision.
