# Plan #890 - Take an earlier run as the context of a new run

## Stack

Go server (`internal/handlers`, `internal/db`, `internal/taskmcp`), Go agent
(`internal/agent`, `internal/agentconfig`, `internal/agentmcp`), Sectile
Desktop (`desktop/src`, `desktop/electron`), and the React web client for the
read-only context line. One migration (SQLite and PostgreSQL). No new HTTP
endpoint on the server: the launch and the retry gain one optional field each.

Files touched:

- Changed: `internal/models/models.go`, `internal/agentconfig/config.go`,
  `internal/db/migrations.go`, `internal/db/activityattachment.go` (only if the
  rebuild column list must carry the column, see Schema),
  `internal/db/retry_launch.go`, `internal/db/remoterun.go`, `internal/db/db.go`
  (activity scans), `internal/handlers/handlers.go`, `internal/taskmcp/server.go`,
  `internal/agentmcp/mcp.go`, `internal/mcptest/contract.go`,
  `internal/agent/agent.go`, `internal/agent/agent_desktop.go`,
  `desktop/electron/preload.cjs`, `desktop/electron/main.cjs`,
  `desktop/src/main.js`, `web/src/types/index.ts`,
  `web/src/components/TaskDetailModal.tsx`, the web locale file that holds
  `shell.remoteRun`, `CHANGELOG.md`.
- New: `internal/db/runcontext.go` (eligibility and server record),
  `internal/agent/run_context.go` (context file), their tests, a desktop UI test.

## Identifier

