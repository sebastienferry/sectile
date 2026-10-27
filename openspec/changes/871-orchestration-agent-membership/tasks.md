# Implementation checklist

## 1. Prepare and confirm contracts

- [x] 1.1 Preserve accepted clarification/specification commits and integrate the current feature baseline into `feat/871` through the repository workflow; completed by conflict-free rebase onto `origin/main` during owner-authorized PR preparation on 2026-10-27.
- [x] 1.2 Inspect actual session producers, discovery and restoration. Document the authoritative dedicated-session identity if present; otherwise explicitly keep current policy evaluations and workers distinct with no manufactured sessions.

## 2. Correct membership and actions

- [x] 2.1 Define one positive responsibility predicate; exclude ordinary and legacy unclassified workers regardless of skill, prompt, phase or launch origin. Add/preserve minimal metadata only for an existing dedicated producer if required.
- [x] 2.2 Apply it to project visibility, profile association and standalone sessions, retaining disabled projects only for live genuine sessions and retaining disconnected-project exclusion.
- [x] 2.3 Validate overview selection on refresh; derive session state, console/retry/stop targets only from eligible sessions. Keep configured policy controls, instruction previews, evaluation pending/error/results and capacity display.
- [x] 2.4 Preserve normal execution-list access, queue ordering, lifecycle and capacity by keeping shared execution data unchanged.

## 3. Verify behavior

- [x] 3.1 Replace baseline tests that expect ordinary workers in Orchestration. Test enabled idle policies; disabled worker-only projects; mixed roles; manual clarify and other workflow skills; orchestrator-launched IDs and valid-looking prompt snapshots; cross-project/profile isolation; missing/unknown identity; genuine sessions without task IDs.
- [x] 3.2 Cover preparing, pending, queued, running, waiting, completion, failure and cancellation; running plus waitingSince; disable/re-enable; removed definitions; disconnected projects; restore/refresh and stale selections. Worker state changes must never leak into membership, profile state or actions.
- [x] 3.3 Verify evaluation pending, success reports and errors with no dedicated sessions, and demonstrate no worker console/stop invocation from Orchestration. Verify workers remain available in the normal list/queue and project-wide usage is unchanged.
- [x] 3.4 Run focused Desktop tests, Desktop test suite/build and applicable existing UI tests. If Go metadata changes, test creation, discovery and restoration plus the project's required Go checks. Record actual results and limitations.

## 4. Document and hand off

- [x] 4.1 Add one English user-facing Fixed entry under CHANGELOG.md [Unreleased] describing corrected Orchestration membership.
- [x] 4.2 Review diff for scope: no new orchestration capabilities, scheduling changes or context-repository writes. Follow the implementation-stage publication policy; preserve existing accepted artifacts and task stage on retries.

## Implementation evidence (2026-10-27)

- Inspected `desktopRun` discovery, constructors and restored-run representation in `internal/agent/agent_desktop.go`, and the evaluation relay in `agent_desktop_orchestration.go`. No authoritative dedicated-session producer exists. The shared positive predicate therefore explicitly rejects current records; no speculative role fields, producer or persistence contract were introduced. Positive dedicated-session scenarios remain conditional on an actual producer and cannot truthfully be exercised against this baseline.
- Removed prompt/header and launched-activity association. Policy evaluations retain controls, pending/error/report state and project-wide usage. Shared discovery, execution lists, queue, scheduling and capacity paths are unchanged.
- Updated selector and UI regression fixtures. Genuine-session positive fixtures are not applicable without an authoritative producer; all existing worker classes remain ineligible.
- `npm run build` in `desktop`: passed, 1021 modules transformed, with a chunk-size advisory.
- After explicit user authorization to run local-listener tests outside the sandbox, `npm test` in `desktop`: 314 tests passed, 0 failed. Orchestration UI: 1 passed; sidebar hold and queue checks: 3 passed. UI fixture assertions were corrected to target `.run` rows, account for the startup console, and leave held navigation before asserting refreshed capacity. Strict OpenSpec validation and `git diff --check` passed.
- Reviewed the final diff: only Desktop presentation, regression tests, changelog and task evidence changed. No execution, scheduling, capacity, Go contract or context-repository changes.
- Publication reuses draft PR #873 on `feat/871`. The branch includes current `origin/main`; remote branch identity was verified before publication. SSH authentication is unavailable in this session; HTTPS fetch succeeded without changing repository remote configuration.
- The original supplied run ID was already finished as failed at the initial sandbox blocker; `start_run` refuses reopening it. No replacement run identity was created.
