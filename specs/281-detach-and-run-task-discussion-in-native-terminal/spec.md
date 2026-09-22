# #281 — Detach and run task discussion in native terminal (Ghostty, Terminal)

## Context

Currently, interactive task discussions (`skillId = 'discuss'`) in Sectile run inside an embedded pseudo-terminal (PTY) managed by the local Sectile agent daemon. Output is streamed over WebSockets to an embedded xterm.js canvas in the Electron desktop companion app.

While this embedded terminal provides an integrated view for reviewing automated skill runs, software engineers frequently prefer their host workstation terminal emulator (such as Ghostty, Apple Terminal, iTerm2, or Alacritty) during interactive, conversational CLI workflows (e.g. Claude Code, Codex, or shell dialogues). Host terminal emulators offer native font rendering, clipboard integration, split panes, custom keybindings, and accessibility features that cannot be fully matched inside an embedded canvas.

Because the background agent daemon already manages the underlying PTY session lifecycle independently of the Electron desktop UI, discussion sessions can run in or be detached to a native terminal emulator while remaining tracked, observable, and stoppable from the Sectile desktop application.

This specification describes behaviour and acceptance criteria only. Technical decisions, architecture, and data contracts are specified in `plan.md`, and the ordered implementation checklist is in `tasks.md`.

---

## Decisions being specified

1. **Native Terminal Discussion Launch**: Users can launch a free-form task discussion (`skillId = 'discuss'`) directly into their configured native terminal emulator (Ghostty, Terminal.app, iTerm, custom command) from the desktop UI ticket list.
2. **On-the-Fly Detach without Process Interruption**: When a discussion session is active in the desktop embedded console, the user can detach it to an external native terminal window. The underlying discussion process and session history are preserved without interruption.
3. **Dual Observability & Live Output Mirroring**: The desktop application continues to mirror session output in real time within its console view. An active status badge (e.g. `Active in Ghostty`) indicates the external attachment state, and the session remains stoppable from the desktop interface.
4. **Execution Context Retention**: Sessions launched or detached into a native terminal retain the full task execution context:
   - Working directory set to the task worktree or local repository checkout.
   - Git branch set to the assigned task branch (e.g. `feat/281`).
   - `SECTILE_*` environment variables (`SECTILE_TASK_KEY`, `SECTILE_TASK_BRANCH`, `SECTILE_TASK_WORKTREE`, `SECTILE_TASK_ID`, `SECTILE_RUN_ID`, `SECTILE_LOOPBACK_URL`, `SECTILE_AGENT_TOKEN`).
5. **Configurable Terminal Emulator Precedence**: The preferred terminal application is resolved according to a defined hierarchy:
   1. Explicit invocation override (`terminalOverride` if supplied).
   2. Project configuration (`models.Project.ExternalTerminalCommand` / Desktop project settings).
   3. Workstation setting (`agentconfig.Overrides.Terminal` in `settings.json` / `user_settings.external_terminal_command`).
   4. Auto-detection (`Ghostty` → `iTerm` → `Terminal.app` on macOS; `wt` → `cmd` on Windows; `x-terminal-emulator` on Linux).
6. **Workflow Isolation**: Task discussions remain free-form interactive sessions. They carry no automated skill prompts and trigger no workflow stage transitions (they do not advance tickets from `clarified` to `specified` or `implemented`).

---

## Boundaries & Out of Scope

- **Non-Discussion Skills**: Headless, autonomous, or managed skill runs (`clarify`, `specify`, `implement`, `adjust`, `handoff`) continue following their standard execution pipelines and are not launched in external terminals.
- **Workflow Stage Transitions**: Discussions do not modify task workflow labels or status.
- **Disconnection of Local Daemon**: Running an external terminal requires the local Sectile agent daemon to be active; standalone server-only web terminals without an agent daemon cannot spawn workstation terminal windows.

---

## User stories

### US1 — Launch task discussion directly in native terminal (P1)

