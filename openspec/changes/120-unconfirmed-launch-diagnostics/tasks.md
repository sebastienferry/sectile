# Tasks

## 1. Separate an unconfirmed launch from a failed one
- [x] 1.1 Return a sentinel error from `DispatchAndWait` when the wait ends on the context,
      testable with `errors.Is`.
- [x] 1.2 In the run-skill path, skip `FinishRemoteRun` for that case so the run stays open.
- [x] 1.3 Record the activity with a summary saying the launch was not confirmed and the run
      stays open until the agent reports.

## 2. Diagnosable timeouts
- [x] 2.1 Measure the actual wait in `DispatchAndWait` and `CallOperation`.
- [x] 2.2 Name the action, the elapsed wait and the agent device in both errors.
- [x] 2.3 Log the same detail server-side when a wait ends without confirmation.

## 3. Tests
- [x] 3.1 An unconfirmed launch leaves the run open, and the agent can then finish it.
- [x] 3.2 A launch the agent reports as failed still closes the run as failed.
- [x] 3.3 Both timeout errors carry the action, an elapsed wait and the device.

## 4. Gates
- [x] 4.1 Run build, vet, the Go suite and the web gate.
- [x] 4.2 `openspec validate 120-unconfirmed-launch-diagnostics --strict`.
