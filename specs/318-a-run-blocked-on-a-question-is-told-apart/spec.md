# #318 - A run blocked on a user question is told apart from a working run

- **Ticket**: [#318](https://github.com/sebastienferry/sectile/issues/318) · Bug · medium
- **Branch**: `feat/batch-318-319` (batch with #319)
- **Clarification**: `docs/clarifications/318.md` (round 2, owner-confirmed, zero open product question)

This file states behaviour only. Technical choices live in `plan.md`, the ordered
work in `tasks.md`.

## Problem

A run whose agent is blocked on a question to its owner looks exactly like a run
that is compiling or testing: `running`, and after four hours one sentence about
silence. Nobody is told that the run is waiting for them. The waiting model of
ADR 0012 (`waitingSince`, the amber board state, the desktop banner) still exists,
but nothing has fed it since the Claude Code hook was removed (#260).

## Users and value

- **The owner of a run** learns that it waits for them, on the board and through
  the desktop banner, instead of finding out hours later.
- **An operator on the board** tells a blocked run from a busy one.

## User stories, by priority

### US1 - An agent declares that it waits for its owner (must)

- **Given** a running run that the calling session is allowed to report on,
  **when** the agent calls `report_waiting` with `waiting: true`,
  **then** the run carries `waitingSince` and the board shows it as waiting.
- **Given** a run already marked waiting,
  **when** the agent reports `waiting: true` again,
  **then** `waitingSince` keeps its original instant.
- **Given** a run owned by somebody else and a caller who is not an admin,
  **when** the agent reports a wait on it,
  **then** the call is refused and the run is unchanged.
- **Given** a headless (autonomous) run,
  **when** the agent reports `waiting: true`,
  **then** the call succeeds, says the mark was not applied, and the run shows no
  wait.
- **Given** a run that is no longer running, or that does not belong to the named
  task,
  **when** the agent reports a wait,
  **then** the call is refused.

### US2 - The wait ends without the agent having to remember it (must)

- **Given** a run marked waiting by a session,
  **when** that same session makes any other tool call,
  **then** the wait is cleared before the call is served.
- **Given** a waiting run,
  **when** the session only pings, or calls `report_waiting` again,
  **then** the wait stays.
- **Given** a waiting run,
  **when** the agent reports `waiting: false`, or the run reaches any terminal
  status, or its session ends,
  **then** the wait is cleared.
- **Given** a waiting run declared by one session,
  **when** another session makes a call,
  **then** the wait stays.

### US3 - The desktop banner fires again (must)

- **Given** a run launched on the workstation of its owner,
  **when** it is marked waiting on the server,
  **then** the local agent's run list carries `waitingSince` for that run and the
  desktop raises its "waiting for you" banner once.
- **Given** the wait is cleared,
  **then** the agent's run list drops `waitingSince` for that run.
- **Given** a run owned by another user, or a run the agent does not hold,
  **then** no agent other than the owner's records the wait.

### US4 - Shipped skills report the wait (must)

- **Given** any stage skill rendered by Sectile,
  **when** a standalone run is about to ask its owner a blocking question,
  **then** its instructions say to call `report_waiting` first.

## Functional requirements

| Id | Requirement |
| --- | --- |
| FR1 | The MCP catalog gains `report_waiting {taskKey, runId, waiting}`. The catalog check of the stdio bridge and the naming contract list it. |
| FR2 | Ownership follows `finish_run`: owner, admin, or anyone on an ownerless run. |
| FR3 | The first mark wins, on the MCP path and on `POST /api/activities/{id}/waiting` alike. |
| FR4 | A wait on a headless run is accepted, not applied, and the result says so. |
| FR5 | A wait is cleared by the declaring session's next tool call other than `report_waiting`, by `waiting: false`, by any terminal status, and when the declaring session ends. |
| FR6 | A waiting change on a running remote run with an owner is pushed to that owner's agent as a `run_waiting` message; an agent holding the run records `waitingSince` on its run list, except for a headless run, and ignores a run it does not hold. |
| FR7 | Tool permission prompts are not detected, and the documentation says so. |
| FR8 | No schema change, no question text, no tracker notification. |
| FR9 | The shared standalone contract fragment tells skills to report a blocking question; golden files follow. |
| FR10 | ADR 0012 gets a revision naming the new reporter; README, the interface contract, CAPABILITIES and the changelog are updated. |

## Out of scope

A new "silent" state (#319), the #307 silence sentence, macro runs (they have no
task key and no desktop banner), a notification outside Sectile.
