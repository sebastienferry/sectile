# Unconfirmed launches and diagnosable timeouts

## Why
A launch whose confirmation does not reach the server within 45 seconds is recorded as a
failed run, even when the agent did launch and the skill runs to completion. The run is
closed as failed before any work happens, so the skill's own `finish_run` is later refused
with "already finished with another status", and the board shows a failure next to work that
succeeded. This happened on #113: the clarify run was marked failed 45 seconds in while the
delegated skill ran for about fifteen minutes and completed.

The second half of the report, operations timing out while the server answers normally, has
no established cause. Neither timeout error says which action it was waiting for, how long it
waited, or which agent it was talking to, so a recurrence cannot be triaged from the activity
log at all.

## What Changes
- An unconfirmed launch is no longer reported as a failed run. The remote run stays open so
  the agent, which is still working, can finish it itself.
- The activity records the launch as unconfirmed, naming what that means for the reader: the
  skill may still be running, and the run will close when the agent reports.
- Both timeout errors, and the matching server log lines, name the action, the elapsed wait
  and the agent device.

## Impact
`internal/handlers` launch and operation dispatch, the launch path in `handlers.go`, and their
tests. No schema change, no protocol change, no new endpoint, and no change to any timeout
value. The cause of the operation stall is explicitly not addressed here; this change makes
the next occurrence explainable.
