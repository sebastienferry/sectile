# Tasks

## 1. Recovery and diagnostics
- [x] Reset retry history after an established connection and retain bounded delay for failures.
- [x] Distinguish abnormal transport loss from an explicit server close reason.
- [x] Cover both cases with regression tests.

## 2. Launch during reconnection
- [x] Wait briefly for a recently connected agent before starting a task run.
- [x] Keep never-connected failure immediate and avoid orphaned activity on expiry.
- [x] Cover recovery, expiry, and cancellation with handler tests.

## 3. Release and gates
- [x] Add a user-facing changelog entry.
- [x] Run `openspec validate 568-agent-reconnection --strict`.
- [x] Run Go build, vet, and tests; review the complete diff.
