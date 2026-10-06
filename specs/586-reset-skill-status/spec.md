# Specification #586 - Reset skill status

- Ticket: https://github.com/sebastienferry/sectile/issues/586
- Branch: `feat/586`
- Clarification: `docs/clarifications/586.md` (rounds 1 and 2, confirmed by
  the owner on 2026-09-28)
- Framework: Spec Kit

## Summary

Sectile Desktop shows, beside a task row and in the execution header, the
result of the skill an execution launched (`✓ Skill completed`, `◷`, `!`,
`⊘`, `?`). When a console is left open after its skill finished and a new
skill then starts on the same task, that badge keeps the previous skill's
verdict. It must describe the skill that is now running instead.

## Scope

In scope: the skill-result badge of a task row and of the execution header in
Sectile Desktop, and the local agent route that feeds it
(`/desktop/run-result`).

Out of scope:

- The run-state badge (whether the process is alive).
- The server's activity lifecycle, `start_run` and its refusal of an ended
  runId.
- The web board's activity list.
- How or when a stage is validated.

## Definitions

- **Execution**: a run the desktop lists, launched by the local agent, with
  its own id, skill and console.
- **Own activity**: the server activity whose id is the execution's id.
- **Successor activity**: a `remote_run` activity of the same task, created
  inside an execution's console after its own activity, by a skill started in
  that console (`start_run` without a runId, since the inherited
  `SECTILE_RUN_ID` is refused once ended).
- **Path A**: a new skill started inside a console that stays open.
- **Path B**: a new execution launched while an older console of the same
  task stays open.
- **Row run**: the execution a task row's skill badge describes.
- **Poll**: one cycle of the desktop's run-list refresh (every two seconds).

## User stories

### US1 - A new skill in the same console (path A), P1

As the owner, when I start `/specify-issue` in the console where
`/clarify-issue` just completed, I want the `✓` to disappear and the badge to
follow `specify`, so that I never read a finished verdict on work in progress.

- **Given** an execution of `clarify` whose own activity is `completed` and
  whose console is still running, **when** a skill started in that console
  creates a successor activity that is `running`, **then** within one poll the
  task row and the header (when this execution is selected) show no skill
  glyph.
- **Given** that successor activity, **when** the skill declares a wait,
  **then** both badges show `? Waiting for your answer`.
- **Given** that successor activity, **when** it ends `completed` and its stage
  is reached, **then** both badges show `✓ Skill completed`; when the stage is
  not reached yet, `◷ Awaiting stage validation`; when it ends `failed` or
  `canceled`, `!` or `⊘`.
- **Given** several successor activities, **then** the newest one is reported.

### US2 - A new execution beside an open console (path B), P1

As the owner, when I launch the next step from the board or with
`Launch anyway` while the previous console is still open, I want the task row
to describe the new execution, so that the row never shows the old `✓` next to
work that has not started.

- **Given** an old execution whose skill completed and whose console is still
  running, **when** a new execution of the same task is queued, preparing or
  running, **then** the task row shows the new execution's skill result (no
  glyph while it is queued or running without a verdict), even though the old
  console still ranks first and still leads the row.
- **Given** the same situation, **when** I select the old console, **then** its
  header still shows its own `✓ Skill completed`.
- **Given** the same situation, **when** I select the new execution, **then**
  its header shows its own skill result.

### US3 - One skill per console, P1

- **Given** an execution that runs a single skill, **then** the row and the
  header behave exactly as before this change, including the `◷ Execution ended
  · skill completion unconfirmed` case.

## Functional requirements

- **FR1** The agent's `/desktop/run-result` answer gains a `successor` field:
  the newest `remote_run` activity of the execution's task that
  - was created after the execution's own activity,
  - is not the own activity of any execution the agent holds (an execution of
    path B is never a successor of another),
  - was created while the execution's console was alive: before the moment the
    agent first saw the process exit, or at any time while it still runs.

  It carries `id`, `taskId`, `skillId` (the activity's skill name), `status`
  and `waitingSince`. It is `null` when there is none, and when the own
  activity is missing or has not ended. The existing `activity` and `task`
  fields are unchanged.
- **FR2** The desktop's skill result, given a successor, describes the
  successor instead of the own activity, with the same glyphs and labels.
  The successor's skill name is mapped to a workflow skill (`clarify-issue`,
  `sectile:clarify-issue` and `clarify` all mean `clarify`) to decide whether
  its stage is reached; a name that maps to no stage skill reports `✓` as soon
  as it completes. A wait is read from the successor's `waitingSince` while the
  execution's process is running.
- **FR3** A free console and a discussion keep reporting no skill result,
  whatever successor exists.
- **FR4** A task row's skill badge describes the row's newest execution that
  runs a skill (neither a free console nor a discussion), by submission time.
  When the task has none, it describes the run that leads the row, as today.
  The row's ordering, its leading run, its run-state badge and the run it
  selects on click are unchanged.
- **FR5** The header's badge keeps describing the selected execution (its own
  activity, or its successor per FR2).
- **FR6** A live execution whose last result showed its skill ended (own
  activity or successor `completed`, `failed` or `canceled`) is read again at
  every poll, so that a successor appears within one poll. Every other read
  keeps the cadence of `skill-result-refresh.mjs`.
- **FR7** No result cached for one execution is ever shown for another: the
  row badge reads the cache of the execution it describes.

## Edge cases

- A second, parallel session on the same task (not started in the open
  console) can be taken for a successor while that console is alive. This is
  the accepted heuristic of the clarification (round 2).
- A successor created after the console exited is not attributed to it.
- Executions restored from the agent's run store still count for FR1's
  exclusion.
- An older agent that sends no `successor` keeps today's behaviour for path A;
  path B is fixed in the desktop alone.

## Success criteria

- Path A: after `clarify` completed in a console left open, starting `specify`
  in it clears the `✓` of the row and of the header within one poll, and the
  badge then reports `specify`'s state.
- Path B: a new execution beside an older open console makes the row show the
  new execution's skill result; the old console's header keeps its `✓`.
- A single skill per console behaves exactly as today (existing tests pass
  unchanged).
- `CHANGELOG.md` gains one `Fixed` line under `[Unreleased]`.

## Open points

None: every product question was settled in round 2.
