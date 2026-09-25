# Plan - the waiting glyph clears once the question is answered

Behaviour and acceptance criteria are in `spec.md`. This file holds the
implementation choices.

## Stack and surfaces

- Go server: `internal/taskmcp` (MCP tools, session registry), `internal/db`
  (runs, migrations), `internal/handlers` (agent WebSocket, relay to agents).
- Go desktop agent: `internal/agent` (run queue, run list the desktop polls),
  `internal/terminal` (PTY sessions and their WebSocket).
- Wire protocol: `internal/agentprotocol`.
- No change in `desktop/src` or `web/src`. Both already derive the glyph from
  `waitingSince`: the desktop from the agent's run list
  (`desktop/src/skill-result.mjs`, `shared/runStates.ts`), the web board from
  the server's activity (`web/src/components/RemoteRunBadge.tsx`).

## Where the wait lives today

- `report_waiting(true)` writes `task_activities.waiting_since`
  (`SetRemoteRunWaiting`, first mark wins) and records the run in the session's
  in-memory entry (`SessionRegistry.MarkWaiting`). That entry exists only if
  the session is in `SessionRegistry.live`, on the instance that serves it.
- `Resume(sessionID)`, called from the receiving middleware on each
  `tools/call` except `report_waiting`, clears only the runs in that
  in-memory map (cause C when the entry is missing or was lost).
- Each change to an activity reaches `pushRunWaiting`
  (`internal/handlers/handlers.go`), which sends `run_waiting` to the owner's
  agent. A clear is only sent for a run whose mark was sent by the same
  process (`pushedWaits`, in memory), and nothing is re-sent on agent
  reconnect (cause B).
- Owner input reaches the run's PTY through `terminal.Manager`: the desktop
  console over the agent's `/desktop/terminal` WebSocket
  (`HandleWebSocket`), the web terminal as `pty_input` relayed by the server
  (`SendInput`). Nothing observes it (cause A).

## Design

### 1. The wait is attached to its session in the database (cause C)

- Migration 23 `task_activities.waiting_session`: `ALTER TABLE
  task_activities ADD COLUMN waiting_session TEXT NOT NULL DEFAULT '';`
  in `internal/db/migrations.go` only, never in the baseline
  (see the project memory on new columns).
- `SetRemoteRunWaiting` takes the declaring session: a mark sets
  `waiting_session` to it (the latest declaring session wins; `waiting_since`
  keeps its first-mark-wins rule). Every statement that nulls `waiting_since`
  today also resets `waiting_session` to `''` (`remoterun.go`, `db.go`
  cancel, `instances.go` recovery, `finish` paths).
- New `DB.ResumeWaits(sessionID string) ([]string, error)`: selects the
  running `remote_run` activities with `waiting_session = ?` and
  `waiting_since IS NOT NULL`, clears both columns on them, notifies the
  listeners of each, and returns their ids. An empty session id is a no-op.
- `SessionRegistry.Resume` calls it through the `RunWaiter` interface instead
  of reading its own map; `MarkWaiting`/`ForgetWaiting` and the `waiting` map
  are removed, and `Close` clears through the database the same way. Any
  instance can therefore end a wait for any session: the session id is the
  key, whether or not the session is in `live` here.
- Cost: one indexed-by-filter `SELECT` per tool call. The table is filtered by
  `status = 'running'` and `skill_id = 'remote_run'` first; no index is added
  unless a test on PostgreSQL shows a sequential scan hurts.

### 2. The owner's Enter is observed and relayed (cause A)

- `terminal.Session` gains input listeners, mirroring the output listeners:
  `Manager.AddInputListener(sessionID, fn func([]byte))`, invoked from
  `SendInput` and from every write path of `HandleWebSocket` (binary, raw
  text, JSON `input`) before the bytes reach the PTY.
- The agent registers one listener per interactive run when it attaches the
  run to its terminal session. On input containing `\r` or `\n`, if the run's
  `desktop.WaitingSince` is set, it:
  1. clears `desktop.WaitingSince` and records `answeredAt = WaitingSince`
     on the run (the mark it answered);
  2. sends `run_answered` `{runId, waitingSince}` to the server.
- `agentprotocol.RunAnsweredType = "run_answered"`, payload
  `RunAnswered{RunID string; WaitingSince time.Time}`.
- Server: a `run_answered` case in the agent read loop
  (`handlers.go`, next to `running_tasks`) calls
  `DB.AnswerRemoteRunWait(userID, runID, waitingSince)`, which clears the wait
  only if the run is `running`, is an agent-owned `remote_run` of that user,
  and its `waiting_since` equals the answered mark. A newer mark (a question
  asked after the answer was typed) is kept. An older server logs the unknown
  type and ignores it (requirement 9).
- Undelivered signal: if the agent is not connected, it keeps `answeredAt`
  and re-sends `run_answered` for every such run right after it reconnects,
  before answering the server's pull.

