# Server/agent contract, version 1

## Boundaries

The server owns task data, tracker synchronization, project configuration and
managed skill content. The agent owns local repository mappings, worktrees,
provider MCP installation and native process launch. No server filesystem path is
interpreted as a workstation path. The agent and its MCP bridge never open a task
database. Shared DTOs live in `internal/agentconfig` and do not depend on DB code.

## Configuration download

`GET /api/v1/agent/config?projectId=<ID>` or `?taskKey=<task-ID>` requires the
agent bearer token. An optional `framework` parameter generates installation
templates for a supported SDD framework without changing project settings. Use exact project IDs, not names. Prefer full task IDs over
ambiguous tracker keys. A task lookup resolves the actual owning project.

| Field | Meaning |
| --- | --- |
| `schemaVersion` | Must be `1`. Unsupported versions stop preparation. |
| `projectId`, `projectName`, `description` | Identity and project context. The ID must match an explicit project request. |
| `gitRemoteUrl` | Repository identity for automatic local matching, not a path to clone automatically. |
| `githubRepo`, `issueTracker`, `trackerUrl`, `jiraProject` | Optional effective project-over-global repository and tracker metadata for local command placeholders. Missing fields use local directory basename and task source (then `github`) fallbacks. No credentials or server paths. |
| `specFramework` | Specification framework used by the project skills. |
| `skills` | Array of `{id, directory, command, content, commandContent}`. IDs and installation destinations must be unique and safe. `command` is the stage's standard command; a workstation replaces it with its own command name (see *Execution defaults and local overrides*). |
| `specArtifacts` | Optional, `keep` or `drop`. `drop` keeps the tasks' clarification and specification files out of the repository: before a task's session starts, the agent writes their ignore rules in a Sectile-managed block of the primary checkout's `.git/info/exclude`, and removes the block when the effective value is `keep`. Absent (an older server) reads as `keep`. A workstation may override it (see *Execution defaults and local overrides*). |
| `monoRepo` | Optional repository layout. `true` lets the code checkout carry the macro specifications when no specifications folder is set on the workstation; `false` requires that folder for every macro operation. Absent (an older server) reads as `true`. |

**No longer sent since #305** (ADR 0031): `useWorktrees`, `aiProvider`,
`aiCommandTemplate`, `aiCommandTemplateAutonomous`, `aiModel`, `aiSkillModels`,
`externalTerminalCommand` and `setupProviders`. They are workstation settings.
An agent that still receives them from an older server discards them before
resolving its own; an agent that predates #305 reads none of them from a new
server and runs its local values over the provider defaults, which is why
ADR 0006 asks to upgrade both together.

An explicit project connection downloads, validates and installs configuration
before registering its WebSocket. Every dispatch downloads it again, even when
the project has already been prepared. Effective local overrides are validated
before creating a worktree or writing skills. API failures stop execution and
include the server's structured error message when available. Unknown additive
JSON fields are ignored; incompatible changes require a new version.

The snapshot `.taskflow/remote-config.json` is for inspection only. No code reads
it as a configuration cache. An unavailable server prevents a new launch.
Already running coding clients are not restarted or modified by a later download.

## Precedence

The server resolves the method only: project metadata over the deployment's.
Every execution setting is resolved by the agent from
`~/.config/sectile/settings.json` (ADR 0031): the project section, then the
workstation defaults, then the provider defaults (the shipped model list, the
detected terminal, editor `code`, worktrees on, one execution at a time, no
extra setup provider, the stage's standard command). These values are never
uploaded, except as the capability report below.

The engine (provider, interactive and headless templates, model, per-skill
models) is a catalogue entry since #510 (ADR 0033), resolved as a whole: the
engine a task was switched to on this workstation, else its project's default
engine, else the workstation default engine. Nothing is inherited from another
engine; an empty template runs the provider's own command and an empty model
the provider's own model. A resolution without a task (capability report,
macro skills, project refresh, free console) uses the project default engine.
A file stating no engine runs `agy` with its defaults.

Model selection within the engine: a per-skill model outranks the engine
model. The launch's one-off model outranks both, but only when the task runs
its project default engine, the one the capability report describes; on
another engine it is ignored, and the run record says what ran.

Skills and the Sectile MCP registration are set up for the running provider,
the extra setup providers, and the provider of every catalogue engine that
takes skills, so a task switched to any of them finds them in place.

Parallelism is 1 to 10, from the project section, else the defaults, else 1,
and 1 whenever worktrees are off. Extra setup providers from the project
section replace the defaults' list rather than adding to it; an empty list is
the decision "none".

The checkout's legacy `.taskflow/agent.json` is still read as a fallback for
the keys the workstation settings leave unset, and never written.

Terminal application selection is: the terminal picked for the action itself,
explicit `--terminal`, the project section, the workstation defaults, the
agent's default terminal, then environment/platform detection. The server sends
no terminal since #305 (`terminalOverride` is gone from the dispatch). The
editor that `open_editor` runs is the workstation default `editorCommand`, else
the `editor` an older server still sends, else `code`. Selecting `pty` or `none` uses a local PTY;
the explicit `open_terminal` action requires an external window. Legacy
`.taskflow/config.json` is not a second terminal-settings source.

## Skill ownership and recovery

The incoming skill list is installed in the user configuration of each agent the
project sets up, each as a single `SKILL.md`: `~/.claude/skills` for Claude,
`~/.agents/skills` for Codex, `~/.gemini/config/skills` for Antigravity. An agent that
substitutes arguments into the skill body receives the body carrying the ticket
reference. Providers without a skill convention receive the MCP registration
alone. `setupProviders` lists the additional agents; the provider that runs the
task is always included. All IDs and destinations are validated before any
installation. Duplicate paths, path traversal and symlink escapes are rejected.
The checkout receives no managed file.

Incoming managed content is authoritative at these paths. If an existing file
differs from the incoming content and from its last installed hash, save it at
`.taskflow/skill-backups/<content-hash>/<original-path>` before replacement. This
also covers legacy generated files without a manifest. Files outside incoming
or previously managed paths remain untouched. Durable customization belongs in
explicit local skill overrides, not edits to managed files.

For removed or renamed skills, unchanged previously managed files are backed up
and removed. Modified retired files are preserved and become unmanaged. The
manifest cannot claim arbitrary repository files. Writes are atomic per file;
an I/O failure aborts the launch, and a later fresh synchronization can repair a
partial installation. Installation is not a transaction across the entire tree.

## Launch intent and acknowledgement

`dispatch_step` uses the existing WebSocket envelope (`msgId`, `taskId`, `payload`).
The payload is the shared `agentconfig.Dispatch` type:

```json
{
  "schemaVersion": 1,
  "taskId": "full-server-task-id",
  "taskKey": "#48",
  "projectId": "actual-project-id",
  "skillId": "clarify",
  "action": "clarify",
  "prompt": "Additional instructions"
}
```

`open_terminal` may additionally supply `command` and `terminalOverride`. The
payload carries intent, not a second copy of provider/terminal settings. The
agent ignores obsolete configuration/path fields and fetches current settings.
Versionless legacy dispatches are accepted; other nonzero versions are rejected.
Upgrade server and agent together for the initial v1 rollout.