The context run ID is the run's activity ID: the remote run (`skill_id =
remote_run`) that the desktop lists, that `SECTILE_RUN_ID` carries and that
`start_run` returns. A launch records two activities, the `agent_launch` row
and the remote run; the retry already accepts either (`RetryTaskRequest`
normalises the skill). Eligibility resolves an `agent_launch` ID to its
`remote_run_id` so both name the same run, and the recorded `context_run_id`
is always the remote run's ID.

## Schema (`internal/db/migrations.go`)

Migration 57, `task_activities.context_run_id`:

```sql
ALTER TABLE task_activities ADD COLUMN context_run_id TEXT NOT NULL DEFAULT '';
```

Same pattern as migration 54. `models.TaskActivity` gains
`` ContextRunID string `json:"contextRunId,omitempty"` ``, read by the activity
scans next to `remote_run_id` (`GetActivityByID`, `GetActivities`, the task
detail's activities). Check whether `taskActivityColumns` must carry it: it
does not carry `remote_run_id`, so the rebuild predates these columns and the
answer is expected to be no.

`db.RunLaunch` gains `ContextRunID`; `StartAgentRun` writes it on the remote
run, and the handler sets it on the `agent_launch` activity too.

## Eligibility and server record (`internal/db/runcontext.go`)

```go
// RunContextSource is a run another run may take as its context (#890).
func (d *DB) RunContextSource(id, projectID string) (*models.TaskActivity, error)
```

Refusals, each a typed error the handlers map to 400 (404 for unknown), in
English, checked in this order:

1. unknown ID: `context run not found`;
2. macro or project run (`task_id` empty): `only a task run can serve as context`;
3. other project: the run's `run_project_id`, or for a run without one the
   task's projects, does not contain `projectID`: `the context run belongs to another project`;
4. still active (`queued`, `pending`, `running`): `the context run is still active`.

`get_run` calls it with an `allowActive` flag (US4.3).

Visibility (D4): the run list (`GetActivities`) filters by project only, so
"same project" is the visibility rule. Nothing more is exposed.

```go
// RunContextRecord renders the server record of a run, bounded.
func RunContextRecord(run *models.TaskActivity, task *models.Task) string
```

Markdown, in this order: header (run ID, skill, ticket key and title, status,
created and completed times, provider and model), the final note (`finish_run`
note, else `summary`), `error`, `steps`, the prompt, the posted `output` tail.
Budget `runContextServerBudget = 32 KiB`; prompt capped at 8 KiB; output keeps
its tail. A cut writes `[... N bytes cut ...]`.

## Launch (`internal/handlers/handlers.go`)

`models.RunSkillRequest` gains `` ContextRunID string `json:"contextRunId,omitempty"` ``.

In `run-skill`, after `ResolveRunProject` and before the busy check (so a
refusal records nothing): when `req.ContextRunID != ""`, call
`RunContextSource(req.ContextRunID, projectID)` and build the record. The
remote run and the `agent_launch` activity get `ContextRunID`. The dispatch
carries it:

```go
// in agentconfig.Dispatch
// ContextRunID names the earlier run this one takes as context (#890), and
// ContextRecord is that run's server record, bounded by the server. An agent
// that predates the fields ignores them and runs without the context.
ContextRunID  string `json:"contextRunId,omitempty"`
ContextRecord string `json:"contextRecord,omitempty"`
```

The record goes on the dispatch rather than being fetched by the agent: no new
agent-to-server call, and the size is already bounded.

The no-agent path of `run-skill` (server-side execution, below line ~2760)
refuses a `contextRunId` with 400: there is no workstation to hand the context
to. Check what that path still runs before deciding; if it is dead for task
skills, the refusal is enough.

## Retry with context (`handlers.go` ~3657, `internal/db/retry_launch.go`)

The retry body gains `` WithContext bool `json:"withContext"` ``. When true and
the original is a task run handled by the `RetryTaskRequest` branch, the
handler sets `request.ContextRunID` to the original's remote run ID before
marshalling it into `run-skill`. A `withContext` retry of a run outside that
branch (sync, tracker op, macro) is refused with 400. Everything else in the
retry (busy rule, project, mode, model, batch members, device) is untouched.

## Agent: context file (`internal/agent/run_context.go`, `agent.go`)

In the dispatch handler, where the prompt notices are appended (`agent.go`
~1140-1229), when `payload.ContextRunID != ""`:

```go
// writeRunContext writes what an earlier run did for the run taking it as
// context (#890), and returns the file's path. The transcript comes from this
// workstation's run store when it still holds the run; the server record the
// dispatch carries always comes with it.
func (d *agentDaemon) writeRunContext(runID string, payload agentconfig.Dispatch) (path, source string, err error)
```

- File: `<run store dir>/../run-context/<runID>.md`, directory `0700`, file
  `0600`; `runID` checked against `safeRunID`. Removed with the run when the
  run store prunes or clears it.
- Transcript: `d.store` gains `read(id string) (storedRun, bool)`. A stored run
  with `Trace` gives its lines; otherwise `Console` rendered as plain text
  (escape sequences and carriage-return overwrites removed; reuse a helper of
  `internal/terminal` if one exists, else a small one in this file, tested).
- Layout: a header naming the source (`transcript` or `server record`) and the
  earlier run, the server record, then the transcript tail. Total budget
  `runContextBudget = 256 KiB`, of which the server record takes at most 32 KiB;
  the transcript keeps its end, which is where a run stops.
- Prompt notice, appended like the others:
  `\nContext from run <id> (<source>): read <path> before starting. It is what that earlier run did; continue from it instead of starting over.`
- Failure to write: log, and append
  `\nContext from run <id> was unavailable on this workstation; call get_run with runId <id> to read it.` The run goes on (US2.6).

`desktopRun` gains `` ContextRunID string `json:"contextRunId,omitempty"` ``, set
from the dispatch and saved with the run, so the console view can show it.

## Desktop resume route (`internal/agent/agent_desktop.go`)

New loopback route `POST /desktop/runs/resume`, body `{runId, projectId}`:

- `runId` checked with `safeRunID`; the run is read from the agent's run list
  to get its task (a free console or a macro run is refused with 400).
- Forwards to the server `POST /api/activities/{runId}/retry` with
  `{"projectId": projectId, "deviceId": d.link.deviceID, "withContext": true}`.
  The device is this workstation, the one whose run store holds the
  transcript (D7): the desktop lists only its own workstation's runs.
- Relays the server's status and body as the existing launch route does, so a
  busy refusal (409) reaches the desktop with its message.
- New capability `run-resume` in `/desktop/status`.

## Desktop UI (`desktop/src/main.js`, `desktop/electron`)

- `preload.cjs`: `resumeRun:(projectId,runId)=>ipcRenderer.invoke('resume-run',projectId,runId)`.
- `main.cjs`: `resume-run` checks the `run-resume` capability (message:
  `Update and restart the local agent to resume a run.`), then posts to
  `/desktop/runs/resume`.
- Sidebar row (~1053) and toolbar: a "Resume" icon button, `aria-label` and
  `title` `Resume with this run as context`, shown when
  `!activeRun(run)&&!freeConsole(run)&&!macroRun(run)` and the status is
  completed, failed or canceled. Project: `relaunchProject(run,boardProject())`,
  with `NO_RUN_PROJECT` when none, as `#rerun` does.
