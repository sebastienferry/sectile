# Sectile Desktop

Local task execution consoles without a separate chatbot UI.

## Start

From the repository root:

```sh
make desktop-build
cd desktop
npm start
```

The first window asks only for the server URL and authentication token. Account
sign-in is not implemented yet. Repository directories are configured per project
after connecting. New installations keep mappings in private application data. The bundled binary is
selected automatically. Local servers without TASKFLOW_SERVER_TOKEN accept any
non-empty agent token.

Connect to an existing local agent instead of starting another agent for the same
project scope. Subsequent launches reconnect to the application's existing agent.

## Use

Launch a skill from the web. Select its local execution to see the Codex/Claude
console, type answers, resize it, stop it, or export its scrollback. Project
directories discovers server projects and saves local Git repository mappings.

Closing the window or quitting Electron keeps the detached agent and tasks alive.
Reopening restores the connection. Agent diagnostics are in agent.log under
Electron's user data directory. Its private connection file contains a credential.

History and the execution index last for the daemon's lifetime. Restarting the
agent does not recover processes or historical sessions. Replay is bounded;
exported logs contain plain text scrollback.

Executions that fail or are canceled before a console is created show an
explanation instead of opening a terminal connection. For launch failures, check
the task activity and `agent.log` in the application's data directory.
When a task's assigned branch is already open in the main repository checkout,
the agent reuses that checkout and preserves its local changes.

## Package and verify

```sh
make desktop-package
cd desktop
npm run test:ui
```

The unsigned package is in desktop/release. Tests use an isolated temporary
profile and mock agent. Go tests exercise real PTY replay and supervision.

### Restarting the agent

Use **Restart agent** in the desktop header. The confirmation explains that active
executions will be stopped and in-memory console history cleared. Restart waits
for confirmed process exits; if a new execution arrives or an exit cannot be
confirmed, restart is refused. The daemon relaunches its current executable with
the same arguments and environment, preserving server credentials and local
project mappings. The desktop reconnects automatically.

An agent started with an older binary must be stopped and relaunched once to
enable this endpoint. Restart requires a responsive agent; it is not a force-kill
recovery mechanism.

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

On macOS, replace executables atomically (build or copy to a new file, then
rename). Overwriting an existing executable in place can trigger a
`Taskgated Invalid Signature` termination even when codesign verifies the file
on disk. The Makefile uses atomic replacement for local binaries.

### Build all components

Run `make all` to build the embedded web server, standalone local agent and
packaged desktop app. Server and agent currently share `bin/taskflow`; start
the agent with its `agent` subcommand. Use `make server-build` or
`make agent-build` for the shared executable only, and `make desktop-build`
for the desktop development assets. On Apple Silicon the app is produced at
`desktop/release/TaskFlow-darwin-arm64/TaskFlow.app`.

The optional companion groups local executions under projects in a collapsible
sidebar. Add projects by discovering the server catalog and mapping a local Git
directory. Local worktree preferences are stored per project in
`~/.config/taskflow/settings.json`. Repository layout, remote URL, SDD selection and skill
content remain server-owned and read-only. Explicit deployment buttons install
the server skills or initialize its SDD framework in the mapped directory.
The profile is a placeholder for future account management.

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

Project configuration has three tabs: **Local**, **Deployment** and **Server**.
Use **Choose folder…** to select a repository through the native directory dialog.
Worktrees use Yes/No buttons; parallelism uses 1/2/3 buttons. Reset icons restore
inheritance from server defaults. Changes take effect after **Save local
configuration**. Server metadata and skill content remain read-only.

Use **Launch task** beneath a project to search its server tasks, select a
server-provided skill and **Launch**. Submission uses the server's
existing run-skill dispatch and local queue; it does not create a duplicate task.
The project must have a valid local repository mapping.

The launcher shows results only after a search. Each result shows its status and a skill selector; Custom instructions sends a free-text request to the configured AI client. Executions appear in the local task list.

The Local project tab includes the effective **CLI command**. Edit it to save a
per-project override under `commands` in user settings; the reset icon restores
the server template (or provider default when empty). Save to apply to subsequent
executions. Command templates execute on the local agent and support these placeholders:

| Placeholder | Value |
| --- | --- |
| `{prompt}` | Assembled task instructions and run metadata (required) |
| `{issueKey}` | Task display key, for example `#63` |
| `{issueTitle}` | Current task title |
| `{issueDesc}` | Current task description, possibly empty |
| `{branchName}` | Resolved local execution branch |
| `{repoPath}` | Absolute local execution directory, including the task worktree when enabled |
| `{tracker}` | Lowercase task source, then effective tracker, then `github` |
| `{repo}` | Effective configured GitHub repository, otherwise local directory basename |

For example: `codex --cd "{repoPath}" "Task {issueKey}: {prompt}"`.
Use placeholders as ordinary CLI arguments, either unquoted, single-quoted or
double-quoted, including inside a larger argument. Inserted values remain literal
shell data and are not expanded again. Unknown tokens remain literal. Templates
are trusted shell commands; do not place placeholders inside shell programs,
command substitutions or here-documents. Explicit raw terminal commands and
interactive launches without a skill retain their existing behavior.

Values are refreshed for every launch, including relaunches and custom instructions.
Saving or resetting settings stores the template, never the expanded task values.

**Refresh from server** reloads project metadata, skills and inherited execution
settings in the open dialog. Local overrides and unsaved local edits remain
intact. Reset buttons then use the refreshed server values. Refresh does not
deploy tooling or modify running executions.

Select a completed, failed or canceled execution and choose **Relaunch**.
The dialog restores its skill and original instructions, allows editing both,
and submits a new execution using current project settings. The previous run
and console remain in history. Instructions are retained in agent memory;
older executions without saved instructions open with an empty field.

The sidebar groups executions by project and task. Projects can be collapsed;
their **+** button opens the task launcher. A task's **…** menu provides relaunch,
local rename and archive actions. Archiving hides its existing executions without
changing the server task. Active executions require explicit confirmation and
confirmed stop before archiving. A new execution makes the task visible again.
The TTY toolbar's execution selector provides access to previous runs of the
selected task. Local names and archive visibility persist in companion storage.

### Desktop Quick add

Press **Cmd+K** (macOS) or **Ctrl+K** to open the command palette and choose
**Quick add task**. The selected project's identity is prefilled; without a
selection, choose a project explicitly. Enter a title and optional description.
The server creates the task using its project tracker configuration.
GitHub and Linear creation must succeed remotely; errors do not silently create
a local fallback. Local projects remain local. Jira remote creation is not
implemented and returns an explicit error. Creation does not start an execution;
the success screen offers a separate **Launch task** action.

The task launcher excludes finished tasks, including the finished workflow
label. Each result shows its current workflow stage and tracker status when
available. The agent checks again before submitting a launch, so a task finished
after the search must be reopened on the server first.

### Next workflow step

The status line beneath the task console shows its current server workflow stage.
Use **Next: Clarify**, **Next: Specify**, **Next: Implement**, or
**Next: Review and create PR** to launch one step with the project's current
configuration. Historical consoles use the task's current state too. The action
is disabled while that task has an active execution or a launch is pending.
The desktop rechecks state before submission; if the next step changed, review
the updated button and click again. Metadata failures offer **Retry**.
Reviewed tasks show **Awaiting human merge**; finished tasks have no next action.
