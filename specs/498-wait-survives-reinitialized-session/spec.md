# Specification #498 - A launched run's next call ends its wait after a re-initialization

- Ticket: https://github.com/sebastienferry/sectile/issues/498
- Branch: `feat/batch-393-497-498-584-647-648`
- Clarification: `docs/clarifications/498.md` (rounds 1 and 2, reading A)
- Framework: Spec Kit

## Summary

A console the agent launched for a run tells the server which run it belongs
to on every call. When its MCP client has to initialize a new session, after a
server restart for example, its next Sectile call still ends the run's wait.

## Scope

In scope: the stdio bridge, the server's per-call wait handling, tests, the
#475 specification's residual and the changelog.

Out of scope: a session started by hand without a launcher runId (it keeps the
Enter and `finish_run` paths), waits set by hand or by a launch, the index of
#497.

## Vocabulary

- **Launched console**: a session the agent started for a run, with
  `SECTILE_RUN_ID` in its environment.
- **Run header**: `X-Sectile-Run-Id`, sent by the stdio bridge on every
  request when `SECTILE_RUN_ID` is set.

## User stories

### US1 (P1) - The glyph clears after a re-initialization

1. Given a launched console whose run is waiting, declared by session S1,
   when the server restarts, the client initializes session S2 and calls
   `get_task`, then the run's wait ends and the glyph clears on the board and
   the desktop.
2. Given the same wait, when S2 calls `report_waiting(true)` again, then the
   wait is kept and the declaring session becomes S2.

### US2 (P1) - Nothing else clears the wait

1. Given the waiting run R, when a session whose run header names another
   run calls any tool, then R stays waiting.
2. Given the waiting run R, when a session with no run header calls any tool,
   then R stays waiting.
3. Given the waiting run R owned by user A, when a call by user B carries R's
   header, then R stays waiting.
4. Given a run whose wait was set by hand or by a launch (no declaring
   session), when a call carries its header, then the wait stays.

## Functional requirements

- FR1. The stdio bridge sends `X-Sectile-Run-Id` with the value of
  `SECTILE_RUN_ID` when it is not empty, and no header otherwise.
- FR2. A tool call other than `report_waiting` that carries the header ends
  the named run's wait when the run is running, its wait was declared by a
  session, and the caller owns it or it has no owner.
- FR3. The session-based ending of #475 is unchanged.
- FR4. An agent older than the server sends no header and keeps today's
  behaviour.

## Acceptance criteria

- AC1. US1 and US2 are covered by tests.
- AC2. The #475 specification no longer lists the residual for launched
  consoles.
- AC3. A `Fixed` line in `CHANGELOG.md`.

## Open points

None.
