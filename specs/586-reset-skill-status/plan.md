# Plan #586 - Reset skill status

## Stack

- Local agent: Go, `internal/agent`.
- Desktop renderer: plain ES modules under `desktop/src`, tested with
  `node --test` (`*.test.mjs`, `*.ui.cjs` for the Electron-driven ones).
- No server change, no migration.

## Agent: `successor` in `/desktop/run-result` (FR1)

`desktopRunResult` (`internal/agent/agent_desktop.go`) already reads the task
and all of its activities. It also:

1. Under the queue lock, copies the ids of every execution the agent holds,
   and for the requested one whether its process exited and when. A run whose
   `exited` channel is closed and whose `finishedAt` is still zero gets it set
   now: the field is documented as "when the run was first seen to have
   exited", which this read is.
2. Finds the own activity (as today). When it is found with an ended status
   (`completed`, `failed`, `canceled`), picks among the activities the newest
   by `CreatedAt` that has `SkillID == "remote_run"`, the same `TaskID`, a
   `CreatedAt` strictly after the own activity's, an id that is no held
   execution's, and, when the process exited, a `CreatedAt` before
   `finishedAt`.
3. Encodes it as
   `{"id","taskId","skillId": SkillName,"status","waitingSince"}` under
   `successor`, or `null`.

The selection is a pure function, `skillSuccessor(own, activities, held,
exitedAt)`, so it is tested without HTTP.

## Desktop: the result of a successor (FR2, FR3, FR5)

`desktop/src/skill-result.mjs`:

- `workflowSkill(name)`: strips a `<plugin>:` prefix and an `-issue` suffix,
  and returns the name when it is a key of `expected`, else `''`.
- `skillResult(run, result)`: after the console and discussion exits (FR3),
  when `result.successor` is set and the own activity is matched, the verdict
  is computed from the successor:
  - `completed`: `◷ Awaiting stage validation` when `workflowSkill` names a
    stage the task has not reached, else `✓`;
  - `failed`, `canceled`: `!`, `⊘`;
  - otherwise a pending stop, then `?` when `run.status === 'running'` and the
    successor carries `waitingSince`, then nothing.
  The existing branches are untouched when there is no successor (US3).

The header keeps calling `skillResult(selectedRun, cache[selectedRun.id])`
(FR5): a console of path A carries its successor, one of path B does not.

## Desktop: the run a row describes (FR4, FR7)

`desktop/src/task-order.mjs`: `orderedTaskGroups` adds `skillRun` to each
group: the executions that run a skill (`kind !== 'console'`,
`skill !== 'discuss'`), newest by submission time with the existing
tie-break, falling back to `run`. `run` and the order stay as they are.

`desktop/src/main.js`, row rendering: `status.dataset.runId = skillRun.id`.
`renderTaskSkillStatuses` and `refreshVisibleSkillResults` already resolve the
badge's run from `dataset.runId`, so both the read and the cached result
follow it (FR7).

## Desktop: reading a successor in time (FR6)

`desktop/src/skill-result-refresh.mjs`: `skillResultStamp(run, now, last,
ended)` records `ended`, whether the result read showed its skill ended (own
activity or successor). `skillResultDue` returns true at every poll for a live
run whose last stamp says `ended`. `refreshSkillResult` passes it from the
result it just read.

## Tests

- Go: `skillSuccessor` table test (none, newest of two, older than own,
  another held execution, own still running, created after exit); handler test
  extending `TestDesktopSkillResultMatchesOwnedExecution` with a successor and
  with `null`.
- `skill-result.ui.cjs`: successor running, waiting, completed with and
  without the stage, failed, canceled, mapped names, console/discussion
  ignore it, no successor unchanged.
- `task-order.test.mjs`: `skillRun` is the newest skill execution while `run`
  stays the running console; free consoles and discussions are skipped.
- `skill-result-refresh.test.mjs`: a live run with an ended result is due at
  every poll; an ended run is not.
- `task-order-render.ui.cjs` (or a new UI test): path B, the row badge is
  empty for a queued new execution beside a running console whose own result
  is `✓`, and selecting the console still shows `✓` in the header.

## Target files

- `internal/agent/agent_desktop.go`, `internal/agent/agent_skill_result_test.go`
- `desktop/src/skill-result.mjs`, `desktop/src/task-order.mjs`,
  `desktop/src/skill-result-refresh.mjs`, `desktop/src/main.js`
- `desktop/tests/skill-result.ui.cjs`, `desktop/tests/task-order.test.mjs`,
  `desktop/tests/skill-result-refresh.test.mjs`, a desktop UI test for path B
- `CHANGELOG.md`

## Rejected alternatives

- Letting `start_run` adopt an ended runId again: changes the server's run
  lifecycle, out of scope, and would merge two skills' outcomes into one
  activity.
- Linking the successor through a new field set by the agent's MCP bridge:
  needs a server API and migration for a heuristic the owner accepted as is.
- Making the newest execution lead the row: moves rows and changes what a
  click selects, which the clarification kept out.
