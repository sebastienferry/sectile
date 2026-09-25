# Tasks - the waiting glyph clears once the question is answered

Ordered so that each step builds and passes its tests on its own. See
`plan.md` for the design and `spec.md` for the acceptance scenarios.

## 1. Database: the wait carries its session

- [x] Add migration 23 `task_activities.waiting_session` in
      `internal/db/migrations.go` (not in the baseline).
- [x] Drop `waiting_session` in the rewind helpers (`forgetSchemaVersion`,
      the stamped-database test in `migrations_test.go`, the migration-nine
      and migration-sixteen tests in `activerun_test.go`).
- [x] `SetRemoteRunWaiting(runID, sessionID, waiting)`: a mark sets
      `waiting_session`; a clear resets it; it reports whether a row changed.
- [x] Reset `waiting_session` wherever `waiting_since` is nulled
      (`remoterun.go`, `db.go` cancel, `instances.go` recovery).
- [x] `ResumeWaits(sessionID)` and `AnswerRemoteRunWait(userID, runID,
      waitingSince)`, each notifying the wait listeners on a real change.
- [x] Add the wait listener registration next to the postback listeners.
- [x] Tests (`internal/db/remoterun_test.go`): mark then resume by session;
      resume by another session keeps the mark; a closed run is untouched;
      answer with the matching mark clears; answer with an older mark keeps a
      newer one; answer by another user is refused; a re-mark after a clear
      gets a new `waiting_since`.

## 2. MCP: the next call ends the wait from the database

- [x] `SessionRegistry.Resume` and `Close` clear through `RunWaiter`
      (`ResumeWaits`); remove `MarkWaiting`, `ForgetWaiting` and the
      `waiting` map.
- [x] `report_waiting` passes the calling session id to
      `ReportRemoteRunWaitingAs` / `SetRemoteRunWaiting`.
- [x] Tests (`internal/taskmcp`): a wait declared by a session the registry
      never watched ends on that session's next `get_task`; a wait survives
      `report_waiting` calls; a wait declared through one registry ends
      through a second registry sharing the database (restart or other
      instance); another session's call leaves it.

## 3. Relay: every change reaches the agent, and reconnect catches up

- [x] Push `run_waiting` from the wait listener unconditionally; remove
      `pushedWaits` and the push from the generic postback path.
- [x] After `pullAndApplyAgentTasks` and `pullAndApplyRemoteAgentTasks`, send
      the current `waiting_since` (set or nil) for each running run reported.
- [x] Handle `run_answered` in the agent read loop (`handlers.go`) with the
      connection's user.
- [x] Tests (`internal/handlers`): a clear is pushed although this handler
      never pushed the mark; reconnect re-sends a set and a cleared state;
      `run_answered` from the owner clears, from another user does not.

## 4. Protocol and terminal

- [x] `agentprotocol.RunAnsweredType` and `RunAnswered`.
- [x] `terminal.Manager.AddInputListener`, invoked from `SendViewerInput` (the
      relayed web terminal input) and every `HandleWebSocket` write path, before
      `normalizeInput`; not from `SendInput`, which carries the agent's own lines.
- [x] Tests (`internal/terminal`): the listener sees binary, raw text, JSON
      `input` and `SendViewerInput` data, and not the agent's own `SendInput`
      lines. Not blocking is the listener's contract: the agent's sends the
      answer from a goroutine.

## 5. Agent: Enter answers the wait

- [x] Register the input listener when an interactive run is attached to its
      terminal session; on `\r` or `\n` with `WaitingSince` set, clear it,
      record `answeredAt`, send `run_answered`.
- [x] Keep unsent answers and re-send them right after reconnecting.
- [x] `handleRunWaiting`: ignore a mark equal to `answeredAt` (re-send the
      answer); apply any other mark; a nil clears `answeredAt`.
- [x] Tests (`internal/agent`): Enter clears the run list's `waitingSince`
      and sends `run_answered`; an arrow key or plain characters do not; Enter
      on a non-waiting run sends nothing; a headless run is never touched; an
      echoed old mark does not restore the glyph; a newer mark does; an answer
      made while disconnected is re-sent on reconnect.

## 6. Documentation and checks

- [x] `CHANGELOG.md`, `## [Unreleased]` / `Fixed`: "The waiting glyph now
      clears as soon as the question is answered in the run's console, and no
      longer stays after the server restarts or the desktop reconnects (#475)."
- [x] Update the `report_waiting` tool description if its wording about how a
      wait ends changes (Enter in the console now ends it too).
- [x] `go test ./...` (with `-race` on `internal/agent`, `internal/taskmcp`,
      `internal/handlers`), `go test ./internal/db/` against PostgreSQL with a
      throwaway DSN, `go vet ./...`.
- [ ] (owner) Manual check on a copy of the dev database (never the dev database, and
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
