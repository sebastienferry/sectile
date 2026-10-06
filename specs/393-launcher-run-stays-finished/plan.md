# Plan #393 - A finished launcher-owned run stays finished

## Stack

Go tests only: `internal/db` (shared SQLite/PostgreSQL harness) and
`internal/handlers`. No production change.

## Design

- `internal/db/launcher_run_finished_test.go`: build a task, create the run
  with `StartAgentRun`, adopt it with `StartRemoteRunBy(userID, task.ID,
  skill, run.ID)`, close it with `FinishRemoteRunAs(completed)`, then drive
  `SyncRemoteRunStatusFor` with `running` and `queued` and assert status,
  `completed_at` and summary. Same file: second `finish_run` (same status
  accepted, other status refused), `reclaimDeadInstances` on a dead instance,
  and `StartRemoteRunBy` with the runId refused.
- `internal/handlers`: a test that applies
  `ApplyAgentRunningTasksFor(owner, []agentprotocol.RunningTask{...})` with the
  run's id, task and `running` status, then reads the run back.
- The pre-#407 check (FR3): drop the `status NOT IN (...)` guard locally, run
  US1.1, see it fail, restore. Not committed.

## Target files

- `internal/db/launcher_run_finished_test.go` (new)
- `internal/handlers/agent_running_tasks_test.go` (new, or the closest
  existing handler test file for agent reports)
