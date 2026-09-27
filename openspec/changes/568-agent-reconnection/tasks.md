# Tasks

## 1. Recovery and diagnostics
- [ ] Reset retry history after an established connection and retain bounded delay for failures.
- [ ] Distinguish abnormal transport loss from an explicit server close reason.
- [ ] Cover both cases with regression tests.

## 2. Launch during reconnection
- [ ] Wait briefly for a recently connected agent before starting a task run.
- [ ] Keep never-connected failure immediate and avoid orphaned activity on expiry.
- [ ] Cover recovery, expiry, and cancellation with handler tests.

## 3. Release and gates
- [ ] Add a user-facing changelog entry.
- [ ] Run `openspec validate 568-agent-reconnection --strict`.
- [ ] Run Go build, vet, and tests; review the complete diff.
