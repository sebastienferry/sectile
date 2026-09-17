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
| `useWorktrees` | Create/reuse task worktrees when true; validate the existing checkout when false. |
| `aiProvider` | `codex`, `claude`, `agy`, `gemini`, `cursor`, `vibe`, or `custom`; empty uses the legacy `agy` default. |
| `aiCommandTemplate` | Optional shell template containing `{prompt}`. Required for `custom`; argument placeholders are shell-safe: `{prompt}`, `{issueKey}`, `{issueTitle}`, `{issueDesc}`, `{branchName}`, `{repoPath}`, `{tracker}`, `{repo}`. Task values are fetched for each launch; branch/path identify local execution. See [desktop usage](../../desktop/README.md). Custom providers still require supported native MCP bootstrap. A template without `{prompt}` on a named provider is legacy data: the server serves it as empty and the provider default runs. |
| `aiModel` | Optional model the engine runs against. Passed as `--model <value>` to `claude`, `codex`, `gemini` and `cursor`; ignored by `agy` and `vibe`. Empty keeps the CLI default. A command template supersedes it: no flag is injected, and the value reaches the template only through its optional `{model}` placeholder, and an unresolved placeholder is removed together with the option that introduces it. Validated on shape (`^[A-Za-z0-9][A-Za-z0-9._:@/-]*$`), never against a list of known models. |
| `aiSkillModels` | Optional `skillId -> model` map for the skills that depart from `aiModel`. An absent or empty entry inherits; it never means "no model". Entries naming no configured skill are ignored. |
| `externalTerminalCommand` | Terminal application/launcher selection. No silent fallback to a hidden PTY after launch failure. |
| `skills` | Array of `{id, directory, command, content, commandContent}`. IDs and installation destinations must be unique and safe. |

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

Server settings resolve project overrides over global defaults. On the workstation,
`.taskflow/agent.json` supports repository mappings (`projects`), `aiProvider`,
`aiCommandTemplate`, `aiModel`, `aiSkillModels`, `terminal` and skill content
overrides (`skills`).
These values are never uploaded. Changing the provider locally without a local
command template clears the inherited provider's command template.

Model selection resolves level by level, most specific first: workstation, then
project, then global. The most specific statement wins: naming a skill outranks a
bare model, whatever level that bare model sits on, so a bare model governs only
the skills no level singles out. Every level empty reproduces the command lines
that predate model selection.

Execution parallelism has no server-side counterpart: the configuration payload
carries no `parallelism` field, and the workstation value in
`~/.config/sectile/settings.json` is the only source. An agent predating this
removal reads no value and falls back to a single execution per project.

Terminal application selection is: explicit `--terminal`, explicit request
`terminalOverride`, local overrides, project configuration, global configuration,
then environment/platform detection. Selecting `pty` or `none` uses a local PTY;
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
| Agent Loopback | `http://127.0.0.1:8091` or a dynamically assigned loopback port; the `/mcp` and `/api/` proxies take the same API key, desktop/control calls use the private discovered desktop token | `/desktop/*`, `/control/*`, consoles, local repository mappings and MCP proxy |
| MCP | Server `/mcp`, a stateful Streamable HTTP endpoint, addressed directly with the API key; the `mcp --url <server>` stdio bridge relays for clients without HTTP transport | Typed tools with server-owned state; no local SQLite; one server session per connected client; no agent required |

The existing web REST API relies on the deployment's access-control boundary.
Machine bearer authentication does not add multi-user authorization to that API.
Cross-origin loopback requests are rejected. The private discovery file is
`~/.taskflow/agent-connection.json`; tokens never enter project configuration.

The server pings each agent connection every 10 seconds and drops a connection
that produces neither a frame nor a pong for 30. The agent answers these pings
from its own read loop, which stays responsive because operations run
concurrently. Without this, a connection lost without a close frame — a
suspended machine, a dropped VPN, an expired NAT binding — stays registered and
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
recorded link is that follow-up and is appended — a merged link never vetoes it.
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
Nested skills reuse their owner's run; intermediate transitions do not close it.
These activities never acquire the managed-stage transition guard.

