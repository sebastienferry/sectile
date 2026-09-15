# Design

## Context
`DispatchAndWait` (`internal/handlers/agent_dispatcher.go`) waits on three outcomes: the agent
reports a launch status, the connection closes, or the caller's context expires. The caller
(`internal/handlers/handlers.go`, the run-skill path) treats all three the same way: it sets
the activity to failed and calls `FinishRemoteRun(..., "failed", ...)`.

That last call is the damage. `FinishRemoteRun` only updates a row still in `running`, so once
the server has closed the run as failed, the agent's own `finish_run` is refused with "remote
run not found or already finished with another status". The work then completes with no record
of it.

## Decisions

### An unconfirmed launch is its own outcome
`DispatchAndWait` distinguishes a launch the agent reported as failed from a launch whose
confirmation did not arrive. The second is returned as a sentinel the caller can test with
`errors.Is`, rather than as a string the caller would have to match.

The caller then leaves the remote run alone. A run left `running` is correct: the agent is
still working and will finish it. A run closed as failed is a statement about work that has
not been observed to fail.

The HTTP response stays an error, because the caller asked for a confirmed launch and did not
get one. Only the run's fate changes.

### Activity status
The activity is recorded as failed with a summary that says the launch was not confirmed and
that the run stays open, rather than inventing a new status value. `ActivityStatus` is a closed
set used by the board's filters and colours, and widening it for this one case would ripple
into the UI for no benefit the reader can see. The summary is what the reader reads.

### Diagnosability
Both errors carry the action, the elapsed wait and the agent device, and the same detail is
logged server-side. The elapsed time is measured, not assumed from the timeout constant, so a
context cancelled early by the caller is distinguishable from one that ran its full term. That
distinction matters here: the reports on #113 could not tell whether the wait had lasted 45
seconds or been cut short.

## Risks / Trade-offs
- A run left open after an unconfirmed launch stays open indefinitely if the agent never
  reports. That is the honest state, and it is visible on the board; closing it early is what
  produced the wrong record in the first place.
- Naming the device in an error surfaces a machine name in the UI. It is the user's own
  workstation and it is already shown in the launch message.