**As a** developer using the Sectile desktop application,  
**I want** to launch an interactive task discussion directly in my host native terminal (e.g. Ghostty or Terminal.app),  
**So that** I can conduct interactive discussions using my native terminal environment and shortcuts.

- **Given** an open project with an active local agent daemon
- **When** the user opens the ticket row `…` (more options) menu in the desktop application
- **Then** a menu option **Discussion in native terminal** is visible alongside **Discussion (no skill)**.
- **When** the user clicks **Discussion in native terminal**
- **Then** the local agent daemon initializes a PTY session with the task worktree and environment variables.
- **And** the user's preferred host terminal emulator opens in a new window running the interactive session.
- **And** the execution is registered and displayed in the desktop application's active execution list.

---

### US2 — Detach active desktop discussion to native terminal on the fly (P1)

**As a** developer who started a discussion inside the desktop application,  
**I want** to detach the ongoing session into a native terminal window without restarting it,  
**So that** I can continue an in-flight conversation in my terminal without losing conversation history or context.

- **Given** an active task discussion session (`skillId = 'discuss'`) displayed in the desktop console
- **When** the execution toolbar renders
- **Then** a **Detach to native terminal** button is visible and enabled.
- **When** the user clicks **Detach to native terminal**
- **Then** the host native terminal emulator opens and connects to the existing session.
- **And** the underlying discussion process continues running without termination or restart.
- **And** the prior terminal history buffer is rendered in the newly opened native terminal.
- **And** the desktop console toolbar updates to show that the session is active in the native terminal.

---

### US3 — Desktop mirroring, supervision, and lifecycle management (P1)

**As a** developer with a discussion running in an external native terminal,  
**I want** the desktop application to maintain visibility and control over the session,  
**So that** I can monitor progress, inspect output, and stop the session from the desktop app if needed.

- **Given** a task discussion running in or detached to an external terminal
- **When** the user views the execution in the desktop application console
- **Then** a badge indicates the external status (e.g. **Active in Ghostty**).
- **And** terminal output continues to be mirrored live in the desktop xterm.js canvas.
- **And** the **Stop** button in the desktop toolbar remains enabled.
- **When** the user clicks **Stop** in the desktop application
- **Then** the discussion process is terminated gracefully.
- **And** the native terminal attach client exits cleanly.
- **And** the desktop execution list marks the execution as finished.

---

### US4 — Configure preferred terminal emulator per project and workstation (P2)

**As a** developer working across different projects or workstations,  
**I want** to configure my preferred terminal emulator in the desktop project settings and workstation profile,  
**So that** Sectile opens my terminal of choice automatically.

- **Given** the desktop application project settings dialog (`openProject`)
- **When** the user views the **Local** settings tab
- **Then** a **Terminal emulator** setting is available with choices (Ghostty, Terminal.app, iTerm, Custom command).
- **And** an indication shows whether the value is inherited from workstation defaults or overridden locally.
- **When** the user selects a terminal emulator and saves the configuration
- **Then** the setting is saved locally in workstation settings (`settings.json`).
- **And** subsequent external terminal launches for this project use the configured emulator.

---

### US5 — Terminal emulator auto-detection and custom command fallback (P2)

**As a** developer running Sectile on a machine without explicit terminal configuration,  
**I want** Sectile to detect installed terminal emulators automatically or support a custom terminal command,  
**So that** external terminal launching works out of the box without manual setup.

- **Given** a workstation on macOS where Ghostty is installed in `/Applications/Ghostty.app`
- **When** no terminal preference is explicitly configured
- **Then** Sectile automatically selects Ghostty as the default native terminal emulator.
- **Given** a workstation on macOS where Ghostty is absent but iTerm is present
- **When** no terminal preference is configured
- **Then** Sectile selects iTerm as the default.
- **Given** a workstation where a custom terminal command is configured (e.g. `alacritty -e`)
- **When** an external discussion is launched
- **Then** the custom command is executed with the attach command arguments.