Cards and list rows display a single run icon while a run is active — running takes
precedence over queued, and a cancellation stays visible briefly — updated
through server events and polling. Reading a task alone never marks it running.
A run started over MCP is owned by the session that started it, so an abrupt
client termination closes it as canceled instead of leaving the task active; the
note records that the client disconnected. Agent-dispatched runs keep their own
reporting path. This indicator reports declared execution state, not process
liveness.

## MCP session ownership

`/mcp` is served statefully: each client holds one server session, identified by
`Mcp-Session-Id` and told apart from any other session sharing the same bearer
credential. A session begins when its client completes initialization and ends on
client termination, a dropped connection, or silence beyond the idle timeout.

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

`SECTILE_MCP_SESSION_TIMEOUT` bounds a session whose client never announces its
departure, defaulting to fifteen minutes of silence; an unusable value keeps the
default rather than removing the bound. The stdio bridge pings inside that window,
so an idle but live conversation stays connected. `SECTILE_MCP_CLIENT` names the
bridge in the session list, defaulting to host and process id.

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

The server project supplies the `useWorktrees` default, which **Inherit worktrees
from server** restores in the desktop project settings. Parallel executions
(1 to 5) are workstation-owned: the server neither stores nor supplies a value,
the desktop app is the only surface that sets one, and a project without a local
value runs a single execution at a time.
Workstation settings are saved in `~/.config/sectile/settings.json` as project-ID maps:

```json
{
  "projects": {"project-id": "/path/to/repository"},
  "worktrees": {"project-id": true},
  "parallelism": {"project-id": 2}
}
```

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
Legacy repository mappings remain readable and are migrated on the next save.

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
- `GET /api/v1/agent/identity` tells a key holder `{"userId", "deviceId",
  "label", "expiresAt", "sharedToken"}`; the agent calls it before connecting and
  logs a warning when fewer than ten days remain.

A key past its expiry is refused on every surface with `401 {"error":"API key
expired"}`, distinct from the generic refusal, so the owner renews it rather
than retyping it. Revocation cuts every surface at once.

The agent gateway takes the same key on `/mcp` and `/api/`, and consoles it
launches receive it as `SECTILE_AGENT_TOKEN` with `SECTILE_AGENT_URL` set to the
server. Deprecated for one release: `SECTILE_SERVER_TOKEN`, accepted with a
startup warning, and the open mode of a server without it, which accepts any
nonempty token only until the first key is issued. Credentials issued before
keys expired keep working as keys without expiry.

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
(default). Adjustment requires an existing matching PR — open, or already merged by the
human, in which case it reviews the merged state without pushing — performs full review
and feedback disposition, checks the final code, updates the same PR and verifies
readiness. Lookup failure is not absence. Creation-owner recovery retains an already
implemented stage. Completion records the PR URL at the owning stage.

Skill records may include `requiresReconciliation`. Native dispatch refuses these
customizations until their legacy content has been reviewed and saved under Adjust
or reset in the skill editor. Legacy entries and divergent installed files remain
available; custom adjustment content also receives the current built-in contract.

Managed adjustment verifies the PR identity against the task's recorded set —
the same PR, or a newer one on a branch the task already used — together with the
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
`finish_run` and `create_task`. The former `sectile_` names are unsupported on both
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

### Free desktop agent consoles

`GET /desktop/status` advertises `free-console`. Authenticated
`POST /desktop/consoles` accepts `{"projectId":"...","provider":"codex"}` or
`provider: "claude"` and returns HTTP 202 with the admitted local run. Browser
Origin headers are rejected. Invalid input returns 400, unavailable server
configuration returns 502, and local mapping or shutdown conflicts return 409.

The run has `kind: "console"`, a `provider`, a unique local ID, and empty task,
skill, and prompt fields. Its CLI receives no arguments. Project mappings and
execution limits apply; the console reserves the mapped shared checkout through
the existing queue. It does not scaffold tooling, create a worktree, or mutate
remote tasks. The daemon owns the launch after admission even if the request ends.

These runs use the usual runs, terminal, stop, restart protection, and history
endpoints. Completion updates only local process status. Task result lookup
returns 404, and clients must omit task workflow and PR controls for these runs.
