# Tasks - the waiting glyph clears once the question is answered

Ordered so that each step builds and passes its tests on its own. See
`plan.md` for the design and `spec.md` for the acceptance scenarios.

## 1. Database: the wait carries its session

- [ ] Add migration 17 `task_activities.waiting_session` in
      `internal/db/migrations.go` (not in the baseline).
- [ ] Drop `waiting_session` in the rewind helpers (`forgetSchemaVersion`,
      the stamped-database test in `migrations_test.go`, the migration-nine
      and migration-sixteen tests in `activerun_test.go`).
- [ ] `SetRemoteRunWaiting(runID, sessionID, waiting)`: a mark sets
      `waiting_session`; a clear resets it; it reports whether a row changed.
- [ ] Reset `waiting_session` wherever `waiting_since` is nulled
      (`remoterun.go`, `db.go` cancel, `instances.go` recovery).
- [ ] `ResumeWaits(sessionID)` and `AnswerRemoteRunWait(userID, runID,
      waitingSince)`, each notifying the wait listeners on a real change.
- [ ] Add the wait listener registration next to the postback listeners.
- [ ] Tests (`internal/db/remoterun_test.go`): mark then resume by session;
      resume by another session keeps the mark; a closed run is untouched;
      answer with the matching mark clears; answer with an older mark keeps a
      newer one; answer by another user is refused; a re-mark after a clear
      gets a new `waiting_since`.

## 2. MCP: the next call ends the wait from the database

- [ ] `SessionRegistry.Resume` and `Close` clear through `RunWaiter`
      (`ResumeWaits`); remove `MarkWaiting`, `ForgetWaiting` and the
      `waiting` map.
- [ ] `report_waiting` passes the calling session id to
      `ReportRemoteRunWaitingAs` / `SetRemoteRunWaiting`.
- [ ] Tests (`internal/taskmcp`): a wait declared by a session the registry
      never watched ends on that session's next `get_task`; a wait survives
      `report_waiting` calls; a wait declared through one registry ends
      through a second registry sharing the database (restart or other
      instance); another session's call leaves it.

## 3. Relay: every change reaches the agent, and reconnect catches up

- [ ] Push `run_waiting` from the wait listener unconditionally; remove
      `pushedWaits` and the push from the generic postback path.
- [ ] After `pullAndApplyAgentTasks` and `pullAndApplyRemoteAgentTasks`, send
      the current `waiting_since` (set or nil) for each running run reported.
- [ ] Handle `run_answered` in the agent read loop (`handlers.go`) with the
      connection's user.
- [ ] Tests (`internal/handlers`): a clear is pushed although this handler
      never pushed the mark; reconnect re-sends a set and a cleared state;
      `run_answered` from the owner clears, from another user does not.

## 4. Protocol and terminal

- [ ] `agentprotocol.RunAnsweredType` and `RunAnswered`.
- [ ] `terminal.Manager.AddInputListener`, invoked from `SendInput` and every
      `HandleWebSocket` write path, before `normalizeInput`.
- [ ] Tests (`internal/terminal`): the listener sees binary, raw text, JSON
      `input` and `SendInput` data; a listener cannot block the read loop.

## 5. Agent: Enter answers the wait

- [ ] Register the input listener when an interactive run is attached to its
      terminal session; on `\r` or `\n` with `WaitingSince` set, clear it,
      record `answeredAt`, send `run_answered`.
- [ ] Keep unsent answers and re-send them right after reconnecting.
- [ ] `handleRunWaiting`: ignore a mark equal to `answeredAt` (re-send the
      answer); apply any other mark; a nil clears `answeredAt`.
- [ ] Tests (`internal/agent`): Enter clears the run list's `waitingSince`
      and sends `run_answered`; an arrow key or plain characters do not; Enter
      on a non-waiting run sends nothing; a headless run is never touched; an
      echoed old mark does not restore the glyph; a newer mark does; an answer
      made while disconnected is re-sent on reconnect.

## 6. Documentation and checks

- [ ] `CHANGELOG.md`, `## [Unreleased]` / `Fixed`: "The waiting glyph now
      clears as soon as the question is answered in the run's console, and no
      longer stays after the server restarts or the desktop reconnects (#475)."
- [ ] Update the `report_waiting` tool description if its wording about how a
      wait ends changes (Enter in the console now ends it too).
- [ ] `go test ./...` (with `-race` on `internal/agent`, `internal/taskmcp`,
      `internal/handlers`), `go test ./internal/db/` against PostgreSQL with a
      throwaway DSN, `go vet ./...`.
- [ ] Manual check on a copy of the dev database (never the dev database, and
      without the tracker token): launch an interactive run, make it call
      `report_waiting(true)`, answer with Enter in the desktop console, check
      the desktop glyph and the web badge disappear without any further
      Sectile call.

## Test plan summary

| Scenario (spec.md) | Covered by |
| --- | --- |
| Enter in the desktop console | agent test 5, terminal test 4, manual check |
| Enter in the web terminal | terminal test 4 (`SendInput`), agent test 5 |
| Other keystrokes | agent test 5 |
| Enter on a non-waiting run | agent test 5 |
| Clear while the agent is disconnected | handlers test 3 (reconnect re-sync) |
| Enter while disconnected | agent test 5 (re-send), db test 1 (answer) |
| Server restart, then the same session calls | taskmcp test 2 (second registry) |
| Session unknown to the registry | taskmcp test 2 |
| New wait after an answered one | db test 1, agent test 5 |
| Headless run | agent test 5 |
