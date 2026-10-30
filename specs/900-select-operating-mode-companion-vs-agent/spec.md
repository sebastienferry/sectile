# Spec #900 - Desktop: Select operating mode (Companion vs Agent) at first launch and allow switching in Settings

Ticket: https://github.com/sebastienferry/sectile/issues/900  
Branch: `feat/tauri-monorepo` (or `feat/900-select-operating-mode-companion-vs-agent`)

## Problem

When launching Sectile Desktop (Tauri) for the first time without an existing configuration, there is no explicit onboarding or selection flow to choose between running as an autonomous Local Companion or attaching to an Agent/Fleet server. The user is dropped directly into the settings pane with an ambiguous initial state.

Furthermore, once a mode is configured (`local` or `fleet`), the settings UI in `LocalSettings.tsx` hides the configuration entry point for the alternative mode, locking the user into their initial choice without a straightforward way to switch between Local and Fleet modes.

## Scope

### In
- First-launch mode selection screen in Sectile Desktop (Tauri) when `mode` is `unconfigured` or absent:
  - **Local Companion Mode**: autonomous local runtime, embedded SQLite database, zero remote server dependency.
  - **Agent / Fleet Mode**: workstation client paired with a central Sectile Fleet server via URL and one-time pairing code (or reusing an existing workstation agent).
- Operating mode switcher in Desktop Settings (under Services / Mode d'exécution):
  - Clear indication of the currently active operating mode.
  - Ability to switch to Fleet mode when currently in Local mode.
  - Ability to switch to Local mode when currently in Fleet mode (with confirmation/warning regarding active local tasks vs remote tasks).
- Lifecycle preservation:
  - In both modes, the background agent/companion daemon continues to run independently of the UI window, ensuring active discussions (Claude, Codex, Antigravity) and tasks survive window closing and reopening.

### Out
- Redesigning the central Fleet server API or authentication mechanism.
- Changes to the standalone headless CLI invocation (`sectile-agent`).

## Vocabulary

- **Local Companion Mode**: Self-contained workstation mode running an embedded SQLite server and local companion daemon. Projects and tickets are managed locally.
- **Agent / Fleet Mode**: Workstation execution mode attached to an enterprise or central Sectile server. The server manages projects, tickets, and team orchestration while task execution occurs locally.
- **Onboarding View**: Dedicated initial screen displayed when the desktop app has not yet chosen or configured an operating mode.
- **Mode Switcher**: UI control in Settings allowing explicit transition from one mode to another.

## User stories

### US1 - First-launch mode selection onboarding (P1)
- **US1.1** Given a fresh installation of Sectile Desktop with no configured mode, when the application opens, it displays a welcoming onboarding screen presenting two clear options: "Local Companion" and "Agent (Fleet)".
- **US1.2** When the user chooses "Local Companion", the application initializes the local runtime, starts the companion daemon, and transitions to the local workspace.
- **US1.3** When the user chooses "Agent (Fleet)", the application prompts for the Fleet server URL and pairing code (or offers to attach an existing local agent), verifies connectivity, and transitions to the Fleet workspace.

### US2 - Switching operating mode in Settings (P1)
- **US2.1** Given an application currently operating in Local mode, when the user navigates to Settings -> Services, they can see an explicit "Switch to Fleet mode" section allowing them to pair with a Fleet server.
- **US2.2** Given an application currently operating in Fleet mode, when the user navigates to Settings -> Services, they can see an explicit "Switch to Local Companion" button to transition back to standalone local operation.
- **US2.3** Switching modes persists the selection in `runtime-mode.json` and updates the active runtime without requiring an application restart.

### US3 - Background execution and discussion continuity (P1)
- **US3.1** Given an active conversation, task execution, or PTY terminal session, when the user closes the Desktop application window, the background companion/agent daemon continues running undisturbed.
- **US3.2** When the user reopens the Desktop application, it reconnects to the running daemon and restores the active sessions, conversation transcripts, and execution monitors.
