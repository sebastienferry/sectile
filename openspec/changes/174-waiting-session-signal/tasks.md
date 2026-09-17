# Tasks

Sections 1 to 3, 5 and 7 were delivered by the first implementation and are the foundation this
revision keeps. Sections 4, 6, 8 and 9 are the notification layer, revised after the alert moved
from the hook to the desktop application (design D7 to D9).

## 1. Data model
- [x] 1.1 Add `WaitingSince *time.Time \`json:"waitingSince,omitempty"\`` to `models.TaskActivity`.
- [x] 1.2 Add the `waiting_since` column to `task_activities` with the existing idempotent-by-failure `ALTER TABLE` migration style, and read/write it in `AddTaskActivity`, `GetActivityByID`, `GetTaskActivities` and `GetActivities`. Update every explicit column list and its placeholder count in the same edit.
- [x] 1.3 Add `SetRemoteRunWaiting(runID string, waiting bool) error` in `internal/db/remoterun.go`: sets or clears `waiting_since` only while the run is still `running`, and ends on `notifyPostBackListeners` like its neighbours.
- [x] 1.4 Clear `waiting_since` in `FinishRemoteRun` and in the cancellation path, so no terminal run stays waiting.
- [x] 1.5 Go tests: set then clear, clearing on completion and on cancellation, and a no-op on an unknown or already finished run.

## 2. Server endpoint
- [x] 2.1 Add the `POST /api/activities/{id}/waiting` sub-action alongside `retry` and `cancel`, decoding `{"waiting": bool}` and delegating to `SetRemoteRunWaiting`.
- [x] 2.2 Return 404 for an unknown run and 400 for a malformed body; require the same authentication as the neighbouring sub-actions.
- [x] 2.3 Go test in `internal/handlers`: set, clear, unknown run, malformed body, and that a `task_updated` event is emitted.

## 3. Agent loopback
- [x] 3.1 Add `POST /control/runs/{id}/waiting` to the mux, rejecting a request carrying an `Origin` header, validating the host, authenticating with the workstation API key and refusing an unknown run id.
- [x] 3.2 Relay the report to the server through the existing authenticated REST path, with a short timeout, and answer the hook immediately rather than blocking on the relay.
- [x] 3.3 Set `SECTILE_RUN_ID` to `run.desktop.ID` for free console runs, which previously exported an empty value.
- [x] 3.4 Export `SECTILE_LOOPBACK_URL` in the launched-session environment: `SECTILE_AGENT_URL` names the server, so nothing in a session knew the local agent's address.
- [x] 3.5 Go tests in `internal/agent`: accepted report, missing token, wrong token, `Origin` present, unknown run id, and the console environment now carrying its run id.

## 4. Hook scripts (revised)
- [x] 4.1 Write `notification.sh` and `stop.sh` as POSIX shell: read the JSON payload on stdin, extract `cwd` without requiring `jq`, derive the session name from its basename with a fallback when absent.
- [x] 4.2 Trap every failure and `exit 0` on every path, writing nothing on stdout.
- [x] 4.3 Embed the scripts in the Go binary next to the other managed agent assets.
- [x] 4.4 Remove the `osascript` alert from both scripts: the hook is a reporter, and the banner is the desktop's (D7).
- [x] 4.5 Report the run state to `<loopback>/control/runs/{id}/waiting` when the session carries a run, with a bounded timeout.
- [x] 4.6 When the session carries no run, read `~/.taskflow/agent-connection.json` for the loopback URL and credential, and post a session-level report naming the session by its `cwd` basename. A workstation with no such file is a silent no-op (D9).
- [x] 4.7 Shell-level tests: both scripts exit 0 and print nothing with an unparseable payload, with no connection file, and against a dead loopback; the session name is the `cwd` basename and falls back when absent.