### 3. The desktop learns every change, and catches up on reconnect (cause B)

- `SetRemoteRunWaiting`, `ResumeWaits` and `AnswerRemoteRunWait` report a real
  change (rows affected) and, on one, call a dedicated wait listener
  (`d.notifyWaitListeners(task, activity)`) in addition to the postback
  listeners. The handler pushes `run_waiting` from that listener
  unconditionally; `pushRunWaiting` stops relying on `pushedWaits`, which is
  removed, and is no longer called from the generic postback path.
- Reconnect: at the end of `pullAndApplyAgentTasks`, and of
  `pullAndApplyRemoteAgentTasks`, the server sends `run_waiting` with the
  current `waiting_since` (set or nil) for each running run the agent
  reported. The agent already ignores a run it does not hold and a headless
  run.
- The agent ignores a pushed mark whose `WaitingSince` equals the run's
  `answeredAt` (requirement 5): that is the wait the owner already answered,
  echoed by a push that raced the answer. It re-sends `run_answered` instead.
  Any other mark is applied, and a nil clears `answeredAt`.
- Cross-instance: `Dispatch` already reaches an agent held by another
  instance; nothing new is needed there.

## Adjustments made during implementation

- **Migration 23, not 17.** Main landed migrations 17 to 22 (#456, #464) while
  this ticket was specified.
- **`waiting_reason` (#456).** A launch parked until its ticket is pinned to a
  repository also sets `waiting_since`, with `waiting_reason = 'repository'`.
  `AnswerRemoteRunWait` leaves such a wait: it is answered by a pin on the
  ticket, not by a key in the console. `MarkRunAwaitingRepository` resets
  `waiting_session`.
- **Viewer input only.** `terminal.Manager.SendInput` also carries the lines
  the agent types itself (the launch line, the next step of a ticket sent to
  a running agent). The listeners are therefore not called from it: the relayed
  web terminal input goes through a new `SendViewerInput`, and the console
  WebSocket paths call the listeners directly.
- **Where the listener is registered.** `runInPty` is the one place every
  interactive launch passes through (task, macro, free console, desktop
  launch), with the run id as session id; `watchAnswers` registers once per run
  (`controlledRun.answerWatched`), outside the queue lock, since the listener
  takes that lock under the session's listener lock.
- **API shape.** `SetRemoteRunWaiting(runID, waiting)` stays for the hand-set
  route and sets no session; `ReportSessionRunWaitingAs` carries the session
  for `report_waiting`. `SessionRegistry.Close` clears the session's waits
  through `ResumeWaits` too.
- **Wait listeners** are registered with `RegisterWaitListener` and run the
  same way as the postback listeners (one goroutine each); `pushRunWaiting`
  still re-reads the run so the last message sent is the true one.

## Rejected alternatives

- **Clear on any keystroke.** Rejected by the owner: arrows, Escape and
  Ctrl-C would clear a wait the owner has not answered.
- **Clear on output resuming.** Spinners and redraws print without any
  answer.
- **Clear every wait of the task on any task-scoped call.** A task can run
  concurrent sessions (`start_run` reports `concurrent`); one session's call
  would clear another's wait.
- **Keep the in-memory map and add a reconnect re-sync only.** It fixes B but
  not C: the map is per instance and lost on restart.
- **A clear API over HTTP from the agent.** The agent already holds an
  authenticated WebSocket carrying its user; a new message avoids a second
  credential path.

## Data contracts

- `task_activities.waiting_session TEXT NOT NULL DEFAULT ''` (migration 23).
  Not exposed in `models.TaskActivity` JSON: it is a server-side key.
- `run_waiting` (server -> agent): unchanged.
- `run_answered` (agent -> server): `{"runId": string, "waitingSince": RFC 3339}`.

## Target files

- `internal/db/migrations.go`, `internal/db/remoterun.go`, `internal/db/db.go`,
  `internal/db/instances.go`, and the rewind helpers in
  `internal/db/migrations_test.go`, `internal/db/activerun_test.go`.
- `internal/taskmcp/sessions.go`, `internal/taskmcp/server.go`.
- `internal/handlers/handlers.go`.
- `internal/agentprotocol/message.go`.
- `internal/agent/agent.go` (and the run struct holding `desktop`),
  `internal/agent/agent_desktop.go` if the listener is attached there.
- `internal/terminal/terminal.go`.
- `CHANGELOG.md`.

## Risks

- Migration 23 breaks the tests that rewind `schema_migrations` unless their
  helpers drop `waiting_session` too (project memory). Run the whole
  `go test ./internal/db/`, and on PostgreSQL with a throwaway DSN.
- The input listener runs on the WebSocket read path: it must not block
  (send `run_answered` through the agent's existing send queue).
- Windows `normalizeInput` rewrites line terminators; detection runs on the
  input as received, before normalization.
