# Plan #498 - A launched run's next call ends its wait after a re-initialization

## Stack

Go: `internal/agentmcp` (stdio bridge), `internal/taskmcp` (server
middleware, session registry), `internal/db` (wait writes).

## Design

### Bridge

`agentmcp.Run` wraps the HTTP client's transport so every request carries
`X-Sectile-Run-Id: $SECTILE_RUN_ID` when set. A small `runHeaderTransport`
RoundTripper clones the request and adds the header; the token client stays
as it is.

### Server

- In the receiving middleware of `NewServerWithCallers`, next to
  `sessions.Resume(session.ID())`, when `resumesWaits` holds and the request is
  a `*mcp.CallToolRequest` whose `Extra.Header` carries the run header, call
  `sessions.ResumeRun(runID, session.ID(), caller)`.
- `SessionRegistry.ResumeRun` relays to `db.ResumeRunWait(runID, sessionID,
  userID)`.
- `db.ResumeRunWait`: one guarded update
  `UPDATE task_activities SET waiting_since=NULL, waiting_session='',
  waiting_reason='' WHERE id=? AND skill_id='remote_run' AND status='running'
  AND waiting_since IS NOT NULL AND waiting_session <> '' AND waiting_session
  <> ? AND (user_id='' OR user_id=?)`, then `notifyWaitChange` when a row
  changed. The calling session is excluded because its own wait is ended by
  `ResumeWaits` already, so a single call never notifies twice.
- The caller is resolved the same way the tools resolve it (`callerOf`); an
  unidentified caller only matches runs with no owner.

### Documentation

- `specs/475-restart-after-question/spec.md`: the residual paragraph says it
  now concerns only sessions started without a launcher runId.

## Target files

- `internal/agentmcp/mcp.go`, `internal/agentmcp/*_test.go`
- `internal/taskmcp/server.go`, `internal/taskmcp/sessions.go`
- `internal/db/remoterun.go`, `internal/db/*_test.go`
- `internal/handlers/run_waiting_test.go`
- `specs/475-restart-after-question/spec.md`, `CHANGELOG.md`
