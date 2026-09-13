# Server/agent contract, version 1

## Boundaries

The server owns task data, tracker synchronization, project configuration and
managed skill content. The agent owns local repository mappings, worktrees,
provider MCP installation and native process launch. No server filesystem path is
interpreted as a workstation path. The agent and its MCP bridge never open a task
database. Shared DTOs live in `internal/agentconfig` and do not depend on DB code.

## Configuration download

`GET /api/v1/agent/config?projectId=<ID>` or `?taskKey=<task-ID>` requires the
agent bearer token. Use exact project IDs, not names. Prefer full task IDs over
ambiguous tracker keys. A task lookup resolves the actual owning project.

| Field | Meaning |
| --- | --- |
| `schemaVersion` | Must be `1`. Unsupported versions stop preparation. |
| `projectId`, `projectName`, `description` | Identity and project context. The ID must match an explicit project request. |
| `gitRemoteUrl` | Repository identity for automatic local matching, not a path to clone automatically. |
| `githubRepo`, `issueTracker` | Optional effective project-over-global repository and tracker metadata for local command placeholders. Missing fields use local directory basename and task source (then `github`) fallbacks. No credentials or server paths. |
| `specFramework` | Specification framework used by the project skills. |
| `useWorktrees` | Create/reuse task worktrees when true; validate the existing checkout when false. |
| `aiProvider` | `codex`, `claude`, `agy`, `gemini`, `cursor`, `vibe`, or `custom`; empty uses the legacy `agy` default. |
| `aiCommandTemplate` | Optional shell template containing `{prompt}`. Required for `custom`; argument placeholders are shell-safe: `{prompt}`, `{issueKey}`, `{issueTitle}`, `{issueDesc}`, `{branchName}`, `{repoPath}`, `{tracker}`, `{repo}`. Task values are fetched for each launch; branch/path identify local execution. See [desktop usage](../../desktop/README.md). Custom providers still require supported native MCP bootstrap. |
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
`aiCommandTemplate`, `terminal` and skill content overrides (`skills`).
These values are never uploaded. Changing the provider locally without a local
command template clears the inherited provider's command template.

Terminal application selection is: explicit `--terminal`, explicit request
`terminalOverride`, local overrides, project configuration, global configuration,
then environment/platform detection. Selecting `pty` or `none` uses a local PTY;
the explicit `open_terminal` action requires an external window. Legacy
`.taskflow/config.json` is not a second terminal-settings source.

## Skill ownership and recovery

The incoming skill list declares managed paths under `.agents/skills`,
`.agy/skills`, `.claude/skills`, `.gemini/skills`, `.skills` and `.claude/commands`.
All IDs and destinations are validated before any installation. Duplicate
paths, path traversal and symlink escapes are rejected.

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
native client uses TaskFlow MCP to read tasks/comments and submit verified stage
reports. True server-managed workers retain their separate result-file contract.

## Future binary separation

This contract works without a shared executable. A later `taskflow-server` can
own the web/DB/trackers while `taskflow` owns local launch and stdio MCP proxying.
The split, caching and automatic refresh of already-open client sessions are
outside this change.

## Project discovery and task identity

Authenticated `GET /api/v1/agent/projects` returns
`{"schemaVersion":1,"projects":[{"id":"server-primary-key","name":"Project","gitRemoteUrl":"..."}]}`.
Discovery excludes server filesystem paths and credentials.

`taskflow agent` defaults to `--project all`. Use `--list-projects` to
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
invocation uses the full task reference, also exposed as `TASKFLOW_TASK_ID`;
`TASKFLOW_PROJECT_ID` identifies its project and `TASKFLOW_TASK_KEY` remains
available for display. MCP's historical `taskKey` argument accepts the full task
primary key, which should be preferred for transitions across projects.

Terminal settings continue to come from the server. `--terminal` is optional
and serves only as an explicit local override.

For compatibility, legacy registrations with project `default` or an empty
project remain wildcard registrations. New clients should use `all` explicitly.
Other scoped registrations never receive another project's dispatch.

Tracker configuration, stage mappings and transition validation remain server-side.
The agent contract does not carry tracker fields, stages, stage mappings or ttyMode.

