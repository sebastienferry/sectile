# Plan #648 - report_waiting marks a macro run as waiting

## Stack

Go (`internal/taskmcp`, `internal/db`), React + TypeScript (`web/src`),
`node --test`.

## Design

- `reportWaitingInput` gains `projectId` and `macroKey` (`taskKey` becomes
  optional in the schema); the handler calls `runTarget` and dispatches.
- `db.ReportSessionMacroRunWaitingAs(caller, admin, sessionID, projectID,
  macroKey, runID, waiting)`: `macroOf`, then the run must exist, have an
  empty task, the project and the macro key; ownership and headless rules as
  in `ReportSessionRunWaitingAs`; then `setRemoteRunWaiting`.
- One helper builds the three refusals for both paths:
  `remote run %s not found`, `remote run %s is not attached to %s`,
  `remote run %s is no longer running (%s)`. The task path keeps the text
  "no longer running" so existing callers that search for it still match.
- `notifyWaitChange`: when the run has no task, notify the post-back
  listeners with a nil task if they accept it, otherwise broadcast through the
  macro run path used by `FinishMacroRunAs`; check during implementation
  which listener refreshes the macro panel.
- `macroRuns` selects `waiting_since`; `MacroRun` in `web/src/lib/macroRuns.ts`
  gains `waitingSince?`; `MacroRealignButton` shows the waiting label when the
  active run has it (strings in French and English).

## Target files

- `internal/taskmcp/server.go`, `internal/db/remoterun.go`,
  `internal/db/macroruns.go` and tests, `internal/mcptest/contract.go`
- `web/src/lib/macroRuns.ts`, `web/src/components/MacroRealignButton.tsx`,
  the i18n strings, a `node --test` file
- `CHANGELOG.md`
