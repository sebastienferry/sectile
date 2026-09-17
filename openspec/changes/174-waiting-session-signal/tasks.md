# Tasks

## 1. Data model
- [ ] 1.1 Add `WaitingSince *time.Time \`json:"waitingSince,omitempty"\`` to `models.TaskActivity` (`internal/models/models.go:46-65`).
- [ ] 1.2 Add the `waiting_since` column to `task_activities` with the existing idempotent-by-failure `ALTER TABLE` migration style, and read/write it in `AddTaskActivity`, `GetActivityByID`, `GetTaskActivities` and `GetActivities`. Update every explicit column list and its placeholder count in the same edit.
- [ ] 1.3 Add `SetRemoteRunWaiting(runID string, waiting bool) error` in `internal/db/remoterun.go`: sets or clears `waiting_since` only while the run is still `running`, and ends on `notifyPostBackListeners` like its neighbours.
- [ ] 1.4 Clear `waiting_since` in `FinishRemoteRun` and in the cancellation path, so no terminal run stays waiting.
- [ ] 1.5 Go tests: set then clear, clearing on completion and on cancellation, and a no-op on an unknown or already finished run.

## 2. Server endpoint
- [ ] 2.1 Add the `POST /api/activities/{id}/waiting` sub-action alongside `retry` and `cancel` (`internal/handlers/handlers.go:2537-2560`), decoding `{"waiting": bool}` and delegating to `SetRemoteRunWaiting`.
- [ ] 2.2 Return 404 for an unknown run and 400 for a malformed body; require the same authentication as the neighbouring sub-actions.
- [ ] 2.3 Go test in `internal/handlers`: set, clear, unknown run, malformed body, and that a `task_updated` event is emitted.

## 3. Agent loopback
- [ ] 3.1 Add `POST /control/runs/{id}/waiting` to the mux (`internal/agent/agent.go:401-410`), handler in `internal/agent/agent_run.go` next to `handleRunControl`: reject a request carrying an `Origin` header, validate the host, authenticate with the loopback token, and refuse an unknown run id.
- [ ] 3.2 Relay the report to the server through the existing authenticated REST path, with a short timeout, and answer the hook immediately rather than blocking on the relay.
- [ ] 3.3 Set `SECTILE_RUN_ID` to `run.desktop.ID` for free console runs (`internal/agent/agent_console.go:94-98`), which currently export an empty value.
- [ ] 3.4 Go tests in `internal/agent`: accepted report, missing token, wrong token, `Origin` present, unknown run id, and the console environment now carrying its run id.

## 4. Hook scripts
- [ ] 4.1 Write `notification.sh` and `stop.sh` as POSIX shell: read the JSON payload on stdin, extract `cwd` without requiring `jq`, derive the session name from its basename with a fallback when absent.
- [ ] 4.2 Raise the alert with `osascript` — sound `Funk` for waiting, `Glass` for a finished turn — and no-op silently when the command is missing.
- [ ] 4.3 When `SECTILE_RUN_ID`, `SECTILE_AGENT_URL` and `SECTILE_AGENT_TOKEN` are all present, post the waiting (`notification.sh`) or resumed (`stop.sh`) report to the loopback with a bounded timeout; skip it entirely otherwise.
- [ ] 4.4 Trap every failure and `exit 0` on every path, writing nothing on stdout.
- [ ] 4.5 Embed the scripts in the Go binary next to the other managed agent assets.

## 5. Installation
- [ ] 5.1 Write the two scripts, executable, under the user Claude directory through the same rooted, atomic mechanism as `Scaffold` (`internal/agentconfig/local.go:115-208`), and register their paths in the managed-file whitelist (`internal/agentconfig/validation.go`).
- [ ] 5.2 Add a `~/.claude/settings.json` merge: read, decode, add or update only the Sectile-owned `Notification` and `Stop` hook entries, preserve every other key and third-party hook, rewrite atomically with mode 0600.
- [ ] 5.3 Leave an unparseable settings file untouched and report the failure without aborting the rest of the agent setup.
- [ ] 5.4 Wire the installation into the Claude provider setup path only; no other provider is touched.
- [ ] 5.5 Go tests in `internal/agentconfig`: fresh install, existing unrelated keys and hooks preserved, idempotent second run, unparseable file left untouched, and no write for a non-Claude provider.

## 6. Web UI
- [ ] 6.1 Add `waitingSince?: string` to `TaskActivity` in `web/src/types/index.ts`.
- [ ] 6.2 Add `'waiting'` to `RunIndicatorState` in `web/src/lib/remoteRunIndicator.ts` with the precedence waiting > running > queued > cancelled, deriving it from `waitingSince` on a still-running run; a waiting run is not cancellable through a new path — keep the existing cancellability rule.
- [ ] 6.3 Add the waiting entry to the `PRESENTATION` map and its icon in `web/src/components/RemoteRunBadge.tsx`, and report the elapsed waiting time in the tooltip and the accessible label, omitting the duration when `waitingSince` is missing or unparseable.
- [ ] 6.4 Add the waiting filter and its badge to `web/src/components/ActivitiesView.tsx`.
- [ ] 6.5 Add the user-facing strings to `web/src/locales`.
- [ ] 6.6 Extend `web/tests/remoteRunIndicator.test.mjs`: waiting wins over running, resuming returns to running, a waiting run with no timestamp, and the unchanged existing precedence.

## 7. Documentation
- [ ] 7.1 Document the hooks, their installation and the waiting state in the relevant `docs/` pages.
- [ ] 7.2 Add an ADR for the waiting-state transport and the decision to model it as a timestamp rather than a status.
- [ ] 7.3 Add an entry to `.agents/MEMORY.md` recording that Sectile now writes `~/.claude/settings.json`, which until this change it only read.

## 8. Validation
- [ ] 8.1 `make test` (Go suite, web tests, `tsc --noEmit`, `oxlint`).
- [ ] 8.2 Manual check on macOS: a permission prompt raises the waiting alert with its own sound, the end of a turn raises the other, and the run shows as waiting then back to running in the UI.
- [ ] 8.3 Manual check that a hook failure, an absent `osascript` and a stopped local agent all leave the session running normally.
