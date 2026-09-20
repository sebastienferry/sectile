# #281 — Technical Plan: Detach and run task discussion in native terminal

## Stack

- **Workstation CLI & Daemon (Go)**:
  - `cmd/agent/main.go`: Subcommand dispatching for `attach`.
  - `internal/agentattach/`: WebSocket client placing the local terminal in raw mode, relaying I/O and handling window resizing (`SIGWINCH`).
  - `internal/agent/`: Native terminal spawning (`dispatchTerminal`, launcher helper), desktop HTTP routes (`/desktop/terminal/detach`, `/desktop/tasks/terminal-external`).
  - `internal/terminal/`: Multi-client PTY session management, history buffer replay, and process lifecycle supervision.
- **Server HTTP Layer (Go)**:
  - `internal/handlers/`: `POST /api/tasks/{id}/tty-external` and `launchTaskExternalTerminal` delegating dispatch to connected agent.
- **Desktop Companion (Electron & JavaScript)**:
  - `desktop/src/main.js`: Ticket row `…` menu, execution toolbar "Detach to native terminal" button, external status indicator badge, and project settings terminal dropdown.
  - `desktop/electron/preload.cjs` & `desktop/electron/main.cjs`: IPC channels for launching native terminal discussions and detaching sessions.

---

## Architecture decisions

### D1 — WebSocket-Backed Terminal Attach Client (`sectile-agent attach`)

Rather than having the external terminal spawn an isolated, unmonitored shell process, the native terminal executes a lightweight attach client:
```bash
sectile-agent attach --session <sessionID> [--url <loopback>] [--token <token>]
```

**Attachment Flow**:
1. When launched, `sectile-agent attach` resolves the daemon loopback URL and authentication token (via command line flags, `SECTILE_LOOPBACK_URL`/`SECTILE_AGENT_TOKEN` environment variables, or reading `~/.taskflow/agent-connection.json`).
2. It establishes a WebSocket connection to `ws://<loopback>/desktop/terminal?id=<sessionID>` (sending `Authorization: Bearer <token>`).
3. It puts the host terminal standard input into **raw mode** using `golang.org/x/term.MakeRaw(int(os.Stdin.Fd()))`, ensuring escape sequences, arrow keys, and Ctrl+C pass transparently to the daemon PTY.
4. It starts bidirectional streaming:
   - Stdin bytes are forwarded over WebSocket as binary/text messages.
   - PTY output from WebSocket is written directly to Stdout.
   - Initial history buffer sent by `terminal.Manager.HandleWebSocket` repaints existing session state immediately.
5. Window resizing (`SIGWINCH` on Unix or console buffer events on Windows) sends `{type: "resize", cols: N, rows: M}` JSON messages to synchronize PTY dimensions.
6. Upon disconnection or process exit, the terminal state is restored via `term.Restore(...)`.

### D2 — Native Terminal Launcher Abstraction

A new launcher helper in `internal/agent/terminal_launcher.go` abstracts launching host terminal applications across operating systems:

- **macOS**:
  - **Ghostty**:
    - If `ghostty` CLI binary is in PATH: `ghostty -e <command> <args...>`
    - If `/Applications/Ghostty.app` exists: `open -a /Applications/Ghostty.app --args -e <command> <args...>` or invoking a temporary executable launcher script via `open -a /Applications/Ghostty.app <script>`.
  - **Terminal.app**:
    - Executes via temporary executable `.command` launcher script opened with `open -a Terminal <scriptPath>`, or AppleScript `osascript -e 'tell application "Terminal" to do script "..."'`.
  - **iTerm2**:
    - Executes via `open -a iTerm <scriptPath>` or AppleScript.
  - **Custom Command**:
    - Executes custom template substituting `{command}` or appending the attach command arguments.
- **Windows**:
  - `wt.exe -w 0 nt <command>` or `cmd.exe /c start <command>`.
- **Linux**:
  - `x-terminal-emulator -e <command>` or detected emulator (`gnome-terminal`, `kitty`, `alacritty`).

### D3 — Dual Observability and Non-Destructive Detach

The Sectile agent daemon's `terminal.Manager` already supports multiple concurrent WebSocket clients connected to a single `Session`:
- `sess.clients[conn] = true` broadcasts all PTY output to every active connection.
- When an active discussion in the desktop app is detached:
  1. The desktop calls the IPC `detachToNativeTerminal(runId)`.
  2. The agent spawns the native terminal emulator executing `sectile-agent attach --session <runId>`.
  3. The desktop app keeps its own WebSocket connection attached (or can detach and reattach seamlessly).
  4. The desktop console view displays an informative badge: `Active in <TerminalApp>` in the toolbar.
  5. The session's `run.desktop.Status` remains `running`.
  6. The **Stop** button in the desktop toolbar sends `POST /desktop/stop?id=<runId>`, terminating the session and cleaning up both the desktop canvas and the native terminal window.

### D4 — Configuration Precedence and Local Overrides

Terminal emulator choice is resolved with strict precedence:
1. **Explicit Call Override**: Value passed in API request (`terminalOverride`).
2. **Project Local Override**: Stored in `agentconfig.Overrides.Terminals[projectID]` (or `models.Project.ExternalTerminalCommand`).
3. **Workstation Setting**: Stored in `agentconfig.Overrides.Terminal` in `~/.config/sectile/settings.json`.
4. **Auto-Detection**: `detectDefaultTerminal()` inspecting filesystem:
   - macOS: `/Applications/Ghostty.app` → `ghostty`, then `/Applications/iTerm.app` → `iterm`, fallback `terminal`.
   - Windows: `wt.exe` in PATH → `wt`, fallback `cmd`.
   - Linux: `x-terminal-emulator`.

### D5 — Desktop UI Controls

