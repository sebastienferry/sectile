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
selected automatically. Local servers without SECTILE_SERVER_TOKEN accept any
non-empty agent token.

Connect to an existing local agent instead of starting another agent for the same
project scope. Subsequent launches reconnect to the application's existing agent.

## Use

### Free agent console

Click **>_ Open agent console** in a configured project's heading, choose **Codex**
or **Claude**, and click **Open console**. The app launches `codex` or `claude`
with no arguments in that project's mapped local repository. Type your first
instructions directly in the TTY. The selected CLI must be installed locally;
its own sign-in and permission prompts remain available in the console.

Each launch is a separate local console. It uses no task, skill, initial prompt,
or workflow command template. It does not create a tracker activity or change a
workflow stage. The sidebar shows process status, and the toolbar supports stop,
export, and relaunch. Rename and archive are available through the console menu.
Closing and reopening the app reconnects while the daemon remains running.

Free consoles use the existing execution queue and reserve the project's shared
checkout while active. Other executions that need that checkout wait until the
console exits. No branch or worktree is created for a free console.


Launch a skill from the web. Select its local execution to see the Codex/Claude
console, type answers, resize it, stop it, or export its scrollback. The green
completion mark in the terminal toolbar ends the selected execution, which is how
the current workflow step is closed and the next one unlocked; its tooltip and
accessible label stay **Stop execution**, the literal effect, because free
consoles use the same control and have no next step. The selected
task uses a highlighted background without a selection border; keyboard focus
remains visible. The project directory browser discovers server projects and
saves local Git repository mappings.

Closing the window or quitting Electron keeps the detached agent and tasks alive.
Reopening restores the connection. Open **Agent logs** in the top toolbar to read
agent diagnostics, including while disconnected or after a failed start. Logs fill
the content area beside the usable project sidebar, retaining its width and collapse
preference. **Close logs** or Escape returns to the execution or offline setup;
selecting an execution in the sidebar returns to its execution view. Terminal colors, cursor
commands and title sequences are removed from the display while readable Unicode
and line breaks are preserved. The stored log is unchanged. The viewer
shows the desktop-owned `agent.log` path and the latest 256 KiB, with a notice when
earlier output is omitted. Use **Refresh** for a new snapshot; missing, empty, and
unreadable files have explicit messages. The view stays local and does not alter
the log or selected execution. Agents started outside the desktop may write to
their original terminal instead. The private connection file contains a credential.

History and the execution index last for the daemon's lifetime. Restarting the
agent does not recover processes or historical sessions. Replay is bounded;
exported logs contain plain text scrollback.

Executions that fail or are canceled before a console is created show an
explanation instead of opening a terminal connection. For launch failures, check
the task activity and **Agent logs** in the top toolbar.
When a task's assigned branch is already open in the main repository checkout,
the agent reuses that checkout and preserves its local changes.

### Inspect worktree changes

Select a local execution and open **Changes**. Choose a file to read its unified
patch, or use **Refresh** after edits. **Console** restores terminal focus without
restarting, stopping, or detaching the execution. Inspection also works for stopped
runs while their recorded checkout and agent session remain available.

The comparison includes committed, staged, unstaged, and non-ignored untracked
contents as one net result. Reverted edits disappear, and a recreated staged
deletion is compared once with its original contents. The header identifies the
actual directory, branch, local default reference, common ancestor, and read time.
Inspection reads current files even when selecting an older execution.

The baseline uses the local symbolic `origin/HEAD`, then remote `origin/main` or
`origin/master`, then local `main` or `master`. A broken recorded default or missing
or ambiguous common ancestor produces an explanation. The viewer does not fetch;
update local history outside the viewer if necessary. Branch changes, missing
checkouts, unmerged indexes, and detected concurrent edits require recovery or retry.

Binary and submodule changes have explicit markers and unavailable text counts.
Symlinks show their link text, executable-bit changes remain visible, and paths and
patches render as plain text. Git detects renames at 50% similarity, with a bounded
rename search that may represent a rename as deletion plus addition.

Inspection has a 10-second deadline, 1,000 displayed entries, 256 KiB per patch,
and a 4 MiB response budget. File reads and Git metadata are bounded to 8 MiB each;
the temporary content snapshot has a 64 MiB budget. Omitted patches and incomplete
results are identified, and totals are labeled partial. Large metadata may prevent
inspection entirely. Files using non-UTF-8 names produce an explicit unsupported
path error; non-UTF-8 contents receive a non-text marker.

