# #439: Technical plan

References: [`spec.md`](spec.md).

## `internal/db/db.go` `processSkillJob`

- Keep the rename's error in `renameErr`.
- Replace the final `UPDATE` by `d.closeLaunch(activityID, renamed bool, status, summary,
  errorText)`:
  - renamed: `UPDATE task_activities SET skill_id='agent_launch', status=?, summary=?,
    error=?, completed_at=? WHERE id=? AND status != 'canceled'` (unchanged);
  - not renamed: the same without `skill_id`;
  - up to `launchCloseAttempts` (3) attempts, `launchCloseRetryPause` (100 ms) apart,
    each under `d.mu`, the pause outside it; the last error is logged with the id.

## Test

`internal/db/launch_rename_test.go`, on SQLite, with the fake agent the other launch
tests use:

1. Install `CREATE TRIGGER refuse_agent_launch BEFORE UPDATE OF skill_id ON
   task_activities WHEN NEW.skill_id = 'agent_launch' BEGIN SELECT RAISE(ABORT,
   'forced rename failure'); END;`.
2. Queue a stage skill on a task and wait for the job to finish.
3. Assert the row is `failed`, keeps its stage skill id, has the summary and an error
   naming the forced failure, and has a completion time.
4. Drop the trigger, queue another launch on the same task: accepted (no `ErrTaskBusy`).

## Target files

`internal/db/db.go`, `internal/db/launch_rename_test.go`.