A macro-scoped skill (`realign_macro`, #426) is dispatched with the same
message and no task: `taskId`, `taskKey` and the envelope's `taskId` are empty,
`projectId` is set, and the payload names the macro:

```json
{
  "schemaVersion": 1,
  "taskId": "",
  "taskKey": "",
  "projectId": "actual-project-id",
  "macroKey": "M-7",
  "macroTitle": "Ux improvements and fixes",
  "skillId": "realign_macro",
  "action": "realign_macro",
  "mode": "interactive",
  "runId": "remote-run-id"
}
```

The agent runs it interactively in the project's mapped checkout, after
preparing the macro worktree (below), with `SECTILE_MACRO_KEY`,
`SECTILE_MACRO_PROJECT_ID`, `SECTILE_SPEC_REPO`, `SECTILE_SPEC_BRANCH` and
`SECTILE_SPEC_WORKTREE` in its environment and every `SECTILE_TASK_*` empty. On
a specifications folder that is not a Git repository, `SECTILE_SPEC_BRANCH` is
empty and `SECTILE_SPEC_WORKTREE` is `false`. It reads no task and records no
branch. An agent that predates `macroKey` refuses
the dispatch as a task dispatch without a task.

`step_status` correlates with `msgId` and reports `running`, `completed` or
`failed`, with a summary. `completed` acknowledges launch only, not completion of
the requested workflow stage. Web launch requests wait up to 45 seconds. The
native client uses Sectile MCP to read tasks/comments and submit verified stage
reports. All local workflow workers dispatch to the agent; the former server-managed result-file worker is retired.

## Runtime artifacts

`sectile-server` owns SQLite, HTTP APIs, tracker queues and the upstream MCP
service. `sectile-agent` starts the workstation daemon directly and owns the
`mcp` stdio bridge and internal `agent-exec` supervisor. Electron bundles only the
agent. Shared relay envelopes live in `internal/agentprotocol`; agent production
code does not import the server handlers, database or embedded UI.

## Three-party transport and errors

| Interface | Address and authentication | Ownership |
| --- | --- | --- |
| Server API | `http(s)://<server>:8090`; machine endpoints take the workstation API key as bearer credential | Tasks, project settings, tracker queues, `/api/v1/agent/*`, `/ws/agent-connect`, `/mcp` |
| Agent Loopback | `http://127.0.0.1:8091` or a dynamically assigned loopback port; the proxies take the same API key by default (`/mcp` alone permits explicit local no-auth opt-in), desktop/control calls use the private discovered desktop token | `/desktop/*`, `/control/*`, consoles, local repository mappings and MCP proxy |
| MCP | Server `/mcp`, a stateful Streamable HTTP endpoint, addressed directly with the API key; the `mcp --url <server>` stdio bridge relays for clients without HTTP transport | Typed tools with server-owned state; no local SQLite; one server session per connected client; no agent required |

The existing web REST API relies on the deployment's access-control boundary.
Machine bearer authentication does not add multi-user authorization to that API.
Cross-origin loopback requests are rejected. The private discovery file is
`~/.taskflow/agent-connection.json`; tokens never enter project configuration.

The server pings each agent connection every 10 seconds and drops a connection
that produces neither a frame nor a pong for 30. The agent answers these pings
from its own read loop, which stays responsive because operations run
concurrently. Without this, a connection lost without a close frame (a
suspended machine, a dropped VPN, an expired NAT binding) stays registered and
every request routed to it waits out its full deadline. The agent keeps sending
its own 30-second `heartbeat` message, which also refreshes the deadline.

Local workspace requests use the authenticated agent WebSocket:

```json
{"msgId":"request-uuid","type":"workspace_request","taskId":"task-id","payload":{"projectId":"project-id","taskId":"task-id","action":"git_evidence"}}
```

The agent fetches current configuration, validates task/project identity and
resolves its local mapping. A result is correlated to the exact connection:

```json
{"msgId":"request-uuid","type":"workspace_result","payload":{"value":{"sha":"commit","branch":"feat/task","clean":true}}}
```

An error result has `payload.error` instead. Supported actions cover Git status,
branches, checkout, deletion, diff and evidence; worktree preparation/removal and
inspection; editor opening; CLI/skill/SDD status and provisioning; skill reading;
and LLM prompt execution. There is no arbitrary shell action or working-directory parameter. Explicit
editor/provider settings retain their existing configuration behavior. Launches use the existing `dispatch_step` contract.

`pr_evidence` reads the pull or merge request carrying a branch (`payload.branch`,
else the task branch) with the forge CLI logged in on the workstation. The server
asks for it when the project's code remote is not a GitHub one, since it cannot
reach that forge itself. A forge that answered without a usable request returns
a `refusal` value instead of an error; an error result always means the lookup
itself failed and never that the request is absent:

```json
{"value":{"forge":"gitlab","url":"https://gitlab.example/g/app/-/merge_requests/9","branch":"feat/task","sha":"commit","open":true,"draft":false,"merged":false}}
{"value":{"forge":"gitlab","refusal":"no matching open or merged merge request; recover through the configured creation owner"}}
```

Without `origin` in the task checkout, the lookup fails with
`project checkout has no origin remote; ...` rather than a Git exit status.

`macro_worktree` (`payload.macroKey`, `payload.macroTitle`, no task) prepares a
macro's specification checkout on the workstation and answers
`{"path", "branch", "worktree", "warning"}`. The specifications folder is the
workstation's own setting for the project (`specRepos` in the local settings,
edited as "Specifications folder" in the desktop project dialog), else, on a
mono-repo project, the project's mapped checkout; a multi-repo project without
one is refused with a message naming the setting. The server holds no
specifications path (#443). On a Git folder the worktree is
`.tasks/worktrees/<KEY>` in that repository, on the existing branch named after
the key or a new `<KEY>-<slug>` from the fetched default branch; an existing tree
is reused as is. With worktrees off, the checkout itself is returned with
`worktree: false` and nothing is created. On a folder outside any Git checkout,
nothing is created either: the answer is the folder itself, an empty `branch`,
`worktree: false` and a `warning` saying nothing will be committed or pushed.
The server adds `projectId`, `macroKey` and the macro's `todos` when it relays
the answer through the `prepare_macro_worktree` MCP tool, so a skill invoked by
hand, which holds no API token, reads its input from the same call.

`macro_spec_file` (`payload.macroKey`, `payload.framework`, `payload.specFile`,
no task) reads one file of a macro's specification for the server's slicing
import: `specFile` is `tasks.md` or `spec.md`, looked for under `specs/` (or
`openspec/changes/` for OpenSpec) in the same specifications folder as
`macro_worktree`, in a folder named after the key. The working tree is read
first; on a Git folder only, the macro's branch is read when the working tree
does not carry the folder. It answers `{"content", "origin"}`, `origin` being
the path read or `<branch>:<path>`. Refusals are written for the user, in
French, and shown as they are. An agent that predates the action answers
`unknown local operation "macro_spec_file"`, which the server turns into a
request to update the desktop app; no agent connected for the requesting user
is likewise reported as the desktop app to connect.

`repository_worktree` (`payload.taskId`, `payload.repository`, `payload.branch`)
prepares a task's worktree in a secondary repository of a multi-repo project
(#456), with the logic of the primary worktree: the worktree that already has
the task branch checked out is reused, else `.tasks/worktrees/<KEY>` is created
in that repository's mapped folder. It answers `{"repository", "path", "branch"}`,
`repository` echoing the request; the server reads a missing echo as an agent
too old to answer. A repository outside the project, not mapped on the
workstation, or on a mono-repo project is refused. `remove_workspace` with
`payload.repositories` removes the task worktree in each of those repositories
and answers `{"removed": [...], "failed": [{"repository", "error"}]}`.

Each dispatch resolves the task's primary repository before anything starts:
its pin (`task.repository`), else the project's only repository, else the code
remote of a mono-repo project, else the only repository mapped on the
workstation, which the agent then pins. When several are mapped and none is
pinned, the agent parks the dispatch, marks the run through
`POST /api/activities/{id}/awaiting-repository` `{"waiting": true}` (autonomous
runs included), releases its run slot, and reads the task back every five
seconds until it is pinned; it then clears the mark and resumes the same run.
The launch receives the task's folder map as `SECTILE_REPOSITORIES`, a JSON
array of `{"remote", "identity", "role", "path", "worktree"}` where `role` is
`primary`, `changed`, `context` or `spec` and `path` is empty when the
repository is not mapped here. The first agent to see a project whose
`repositoriesMigration` is empty reads `GET /api/projects/{id}/legacy-repo-paths`,
resolves each path's `origin` locally and posts the result to
`POST /api/projects/{id}/repositories/convert`; the server applies the first
report and answers 409 to the others.

A macro run is stopped with `POST /api/projects/{id}/macros/{key}/cancel-run`
`{runId, force}`, which dispatches `cancel_run` with no task to the owner's agent,
under the same rules as a task run: an agent that no longer has the run closes it
as orphaned, and an unreachable agent needs `force`.

A pull request can live in a repository other than the project's (#392): a
project without a code remote, or not mono-repo, may name one through `prUrl`.
The server then sends `payload.repository`, the `host/path` identity of that
repository. `pr_evidence` looks the branch up there (GitLab only: the server
reads GitHub itself), using the task checkout, or the project root when there is
none, only as a working directory. `git_evidence` answers for a verified
checkout of that repository instead of the task checkout: the first of the
workstation's mapping of that repository (#456), the
task's `repoPath`, the project's `repoPath` and `repoPaths` whose own `origin`
names the repository and which has the branch checked out. Server paths are
hints; the local `origin` decides. Both answers echo `repository`; the server
reads a missing echo as an agent too old to answer, never as evidence about the
project checkout:

```json
{"value":{"repository":"gitlab.example/g/tools","found":true,"path":"/work/tools","sha":"commit","branch":"feat/task","clean":true}}
{"value":{"repository":"gitlab.example/g/tools","found":false,"path":"","sha":"","branch":"","clean":false}}
```

`found: false` means no verified checkout; the stage then proceeds on the
forge's evidence and its report says the head was not verified locally.

Requests normally have a 45-second deadline; purely local read-only inspections
(Git evidence, status and branches, worktree info, SDD/skill status, skill
reading, editor opening) allow 15 seconds and CLI probing 30, so an unreachable
agent fails quickly instead of stalling the caller; free-form prompt runs
(`run_prompt`) allow 12 minutes and SDD installation allows seven minutes.
Cancellation sends `workspace_cancel` with the same `msgId`. Disconnects and
unconfirmed results fail visibly and never trigger local server execution or an
automatic retry of a possibly completed mutation. Some local tool installers
cannot interrupt immediately; inspect the agent before retrying an uncertain operation.
Legacy server terminal endpoints return 410 and direct callers to desktop consoles.

No `/api/v1/agent/*` handler answers 404: an unknown project is a 400, a rejected
credential a 401, a wrong method a 405. A 404 on one of these routes therefore
means the route is not registered at all, which is a server build predating the
contract, and the agent reports it as a contract mismatch rather than a transport
failure. A route that answers but carries an unsupported `schemaVersion` is the
same failure and reads the same way. Both name the server, the route and the
build to update; the connection loop keeps retrying, because updating and
restarting the server is what clears them, but it stops calling them lost
connections. The standing mismatch is exposed as `contractError` on
`/desktop/status`, so the desktop reports an incompatible server instead of a
disconnected one. A contract route answering correctly retires it.

GitHub uses REST (GraphQL for Projects and issue transfer) from the server with
[explicit server credentials](../../README.md#server-tracker-credentials).
Pagination, authentication, rate-limit and transport errors propagate to tracker
activities. Jira synchronization is unsupported in the current baseline; no
Atlassian CLI fallback remains. Tracker credentials are not sent to agents.

## Project discovery and task identity

Authenticated `GET /api/v1/agent/projects` returns
`{"schemaVersion":1,"projects":[{"id":"server-primary-key","name":"Project","gitRemoteUrl":"..."}]}`.
Discovery excludes server filesystem paths and credentials.

`sectile-agent` defaults to `--project all`. Use `--list-projects` to
print available projects without starting the gateway or modifying repositories.
A single agent accepts launches for multiple projects. Each launch fetches fresh
project settings and resolves its repository using the local
`.taskflow/agent.json` `projects` mapping (server project primary key to local
directory), or by matching the current repository's origin URL. Unmapped projects
are reported at connection time and their launches fail explicitly. Repositories
are never cloned implicitly. Skills and MCP configuration are installed when a
multi-project launch is prepared.

`projectId` and `taskId` already mean server primary keys; `taskKey` is
the human-readable tracker reference. New dispatches carry all three. Native skill
invocation uses the full task reference, also exposed as `SECTILE_TASK_ID`;
`SECTILE_PROJECT_ID` identifies its project and `SECTILE_TASK_KEY` remains
available for display. MCP's historical `taskKey` argument accepts the full task
primary key, which should be preferred for transitions across projects.

Terminal settings continue to come from the server. `--terminal` is optional
and serves only as an explicit local override.

For compatibility, legacy registrations with project `default` or an empty
project remain wildcard registrations. New clients should use `all` explicitly.
Other scoped registrations never receive another project's dispatch.

Tracker configuration, stage mappings and transition validation remain server-side.
The agent contract carries secret-free tracker identity, but no credentials, stage mappings or ttyMode.

## MCP project discovery and review links

Native clients can call `list_projects({})` to discover the same
secret-free project records as the HTTP discovery endpoint, then pass an ID to
`get_project_context` or `list_tasks`.

`get_task` returns the task whenever it is known to the server. Its comments come
from the tracker, so a tracker that cannot be reached costs the caller the
discussion and not the ticket: the response then carries `commentsError` with the
reason and no `comments` field, so an unreadable discussion is never mistaken for
an empty one. Only an unknown task key is an error.

`create_task` files a new ticket on an explicitly named project and returns it with
its allocated key and external URL. The project identifier is required and is never
inferred: the HTTP creation path falls back to the first project when an identifier
does not resolve, so the tool rejects an unknown one rather than filing on another
board. Creation is remote whenever the project's tracker supports it, and a tracker
that cannot create remotely fails the call instead of leaving a ticket that exists
only locally. The new task enters the workflow at its first stage; no argument
places it at a later one.

`update_task` updates mutable descriptive fields of an existing task (`title`,
`description`, `priority`, `issueType`, `labels`) and returns the updated task.
`taskKey` is required and matches either the task's primary ID or its key. At least
one mutable field must be provided, and `title` cannot be empty. `description` can
be cleared by providing an empty string. The task's workflow stage and status cannot
be altered here; incoming stage labels are stripped and the task's existing stage
label is preserved. Updates are attributed to the authenticated caller and enqueue
tracker synchronization under that caller's identity.

`get_project_context` returns project identity, execution settings, specification
framework, pull-request creation stage and skill references (`id`, `directory`,
`command`), plus `skillDirectories`, the directories the agent writes skill files
into. Skill and command bodies are not inlined: a caller that needs one opens
`<skill directory>/SKILL.md` under one of those directories in its checkout. The
`.taskflow/config.json` and `AGENTS.md` writers keep using the full agent
configuration, which is unchanged.

`transition_stage` accepts `prUrl` for either a pull request or a merge
request. A task holds an ordered set of such links, oldest first, each keeping
the branch it was opened from; `prUrl` is its last entry, the task's current pull
request, and it is what the tracker synchronization job carries. A link the task
already holds is not recorded twice, and omitting the argument preserves the set.

One ticket routinely produces several pull requests: a first one merged, then a
follow-up pushed on the same branch. A pull request that shares a branch with a
recorded link is that follow-up and is appended: a merged link never vetoes it.
A pull request on a branch no recorded link mentions is a substitution and is
refused, naming the branches the task actually recorded; correcting or detaching
those links from the task detail view is how that refusal is resolved. For
example:

```json
{"taskKey":"full-task-primary-key","stage":"reviewed","note":"Review complete","branch":"feat/example","prUrl":"https://gitlab.com/example/repo/-/merge_requests/42"}
```

## Remote execution visibility

A delegated skill creates a `remote_run` activity before dispatch. Its ID travels
as `runId` and `SECTILE_RUN_ID`. Launch failure closes the run as failed;
successful process launch leaves it running.

Standalone skills call `start_run(taskKey, skill, runId?)`, retaining
the returned activity ID. A supplied launcher run ID reuses the existing run.
The invocation owner calls `finish_run(taskKey, runId, status, note)`
with completed, failed or canceled when it ends, including a stop for user input.
A macro skill run names `projectId` and `macroKey` instead of `taskKey`, in both
calls; naming both forms, or a macro key without its project, is refused. Such a
run is a project activity with no task, and its end hands nothing back to a
workflow chain.
Nested skills reuse their owner's run; intermediate transitions do not close it.
These activities never acquire the managed-stage transition guard.

A batch pickup (`pickup_issues`) is one run on the batch's first ticket, whose
other tickets the server records as its members (ADR 0034). The agent reuses the
launch run ID on every ticket: `start_run(taskKey, skill, runId)` on a member
with the batch run's ID returns the batch run, creates no run, and marks that
member as the one being processed, the previous one becoming done. The same call
on a ticket outside the batch, or with a batch run that ended, is refused as any
unmatched `runId` is. `finish_run` is called once, on the first ticket, when the
whole batch ends; every member stops showing the batch then.

Cards and list rows display a single run icon while a run is active: running takes
precedence over queued, and a cancellation stays visible briefly, updated
through server events and polling. Reading a task alone never marks it running.
A run started over MCP is owned by the session that started it, so an abrupt
client termination closes it as canceled instead of leaving the task active; the
note records that the client disconnected. Agent-dispatched runs keep their own
reporting path. This indicator reports declared execution state, not process
liveness.

### Declaring a wait for the user

A standalone skill calls `report_waiting(taskKey, runId, waiting)` with
`waiting: true` right before it asks its user a question it cannot continue
without. Ownership follows `finish_run`: the run's owner, an administrator, or
anyone on a run with no recorded owner. The run keeps `running` and gains
`waitingSince`; a repeated mark keeps the first instant. A headless run is left
unmarked and the result says so (`applied: false`). The wait ends on the
declaring session's next tool call other than `report_waiting` (a ping does not
count), on `waiting: false`, on any terminal status, and when the declaring
session ends. Tool permission prompts are not reported: only a question the model
asks deliberately is.

When a run an agent dispatched starts or stops waiting, the server sends the
owner's agent a `run_waiting` message, `{"runId": "...", "waitingSince":
"<RFC3339>"|null}`, locally or through another instance. An agent holding that
run records `waitingSince` on its `/desktop/runs` entry, except for a headless
run, and ignores a run it does not hold; the desktop raises its banner from that
list. The message is additive: an agent that predates it logs the unknown type
and carries on. A message sent while no agent is connected is lost, and the
agent's list catches up on the next change.

## MCP session ownership

`/mcp` is served statefully: each client holds one server session, identified by
`Mcp-Session-Id` and told apart from any other session sharing the same bearer
credential. A session begins when its client completes initialization and ends on
client termination, a dropped connection, or a server restart.

`GET /api/mcp/sessions` lists live sessions with the identity the client declared
in `clientInfo`, its connection time, and the runs it owns. It is a browser-facing
status view and carries no credential; the MCP endpoint itself keeps the machine
API authentication. The board's status bar polls it and shows the connected
clients with the runs each one holds. Connecting and disconnecting raise no
server event, so that view is refreshed by polling and is stale by at most one
interval.

A run created by `start_run` is adopted by the calling session. Ending the
session closes the runs it still owns with status `canceled` and a note naming the
disconnection; `finish_run` releases a run first, so a reported outcome is never
overwritten. A run reused from a launcher through `runId` or `TASKFLOW_RUN_ID` is
not adopted: it belongs to the agent that dispatched it, whose supervisor reports
the real process exit. A client connected through a transport without sessions
keeps the previous behaviour, where only `finish_run` closes a run.

`SECTILE_MCP_SESSION_TIMEOUT` bounds how long a client may say nothing before the
silence is remarked upon, defaulting to four hours; an unusable value keeps the
default. Crossing it never ends a session and never cancels a run: it appends one
sentence to each run the session owns, once per silent stretch, and any later
message rearms the observation. A run canceled by a real disconnection stays
recoverable: its owner, or an administrator, may still report its outcome through
`finish_run`, which replays the hand-back on the corrected status. `SECTILE_MCP_CLIENT` names the
bridge in the session list, defaulting to host and process id.

`SECTILE_MCP_SESSION_ABANDON_AFTER` bounds how long a silence lasts before the
session is taken for a client that died without closing its connection,
defaulting to eight hours; a value below the silence bound is raised to it, and
an unusable value keeps the default. Crossing it closes the session, the
transport's included, and cancels the runs it owns with the disconnect note,
after the silence sentence; their owner may still report the real outcome. The
rewrite matches the disconnect note anywhere in the summary, so a run silenced
and then closed stays recoverable.

The server pings every session every `SECTILE_MCP_KEEPALIVE_INTERVAL` (25s by
default) over the standalone `GET /mcp` stream, so a proxy never finds that
stream idle. A session that owns no run is closed, transport included, once
`SECTILE_MCP_KEEPALIVE_FAILURES` pings in a row (3 by default) got no answer and
its client sent nothing in between. A session that owns a run is never closed by
a missed ping: the silence and abandon bounds above still decide for it. A ping
reply is not a client message and does not reset the silence. An unusable value
of either setting keeps its default.

A run a client created has no agent to stop. `POST /api/tasks/{id}/cancel-run`
closes it instead, for its owner or an administrator only (an ownerless run is an
administrator's), through the same path as `finish_run`: status `canceled`, the
disconnect note, the hand-back, and the run released from its session. The
activities view's `POST /api/activities/{id}/cancel` closes it the same way,
with a note of its own that makes the cancellation final. Agent-dispatched runs
are unchanged.

A restart destroys every session at once, so startup closes the runs those
sessions owned, with status `canceled` and a note naming the restart. A run's
action records its owner and survives the restart: `Agent-owned remote execution`
keeps a supervisor that reconnects and reports the real process exit, so it is
preserved, while a run a client created has nothing left to close it. An outcome
already reported is never rewritten. This refines ADR 0006, which preserves
active remote executions across startup: the guarantee holds for the executions
whose owner the restart did not take down with it.

## Canceling agent-owned executions

Agent-dispatched skills use a terminal-side supervisor with an isolated foreground
process group on macOS/Linux. The task's **Stop** button sends a correlated
`cancel_run` dispatch for its run ID. The owning agent requests an interrupt,
escalates to process-group termination after two seconds if needed, and confirms
exit through an authenticated loopback control endpoint. The server marks the
activity canceled only after confirmation; a timeout leaves its state unchanged.
Stopping a run preserves repository changes and does not change workflow stages.
Independently launched native clients are not owned by the agent and have no
process-stop button. Supervised native execution is not supported on Windows.

## PR/MR creation timing

Projects persist `prCreationStage`: `implemented` (default, existing behavior)
or `specified`. This setting is included in the agent/MCP project configuration
and effective project skill instructions. With `specified`, the specification
skill commits and pushes validated specs, opens or reuses a draft PR/MR, and
attaches its URL in the specified transition. Implementation and review update
that same PR/MR; only completed review makes it ready. Opening the draft alone
does not advance the task to reviewed. Tracker synchronization remains server-owned.

A workstation that drops the specification artefacts (`specArtifacts`) has
nothing to show on the branch at specification. When a `specified` transition
of such a project names no pull request, the server asks the actor's agent
with the `spec_artifacts` operation, which answers `{"mode":"keep"|"drop"}`
with its effective value for the task. On `drop` the transition is accepted
without a pull request and its note says that it is deferred to the
implemented stage, which requires it as usual. Any other answer, including an
error from an agent that predates the operation, keeps the requirement.

## Local console service

The agent always hosts task consoles in local PTYs regardless of the
server terminal preference. The agent creates a random control credential and
publishes a private connection file for the optional Electron companion.

The /desktop/runs, /desktop/projects, /desktop/stop and /desktop/terminal endpoints
are authenticated loopback-only capabilities. Browser Origins are rejected.
Project mappings are saved locally and never uploaded. MCP remains available
to native clients through the local gateway.

`GET /desktop/runs` exposes `createdAt` (submission time) and optional `startedAt`
(UTC time when the command is successfully submitted to its PTY). Runs that have
not launched omit `startedAt`; completion preserves both timestamps. Desktop
clients fall back to `createdAt` for legacy records without a valid start time.
This display metadata does not change queue scheduling.

Web skill launches without a connected agent fail explicitly rather than falling
back to server-side execution.

### Desktop agent restart

Authenticated `POST /desktop/restart` returns 204 and schedules a daemon restart
only when all registered processes have confirmed exit. Active runs or a restart
already in progress return 409. New run registration is rejected once restart
begins. The desktop confirms with the user, stops active runs through
`/desktop/stop`, requests restart, and reconnects using the rewritten private
connection file. Arguments, environment and local mappings are preserved;
in-memory console history is cleared.

The **Local agent** panel exposes launch configuration. Stop the daemon before
changing settings, then use **Start local agent**. **Stop agent** uses authenticated
`POST /desktop/shutdown` with the same confirmed-exit guard as restart.
Desktop launch settings are saved locally; the API key is encrypted with
Electron safeStorage when OS encryption is available, and kept in the same
owner-only settings file otherwise, so a single-use pairing code is never lost. Existing agents launched outside the desktop do not expose their
server credentials to this panel.

**Clear finished consoles** removes completed, failed and canceled consoles from
the local agent through authenticated `DELETE /desktop/history`. Only sessions
with confirmed process exit are removed. Active executions, server task comments,
and the AI provider's own saved conversations are preserved.

### Optional desktop companion

The server, local agent and desktop app are independent components. Start the
agent without the app:

```sh
sectile-agent pair --url http://localhost:8090 --code '<pairing code>'   # once
sectile-agent --url http://localhost:8090 --repo /path/to/repository
```

The agent owns PTYs, supervision and console history. The desktop discovers it
through `~/.taskflow/agent-connection.json` (private, mode 0600), including when
opened after executions begin. Closing the app leaves executions running.
The desktop can also start the same agent when none is running.
`--desktop` is a deprecated no-op; `--terminal` is accepted for compatibility
but executions always use agent-owned consoles. `--desktop-info` can override
the discovery file for isolated instances; the app automatically discovers the
default file and its legacy private connection file.

### Execution defaults and local overrides

The workstation settings file is written in layout 3:

```json
{
  "server": "https://sectile.example", "deviceId": "laptop", "apiKey": "...",
  "layout": 3,
  "defaults": {
    "aiProviderModels": {"claude": ["claude-opus-5", "claude-sonnet-5"]},
    "terminal": "ghostty", "editorCommand": "cursor",
    "useWorktrees": true, "parallelism": 2, "setupProviders": ["codex"]
  },
  "projectSettings": {
    "project-id": {
      "path": "/path/to/repository", "specPath": "/path/to/specs", "terminal": "iterm",
      "useWorktrees": false, "parallelism": 1, "setupProviders": null,
      "skillCommands": {"implement": "code-issue"}, "specArtifacts": "drop"
    }
  },
  "engines": {
    "catalogue": [
      {"id": "e-1f3a9c20b7d4", "name": "Claude Opus", "provider": "claude",
       "model": "claude-opus-5", "skillModels": {"implement": "claude-sonnet-5"},
       "command": "", "commandAutonomous": ""},
      {"id": "e-7c01d2e9aa51", "name": "Codex", "provider": "codex", "model": "gpt-5"}
    ],
    "default": "e-1f3a9c20b7d4",
    "projects": {"project-id": "e-7c01d2e9aa51"},
    "tasks": {"task-id": "e-1f3a9c20b7d4"}
  },
  "repositories": {"github.com/owner/other": "/path/to/other"},
  "seeded": {"defaults": "https://sectile.example", "defaultEngine": "e-1f3a9c20b7d4",
             "projects": {"project-id": "2026-09-25T00:00:00Z"}}
}
```

Every field is optional and an empty one inherits; `useWorktrees` absent,
`parallelism` 0 and `setupProviders` null inherit. A present
`aiProviderModels` key is a choice, even with an empty list; an absent one
offers the shipped list. `skillCommands` replaces the slash command a stage
runs, a single word with an optional leading `/`. `specArtifacts` overrides
whether this workstation keeps or drops the tasks' specification artefacts
(`keep` or `drop`, #487); absent follows the server. Saving the desktop project
settings with an effective `keep` removes the project's block from every
checkout the workstation maps for it; lines outside the block are never
touched.

`engines` (#510) holds the catalogue in its order (1 to 20 engines, names
unique ignoring case, at most 64 characters), the workstation default engine,
each project's default engine and each task's engine, by task primary key.
Identities are the agent's: a choice naming no catalogue engine reads as none
and is dropped on the next write, and removing an engine drops the choices
pointing at it. The key sits outside the ones an agent of layout 2 replaces,
so such an agent keeps it when it saves.

The engine fields of layout 2 (`aiProvider`, `aiCommandTemplate`,
`aiCommandTemplateAutonomous`, `aiModel`, `aiSkillModels` in `defaults` and in
project sections) are converted into catalogue entries on every read, and the
agent persists the conversion once at start, after copying the previous file
to `settings.json.bak-layout<N>`. Each level becomes an entry equal to what it
resolved, identical profiles share one entry, and a project that stated
engine fields picks its entry. The conversion runs again, reusing identical
entries and keeping the default engine, if an older agent writes engine fields
back.

A file written before #305 (flat `aiProvider`, `aiModel`, `terminal`... and the
per-project maps `projects`, `specRepos`, `worktrees`, `parallelism`,
`terminals`, `aiProviders`, `aiModels`, `commands`, `commandsAutonomous`,
`specArtifacts`) is
read with the same meaning and rewritten in the current layout on the next
save. An emptied map or list leaves the file rather than keeping its previous
content.

The desktop edits both levels through the agent: `GET`/`PUT /desktop/workstation`
for the defaults, `POST /desktop/projects` for a project section (each field
with an `inherit…` flag), and `GET /desktop/project` answers, per execution
field, `{value, inherited, source}` with `source` one of `project`,
`workstation` or `default`. An invalid value is refused with its reason and the
file is left unchanged. The Electron companion writes connection keys only.
Both saves refuse the engine fields of layout 2 with a 400 naming the engine
catalogue; a project save picks its engine with `defaultEngine` (an engine
identity, unknown: 400) or `inheritDefaultEngine: true`.

`GET /desktop/status` advertises `task-engines` when the agent keeps the
catalogue. The desktop then uses:

- `GET /desktop/engines`: `{catalogue, default, providers, providerModels,
  projects, taskCounts}`, the last two saying what points at each engine;
- `PUT /desktop/engines` with `{catalogue, default}`: the whole catalogue in
  its new order, a new engine without `id`; 400 with the validation message,
  409 when the default engine is left out, then a new capability report;
- `GET /desktop/task-engines?projectId=<id>`: `{projectDefault, catalogue:
  [{id, name, provider, model}], tasks: {taskId: engineId}}`;
- `PUT /desktop/task-engines` with `{projectId, taskId, engineId}`: stores the
  task's engine, none when it is the project default engine; 404 when the
  engine is not in the catalogue. The capability report does not change.

`repositories` maps each repository of a multi-repo project, by its
`host/path` identity, to the folder holding its checkout on this workstation
(#456). It is keyed by repository rather than by project, so one checkout
serves every project that works in it; the desktop project settings write it,
and refuse a folder whose `origin` is another repository. The project's own
repository keeps its folder in its project section's `path`.

Without effective worktrees, the agent enforces one execution and the UI
disables parallelism selection. Requests are acknowledged when queued; their
remote run remains active until completion or cancellation. The agent reserves
capacity before repository preparation, admits queued requests in order within
each project, and holds capacity until confirmed process exit. Queued executions
can be canceled from either UI. Tasks sharing an unisolated repository, or the
same task worktree, cannot execute concurrently. Settings are resolved at
admission into the queue; changes apply to subsequent submissions. Queue and
console history are held in memory for the agent lifetime.

### User configuration and commands

Agent settings and project mappings live in
`~/.config/sectile/settings.json`, shared by the CLI agent and companion.
Writes preserve connection fields, use atomic replacement and mode 0600.
Legacy repository mappings and the pre-#305 layout remain readable and are
migrated on the next save.

### Execution seed

`GET /api/v1/agent/execution-seed[?projectId=<ID>]`, agent bearer, serves the
execution values a server stored before #305, read-only, for one release
(#492 drops them):

```json
{
  "schemaVersion": 1,
  "defaults": {"aiProvider": "claude", "aiCommandTemplate": "...", "aiModel": "...",
               "aiSkillModels": {}, "aiProviderModels": {}, "terminal": "...", "editorCommand": "..."},
  "project": {"projectId": "...", "aiProvider": "...", "aiCommandTemplate": "...", "aiModel": "...",
              "aiSkillModels": {}, "terminal": "...", "useWorktrees": true,
              "setupProviders": [], "skillCommands": {}}
}
```

`defaults` is the deployment row, with the caller's own terminal and editor
(which fall back to the deployment's); the column default editor `code` is not
sent. `project` is what the configuration composed for the project before
#305, project row over deployment, its terminal being the project row's own.
Empty values are omitted.

The agent seeds its defaults after the first identity check of a connection,
and a project the first time it resolves it, once each (`seeded`). It writes
only the keys the file does not set, and only where they change the outcome of
the local resolution, computed with the pre-#305 precedence so a launch
resolves what it resolved before the upgrade. Since #510 the engine goes to
the catalogue: the deployment's becomes the workstation default engine while
the workstation states none (`seeded.defaultEngine`), and a project that
resolves differently picks an entry, found or created, completing the pick it
may already have. A fetch failure writes and marks
nothing; the next connection or resolution retries. A disconnected project is
not seeded, and pairing to another server does not seed the defaults again.

### Capability report

`PUT /api/v1/agent/capabilities`, agent bearer, answers `204`:

```json
{
  "schemaVersion": 1,
  "deviceId": "laptop",
  "projects": [{"projectId": "...", "provider": "claude", "model": "claude-opus-5",
                "skillModels": {"implement": "claude-sonnet-5"},
                "models": ["claude-opus-5", "claude-sonnet-5"],
                "modelSlot": true, "headless": true}]
}
```

The agent sends it, for every project it serves, after registering its
WebSocket and after each local save. It describes the project default engine;
a task switched to another engine changes nothing in it. A model is reported only when it reaches
the command line; `modelSlot` says whether a launch model can; `headless`
whether an autonomous run is possible. The report belongs to the credential's
user and to `deviceId`, the device the agent presents on its WebSocket; the
server keeps the latest per user, device and project.

`GET /api/projects/{id}/engine` (web session) returns the caller's own report
for the workstation connected for that project,
`{"state": "reported", ...the fields above..., "reportedAt": "..."}`, or
`{"state": "unknown"}` when none of their agents is connected for it. A launch
records its provider and model from that report; without one they stay empty
until the agent posts the engine it actually launched
(`POST /api/activities/{id}/engine`).

| Command | Action |
| --- | --- |
| `make server` | Build the server |
| `make agent` | Build the local agent |
| `make desktop` | Build and package the desktop |
| `make all` | Build all components |
| `make start` | Start the local agent |
| `make serve` | Start the server |
| `make run` | Start the desktop |

Server and agent are built as `bin/sectile-server` and `bin/sectile-agent` by the `build-*` targets.
The `serve`, `start` and `run` targets run from source and need no prior build. Pass agent
arguments with, for example, `make start ARGS="--url http://localhost:8090"`;
the workstation must be paired first with `sectile-agent pair`.

## Workstation API keys and identity

Every machine surface authenticates with one workstation API key, prefixed
`sectile_`, that binds one workstation to one user. Only its hash is stored, so a
copy of the database yields no usable key.

- `POST /api/devices` with `{"label", "ttlDays"}` creates a key for the signed-in
  user and returns `{"token", "device"}`; the plaintext is returned once. `ttlDays`
  absent means 90 days, `0` means no expiry. The interface offers this only as the
  advanced case of an MCP client with no agent to pair for it; pairing is the
  normal path and never shows a key.
- `GET /api/devices` lists the user's keys with `ExpiresAt` (`null` for none);
  `PUT /api/devices?id=` with `{"ttlDays"}` moves or clears the expiry without
  changing the secret; `DELETE /api/devices?id=` revokes one.
- `POST /api/pairing-codes` issues a single-use code valid ten minutes;
  `POST /api/v1/agent/pair` with `{"code", "label"}` exchanges it for a key with
  the default expiry and answers `{"token", "deviceId", "userId"}`. This is the
  only unauthenticated agent endpoint: the code is the proof, consumed
  atomically, so a replay returns 401. Unknown, consumed and expired codes all
  answer `401 Invalid or expired pairing code`.
- `GET /api/v1/agent/identity` tells a key holder `{"userId", "role", "mode",
  "deviceId", "label", "expiresAt", "sharedToken"}`; the agent calls it before
  connecting and logs a warning when fewer than ten days remain.

A key past its expiry is refused on every surface with `401 {"error":"API key
expired"}`, distinct from the generic refusal, so the owner renews it rather
than retyping it. Revocation cuts every surface at once.

The agent gateway takes the same key on `/mcp` and `/api/`, and consoles it
launches receive it as `SECTILE_AGENT_TOKEN` with `SECTILE_AGENT_URL` set to the
server. Deprecated for one release: `SECTILE_SERVER_TOKEN`, accepted with a
startup warning. It is the only credential outside the key store that a machine
surface accepts; the open mode of a server without it, which accepted any
nonempty token, is removed, so a server that has issued no key refuses an
invented token like any other. Credentials issued before keys expired keep
working as keys without expiry.

## Roles and owned executions

Every user holds one of two roles, `admin` or `member`, and a workstation API
key confers the role of the user it is bound to on every surface. `GET /api/me`
reports `{"mode", "role", "signedIn", ...}`, where `mode` is `oidc`, `local` or
`implicit`.

- Admin-only: `POST`/`PUT /api/settings` beyond a member's own preferences,
  `POST /api/setup/tracker*`, project creation, update and deletion,
  `GET /api/users` and `PUT /api/users/{id}`, and another user's workstations
  through `?userId=`. A member is refused with `403` naming the role, distinct
  from the `401` an anonymous caller gets.
- Executions carry `userId` and `userName`. Everyone reads every run;
  `POST /api/tasks/{id}/cancel-run` is refused with `403` to anyone but the
  owner and admins, and the cancel is dispatched to the **owner's** agent, so a
  run is closed as orphaned only when the agent that should hold it says it does
  not. A run without recorded owner predates ownership and is admin-only.
- `POST /api/agent/dispatch` routes to the caller's own agent whatever `userId`
  the body names; only an admin may address another user's agent.
- The MCP `finish_run` tool refuses a run owned by another user, for the same
  reason: reporting a run finished hands the workflow back and leaves the real
  process running on its owner's machine. An ownerless run stays closable.
- The agent gateway's `/api/` forwarding authenticates with the workstation key,
  which stands in for a session and carries its user's role. The deprecated
  shared token does not: it names no key and is refused on the interface API.

Sign-in itself is either the OpenID Connect flow of ADR 0008 or, while no
provider is configured, `POST /auth/local {"email"}`, which creates or finds an
account and opens a session. The first account created becomes admin; with a
provider supplying `SECTILE_OIDC_ROLE_CLAIM`, the claim is the authority at
every sign-in. See [ADR 0013](../adrs/0013-roles-owned-executions-and-local-sign-in.md).

## Local project disconnection

The authenticated loopback API supports `DELETE /desktop/projects?id=<project-id>`.
It returns 204 after atomically recording workstation disconnection and removing
the project's mapping, worktree preference, parallelism, and command override.
Repeated removal is safe. Missing IDs return 400, unauthorized requests return
401, active work or busy configuration returns 409, and settings failures return
500. No remote project mutation or repository deletion occurs.

`GET /desktop/status` advertises `remove-project` in `capabilities` and returns
`disconnectedProjects` as an array of IDs. Project discovery includes a
`disconnected` boolean; disconnected entries have empty `path` and false
`configured`. Retained `/desktop/runs` history does not imply reconnection.

Workstation settings store `disconnectedProjects` as a map of true markers. The
field is not imported from repository configuration. Resolution rejects marked
projects before implicit matching, and reconnect-time tooling deployment skips
them. A successful validated project mapping POST clears the marker atomically.
Execution registration and removal share the preparation lock, followed by the
run lock, so removal cannot succeed concurrently with admission of unfinished
work. Existing processes must have confirmed exit before disconnection succeeds.

## Adjustment and PR ownership

The canonical review action is `adjust` (`adjust-issue`). Legacy `review` normalizes to it without rewriting activity history. `create_pr` and `create-pr` identify the standalone Create PR utility, which has no stage transition.
States remain `new`, `clarified`, `specified`, `implemented`, `reviewed`, `finished`.
A reviewed task offers Handoff; repeat Adjust is explicit and requires an open PR.

The `prCreationStage` policy assigns draft creation to specification or implementation
(default). Adjustment requires an existing matching PR (open, or already merged by the
human, in which case it reviews the merged state without pushing), performs full review
and feedback disposition, checks the final code, updates the same PR and verifies
readiness. Lookup failure is not absence. Creation-owner recovery retains an already
implemented stage. Completion records the PR URL at the owning stage.

Skill records may include `requiresReconciliation`. Native dispatch refuses these
customizations until their legacy content has been reviewed and saved under Adjust
or reset in the skill editor. Legacy entries and divergent installed files remain
available; custom adjustment content also receives the current built-in contract.

Managed adjustment verifies the PR identity against the task's recorded set:
the same PR, or a newer one on a branch the task already used, together with the
branch, pushed commit, clean checkout and reported build/lint/test checks at
completion. A branch carrying several merged pull requests and none open is
evidenced by its most recent merge; several *open* pull requests on one branch
remain an unresolvable ambiguity and fail the lookup. Standalone transitions verify forge identity and readiness;
check output remains agent-reported. Human merge and handoff remain separate.

## Local Desktop worktree comparison

The authenticated local agent advertises `git-diff` in `/desktop/status` and accepts
`GET /desktop/git-diff?id=<runID>`. Electron exposes only `localAgent.gitDiff(runID)`;
the main process retains the credential. Existing bearer-token and Origin rejection
rules apply. No server relay or caller-selected filesystem path is supported.
Responses use `Cache-Control: no-store`.

Run metadata includes the verified assigned `branch`. The handler copies execution
identity and its repository root under the run lock, then inspects outside that lock.
An existing checkout must match the recorded directory, repository common Git
directory, and branch. Inspection never prepares or creates a worktree.

Success returns `runId`, `taskId`, `projectId`, `directory`, `branch`, `baseRef`,
`baseCommit`, `mergeBase`, `headCommit`, and UTC `generatedAt`; `isClean`, `complete`,
`countsPartial`, `filesChanged`, `additions`, `deletions`; and `warnings` plus `files`.
Each file has `path`, optional `oldPath`, `status`, `kind`, nullable text counts,
`patch`, and optional `omittedReason`. Status is added/modified/deleted/renamed/type-changed;
kind is text/binary/symlink/submodule/unsupported. Counts sum displayed known text
changes only. Incomplete results cannot be clean.

The baseline resolves existing local refs in this order: symbolic `origin/HEAD`,
remote main/master, local main/master. An invalid recorded default does not permit
fallback. Exactly one merge base is required. The comparison uses a private temporary
index/object directory, with the real object store available only for reads, so
Git can detect net changes and renames without altering repository state. External
diff, textconv, and filesystem-monitor helpers are disabled. HEAD, branch, default
ref, index bytes, file membership and metadata, and inspected submodule HEADs are
rechecked; a detected change retries once within the request deadline.

Bounds: 10 seconds, 1,000 returned files, 256 KiB per patch, 4 MiB aggregate patch
collection and serialized response, 8 MiB metadata/per-file reads, and 64 MiB of
materialized content. Limits produce explicit omissions/partial totals, or an error
when metadata cannot be interpreted. Temporary storage is removed on return.

Errors use `{ "error": { "code": "...", "message": "..." } }`: unknown runs are
404; unavailable/mismatched checkouts, baseline history, unmerged indexes, unsupported
paths, and concurrent changes are 409; unusable metadata limits are 413; timeouts
are 504; other Git/read failures are 500. Unsupported methods are 405 with `Allow: GET`.
Messages explain recovery without returning subprocess output or source contents.

## MCP naming contract

HTTP and stdio initialize with server name `sectile`; managed native registrations
use the same name. The catalog is exactly `get_task`, `transition_stage`,
`add_comment`, `list_tasks`, `get_project_context`, `list_projects`, `start_run`,
`finish_run`, `create_task`, `update_task`, `report_waiting` and `prepare_macro_worktree`. The former `sectile_` names are unsupported on both
transports.
Tool schemas, return values, run ownership and managed-run validation are unchanged.

Agent launch prompts, desktop exit reporting and built-in policy text use the
canonical names. User-owned stored instructions remain untouched. Upgrade server
and agent together, migrate registration through normal bootstrap, reconcile any
explicit policies requiring manual migration, and reconnect clients. See the
[upgrade guide](../../README.md#mcp-naming-upgrade). No change is made to the
versioned agent DTO, `SECTILE_RUN_ID`, filesystem paths or protocol markers.

### Desktop skill result lookup

`GET /desktop/run-result?id=<run-id>` is authenticated with the loopback agent
credential and accepts only an execution owned by that agent. It checks the
server task identity/project, then returns the matching activity's `id`, `taskId`,
`skillId` and `status`, plus task `status` and `labels`. `activity` is null if no
matching activity exists. Unknown runs return 404, mismatched task identity 409,
and unavailable server data 502. No logs or prompts are included.

This read-only result is independent of the console process status. The desktop
uses an exact execution match and completed workflow stage to display completion;
process exit, a prior execution's success, or an idle console are insufficient.
The run registry lives in agent memory, so a run disappears once the history is
cleared or the agent restarts. The desktop treats the resulting 404 as "no
result" rather than an error: the main process returns null to the renderer
instead of rejecting the IPC call, and clearing the history drops the removed
runs locally before the next poll.

### Traced autonomous runs

An autonomous (headless) run has no PTY. When its engine was asked for its
reasoning stream (the default `claude` headless line carries
`--output-format stream-json --verbose`), the agent reads that stream line by
line, keeps the last 2000 rendered lines in memory and reports the run with
`trace: true` in `/desktop/runs`. For such a run `GET /desktop/terminal?id=<run-id>`
upgrades to a read-only websocket that replays the trace and follows it live;
anything the client sends on it is discarded. A headless run with no trace keeps
answering 409 on that route, and the desktop shows its notice instead.

The trace is a local copy for the pane only: the durable record stays the run
activity on the server, fed by the unchanged incremental output transport, which
receives the engine's final answer and any diagnostic printed beside the stream,
never the stream's frames. A run that ends without an answer reports the reason
its result frame carries, so a failure is not recorded as an empty entry.

### Desktop project prompts

`GET /desktop/status` advertises `free-console` and `task-engines`.
Authenticated `POST /desktop/consoles` accepts
`{"projectId":"...","engineId":"..."}`. The engine must exist in the workstation
catalogue; a removed identity returns 404. The desktop offers the catalogue
from `GET /desktop/task-engines?projectId=...` and selects the project default.
The engine's model and interactive template apply to this launch only.
Templates receive an empty prompt and the mapped repository as `{repoPath}`.
Built-in providers open an interactive session without a prompt argument.
Legacy `{"projectId":"...","provider":"codex"}` requests remain supported
for built-in providers. Custom commands must be selected by engine identity.

The run has `kind: "console"`, a `provider`, the selected `engineId`, `engineName` and `model`, a unique local ID, and empty task,
skill, and prompt fields. Its CLI receives no arguments. Project mappings and
execution limits apply; the console reserves the mapped shared checkout through
the existing queue. It does not scaffold tooling, create a worktree, or mutate
remote tasks. The daemon owns the launch after admission even if the request ends.

These runs use the usual runs, terminal, stop, restart protection, and history
endpoints. Completion updates only local process status. Task result lookup
returns 404, and clients must omit task workflow and PR controls for these runs.


### Local MCP provider configuration

The authenticated desktop API exposes `GET /desktop/mcp?provider=<provider>`
and `POST /desktop/mcp?provider=<provider>`. Supported providers are `claude`,
`agy`, `codex`, `cursor`, `gemini` and `vibe`. POST accepts
`{"transport":"http|stdio","target":"remote|local"}` and updates the provider's
user configuration plus the workstation's `mcpConnections` preference.
Responses contain `choice`, `path`, `server` and `localURL`, never the API key.
Invalid providers or choices return 400; a busy preparation returns 409.

The local target enables no-auth access only to the loopback `/mcp` endpoint
while at least one saved provider selects it. Origin and Host validation remain
in force. `/api/`, desktop/control routes and remote surfaces keep their existing
authentication. Agent restart refreshes saved registrations to the current local
port; automatic task setup preserves the selected mode. See ADR 0023.
