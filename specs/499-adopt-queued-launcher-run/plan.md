# Plan #499 - A queued launcher run can be adopted and finished

## Stack

Go, `internal/db` (SQL on SQLite and PostgreSQL), `internal/taskmcp` for the
end-to-end MCP test.

## What exists

- `startRemoteRun` (`internal/db/remoterun.go`) returns a supplied run only when
  `status == "running"`. `startMacroRun` (`internal/db/macroruns.go`) likewise.
- `finishRemoteRun` and `FinishMacroRunAs` close `status='running'`, plus the
  owner's rewrite of a disconnect cancellation.
- `SyncRemoteRunStatusFor` writes `queued` over any non-terminal status.

## Decisions

### 1. Adoption is one conditional UPDATE

`UPDATE task_activities SET status='running', started_at=COALESCE(started_at, ?)
WHERE id=? AND skill_id='remote_run' AND status='queued'`, then the run is read
back and checked as today. The statement is a no-op on a run already running,
so two sessions adopting at once both succeed and see `running`. This matches
"returned unchanged".

### 2. Refusal messages

A helper `adoptionRefusal(activity, scope)` builds the error:
- ended: `remote run <id> already ended as <status>; call start_run without a runId to report a new execution`
- otherwise: `remote run does not match an active execution on this <task|macro>; call start_run without a runId to report a new execution`

The first words of the second message stay as they are, so a log search for the
old text still finds it.

### 3. Closing a queued run

`closable` becomes `status IN ('running','queued')` (plus the existing
disconnect-rewrite clause) in both finish paths.

### 4. No change to the agent's report

The first plan added `AND status <> 'running'` to the `queued` branch of
`SyncRemoteRunStatusFor`. It was dropped: `StartAgentRun` records every
launched run as `running`, so that clause would also have hidden the runs the
agent really keeps waiting for a slot. Decisions 1 and 3 already make a run
re-queued after adoption adoptable and closable again.

## Rejected alternatives

- Making the agent report `running` sooner: the agent's report is right from
  its point of view (the run waits for a slot). The session's own `start_run` is
  the better signal that the work has started.
- Letting the session adopt ownership of the launcher run: the dispatching agent
  owns it and reports the process exit (see `start_run` in `internal/taskmcp/server.go`).

## Target files

| File | Change |
| --- | --- |
| `internal/db/remoterun.go` | adoption, messages, closable |
| `internal/db/macroruns.go` | adoption, messages, closable |
| `internal/db/launcherrun_test.go` | db-level tests |
| `internal/taskmcp/launcherrun_test.go` | MCP queued → running → completed |
| `CHANGELOG.md` | `Fixed` line |