The local agent retains the credential. Source contents remain on the workstation,
with temporary Git storage removed after the request. Refresh does not stage,
commit, alter repository objects, or execute external diff/textconv/fsmonitor
helpers. There is no automatic refresh or persistent source snapshot. Upgrade and
restart older agents to enable inspection; runs without recorded branch metadata
need a new execution. A failed refresh clears the previous result.

### Skill result indicator

The terminal header and each visible task row show the skill's result independently of its console
process. A checkmark means the server reports that exact execution completed;
for workflow stages, the task must also have reached the corresponding stage.
An open console can therefore show **Skill completed**. Process exit alone shows
**Execution ended · skill completion unconfirmed**, and pending stage validation,
failures, cancellations and in-progress executions have distinct labels.
Task-row icons use the same completion rules as the header, with the skill name
and result in their tooltip and accessible label. Visible rows refresh even when
they are not selected, with at most four concurrent result lookups. A task row
represents its current execution; selecting older history does not replace that
row's result. Updates preserve selection and keyboard focus without reattaching
or resetting the terminal. It requires
an updated local agent for server-result lookup; unavailable results never produce
a success checkmark. Whether a session is waiting for you comes from the Claude
Code hooks Sectile installs, never from inactivity: a long build is not a
request for input.

### Execution queue

Hover over a project's heading to reveal its small funnel-shaped queue icon alongside the other
project actions (also available on keyboard focus and touch). Use the queue icon to switch that project's task
list to its execution queue directly below the project heading. Close the queue
with its small cross button, or click the queue icon again, to restore the task list. Its badge
shows the number of waiting executions; the highlighted icon indicates queue
mode. Each project switches independently, and opening queue mode expands a
collapsed project.

The gray `(running/maximum)` counter beside each project name uses the agent's
effective concurrency limit, including local overrides and shared-checkout
serialization. Its numerator includes preparing and stopping executions that
still hold a slot, and excludes queued and finished runs. If the agent cannot
provide the limit, `?` is shown instead. Saving project settings refreshes it.

The queue shows only that project's active, waiting and stopping executions,
including multiple executions of one task. Select an entry to open its console.
Waiting executions appear first in submission order, using the daemon sequence
or creation time for older agents. Project concurrency and shared checkouts
still determine when work can start. Counts update with the desktop refresh.
Canceled runs are excluded from waiting counts. An updated agent reports pending
cancellation separately as **Stopping / canceling**; running processes retain
their scheduler slot until exit is confirmed.

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
export TOKEN='your-server-token'
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

On macOS, replace executables atomically (build or copy to a new file, then
rename). Overwriting an existing executable in place can trigger a
`Taskgated Invalid Signature` termination even when codesign verifies the file
on disk. The Makefile uses atomic replacement for local binaries.

### Build all components

Run `make all` to build the embedded web server, standalone local agent and
packaged desktop app. The outputs are `bin/server` and
`bin/agent`; the agent starts directly. Use `make server` or
`make agent` to build independently, and `make desktop-build`
for the desktop development assets. On Apple Silicon the app is produced at
`desktop/release/Sectile-darwin-arm64/Sectile.app`.

The optional companion groups local executions under projects in a collapsible
sidebar. Add projects by discovering the server catalog and mapping a local Git
directory. Local worktree preferences are stored per project in
`~/.config/sectile/settings.json`. Repository layout, remote URL, SDD selection and skill
content remain server-owned and read-only. Explicit deployment buttons install
the server skills or initialize its SDD framework in the mapped directory.
The profile is a placeholder for future account management.

### Remove a local project

In project settings, choose **Local → Remove from desktop**, then confirm
**Disconnect project**. Removal clears that project's workstation mapping and
execution overrides. It preserves repository files, worktrees, deployed tooling,
server projects, tracker tasks, and other local settings. Stop the project's
executions first: queued, preparing, running, and not-yet-exited processes block
removal, and removal never cancels them automatically.

Disconnected projects and their consoles stay hidden after desktop or agent
restart. Choose **Add project** and save a valid repository to reconnect explicitly.
Finished history remains available after re-add for the agent's existing lifetime;
archived tasks remain archived. Disconnection is stored in workstation settings
under `disconnectedProjects`, a project-ID-to-boolean map. Repository detection
and legacy mappings cannot override a true marker. Older agents must be updated
and restarted before this action is available.

### Execution defaults and local overrides

