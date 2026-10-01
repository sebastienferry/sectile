# Plan #676 - Add a folder from a discussion

Behaviour: `spec.md`. This file says how; the specification says what.

## Starting point

- `feat/676` was created from `feat/desktop-conversation-test`, whose content
  reached `main` squashed in #650 (`875c2e1a`). The conversation, console,
  runner and desktop files are identical on `HEAD` and `origin/main`. The
  implementation stage merges `origin/main` first (a rebase would rewrite
  commits that may already be published; see the "Force push refused after
  rebase" note).
- No server change, no migration, no new external dependency. Every change
  is in the local agent (`internal/agent`, `internal/runner`) and Sectile
  Desktop (`desktop/`).
- No new child process: the conversation already starts `claude` through
  `agentexec.Hidden` / `agentexec.StartDetached`; typing into a PTY uses the
  existing terminal manager.

## Architecture

```
Desktop (renderer)                Electron main          Local agent
-----------------------------     ---------------        ------------------------------
composer "Add folder…"   ─┐
toolbar  "Add folder…"   ─┼─ chooseRepository() ─ dialog
                          └─ addRunFolder(runId, path) ─ POST /desktop/run-folder
                                                          ├─ attachFolder (existing checks)
                                                          ├─ conversation: nothing more,
                                                          │  the next turn re-reads folders
                                                          └─ PTY discussion, Claude Code:
                                                             wait quiet, InjectLine("/add-dir …")
conversation turn ───────────────────────────────────────  projectFolders() → --add-dir=…,
                                                           SECTILE_REPOSITORIES
discussion / free console launch ────────────────────────  live() + addDirArgs(provider, dirs)
```

### 1. Project folders without a ticket

`buildFolderMap` (`internal/agent/repositories.go:93`) already works with an
empty `models.Task` and the project's own identity as primary: every other
repository becomes a context entry, then the specifications folder and the
attached folders follow. Add one helper beside `taskFolderMap`:

```go
// projectFolderMap is the folder map of a project-level session: a
// conversation or a free console, which has no ticket and runs in directory.
func (d *agentDaemon) projectFolderMap(ctx context.Context, config agentconfig.Config, directory string) []models.FolderMapEntry
```

It calls `localProjectRoot`, then `buildFolderMap(ctx, config, overrides,
root, codeIdentity(config), directory, models.Task{})`. `folderMapDirs` turns
it into the `--add-dir` list; it already skips missing folders and the
primary entry. `prepareMu` is taken by the caller where `localProjectRoot`
requires it, as in `desktopConsole`.

### 2. Conversation turns (FR4)

- `claudeConversationCommand` (`agent_conversation.go:158`) gains `dirs
  []string` and `env map[string]string`. Each dir is appended as one
  `--add-dir=<path>` argument. No shell is involved (`exec.Command`), so no
  quoting; the `=` form is kept so a value never swallows the following
  arguments (see `addDirArgs`). `cmd.Env = commandEnv(env)`.
- `conversationTurn` resolves the folders before building the command,
  outside `queue.mu`: `d.fetchConfig(ctx, run.desktop.ProjectID, "")`, then
  `d.projectFolderMap(ctx, config, run.desktop.Directory)`. On error the turn
  runs with no dirs and writes
  `conversationWrite(run.trace, "notice", "Attached folders could not be read for this message", err.Error())`.
  The map, when not empty, goes to `SECTILE_REPOSITORIES` (same JSON as
  `agent.go:1234`) with `SECTILE_PROJECT_ID`.
- Directory and project come from `run.desktop`, so a conversation opened
  from "Claude chat (test)" on a task worktree gets the same project folders.

### 3. Launch of discussions and free consoles (FR7, FR8)

- `dispatchCommand`'s `live()` (`agent_config.go:708`) appends
  `addDirArgs(provider, launch.AddDirs)` to the line `InteractiveAgentLaunch`
  returns, where `launch` is the first `agentCommandContext` and `provider`
  is `config.AIProvider` lowercased. A `custom` provider keeps its bare
  binary: `addDirArgs` returns nothing for it. This covers the in-Sectile
  discussion (`skillID == "discuss"`) and the skillless `open_terminal` used
  by the native terminal, which both already receive `AddDirs` from
  `agent.go:1188`. The macro dispatch (`agent_macro_dispatch.go:108`) passes
  its own contexts and is unaffected unless it reaches `live()` with dirs,
  which is the desired behaviour too.
- `desktopConsole` (`agent_console.go`): for the PTY branch, compute
  `projectFolderMap(ctx, config, root)` while `prepareMu` is held, append
  `addDirArgs(provider, folderMapDirs(map))` to the command built by
  `consoleCommand` (built-in engines only; a configured template keeps its
  own `{addDirs}` slot, filled from the same list through
  `agentCommandContext{AddDirs: …}`), and put `SECTILE_REPOSITORIES` in the
  `env` map of `launchConsole`. The conversation branch needs nothing at
  admission: folders are read per turn.

### 4. Attach from a run: `POST /desktop/run-folder` (FR2, FR3, FR5, FR6)

New route in `desktopHandler` (`agent_desktop.go`), in a new file
`internal/agent/agent_run_folders.go`:

```
POST /desktop/run-folder  {runId, path}
200 {"mappedAs": "<identity>"?, "typed": true|false, "appliesAt": "next-turn"|"now"|"next-launch"}
4xx text/plain reason (from attachFolder, or "This execution has ended")
```

- Find the run by `runId` under `queue.mu`; refuse 404 when unknown, 409
  when it is finished or canceled, or when it is neither a conversation nor
  a running ticket discussion (`run.desktop.Skill == "discuss"` with a
  terminal session).
- `fetchConfig(projectId)` then the existing `attachFolder(r, config,
  path)`; its status and message are returned as is on refusal, so the
  Desktop shows the same reasons as the settings page.
- Conversation: answer `{typed:false, appliesAt:"next-turn"}`.
- Ticket discussion: the engine is the one recorded at launch. Add an
  unexported `interactiveProvider string` to `controlledRun`
  (`agent_run.go:85`), set in `agent.go` next to the `dispatchCommand` call
  when the skill is `discuss`: `config.AIProvider` lowercased, or `""` when a
  custom template is configured. When it is `claude` and the path passes
  `typeablePath`, wait for the session to settle (a new exported
  `Manager.WaitQuiet(ctx, sessionID, quiet, cap)` wrapping the existing
  `waitQuiet`, with 1 s quiet and a 10 s cap; on the cap it types anyway,
  since Claude Code queues input), then
  `d.terminal.manager.InjectLine(sessionID, "/add-dir "+claudePromptPath(path))`.
  Answer `{typed:true, appliesAt:"now"}`; otherwise `{typed:false,
  appliesAt:"next-launch"}`. A failed injection answers `typed:false,
  appliesAt:"next-launch"` with the folder still attached.
- When `attachFolder` maps the folder as a repository (`mappedAs` set), the
  typed path is the same picked folder (FR6).
- `typeablePath` refuses any rune below 0x20 or 0x7f. `claudePromptPath`
  returns the path as is when it contains no whitespace and no `"`, else
  wraps it in double quotes with inner `"` and `\` escaped by a backslash.
  This is the reversible choice the clarification left to the
  specification; see "Risks".
- Capability: add `runFoldersCapability = "run-folders"` to the list in
  `agent_desktop.go:167`.

### 5. Desktop

- `desktop/electron/main.cjs`: IPC `add-run-folder` → `api('/desktop/run-folder','POST',{runId,path})`,
  refusing with "Update and restart the local agent to add folders from a
  discussion." when the status lacks `run-folders`. `preload.cjs`:
  `addRunFolder:(runId,path)=>ipcRenderer.invoke('add-run-folder',{runId,path})`
  and `runFolders:()=>…` capability check exposed the way
  `consoleView`/`createConversation` already read status, or a boolean
  passed in from the existing status poll.
- `desktop/src/conversation.js`: a `button.conversation-add-folder` in
  `.conversation-toolbar`, before the status, with a folder icon and
  `aria-label="Add folder…"`, `title="Attach a folder of this workstation to
  the project; Claude sees it from the next message"`. Enabled when a
  conversation is selected, not read-only, and the capability is present
  (passed as `canAddFolder` to `createConversationView`); it stays enabled
  while `busy` (US1.6). On click: `api.chooseRepository()`, then
  `api.addRunFolder(selected, path)`; the status shows the outcome and is
  not overwritten by the next poll for a few seconds (keep a
  `notice`/`noticeUntil` pair read by `controls()`).
- `desktop/src/main.js`: an `addFolderButton` placed before `#save-log`
  like `conversationButton`, labelled "Add folder…", with a sibling
  `role="status"` span. Shown in the same refresh as
  `conversationButton.hidden` (`main.js:648`) when the selected run is
  `skill==='discuss'`, not a conversation, running, not read-only, and the
  agent has `run-folders`. Same click flow; outcome texts:
  - `appliesAt:"now"` → "Attached <path> and typed /add-dir into the session"
  - `appliesAt:"next-launch"` → "Attached <path>: the discussion sees it at its next launch"
  - `mappedAs` → "<path> is a checkout of <identity>: it is now that repository's folder" (+ the typed suffix when typed)
  - refusal → `ipcMessage(err)`.
- `style.css`: the composer button reuses `.conversation-chip` sizing; the
  toolbar status reuses the execution toolbar text style.

### 6. Documentation

- `CHANGELOG.md`, `## [Unreleased]`: under `Added`, "Add a folder to the
  project from a Claude conversation or a ticket discussion in Sectile
  Desktop; Claude sees it at once." Under `Changed`: "Ticket discussions,
  project consoles and Claude conversations now receive the project's
  attached folders."
- `docs/experiments/desktop-conversation.md`: one paragraph on folders per
  turn and the composer button.
- ADR 0036 is not amended: the folder list stays on the workstation and the
  attach path is unchanged. No new ADR: no architectural trade-off is made.

## Data contracts

| Contract | Change |
| --- | --- |
| `POST /desktop/run-folder` | new, local agent only, loopback |
| `GET /desktop/status` `capabilities` | adds `run-folders` |
| conversation `claude -p` argv | adds `--add-dir=<path>` per project folder |
| discussion / console launch line | adds `--add-dir=<quoted path>` for claude, codex |
| free console env | adds `SECTILE_REPOSITORIES` |
| settings file | unchanged (`projectSettings.<id>.folders`) |

## Target files

- `internal/agent/repositories.go` (`projectFolderMap`)
- `internal/agent/agent_conversation.go` (command and turn)
- `internal/agent/agent_config.go` (`live()`)
- `internal/agent/agent_console.go` (PTY launch dirs and env)
- `internal/agent/agent_run_folders.go` (new endpoint, quoting)
- `internal/agent/agent_run.go` (`interactiveProvider`)
- `internal/agent/agent.go` (record the discussion provider)
- `internal/agent/agent_desktop.go` (route, capability)
- `internal/terminal/run.go` (`WaitQuiet`)
- `desktop/electron/main.cjs`, `desktop/electron/preload.cjs`
- `desktop/src/conversation.js`, `desktop/src/main.js`, `desktop/src/style.css`
- `CHANGELOG.md`, `docs/experiments/desktop-conversation.md`
- tests beside each (see `tasks.md`)

## Rejected alternatives

- **Relaunching the PTY discussion with the new `--add-dir`**: loses the
  session's context; rejected by the owner (Round 3, Q4).
- **Letting the Desktop decide the engine and write to the PTY through
  `api.input`**: the Desktop does not know the engine of a discussion (no
  provider is reported for it, `agent.go:1194`), and the input would go to
  whichever session is selected at the time. The agent owns both facts.
- **Caching the folder list on the conversation at admission**: a folder
  attached from the settings page would be missed until a new conversation;
  re-reading per turn costs one local config read, as
  `prepare_repository_worktree` already does at each call.
- **Detecting Claude's prompt by parsing the TUI**: every version draws it
  differently; the existing quiet-wait (`waitQuiet`) is what the terminal
  manager already relies on.

## Risks

- Claude Code's interactive `/add-dir` may ask to confirm or to remember the
  directory. The person answers in the terminal (US3.3); nothing to
  automate.
- The quoting Claude Code's `/add-dir` accepts for a path with spaces is not
  verified. The implementation checks it by hand against the installed
  Claude Code; if double quotes are not accepted, the fallback is to type
  nothing for such a path and answer `appliesAt:"next-launch"` (the launch
  path quotes it for the shell, which is verified).
- A person typing in the session at the same moment would see the line
  appended to their text. The quiet-wait reduces the window; it is accepted.