1. **Ticket Row Action**:
   - In `desktop/src/main.js` ticket `…` menu:
   - In addition to `Discussion (no skill)`, add `Discussion in native terminal`.
   - Triggers `api.launchNativeDiscussion(projectID, taskID)` which creates the discussion run and immediately spawns the native terminal emulator.
2. **Execution Toolbar Detach Button**:
   - In `desktop/src/main.js` toolbar:
   - For executions where `run.skill === 'discuss'` and status is `running`, render a `Detach to native terminal` button.
   - When clicked, invokes `api.detachToNativeTerminal(run.id)`.
3. **Project Settings Dialog**:
   - In `desktop/src/main.js` (`openProject` dialog → Local tab):
   - Add a dropdown for preferred terminal emulator: `Auto-detect`, `Ghostty`, `Terminal.app`, `iTerm`, `Custom`.
   - If `Custom`, display a text input for the custom command template.

### D6 — Rejected Alternatives

- **Spawning an independent CLI process directly in the external terminal**:
  - *Rejected*: An unmonitored external process cannot be detached from an existing desktop session without losing active conversation state, cannot mirror output back to the desktop console, and cannot be stopped or supervised via desktop controls.
- **Terminating the desktop xterm.js connection upon detach**:
  - *Rejected*: Preserving output mirroring in the desktop console provides continuous observability, ensures export logs remain complete, and lets users monitor progress without switching desktop spaces.

---

## Data contracts

### 1. Agent CLI Attach Command

```bash
sectile-agent attach [--url <loopback-ws-url>] [--token <auth-token>] --session <sessionID>
```

- `--session`: Required. The PTY session ID or task run ID.
- `--url`: Optional. Loopback WebSocket URL (defaults to `SECTILE_LOOPBACK_URL` or `~/.taskflow/agent-connection.json`).
- `--token`: Optional. Workstation authentication token (defaults to `SECTILE_AGENT_TOKEN` or `~/.taskflow/agent-connection.json`).

### 2. Agent Desktop HTTP Endpoints

#### `POST /desktop/tasks/terminal-external`
Launches a new task discussion directly in an external terminal:
```json
// Request
{
  "projectId": "ef5a2777-920f-4744-a7e8-a58a4c257a23",
  "taskId": "gh-ef5a2777-920f-4744-a7e8-a58a4c257a23-281",
  "skillId": "discuss",
  "terminal": "ghostty"
}

// Response: 200 OK
{
  "success": true,
  "runId": "b8f6168e-8b60-40a6-abe6-955e159b78f7",
  "terminal": "ghostty"
}
```

#### `POST /desktop/terminal/detach`
Detaches an existing running session to an external terminal:
```json
// Request
{
  "runId": "b8f6168e-8b60-40a6-abe6-955e159b78f7",
  "terminal": "ghostty"
}

// Response: 200 OK
{
  "success": true,
  "detached": true,
  "terminal": "ghostty"
}
```

### 3. Electron IPC Bridge

In `desktop/electron/preload.cjs`:
```javascript
launchNativeDiscussion: (projectId, taskId, terminal) => ipcRenderer.invoke('launch-native-discussion', { projectId, taskId, terminal }),
detachToNativeTerminal: (runId, terminal) => ipcRenderer.invoke('detach-to-native-terminal', { runId, terminal })
```

In `desktop/electron/main.cjs`:
```javascript
ipcMain.handle('launch-native-discussion', async (_, { projectId, taskId, terminal }) => {
  return await api('/desktop/tasks/terminal-external', 'POST', { projectId, taskId, skillId: 'discuss', terminal })
})
ipcMain.handle('detach-to-native-terminal', async (_, { runId, terminal }) => {
  return await api('/desktop/terminal/detach', 'POST', { runId, terminal })
})
```

### 4. Workstation Settings (`~/.config/sectile/settings.json`)

```json
{
  "terminal": "ghostty",
  "terminals": {
    "ef5a2777-920f-4744-a7e8-a58a4c257a23": "ghostty"
  }
}
```

---

## Target files

1. `cmd/agent/main.go`:
   - Register `attach` subcommand routing to `agentattach.Run(args[1:])`.
2. `internal/agentattach/attach.go` (new package):
   - Implements WebSocket client, raw mode handling, I/O pumping, and `SIGWINCH` resize handling.
3. `internal/agentattach/attach_test.go` (new package):
   - Unit tests for argument parsing, credential resolution, and terminal normalization.
4. `internal/agent/terminal_launcher.go` (new file):
   - Implements native terminal launching for macOS (`Ghostty`, `Terminal.app`, `iTerm`), Windows, and Linux.
5. `internal/agent/terminal_launcher_test.go` (new file):
   - Unit tests asserting terminal resolution and launcher command generation.
6. `internal/agent/agent_desktop.go`:
   - Add `/desktop/terminal/detach` and `/desktop/tasks/terminal-external` HTTP endpoints.
   - Update `desktopProject` and `desktopProjects` to read and persist project-level terminal preferences.
7. `internal/agentconfig/local.go` & `settings.go`:
   - Support `Terminals map[string]string` in `Overrides` for per-project terminal overrides.
8. `desktop/electron/preload.cjs` & `desktop/electron/main.cjs`:
   - Expose `launchNativeDiscussion` and `detachToNativeTerminal` IPC handlers.
9. `desktop/src/main.js`:
   - Add **Discussion in native terminal** to ticket row `…` menu.
   - Add **Detach to native terminal** button to execution toolbar for active discussion runs.
   - Add external terminal indicator badge in console header.
   - Add terminal emulator configuration control in `openProject` Local tab.
10. `desktop/tests/main.test.mjs` (or relevant desktop test suite):
    - Tests for new menu options, toolbar button interactions, and project settings.