The server project supplies the `useWorktrees` default, which **Inherit worktrees
from server** restores in the desktop project settings. Parallel executions
(1 to 5) are workstation-owned: the server neither stores nor supplies a value,
this app is the only surface that sets one, and a project without a local value
runs a single execution at a time.
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
| `make desktop` | Build the desktop app, without packaging |
| `make desktop-package` | Build and package the desktop app |
| `make all` | Build all components |
| `make start` | Start the local agent |
| `make serve` | Start the server |
| `make run` | Start the desktop |

Server and agent are built as `bin/server` and `bin/agent` by the `build-*` targets.
The `serve`, `start` and `run` targets run from source and need no prior build. Pass agent
arguments with, for example, `make start ARGS="--url http://localhost:8090"`; provide
authentication through `TOKEN`.

Project configuration has three tabs: **Local**, **Deployment** and **Server**.
Use **Choose folder…** to select a repository through the native directory dialog.
Worktrees use Yes/No buttons; parallelism uses 1 to 5 buttons. Reset icons restore
inheritance from server defaults, and parallelism has none because it never
inherits. Changes take effect after **Save local
configuration**. Server metadata and skill content remain read-only.

Hover or keyboard-focus a project row and activate **Open tasks** to list its
open server tasks immediately, even when the project is collapsed. Search by title
or task key to narrow the list; submit an empty search to restore all open tasks.
Select a server-provided skill and **Launch**; pickup is selected by default when
available. Submission uses the server's
existing run-skill dispatch and local queue; it does not create a duplicate task.
The project must have a valid local repository mapping.

Each result shows its status and a skill selector; Custom instructions sends a free-text request to the configured AI client. Loading, empty and error states are shown in the list; use Search to retry a failed request. Executions appear in the local task list. Opening the list does not start an execution.

The Local project tab includes the effective **CLI command**. Edit it to save a
per-project override under `commands` in user settings; the reset icon restores
the server template (or provider default when empty or lacking `{prompt}`). Save to apply to subsequent
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

The sidebar groups executions by project and task. Projects are alphabetical;
tasks show active (running/preparing), queued, then finished executions, newest
first within each group. Actual execution start determines recency, with submission
time used for queued/preparing runs and older records without a start timestamp.
A task with several runs uses its highest-priority state and newest run in that
state. Equal times use task/run identities for stable ordering. Refreshes preserve
the selected execution and the history selector stays in submission order.
Linked pull requests appear as an icon on the same task row, after the title and
status. Hover for the URL or activate the icon to open the PR externally without
changing the selected console. Long titles truncate to keep controls inline.
Projects can be collapsed;
their **+** button opens the task launcher. A task's **…** menu provides relaunch,
local rename and archive actions. Archiving hides its existing executions without
changing the server task. Active executions require explicit confirmation and
confirmed stop before archiving. A new execution makes the task visible again.
The TTY toolbar's execution selector provides access to previous runs of the
selected task. Its header shows the task key (or full ID), current task title, and
selected execution skill. Local names take precedence over tracker titles and
persist alongside archive visibility in companion storage. Titles refresh without
reconnecting the console; unavailable titles fall back to identity and skill.
Long headers truncate on one line, with their full text available on hover and
to assistive technology. Toolbar controls wrap at narrow window widths.

### Desktop Quick add

Press **Cmd+K** (macOS) or **Ctrl+K** to open the command palette and choose
**Quick add task**. The selected project's identity is prefilled; without a
selection, choose a project explicitly. Enter a title and optional description.
The server creates the task using its project tracker configuration.
GitHub and Jira creation must succeed remotely; errors do not silently create
a local fallback. Local projects remain local. Creation does not start an execution;
the success screen offers a separate **Launch task** action.

The task launcher excludes finished tasks, including the finished workflow
label. Each result shows its current workflow stage and tracker status when
available. The agent checks again before submitting a launch, so a task finished
after the search must be reopened on the server first.

### Next workflow step

The status line beneath the task console shows its current server workflow stage.
The action itself sits in the console toolbar, right after the green control that
ends the current execution and before **Retry**: use
**Next: Clarify**, **Next: Specify**, **Next: Implement**, or
**Next: Review and create PR** to launch one step with the project's current
configuration. Historical consoles use the task's current state too. The action
is disabled while that task has an active execution or a launch is pending.
The desktop rechecks state before submission; if the next step changed, review
the updated button and click again. Metadata failures offer **Retry**.
Reviewed tasks show **Awaiting human merge**; finished tasks have no next action.
