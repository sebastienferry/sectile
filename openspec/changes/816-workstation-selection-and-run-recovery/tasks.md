# Implementation tasks

All work below is for the implementation stage. No production changes are part of this specification commit.

## 1. Persistence and discovery
- [ ] 1.1 Reuse #815 activity deviceId persistence; add deviceLabel with backward-compatible SQLite/PostgreSQL migration/read/write support; preserve the unique active-run index.
- [ ] 1.2 Resolve caller-owned paired-device labels for availableWorkstations without returning credentials. Add tests for unknown inventory, missing labels, duplicate labels and caller isolation.
- [ ] 1.3 Persist assignment at admission and adoption; preserve it on reconnect, batch display and authenticated legacy synchronization. Test reload, device renaming/removal, real database restart and legacy rows.

## 2. Server admission and dispatch
- [ ] 2.1 Reuse #815 live-inventory task admission/receiving-replica validation and extend agent retries using deviceId; preserve non-Web affinity/default and alias behavior. Cover zero/one/many eligible devices, foreign IDs, stale inventory and shared-project resolution.
- [ ] 2.2 Compute and validate batch eligibility across every selected task before side effects. Cover no intersection and one member becoming unavailable.
- [ ] 2.3 Recheck and pin assignment at dispatch/reconnect. Test disappearance before admission and after queueing, failure handback and no rerouting across local/distributed instances.

## 3. Unified cancellation and recovery
- [ ] 3.1 Route activity cancel and task cancel-run through a common owner/admin-aware cancellation service, retaining empty-body compatibility and client/server execution behavior.
- [ ] 3.2 Atomically cancel proven undispatched queued/pending work against worker claim; cancel batches through their existing lifecycle and release all members. Test cancellation/dispatch races.
- [ ] 3.3 Restore persisted owner/device routing after restart; preserve bounded stop acknowledgements and complete orphan evidence. Test confirmed stop, known device offline, partial evidence and automatic proven-orphan reconciliation.
- [ ] 3.4 Support distinct explicit force closure without claiming process termination; preserve terminal-state guards, completion contract validation and workflow handback. Test owner/admin/foreign permissions, legacy ownerless admin recovery, cancel/completion races, late reconnect/completion and subsequent relaunch.

## 4. Shared Web launch flow
- [ ] 4.1 Add WorkstationPicker and shared eligibility helpers using the established project picker pattern; resolve project first and reset device selection on project change.
- [ ] 4.2 Route runSkill, card shortcuts, detail launch actions, agent activity retries and pickup_issues batches through the shared picker; pass deviceId and retain mode/model/prompt/comment/batch options.
- [ ] 4.3 Cover zero/one/many cases, picker dismissal with no request, duplicate labels, disappearing target refresh, batch intersection and server-only/macro compatibility in meaningful unit/browser tests. Verify the actual outgoing deviceId.

## 5. Attribution and explicit recovery UI
- [ ] 5.1 Extend Web activity types and show assignment on TaskCard, TaskDetailModal and ActivitiesView; distinguish active unavailable devices from historical attribution and unknown legacy rows.
- [ ] 5.2 Share recovery state/permissions across RemoteRunBadge and explicit task/activity actions for queued/pending/running states; show truthful unconfirmed errors and separate force-close confirmation.
- [ ] 5.3 Add localized strings using existing catalogs; verify keyboard focus and accessible names. Browser tests assert labels after reload/reconnect, queued/pending cancel, confirmed running stop, denied cancellation and force-close then relaunch.

## 6. Release and verification
- [ ] 6.1 Add English Added/Fixed changelog entries for workstation selection/attribution and stuck-run recovery at implementation time.
- [ ] 6.2 Run `openspec validate 816-workstation-selection-and-run-recovery --strict`, `make test` (Go suite, formatting, Web unit tests, TypeScript and lint), and `npm --prefix web run build`.
- [ ] 6.3 Run relevant Web browser tests using the repository harness, including launch/project picker, batch pickup and new recovery scenarios; retain evidence of permissions, pinned dispatch, reload/restart and late-report checks.
- [ ] 6.4 Inspect the final diff for locale preservation, unchanged active-run uniqueness/completion validation, absent timeout expiry/rerouting and Windows-hidden child process compliance if any commands were introduced.
