# Specification #648 - report_waiting marks a macro run as waiting

- Ticket: https://github.com/sebastienferry/sectile/issues/648
- Branch: `feat/batch-393-497-498-584-647-648`
- Clarification: `docs/clarifications/648.md` (round 1, no open question)
- Framework: Spec Kit

## Summary

`report_waiting` accepts a macro run, named like `start_run` and `finish_run`
name it, and the macro's run indicator in the web shows that the run waits on
its user. Its refusals say whether the run does not exist, belongs to
something else, or has ended.

## Scope

In scope: the MCP input, the DB path for macro runs, the refusals, the macro
run list and indicator in the web, tests and the changelog.

Out of scope: task run waits (unchanged), #498, #647.

## User stories

### US1 (P1) - A macro skill reports its wait

1. Given a macro run started with `projectId` + `macroKey`, when the session
   calls `report_waiting(projectId, macroKey, runId, true)`, then the run
   records a wait and the answer says it was applied.
2. Given that wait, when the session makes another Sectile call, then the
   wait ends.
3. Given that wait, when `report_waiting(..., false)` is called, then the
   wait ends.
4. Given a headless macro run, when it reports a wait, then it is accepted and
   not shown, as for a task run.

### US2 (P1) - The board shows it

1. Given a waiting macro run, when the macro's panel shows its run, then the
   indicator says the run is waiting on its user rather than only running.

### US3 (P2) - Refusals tell the cases apart

1. Given a run id that does not exist, then the error says no run has this
   id.
2. Given a task run's id with a macro target, or a macro run's id with a task
   key, then the error says the run is not attached to that task or macro.
3. Given a finished run, then the error says it is no longer running and
   gives its status.
4. Given both `taskKey` and `macroKey`, or neither, then the call is refused
   as for `start_run`.

## Functional requirements

- FR1. `report_waiting` takes `taskKey`, or `projectId` + `macroKey`, and
  `runId`.
- FR2. The macro path applies the ownership rule and the headless exception
  of the task path.
- FR3. The macro run list returns `waitingSince`; a wait change on a macro
  run reaches the listeners that refresh the web.

## Acceptance criteria

- AC1. US1 and US3 are covered by DB and MCP tests; US2 by a web unit test.
- AC2. A `Fixed` line in `CHANGELOG.md`.

## Open points

None.
