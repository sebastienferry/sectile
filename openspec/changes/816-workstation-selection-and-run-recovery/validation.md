# Implementation validation — #816

## Scope and preserved artifacts

Implemented on the assigned `feat/816` worktree. The accepted clarification and specification requirements are retained. Design refinements document shared SQL dispatch/cancellation arbitration and exact launcher-to-run linkage. Existing draft PR #820 is reused; this implementation ends at implemented, never reviewed.

## Changes by file

- `internal/models/models.go`: add readable label snapshots and internal dispatch/linkage fields.
- `internal/db/migrations.go`: migration 54 adds empty-default label, dispatch-state and launcher-link columns on both supported SQL dialects; active-run uniqueness is unchanged.
- `internal/db/db.go`: persist/read labels on every activity view, claim only queued/pending work, transfer active launchers safely, preserve prompt/assignment at dispatch, and guard ordinary terminal cancellation.
- `internal/db/remoterun.go`: retain launch prompts, support pending closure and prevent late supervisor outcomes from overwriting terminal runs.
- `internal/db/activerun.go`: expose the existing task-execution kind definition for cancellation routing.
- `internal/db/run_dispatch.go`: shared SQL claim/cancel guards, atomic queue-to-run transfer and exact child lookup prevent dispatch after cancellation and stale launch cancellation of newer runs.
- `internal/db/workstation_label.go`: resolve only owner-scoped paired names with ID fallback; fill authenticated legacy attribution once.
- `internal/db/retry_launch.go`: retain prompt, mode, model and ordered batch members while selecting a new retry workstation.
- `internal/handlers/agent_inventory.go`: expose readable discovery labels without credential data.
- `internal/handlers/agent_selection.go`: retain immutable remembered assignment and stop the exact persisted device; keep legacy fallback routing separate.
- `internal/handlers/handlers.go`: snapshot labels at admission, guard dispatch, route retries through ordinary validated admission, share activity cancellation and authenticate legacy synchronization.
- `internal/handlers/remote_run.go`: common task/activity cancellation for active states, owner/admin checks, queue-claim arbitration, bounded acknowledgment, explicit truthful force closure, terminal and exact-child guards.
- `internal/handlers/run_reconciliation.go`: successful authenticated reconnect snapshots can close only older dispatched runs pinned to that owner/device. Disconnect, failed pull, unknown assignment and in-flight dispatch never prove an orphan.
- `web/src/context/AppContext.tsx`: shared project-first workstation discovery/choice for task launches, shortcuts, retries and compatible batches; preserve launch options and send `deviceId`.
- `web/src/components/WorkstationPicker.tsx`: localized explicit selection, duplicate-name disambiguation, refresh, dismissal, keyboard focus/trapping and visible focus styles.
- `web/src/components/WorkstationAttribution.tsx`: durable label/ID display; availability only for the viewer's own active assignments, never historical or invisible colleague devices.
- `web/src/components/ExecutionRecovery.tsx`: shared owner/admin actions, truthful unconfirmed errors and separate force confirmation.
- `web/src/components/TaskCard.tsx`, `TaskDetailModal.tsx`, `ActivitiesView.tsx`: visible task, batch and historical attribution plus explicit task/activity recovery.
- `web/src/components/RemoteRunBadge.tsx`, `web/src/lib/remoteRunIndicator.ts`, `web/src/lib/workstations.ts`: shared active-state permissions, pending support, confirmed force-close flow and pure eligibility/display helpers.
- `web/src/App.tsx`, `web/src/types/index.ts`, `web/src/locales/translations.ts`: picker mount, label contract and French/English runtime catalogs.
- `CHANGELOG.md`: English Added and Fixed entries under Unreleased.

## Verification evidence

- `openspec validate 816-workstation-selection-and-run-recovery --strict`: `Change '816-workstation-selection-and-run-recovery' is valid`.
- `make test`: successful full Go suite and formatting check; Web output `# tests 735`, `# pass 735`, `# fail 0`; TypeScript and oxlint exit successfully. Existing lint warnings remain; no warning points to the new picker/recovery components.
- `npm --prefix web run build`: successful TypeScript/Vite production build. Existing chunk-size advisory remains.
- Targeted `go test -race ./internal/db ./internal/handlers`: both packages pass with workstation snapshot, cross-instance claim/cancel, queued/pending/running recovery, owner/admin denial, exact persisted stop, reconnect evidence, stale launcher, preserved retry and late-report scenarios.
- Chrome browser harnesses: workstation-execution, run-project-picker, batch-pickup-context and workstation-history pass. Assertions cover actual outgoing IDs/options, zero/one/many selection, duplicate labels, dismissal, refresh/stale refusal without rerouting, keyboard focus, retry, batch submission, reload/history, all active states, denied cancellation, force confirmation and relaunch. The picker was also visually inspected.
- `git diff --check`: passes. Reviewed final changes for runtime locale preservation, unchanged active-run index and completion-result validation, absent disconnection expiry/rerouting and no new production child-process launches.

Database tests include real SQLite restart and two independent connections sharing a database. PostgreSQL variants and migration fixtures are present, but live PostgreSQL tests are skipped without a dedicated test DSN; no PostgreSQL server was available on this workstation. No live remote agent process or tracker execution is launched by the browser fixtures.

## Deliberately excluded

Desktop launch UX, macros, automatic rescheduling/rerouting, remembered preferences, new engines and timeout-based expiry of unconfirmed executions remain outside the accepted scope. Batches keep the existing single execution-project rule; all members must belong to that project. Historical unknown assignments remain unknown.

## Adjustment review — 2026-10-12

Verified existing PR #820 is open, targets `main`, and uses this task's `feat/816` branch. Fetched the configured default branch; `origin/main` is already included, so no integration or history rewrite is needed. The specifications workspace is this same dedicated worktree.

Feedback disposition: PR discussion comments, reviews and inline comments were retrieved successfully and are empty. Historical ticket reports and the owner's accepted clarification remain satisfied; no product question or outstanding feedback item remains.

Review findings fixed:

- An unconfirmed stop through an `agent_launch` row did not expose force closure, although the server delegates that stop to its exact remote child. The shared recovery control now offers the same explicit confirmation for launcher rows. The browser regression verifies that the record stays active until the user sends force.
- Cancellation between launcher insertion and dispatch claim left the launcher history marked running. A failed claim now terminalizes the launcher and closes any still-active remote record without dispatching. A deterministic SQLite trigger regression places cancellation at that boundary and checks both history rows are terminal; it also passes under the race detector.

Final verification: `make test` passes (735 Web tests, Go suite, formatting, TypeScript and lint); `npm --prefix web run build` and `go build ./cmd/server ./cmd/agent` pass. Workstation execution, project picker, batch pickup and workstation history Chrome harnesses pass. Strict OpenSpec validation and diff whitespace checks pass. Existing lint and bundle-size warnings remain. Live PostgreSQL checks require a dedicated test DSN and are not claimed.

Reviewer attention: shared SQL dispatch/cancel arbitration, authenticated reconnect orphan evidence, exact launcher-child cancellation, and preserved retry/batch options. Replay the commands above and `go test -race ./internal/db ./internal/handlers`; browser harnesses use `PLAYWRIGHT_MODULE=../../desktop/node_modules/playwright/index.mjs` when run from the repository root.
