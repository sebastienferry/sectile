# #281 — Implementation checklist

Ordered so the attach client and configuration layers are implemented and verified first, followed by the native terminal launcher, the local agent daemon endpoints, and finally the desktop companion UI.

---

## 1. Storage & Configuration Layer (`internal/agentconfig`)

- [x] **T1** In `internal/agentconfig/local.go`, extend `Overrides` with `Terminals map[string]string` (`json:"terminals,omitempty"`) for per-project terminal emulator overrides.
- [x] **T2** In `internal/agentconfig/local.go`, update `ApplyOverrides(c Config, overrides Overrides) Config` to check `overrides.Terminals[c.ProjectID]`; if non-empty, set `c.ExternalTerminalCommand`.
- [x] **T3** In `internal/agentconfig/settings.go`, include `"terminals"` in `WriteSettings` for atomic persistence to `~/.config/sectile/settings.json`.
- [x] **T4** In `internal/agentconfig/local_test.go` and `internal/agentconfig/settings_test.go`, add unit tests validating `Terminals` precedence, fallback to workstation `Terminal`, and settings file round-trip serialization.

---

## 2. Agent Terminal Attach Client (`cmd/agent`, `internal/agentattach`)

- [x] **T5** Create `internal/agentattach/attach.go`:
  - Implement `Run(args []string) error` parsing `--session`, `--url`, and `--token`.
  - Fall back to `SECTILE_LOOPBACK_URL`, `SECTILE_AGENT_TOKEN`, and `~/.taskflow/agent-connection.json` if connection flags are omitted.
  - Connect to WebSocket endpoint `ws://<loopback>/desktop/terminal?id=<session>` with authentication header.
  - Set standard input to raw mode via `golang.org/x/term.MakeRaw(int(os.Stdin.Fd()))`, with a deferred cleanup restore.
  - Relay stdin bytes to WebSocket and write incoming binary WebSocket messages to stdout.
  - Handle window resize signals (`SIGWINCH` on Unix) and dispatch `{type: "resize", cols, rows}` JSON payloads.
- [x] **T6** In `cmd/agent/main.go`, register the `attach` subcommand:
  ```go
  case "attach":
      if err := agentattach.Run(args[1:]); err != nil {
          log.Fatal(err)
      }
      return
  ```
- [x] **T7** In `internal/agentattach/attach_test.go`, add unit tests:
  - Verify flag parsing and connection information resolution.
  - Verify graceful handling of missing session ID.

---

## 3. Native Terminal Launcher & Agent Daemon Endpoints (`internal/agent`)

- [x] **T8** In `internal/agent/terminal_launcher.go`:
  - Implement `launchExternalTerminal(terminalApp, sessionID, loopbackURL, token string) error`.
  - Support macOS targets: Ghostty (`ghostty -e`), Terminal.app (launcher script or AppleScript), iTerm2 (`open -a iTerm`).
  - Support custom command templates substituting `{command}` or `{session}`.
  - Support Windows (`wt.exe` / `cmd.exe`) and Linux (`x-terminal-emulator`).
- [x] **T9** In `internal/agent/terminal_launcher_test.go`, add unit tests asserting command construction for supported terminal emulators.
- [x] **T10** In `internal/agent/agent_desktop.go`:
  - Add endpoint `POST /desktop/terminal/detach`: takes `runId` and optional `terminal`, looks up active session, invokes `launchExternalTerminal`, and responds with status.
  - Add endpoint `POST /desktop/tasks/terminal-external`: creates a task discussion run in `terminal.Manager` and immediately spawns the native terminal emulator.
  - Update `desktopProject` (`GET /desktop/project`) to include `terminal` and `terminalOverride`.
  - Update `desktopProjects` (`POST /desktop/projects`) to accept and store `terminal` and `inheritTerminal`.
- [x] **T11** In `internal/agent/agent_desktop_test.go`, add tests for `POST /desktop/terminal/detach` and `POST /desktop/tasks/terminal-external`.

---

## 4. Electron Desktop Companion Bridge (`desktop/electron/`)

- [x] **T12** In `desktop/electron/preload.cjs`, expose:
  - `launchNativeDiscussion(projectId, taskId, terminal)`
  - `detachToNativeTerminal(runId, terminal)`
- [x] **T13** In `desktop/electron/main.cjs`, implement IPC handlers:
  - `ipcMain.handle('launch-native-discussion', ...)` calling `POST /desktop/tasks/terminal-external`.
  - `ipcMain.handle('detach-to-native-terminal', ...)` calling `POST /desktop/terminal/detach`.

---

## 5. Desktop UI Integration (`desktop/src/`)

- [x] **T14** In `desktop/src/main.js` (Ticket row menu `renderTicketRow`):
  - Add **Discussion in native terminal** entry to the `…` dropdown menu for open tasks.
  - On click, call `api.launchNativeDiscussion(...)`.
- [x] **T15** In `desktop/src/main.js` (Console toolbar and execution view):
  - For active sessions where `run.skill === 'discuss'` and `run.status === 'running'`, render a **Detach to native terminal** button in `#toolbar`.
  - When clicked, call `api.detachToNativeTerminal(run.id)` and update toolbar status badge.
  - Render an active status badge (e.g. `Active in Ghostty`) when a discussion is running externally.
- [x] **T16** In `desktop/src/main.js` (`openProject` dialog → Local tab):
  - Add **Terminal emulator** dropdown with choices: `Auto-detect`, `Ghostty`, `Terminal.app`, `iTerm`, `Custom command`.
  - Add reset button to revert to workstation default.
  - Persist setting during project save form submission.
- [x] **T17** In `desktop/src/style.css`, add styles for the native terminal status badge and toolbar button.

---

## 6. Quality Gates & Verification

- [x] **T18** Run Go test suite: `go test -v -race ./internal/agentconfig/... ./internal/agent/... ./internal/agentattach/...`.
- [x] **T19** Run Desktop unit tests: `npm test` inside `desktop/`.
- [x] **T20** Verify all acceptance criteria in `spec.md` (US1 through US5).
