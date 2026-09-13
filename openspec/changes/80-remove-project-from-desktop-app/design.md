# Design: workstation project disconnection

## Context

`desktop/src/main.js` builds groups from mapped projects and all console runs; Add project also treats history as evidence of configuration. `cmd/server/agent_desktop.go` supports discovery and mapping but no removal. `localProjectRoot` in `cmd/server/agent_config.go` can infer a repository from agent identity or matching Git remote. `ReadSettings` can restore legacy maps. Deleting one map entry cannot reliably disconnect a project.

The user chose local disconnection and rejection while work remains active. Existing finished history remains in agent memory; this change does not extend its lifetime.

## Persistent state

Extend workstation `agentconfig.Overrides` with `DisconnectedProjects map[string]bool` serialized as `disconnectedProjects`. Only true values mean disconnected. An absent field preserves today's behavior. This field is workstation-owned, never imported from repository overrides or uploaded to the server.

On removal, atomically set the marker and delete the project's entries from `projects`, `worktrees`, `parallelism`, and `commands`. Use `WriteSettings` and preserve connection fields, global overrides, other projects, and unrelated settings. Ensure explicit empty maps prevent legacy fallback. `ReadSettings` reads the marker from shared settings; `localProjectRoot` rejects it before explicit mapping, agent identity, remote matching, or repository overrides can apply.

Re-add uses the existing validated mapping operation. Only a successful atomic mapping save clears the marker. Failed validation or persistence leaves disconnection intact. Workstation per-project overrides removed earlier do not return; the existing inheritance rules apply to the newly saved configuration, including any repository settings that were deliberately preserved.

## Local API and Electron contract

- Add authenticated `DELETE /desktop/projects?id=<project-id>` to the existing local endpoint, with no server deletion call. Require a nonempty ID (400 otherwise), preserve existing authentication (401), return 409 when relevant executions have not exited or preparation/deployment prevents a safe mutation, 500 on settings failure, and 204 after persistence. Repeated removal of an already disconnected idle project succeeds. Permit known local/history project removal without a server round trip; a stale server catalog must not be required to disconnect locally.
- Extend discovery entries with `disconnected: boolean`; disconnected entries report empty path and `configured: false`. Return the authoritative disconnected project IDs with `/desktop/status` so history-only groups can also be filtered even when absent from the server catalog. Add a `remove-project` capability there, following the existing create-task compatibility convention.
- Expose `removeProject(id)` through preload and an Electron handler that URL-encodes the ID, checks capability support, and uses the authenticated local client. An older agent produces an actionable restart/update message rather than a simulated success.
- Keep `/desktop/runs` and finished sessions intact. Visibility uses project disconnection separately from existing task archive state.

## Admission and lifecycle synchronization

Use the established lock order `prepareMu` then `runsMu`; never acquire `prepareMu` while holding `runsMu`. Removal acquires `prepareMu` (a failed TryLock may return a retryable 409), inspects matching runs under `runsMu`, and rejects any whose `exited` channel is not closed, irrespective of display status. Hold the mutation boundary until the settings write finishes. Runs belonging to other projects do not themselves block removal.

The current dispatch path in `cmd/server/agent.go` releases `prepareMu` after resolution and before `enqueueRun`. Keep the lock across resolution, effective-limit calculation, and registration so removal and admission have one ordering. Release it before sending network status, waiting for a slot, or preparing the run. `enqueueRun` already takes `runsMu`. If admission wins, removal sees unfinished work and fails; if removal wins, resolution rejects admission. Preserve the existing preparation-time resolution check as a second guard. Audit all admission paths to ensure none bypass this boundary.

Do not terminate PTYs or invoke history cleanup on removal. A displayed terminal status alone does not establish process exit. Stop remains the existing separate user action. Already disconnected launch requests fail through existing error reporting without registering or preparing a new execution.

## Desktop interaction

Place Remove from desktop in the existing Local settings panel. Confirmation names the project and explains that local configuration is removed while repository and server data remain. Cancel closes the confirmation without mutation; confirm disables duplicate submission. Agent errors stay visible and preserve the project.

After successful removal, immediately update the disconnected set and refresh authoritative status/projects. Filter sidebar groups, run-driven group creation, automatic console selection, and execution actions consistently. If the selected console belongs to the project, detach it, reset terminal/title/directory/history/action controls, and clear its selection. Preserve a selected console from another project. A refresh failure after successful removal reports that refresh failed without reversing or falsely reporting failure of the completed disconnect.

Refresh disconnection state through normal status polling and render when it changes, even if the run list is unchanged. Reopening the desktop must load this state before selecting a console. Add project treats disconnected entries as addable despite retained history. Successful mapping clears disconnection and restores normal visibility; existing task archives remain respected. Quick-add/task selection must clear a stale removed-project selection, and any later launch still requires explicit re-add.

## Alternatives

- UI-only hiding: rejected because it leaves execution configured, contrary to the user decision.
- Removing only the mapping: rejected because automatic resolution and legacy fallback can reconnect it.
- Server DELETE: rejected because its reassignment/deletion semantics exceed workstation scope.
- Stop-and-remove: rejected by the chosen active-work policy.
- Erasing logs or repository files: excluded by clarification; existing retention stays unchanged.

## Validation and documentation

Test settings round trips and preservation, absent-marker compatibility, legacy fallback, implicit repository matching, idempotent removal, explicit re-add, write failure, and authenticated API errors. Test queued/preparing/running and terminal-but-not-exited runs, unrelated projects, and both admission/removal orderings with deterministic synchronization; run the Go race detector.

Extend the Electron UI harness for confirmation/cancel, agent conflict/failure, stale history groups, selected-console detach, another-project selection, restart/reload persistence, polling-only state changes, re-add, archive preservation, and old-agent capability feedback. Document the local endpoint and settings in `desktop/README.md` and relevant API/architecture documentation; add a changelog entry if a changelog exists at implementation. This extends the existing local-agent architecture and does not require a separate architectural redesign.

No product questions remain. Specification stage changes documentation only.