- On success: `refresh()`, then select the new run of the same task, as
  `launchTaskWork` does. On a busy refusal: the same message and the same
  `forceableLaunches` handling as `launchTaskWork`, so "Launch anyway" stays
  available.
- Console view: when `run.contextRunId` is set, a line under the title,
  `Context: run <first 8 chars> (<skill>, <ticket>)`, a button selecting the
  earlier run when it is in `runs`, plain text otherwise.

## Web (`web/src`)

`TaskActivity` type gains `contextRunId?: string`. In `TaskDetailModal.tsx`
(~1698), an activity with it shows `t.shell.remoteRun.context` ("Contexte :
exécution {id}" / "Context: run {id}") with the short ID, scrolling to that
activity when it is in the list. No new button (D8).

## MCP (`internal/taskmcp/server.go`, `internal/agentmcp/mcp.go`)

- `startRunInput` gains `` ContextRunID string `json:"contextRunId,omitempty" jsonschema:"earlier task run of the same project whose context this run takes"` ``.
  Task runs only: with a macro run it is refused. Validated with
  `RunContextSource`; recorded on the run (`StartRemoteRunFor` gains the
  argument, or a follow-up `SetRunContext` on the returned activity when the
  run is reused from a launcher that already recorded one: the launcher's
  value wins and a different one is refused). The response is the activity
  plus `context` (the server record).
- New tool `get_run`, input `{runId, projectId?}`: resolves the project from
  the session as other tools do, validates with `allowActive`, returns
  `{runId, source: "server record", context}`.
- Stdio bridge (`agentmcp`): add `get_run` to the allowed catalog, count 22.
  The bridge serves `get_run` itself when `SECTILE_LOOPBACK_URL` and the agent
  token are set: it asks the local agent `GET /desktop/run-context?runId=`,
  which builds the same file content as the resume (transcript plus server
  record, the record fetched from the server). Otherwise, or when the agent
  answers 404, it forwards to the server. This is what makes `get_run` return
  the transcript when the workstation holds it (US3.2).
- `internal/mcptest/contract.go`: add `get_run` and the new `start_run` field.

The catalog check in the bridge means server and agent must be upgraded
together, which the bridge already enforces with its message.

## Tests

- `internal/db/runcontext_test.go`: eligibility (unknown, macro, other project,
  active refused; `agent_launch` ID resolved to its remote run; another ticket
  of the same project accepted); record layout and truncation markers.
- Handler tests next to the existing `run-skill` and retry tests: a refused
  `contextRunId` records no activity and no remote run; an accepted one is
  recorded on both rows and sent on the dispatch; retry with `withContext` on a
  canceled run carries `contextRunId`; retry without it is unchanged.
- Migration test: column present on SQLite and PostgreSQL (existing harness).
- `internal/agent/run_context_test.go`: file from a stored trace, from a stored
  console (escape sequences removed), from the server record alone; budget and
  marker; file mode `0600`; failure path notice; prompt notice; resume route
  forwards `withContext` and this device, refuses a free console.
- `internal/taskmcp`: `start_run` with `contextRunId` (accepted, refused,
  launcher conflict); `get_run`. `internal/agentmcp`: catalog of 22, loopback
  path and server fallback.
- Desktop UI test (`desktop/tests/run-resume.ui.cjs`): button on completed,
  failed and canceled rows, absent on running and on a free console; click
  posts the resume and selects the new run; busy refusal shows the message;
  context line on the new run.
- Web: the context line in the task detail; `i18n.test.mjs` green.
- `make test` green.

## Windows

No new child process. The bridge's loopback call is HTTP. Nothing to hide.

## Risks

- An older agent ignores `contextRunId` on the dispatch: the run starts without
  context and still records it. The desktop gates the button on `run-resume`,
  which only a new agent reports, so this only happens through a launch made
  elsewhere against an old agent.
- The `run-context` directory grows with resumed runs: it is pruned with the
  run store's retention.
