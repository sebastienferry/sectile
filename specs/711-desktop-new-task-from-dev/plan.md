# Plan #711 - A launch from the web follows the Desktop console view

Behaviour: `spec.md`. This file says how.

## Stack

- Agent: Go, `internal/agent` and `internal/agentconfig`.
- Desktop: Electron main process (`desktop/electron/main.cjs`,
  `preload.cjs`) and renderer (`desktop/src/main.js`).
- No server, web, database or migration change.

## Root cause

The conversation view is a per-launch hint: Desktop sends `view:
"conversation"` with `POST /desktop/tasks`, the agent records it with
`d.conversationViews.mark(task.ID)` (`internal/agent/agent_desktop.go:977`)
and consumes it when the dispatch comes back
(`internal/agent/agent.go:1261`). A launch the web app sends straight to the
server's `/run-skill` leaves no mark, so the dispatch falls back to a PTY.

## Design

### 1. The agent stores the workstation console view

- `internal/agentconfig/workstation.go`, `Defaults`: add
  `ConsoleView string \`json:"consoleView,omitempty"\``, with the constants
  `ConsoleViewTerminal = "terminal"` and `ConsoleViewConversation =
  "conversation"`, and `func (d Defaults) ConsoleViewOrDefault() string`
  returning `terminal` for anything but `conversation`.
- The value lives under `defaults` in `~/.config/sectile/settings.json`, an
  owned key `WriteSettings` already replaces as a whole. Desktop keeps its own
  top-level `consoleView` key in the same file by default; the two keys do not
  collide, and Desktop's `{...previous, consoleView}` rewrite keeps `defaults`.
- `internal/agentconfig/local.go`, `overlay`: carry the field with
  `ConsoleView: firstSet(top.Defaults.ConsoleView, base.Defaults.ConsoleView)`.
  Without it, the value is lost on the next read (see the "New Overrides key
  needs a merge" lesson).

### 2. A Desktop endpoint and its capability

- New file `internal/agent/agent_desktop_console_view.go`:
  - `const consoleViewCapability = "console-view-default"`, appended to the
    capability list of `/desktop/status` (`agent_desktop.go:173`).
  - `func (d *agentDaemon) desktopConsoleView(w, r)` on
    `/desktop/console-view`:
    - `GET` answers `{"view": "<terminal|conversation>"}`;
    - `PUT {"view": "..."}` refuses with 400 a value other than the two
      constants (FR9), else saves it with `agentconfig.UpdateSettings(
      d.localSettingsRoot(), ...)` under `d.prepareMu`, as `desktopEngines`
      does, and answers the saved value.
  - Route it in the `/desktop/...` switch of `agent_desktop.go`, next to
    `/desktop/engines`.
- The endpoint is behind the same loopback and token checks as every
  `/desktop/` route; nothing new reaches the server.

### 3. The dispatch falls back to the workstation console view

`internal/agent/agent.go:1261` becomes, in substance:

```go
marked := d.conversationViews.take(taskRef, task.ID)
if !autonomous && action != "open_terminal" && (marked || d.workstationConsoleView() == agentconfig.ConsoleViewConversation) && conversationDiscussionEngine(config) {
```

- `take` is still called on every dispatch, before the `||`, so a mark is
  consumed whether or not the default would have applied, and expired marks
  are still swept.
- `d.workstationConsoleView()` reads `agentconfig.ReadSettings(
  d.localSettingsRoot())` at dispatch time, so a change applies to the next
  dispatch (FR6); a read error means `terminal`. One settings read per
  dispatch is negligible next to the launch it precedes.
- Eligibility is unchanged (FR5): autonomous, `open_terminal` and engine
  checks are the same conditions in the same expression. **Discussion in
  native terminal** goes through `/desktop/tasks/terminal-external`, which
  does not reach this branch.
- Update the doc comments of `pendingDiscussionViews` and of the `View` field
  of `desktopTasks`: the mark is now an explicit per-launch request that wins,
  the workstation view the fallback.

### 4. Desktop gives the setting to the agent

- `desktop/electron/main.cjs`:
  - `async function syncConsoleView()`: reads the saved setting, calls
    `/desktop/status`, and when `capabilities` includes
    `console-view-default`, sends `PUT /desktop/console-view` with it. Every
    error is swallowed (FR7, FR8).
  - `set-console-view`: after writing the file, calls `syncConsoleView()`
    without awaiting its outcome in the returned value, so saving never fails
    on the agent.
  - `ipcMain.handle('sync-console-view', syncConsoleView)`.
- `desktop/electron/preload.cjs`: expose `syncConsoleView`.
- `desktop/src/main.js`, `ready()`: on each (re)connection, where
  `loadEditorSetting()` already runs once per connection, call
  `api.syncConsoleView().catch(()=>{})` (FR2).
- Desktop keeps sending `view: "conversation"` with its own launches; on an
  older agent that is still what opens the conversation.

## Rejected alternatives

- **The agent reads Desktop's top-level `consoleView` key.** The two files
  differ under `SECTILE_DESKTOP_DATA_DIR` (the UI tests and any custom data
  directory), and it would bind the agent to a key Desktop owns. The
  clarification chose an explicit hand-over.
- **The web app sends the view to the server.** The view is a workstation
  display choice; another workstation may differ, and the server would have
  to learn a preference it must not hold.
- **Desktop sends an explicit `view: "terminal"` mark.** Desktop reconnects
  and gives its setting before it can launch anything, so the default already
  matches it; a terminal mark would add a state for no observable gain.

## Data contracts

```text
GET  /desktop/console-view            -> 200 {"view":"terminal"|"conversation"}
PUT  /desktop/console-view {"view":v} -> 200 {"view":v}
                                      -> 400 unknown view
                                      -> 405 other method
GET  /desktop/status                  -> capabilities += "console-view-default"
settings.json defaults.consoleView    -> "conversation" | absent (terminal)
```

## Target files

- `internal/agentconfig/workstation.go`, `internal/agentconfig/local.go`
- `internal/agent/agent_desktop_console_view.go` (new), its test
- `internal/agent/agent_desktop.go` (route, capability, `View` comment)
- `internal/agent/agent.go` (dispatch condition)
- `internal/agent/agent_discussion_conversation.go` (comment)
- `desktop/electron/main.cjs`, `desktop/electron/preload.cjs`,
  `desktop/src/main.js`
- `desktop/tests/conversation.ui.cjs` (or a new `console-view-sync.ui.cjs`)
- `CHANGELOG.md`, `desktop/README.md` if it describes the setting

## Test plan

- `internal/agentconfig`: a settings round trip keeps `defaults.consoleView`
  through `WriteSettings` then `ReadSettings`, including through `overlay`
  with a legacy repository file; an absent value reads `terminal`.
- `internal/agent`, endpoint: `PUT` then `GET` returns the value; an unknown
  value is refused with 400 and leaves the stored one; `/desktop/status`
  lists `console-view-default`.
- `internal/agent`, dispatch: with the view `conversation` and no mark, an
  interactive Claude dispatch opens a conversation; autonomous, `open_terminal`
  and a non-Claude engine keep their path; with the view `terminal` and no
  mark, a PTY opens; a mark still wins and is consumed. Reuse the harness of
  the existing conversation dispatch tests.
- Desktop UI: changing **Claude consoles** sends `PUT /desktop/console-view`
  to a fake agent announcing the capability, and sends nothing to one that
  does not; connecting sends the saved value; the setting is saved even when
  the agent refuses or is down.
- Run the Go suites outside the sandbox (httptest), and the Electron suites
  after `npx vite build`, sandbox off.