## 5. Installation
- [x] 5.1 Write the two scripts, executable, under the user Claude directory through the same rooted, atomic mechanism as `Scaffold`, and register their paths in the managed-file whitelist.
- [x] 5.2 Add a `~/.claude/settings.json` merge: read, decode, add or update only the Sectile-owned `Notification` and `Stop` hook entries, preserve every other key and third-party hook, rewrite atomically.
- [x] 5.3 Leave an unparseable settings file untouched and report the failure without aborting the rest of the agent setup.
- [x] 5.4 Wire the installation into the Claude provider setup path only; no other provider is touched.
- [x] 5.5 Go tests in `internal/agentconfig`: fresh install, existing unrelated keys and hooks preserved, idempotent second run, unparseable file left untouched, and no write for a non-Claude provider.

## 6. Shared run-state vocabulary (new)
- [x] 6.1 Add one module naming each run state with its glyph and its colour, as the single definition both the web badge and the desktop notification read (D8).
- [x] 6.2 Have `web/src/components/RemoteRunBadge.tsx` take its glyph and colour from that module instead of its local `PRESENTATION` map.
- [x] 6.3 Provide each state's glyph in a form the desktop can put on a notification, generated from the same definition rather than maintained beside it.
- [x] 6.4 Tests: every state the indicator can return has a definition, and the badge and the notification resolve the same glyph for the same state.

## 7. Web UI
- [x] 7.1 Add `waitingSince?: string` to `TaskActivity` in `web/src/types/index.ts`.
- [x] 7.2 Add `'waiting'` to `RunIndicatorState` in `web/src/lib/remoteRunIndicator.ts` with the precedence waiting > running > queued > cancelled, deriving it from `waitingSince` on a still-running run.
- [x] 7.3 Report the elapsed waiting time in the tooltip and the accessible label, omitting the duration when `waitingSince` is missing or unparseable.
- [x] 7.4 Add the waiting filter and its badge to `web/src/components/ActivitiesView.tsx`.
- [x] 7.5 Add the user-facing strings to `web/src/locales`.
- [x] 7.6 Extend `web/tests/remoteRunIndicator.test.mjs`: waiting wins over running, resuming returns to running, a waiting run with no timestamp, and the unchanged existing precedence.

## 8. Desktop notification (new)
- [x] 8.1 Add `waitingSince` to the desktop run payload, set and cleared by the loopback waiting handler, so `/desktop/runs` carries it.
- [x] 8.2 Hold session-level reports from sessions Sectile did not launch in a bounded list on the daemon, drained by the desktop on its existing poll (D9).
- [x] 8.3 Raise the notification from the desktop through Electron's notification API, on the transition rather than the state: not-waiting to waiting, and running to a terminal status.
- [x] 8.4 Put the shared glyph for the state on the notification, and name the session by its task or its directory.
- [x] 8.5 Check notification availability once and degrade to the list alone when it is denied, without prompting on every event.
- [x] 8.6 Desktop tests: a transition notifies, a repeated poll of the same state does not, a terminal status notifies once, and a denied permission raises nothing and throws nothing.

## 9. Documentation
- [x] 9.1 Update the `docs/` pages for the revised notification path.
- [x] 9.2 Add an ADR for the waiting-state transport and the decision to model it as a timestamp rather than a status.
- [x] 9.3 Extend that ADR with the revision: why the banner moved from `osascript` in the hook to the desktop application, and the shared icon vocabulary.
- [x] 9.4 Add an entry to `.agents/MEMORY.md` recording that Sectile now writes `~/.claude/settings.json`.

## 10. Validation
- [x] 10.1 `make test` (Go suite, web tests, `tsc --noEmit`, `oxlint`) and the desktop test suite.
- [x] 10.2 End-to-end check in `desktop/tests/waiting-notification.ui.cjs`: the real application against a stub agent raises one real notification when a session starts waiting, stays silent on repeated polls of the same state, announces a session Sectile did not launch, and raises the other glyph when the turn ends. Attribution to Sectile rather than to Electron needs a packaged build and is left to review.
- [x] 10.3 Covered by `internal/agentconfig/hookscripts_test.go`: exit 0 and empty stdout with an unparseable payload, an empty payload, no connection file and a dead loopback.
