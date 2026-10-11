## 1. Persistence and contracts
- [x] 1.1 Add targetDeviceId alias normalization and conflict validation while retaining deviceId; test both aliases, equal/conflicting values and empty inputs.
- [x] 1.2 Add device_id migration, Go and Web activity fields, and RunLaunch.DeviceID; preserve empty legacy records on SQLite and PostgreSQL.
- [x] 1.3 Audit all activity insert/select/scan/update and event paths; preserve identity through adoption, completion, status sync and batch member reporting.
- [x] 1.4 Implement scoped latest successful agent_launch affinity lookup with deterministic ordering; test task/project/user isolation, failed/unconfirmed exclusion and older valid history surviving newer excluded rows.

## 2. Routing
- [x] 2.1 Add task-specific inventory-aware candidate selection on canonical local and remote routes without changing macro/cancellation semantics.
- [x] 2.2 Implement explicit target, eligible affinity, unavailable affinity refusal, no-affinity recency and ascending-device tie-break; test unknown/empty inventories and wildcard registration exclusion.
- [x] 2.3 Integrate selection before run/launch recording, persist both device identities and launch project/user, handle persistence errors, and use the exact route for engine and dispatch.
- [x] 2.4 Preserve same-device reconnect grace and failed/unconfirmed no-redispatch behavior; cover disconnect, replacement session and cross-replica cases with qualifying inventories.
- [x] 2.5 Verify authorization, busy/force and lead-workstation batch invariants with regression tests. Ensure errors and candidate lists never leak other users' devices.

## 3. Desktop and activity history
- [x] 3.1 Forward Desktop's paired identity as targetDeviceId through its authenticated task launch path; test force requests, missing identity and competing remote affinity.
- [x] 3.2 Render recorded device IDs in latest output and past activity history using existing localization; test completed/disconnected records and unknown legacy records after reload.
- [x] 3.3 Add English Unreleased documentation for workstation routing, Desktop pinning, history and strict reported-inventory eligibility.

## 4. Verification and delivery
- [x] 4.1 Run affected database, handler and agent regression tests and focused race tests for routing/reconnect behavior; verify supported database migration tests.
- [x] 4.2 Run make test and web build; run Desktop tests/build for any Desktop frontend changes, following project checks.
- [x] 4.3 Run openspec validate 815-target-workstation --strict and inspect the final diff for behavior/implementation separation, English repository prose and preserved French runtime surfaces.
- [x] 4.4 Update the same draft PR with implementation evidence; retain attained workflow stage and leave review/merge to their configured owners.

## Implementation evidence — 2026-10-11

- Request aliases, strict inventory eligibility, scoped successful-launch affinity, unavailable-affinity refusal, recency/tie ranking, local Desktop pinning, and durable launch/run identity are implemented.
- Task routing revalidates the selected canonical session before dispatch, including the receiving replica. Generic macro and cancellation selection remain unchanged. Reconnect grace waits only for the explicit or recorded device and requires fresh inventory.
- Activity readers and inserts preserve the new field; adoption, engine/wait updates, completion and batch member reporting retain it. Status synchronization rejects non-run activities so an unconfirmed launch cannot gain affinity from a later execution report.
- `make test build-server build-agent` exited 0: all Go suites passed; web tests reported `# tests 732`, `# pass 732`, `# fail 0`; TypeScript and oxlint completed. Existing lint warnings and the large bundle warning remain.
- Focused routing/reconnect/replica race tests passed: `ok tasks/internal/handlers 8.708s`.
- PostgreSQL 17 migration, launch affinity/identity, presence and workstation checks passed: `ok tasks/internal/db 0.984s`. SQLite migration, legacy unknown identity and restart tests passed in the full suite.
- Browser regression passed with French and English activity labels after completion, disconnection and reload, without inventing legacy identity.
- `openspec validate 815-target-workstation --strict` and the implementation diff whitespace check passed. The accepted clarification report remains unchanged, including its pre-existing trailing blank line.
- No Desktop frontend changed, so its separate frontend build/test suite is not applicable. The Desktop agent regression suite passed.
- Delivery uses the existing draft PR #819 on feat/815. Review and merge remain with their configured owners; no reviewed transition is requested.
