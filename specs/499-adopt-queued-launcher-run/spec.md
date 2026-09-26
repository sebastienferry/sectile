# Specification #499 - A queued launcher run can be adopted and finished

Task: `gh-11a4f59c-b6bd-4747-8ea5-0a10d5b78da4-499`
Clarification: `docs/clarifications/499.md`
Branch: `fix/batch-411-499-517`

## Summary

A launcher records a run and hands its runId to the session it starts. When
the agent reports that run as `queued`, the session can neither adopt it with
`start_run` nor close it with `finish_run`, so the board never shows it running
and its outcome is lost. The session holding the runId must be able to do both.

## Scope

In scope: `start_run` and `finish_run` for task and macro runs, and the agent
status report that demotes a run. Out of scope: the agent's queue and slot
logic, session ownership (a launcher run is not adopted by the session), and
the launch endpoints.

## Definitions

- **Launcher run**: a `remote_run` activity a launcher created before starting a
  session, whose id it passed as runId.
- **Adopt**: `start_run` called with that runId.

## User stories (prioritised)

### US1 - Adopt a queued launcher run (P1)

- **Given** a launcher run in `queued` on task T,
  **when** a session calls `start_run` on T with its runId,
  **then** the run becomes `running`, its start time is set, and it is returned.
- **Given** a launcher run in `running`,
  **when** a session calls `start_run` with its runId,
  **then** it is returned unchanged, as today.
- **Given** a runId that names a finished run,
  **when** `start_run` is called with it,
  **then** it is refused with a message saying the run has already ended with
  its status, and that the session should call `start_run` without a runId.
- **Given** a runId that names no run on this task (unknown, another task, or
  not a remote run),
  **when** `start_run` is called with it,
  **then** it is refused with a message saying so, and that the session should
  call `start_run` without a runId.

### US2 - Finish it (P1)

- **Given** a launcher run adopted as in US1,
  **when** the session calls `finish_run` with `completed`,
  **then** the run is recorded `completed` with the note.
- **Given** a launcher run still in `queued` (never adopted),
  **when** its owner calls `finish_run`,
  **then** the outcome is recorded all the same.

### US3 - An agent report after adoption (P2)

- **Given** a run adopted as in US1,
  **when** the agent later reports it `queued` on a task pull,
  **then** the board may show it queued again, and a second `start_run` or a
  `finish_run` with its runId still succeeds.

Blocking the report instead (a `running` run never going back to `queued`) was
considered and rejected during implementation: the launcher records every run
as `running`, so the rule would also hide the runs genuinely waiting for a slot
on the agent.

### US4 - Macro runs behave the same (P2)

The three stories above hold for a run on a macro (`start_run` and
`finish_run` with `projectId` and `macroKey`).

## Functional requirements

| ID | Requirement |
| --- | --- |
| FR1 | `start_run` with a runId accepts a `running` or `queued` remote run of the named task or macro; a `queued` one becomes `running`, with `started_at` set when empty. |
| FR2 | The refusals of FR1 distinguish "already ended (status)" from "not an execution of this task/macro", and both tell the session to call `start_run` without a runId. |
| FR3 | `finish_run` closes a `running` or `queued` remote run, under the existing ownership rules. |
| FR4 | An agent report of `queued` keeps its current effect; adoption and finishing stay possible after it (US3). |
| FR5 | `CHANGELOG.md` gets a `Fixed` line under `[Unreleased]`. |

## Success criteria

- A test covers queued → running → completed for a launcher run through the MCP tools.
- Tests cover each refusal message, finishing a queued run, the macro path and a run re-queued after adoption.

## Open questions

None.
