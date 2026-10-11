## 1. Persistence and contracts
- [ ] 1.1 Add targetDeviceId alias normalization and conflict validation while retaining deviceId; test both aliases, equal/conflicting values and empty inputs.
- [ ] 1.2 Add device_id migration, Go and Web activity fields, and RunLaunch.DeviceID; preserve empty legacy records on SQLite and PostgreSQL.
- [ ] 1.3 Audit all activity insert/select/scan/update and event paths; preserve identity through adoption, completion, status sync and batch member reporting.
- [ ] 1.4 Implement scoped latest successful agent_launch affinity lookup with deterministic ordering; test task/project/user isolation, failed/unconfirmed exclusion and older valid history surviving newer excluded rows.

## 2. Routing
- [ ] 2.1 Add task-specific inventory-aware candidate selection on canonical local and remote routes without changing macro/cancellation semantics.
- [ ] 2.2 Implement explicit target, eligible affinity, unavailable affinity refusal, no-affinity recency and ascending-device tie-break; test unknown/empty inventories and wildcard registration exclusion.
- [ ] 2.3 Integrate selection before run/launch recording, persist both device identities and launch project/user, handle persistence errors, and use the exact route for engine and dispatch.
- [ ] 2.4 Preserve same-device reconnect grace and failed/unconfirmed no-redispatch behavior; cover disconnect, replacement session and cross-replica cases with qualifying inventories.
- [ ] 2.5 Verify authorization, busy/force and lead-workstation batch invariants with regression tests. Ensure errors and candidate lists never leak other users' devices.

## 3. Desktop and activity history
- [ ] 3.1 Forward Desktop's paired identity as targetDeviceId through its authenticated task launch path; test force requests, missing identity and competing remote affinity.
- [ ] 3.2 Render recorded device IDs in latest output and past activity history using existing localization; test completed/disconnected records and unknown legacy records after reload.
- [ ] 3.3 Add English Unreleased documentation for workstation routing, Desktop pinning, history and strict reported-inventory eligibility.

## 4. Verification and delivery
- [ ] 4.1 Run affected database, handler and agent regression tests and focused race tests for routing/reconnect behavior; verify supported database migration tests.
- [ ] 4.2 Run make test and web build; run Desktop tests/build for any Desktop frontend changes, following project checks.
- [ ] 4.3 Run openspec validate 815-target-workstation --strict and inspect the final diff for behavior/implementation separation, English repository prose and preserved French runtime surfaces.
- [ ] 4.4 Update the same draft PR with implementation evidence; retain attained workflow stage and leave review/merge to their configured owners.
