# Implementation checklist

## 1. Prepare and confirm contracts

- [ ] 1.1 Preserve accepted clarification/specification commits and integrate the current feature baseline into `feat/871` through the repository workflow; do not recreate the missing Orchestration feature.
- [ ] 1.2 Inspect actual session producers, discovery and restoration. Document the authoritative dedicated-session identity if present; otherwise explicitly keep current policy evaluations and workers distinct with no manufactured sessions.

## 2. Correct membership and actions

- [ ] 2.1 Define one positive responsibility predicate; exclude ordinary and legacy unclassified workers regardless of skill, prompt, phase or launch origin. Add/preserve minimal metadata only for an existing dedicated producer if required.
- [ ] 2.2 Apply it to project visibility, profile association and standalone sessions, retaining disabled projects only for live genuine sessions and retaining disconnected-project exclusion.
- [ ] 2.3 Validate overview selection on refresh; derive session state, console/retry/stop targets only from eligible sessions. Keep configured policy controls, instruction previews, evaluation pending/error/results and capacity display.
- [ ] 2.4 Preserve normal execution-list access, queue ordering, lifecycle and capacity by keeping shared execution data unchanged.

## 3. Verify behavior

- [ ] 3.1 Replace baseline tests that expect ordinary workers in Orchestration. Test enabled idle policies; disabled worker-only projects; mixed roles; manual clarify and other workflow skills; orchestrator-launched IDs and valid-looking prompt snapshots; cross-project/profile isolation; missing/unknown identity; genuine sessions without task IDs.
- [ ] 3.2 Cover preparing, pending, queued, running, waiting, completion, failure and cancellation; running plus waitingSince; disable/re-enable; removed definitions; disconnected projects; restore/refresh and stale selections. Worker state changes must never leak into membership, profile state or actions.
- [ ] 3.3 Verify evaluation pending, success reports and errors with no dedicated sessions, and demonstrate no worker console/stop invocation from Orchestration. Verify workers remain available in the normal list/queue and project-wide usage is unchanged.
- [ ] 3.4 Run focused Desktop tests, Desktop test suite/build and applicable existing UI tests. If Go metadata changes, test creation, discovery and restoration plus the project's required Go checks. Record actual results and limitations.

## 4. Document and hand off

- [ ] 4.1 Add one English user-facing Fixed entry under CHANGELOG.md [Unreleased] describing corrected Orchestration membership.
- [ ] 4.2 Review diff for scope: no new orchestration capabilities, scheduling changes or context-repository writes. Follow the implementation-stage publication policy; preserve existing accepted artifacts and task stage on retries.
