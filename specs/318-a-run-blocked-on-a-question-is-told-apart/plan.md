# Plan - #318

## Stack and boundaries

Go server (`internal/db`, `internal/taskmcp`, `internal/handlers`), Go local
agent (`internal/agent`), shared skill templates (`internal/skills`). No web or
desktop code change: both already render `waitingSince` (`shared/runStates.ts`,
`desktop/src/notifications.mjs`); the desktop only lost its input.

## Data contract

No migration. `task_activities.waiting_since` and
`models.TaskActivity.WaitingSince` exist on both stores.

MCP tool, input schema inferred from the struct:

```json
{"taskKey": "string", "runId": "string", "waiting": true}
```

Result: `{"activity": <TaskActivity>, "applied": bool, "reason"?: string}`.

Server to agent WebSocket message, additive (an older agent logs and ignores an
unknown type):

```json
{"type": "run_waiting", "taskId": "<task>", "payload": {"runId": "<run>", "waitingSince": "<RFC3339>|null"}}
```

## Design

1. **DB** (`internal/db/remoterun.go`). `SetRemoteRunWaiting` writes
   `COALESCE(waiting_since, ?)` for a mark, so the first mark wins everywhere.
   `ReportRemoteRunWaitingAs(caller, admin, taskKey, runID, waiting)` checks the
   run (remote, running, on the named task), the owner rule of
   `FinishRemoteRunAs`, and returns `applied=false` for an autonomous run
   (`run_mode`, read through `runOutcomeOf`).
2. **Registry** (`internal/taskmcp/sessions.go`). A `RunWaiter` sink
   (`SetRemoteRunWaiting`) and, per session, the set of runs it declared waiting.
   `MarkWaiting`/`ForgetWaiting` maintain it; `Resume(sessionID)` clears the set
   and the marks. `Close` clears the marks of the runs it did not close (a run it
   closes is cleared by its terminal status). `ReleaseRun(runID)` forgets a run
   in every session (used by #319 as well).
3. **Implicit clear.** The receiving middleware already calls `Touch` on every
   message. It calls `Resume` only for `tools/call` whose tool is not
   `report_waiting`: the stdio bridge pings every 60 s, so counting pings would
   clear a wait within a minute.
4. **Tool** (`internal/taskmcp/server.go`). `report_waiting` resolves the caller
   like `finish_run`, calls `ReportRemoteRunWaitingAs`, then marks or forgets the
   run on the calling session.
5. **Push to the agent** (`internal/handlers/handlers.go`). The existing
   post-back listener already sees every run change. For a running `remote_run`
   with an owner it dispatches `run_waiting` through
   `AgentDispatcher.Dispatch(owner, project, ...)`, which reaches the owner's
   agent locally or through the #406 forwarding. A missing agent is not an error.
   Terminal changes are not pushed: the agent observes the exit itself.
6. **Agent** (`internal/agent`). `desktopRun.WaitingSince` comes back
   (`json:"waitingSince,omitzero"`, as before #260). `handleMessage` gains a
   `run_waiting` case which, under `d.queue.mu`, sets or clears it on a run it
   holds, never on a headless run, and ignores an unknown run.
7. **Skills.** `fragments/contracts/transition.md` gains one rule; golden files
   are regenerated with `UPDATE_GOLDEN=1`.

## Rejected alternatives

- A hook, in any form: withdrawn by ADR 0012 (#260).
- Clearing on any message, pings included: clears within a minute under the stdio
  bridge.
- Relying on `waiting: false` alone: a forgetful model leaves a run waiting
  forever.
- Pushing through the relayed event bus: it carries browser events only and
  would double the push on the originating instance.

## Target files

`internal/db/remoterun.go`, `internal/db/waitingrun_test.go`,
`internal/taskmcp/{sessions,server}.go` and tests, `internal/agentmcp/mcp.go`,
`internal/mcptest/contract.go`, `internal/handlers/handlers.go` and a test,
`internal/agent/{agent,agent_desktop}.go` and a test,
`internal/skills/fragments/contracts/transition.md` and goldens,
`docs/adrs/0012-*.md`, `docs/contracts/server-agent-v1.md`,
`docs/CAPABILITIES.md`, `README.md`, `CHANGELOG.md`, `.agents/MEMORY.md`.