## MCP project discovery and review links

Native clients can call `taskflow_list_projects({})` to discover the same
secret-free project records as the HTTP discovery endpoint, then pass an ID to
`taskflow_get_project_context` or `taskflow_list_tasks`.

`taskflow_transition_stage` accepts `prUrl` for either a pull request or a merge
request. The server persists this link on the task alongside the stage update and
includes it in the tracker synchronization job. Omitting the argument preserves
an existing link. For example:

```json
{"taskKey":"full-task-primary-key","stage":"reviewed","note":"Review complete","branch":"feat/example","prUrl":"https://gitlab.com/example/repo/-/merge_requests/42"}
```

## Remote execution visibility

A delegated skill creates a `remote_run` activity before dispatch. Its ID travels
as `runId` and `TASKFLOW_RUN_ID`. Launch failure closes the run as failed;
successful process launch leaves it running.

Standalone skills call `taskflow_start_run(taskKey, skill, runId?)`, retaining
the returned activity ID. A supplied launcher run ID reuses the existing run.
The invocation owner calls `taskflow_finish_run(taskKey, runId, status, note)`
with completed, failed or canceled when it ends, including a stop for user input.
Nested skills reuse their owner's run; intermediate transitions do not close it.
These activities never acquire the managed-stage transition guard.

Cards and list rows display **Remote execution** while a run is active, updated
through server events and polling. Reading a task alone never marks it running.
Abrupt process termination cannot report completion: the activity remains visible
until explicitly canceled in the activity UI or finished through MCP. This
indicator reports declared execution state, not process liveness.

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
Desktop launch settings are saved locally; the server token is encrypted using
Electron safeStorage when OS encryption is available, otherwise it must be
entered again. Existing agents launched outside the desktop do not expose their
server credentials to this panel.

**Clear finished consoles** removes completed, failed and canceled consoles from
the local agent through authenticated `DELETE /desktop/history`. Only sessions
with confirmed process exit are removed. Active executions, server task comments,
and the AI provider's own saved conversations are preserved.

### Optional desktop companion

The server, local agent and desktop app are independent components. Start the
agent without the app:

```sh
export TASKFLOW_AGENT_TOKEN='your-server-token'
taskflow agent --url http://localhost:8090 --repo /path/to/repository
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

The server project supplies `useWorktrees` and `parallelism` (1–3) defaults.
In the desktop project settings, **Inherit worktrees from server** and
**Inherit from server** for parallel executions remove local overrides.
Workstation overrides are saved in `~/.config/taskflow/settings.json` as project-ID maps:

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
`~/.config/taskflow/settings.json`, shared by the CLI agent and companion.
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

Server and agent currently share `bin/taskflow`. Launch targets use existing
builds and do not rebuild. Pass agent arguments with, for example,
`make start ARGS="--url http://localhost:8090"`; provide authentication through
`TASKFLOW_AGENT_TOKEN`.


## Adjustment and PR ownership

The canonical review action is `adjust` (`adjust-issue`). Legacy `create_pr`,
`review`, and `create-pr` normalize to it without rewriting activity history.
States remain `new`, `clarified`, `specified`, `implemented`, `reviewed`, `finished`.
A reviewed task offers Handoff; repeat Adjust is explicit and requires an open PR.

The `prCreationStage` policy assigns draft creation to specification or implementation
(default). Adjustment requires an existing matching open PR, performs full review
and feedback disposition, checks the final code, updates the same PR and verifies
readiness. Lookup failure is not absence. Creation-owner recovery retains an already
implemented stage. Completion records the PR URL at the owning stage.

Skill records may include `requiresReconciliation`. Native dispatch refuses these
customizations until their legacy content has been reviewed and saved under Adjust
or reset in the skill editor. Legacy entries and divergent installed files remain
available; custom adjustment content also receives the current built-in contract.

Managed adjustment pins the original PR identity before running and verifies the
same ready PR, branch, pushed commit, clean checkout and reported build/lint/test
checks at completion. Standalone transitions verify forge identity and readiness;
check output remains agent-reported. Human merge and handoff remain separate.
