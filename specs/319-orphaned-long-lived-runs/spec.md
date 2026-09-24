# #319 - Long-lived orphaned runs

- **Ticket**: [#319](https://github.com/sebastienferry/sectile/issues/319) · Task · medium
- **Branch**: `feat/batch-318-319` (batch with #318)
- **Clarification**: `docs/clarifications/319.md` (round 2, owner-confirmed, zero open product question)

This file states behaviour only. Technical choices live in `plan.md`, the ordered
work in `tasks.md`.

## Problem

Since #307 a silent MCP session never loses its runs. A client that died without
closing its connection therefore leaves its run `running` for as long as the
server lives: the four-hour bound only appends a sentence, the board keeps the run
as if it were working, the chain it belongs to never moves, and the only stop the
board offers refuses a client-created run.

## User stories, by priority

### US1 - A session silent for too long is abandoned (must)

- **Given** a session that adopted a run and has said nothing for the abandon
  bound (default eight hours),
  **when** the server notices it,
  **then** the session is closed and the run is canceled with the disconnect note,
  after the silence sentence it already carries.
- **Given** that run,
  **when** its owner later calls `finish_run`,
  **then** the reported outcome replaces the cancellation, as for any
  disconnection (#307).
- **Given** a session silent for less than the abandon bound,
  **then** nothing is closed; past the silence bound it only gets its sentence.

### US2 - The abandon bound is configurable (must)

- **Given** `SECTILE_MCP_SESSION_ABANDON_AFTER` set to a duration,
  **then** that duration is the abandon bound.
- **Given** it is unset, unparseable or not positive,
  **then** the default of eight hours applies.
- **Given** it is shorter than the silence bound,
  **then** the silence bound is used, so no session is closed before it was
  remarked upon.

### US3 - A silent run is visible as such (must)

- **Given** a running run whose summary carries the silence sentence,
  **then** the board badge and the activities view show a "silent" state, distinct
  from "running"; a waiting run still shows as waiting.
- The silent state is read from the summary: it stays until the run ends, even if
  the client speaks again. This is the no-migration trade-off the owner chose.

### US4 - A client-created run can be closed from the board (must)

- **Given** a running client-created run and a viewer who owns it or is an admin,
  **then** the badge offers "Close".
- **When** the viewer closes it,
  **then** the run is canceled with the disconnect note, the workflow is handed
  back, its session forgets it, and its owner may still report its real outcome.
- **Given** a viewer who is neither, or an ownerless run and a non-admin,
  **then** no Close is offered and the server refuses the request.

### US5 - Cancelling a client run from the activities view (must)

- **Given** a running client-created remote run in the activities view,
  **when** its owner or an admin cancels it,
  **then** the cancellation goes through the same closing path as `finish_run`
  (owner check, hand-back, session release); anybody else is refused.
- Other activities cancel as before.

### US6 - A run silenced, then closed, stays recoverable (must)

- **Given** a run that received the silence sentence and was then closed by a
  disconnection,
  **when** its owner calls `finish_run`,
  **then** the outcome is recorded. (Today the rewrite predicate matches the
  disconnect note as a prefix and misses it after the silence sentence.)

### US7 - The decision is written (must)

ADR 0007 gets a second amendment; README, `.env.sample`, the interface contract
and the changelog describe the two bounds.

## Functional requirements

| Id | Requirement |
| --- | --- |
| FR1 | Abandon: past the abandon bound, `SessionRegistry.Close` runs for the session and the SDK session is closed. |
| FR2 | `SECTILE_MCP_SESSION_ABANDON_AFTER`, default 8h, raised to the silence bound when shorter, default on invalid values. |
| FR3 | Board and activities "silent" state for a running run whose summary contains `RunSilencePrefix`; precedence waiting > silent > running. |
| FR4 | Board Close for `RunActionClient` runs, owner or admin, writing `RunDisconnectNote`, through `FinishRemoteRunAs`, releasing the run from its session. |
| FR5 | The disconnect-note rewrite predicate matches the note anywhere in the summary, for task runs and macro runs. |
| FR6 | `POST /api/activities/{id}/cancel` on a running client-created task run goes through `FinishRemoteRunAs` with the owner check and the session release; other activities are unchanged. |
| FR7 | Scope: `RunActionClient` runs only; agent-dispatched runs keep their supervisor. |

## Out of scope

Agent-dispatched runs, the restart sweep, folding orphaned runs into a separate
view, a reminder to the owner.
