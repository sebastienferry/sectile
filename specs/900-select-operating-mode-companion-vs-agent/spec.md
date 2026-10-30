# Spec #900 - Desktop: Select operating mode (Companion vs Agent) at first launch and allow switching in Settings

Ticket: https://github.com/sebastienferry/sectile/issues/900  
Clarification: `docs/clarifications/900.md` (rounds 1 and 2, settled)  
Branch: `feat/tauri-monorepo`

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
  - Ability to switch to Local mode when currently in Fleet mode.
  - Active executions guard: confirmation prompt before switching modes if tasks or conversations are running.
- Strict data isolation:
  - Local database and projects stored in `generations/companion-v1/`.
  - Fleet pairing and configuration stored in `fleet/`.
  - Switching modes retains each workspace's data intact.
- Lifecycle preservation:
  - In both modes, the background agent/companion daemon continues to run independently of the UI window, ensuring active discussions (Claude, Codex, Antigravity) and tasks survive window closing and reopening.

### Out
- Redesigning the central Fleet server API or authentication mechanism.
- Changes to the standalone headless CLI invocation (`sectile-agent`).
- Migrating or merging local SQLite databases with remote Fleet databases.

## Vocabulary

- **Local Companion Mode**: Self-contained workstation mode running an embedded SQLite server and local companion daemon. Projects and tickets are managed locally.
- **Agent / Fleet Mode**: Workstation execution mode attached to an enterprise or central Sectile server. The server manages projects, tickets, and team orchestration while task execution occurs locally.
- **ModeOnboarding**: Dedicated full-page view displayed when the desktop app has not yet chosen or configured an operating mode.
- **Mode Switcher**: UI control in Settings allowing explicit transition from one mode to another.

## User stories and acceptance criteria

### US1 - First-launch mode selection onboarding (P1)

- **US1.1** Given a fresh installation of Sectile Desktop with no configured mode (`setup.mode === "unconfigured"` or absent), when the application opens, it displays a full-page onboarding view (`ModeOnboarding.tsx`) presenting two visual cards:
  - Card 1: "Mode Compagnon Local" (autonome, base SQLite locale, aucun serveur requis).
  - Card 2: "Mode Agent Fleet" (jumelage avec un serveur d'équipe Sectile via URL et code).
- **US1.2** When the user clicks "Démarrer en mode Compagnon Local", the application calls `api.enableLocalRuntime()`, updates `runtime-mode.json` to `local`, initializes the local companion, and navigates to the workspace.
- **US1.3** When the user selects "Mode Agent Fleet", an inline pairing form appears prompting for the Fleet server URL and pairing code (with an option to "Attacher un agent existant"). Upon successful pairing, the application updates `runtime-mode.json` to `fleet` and navigates to the Fleet workspace.
- **US1.4** If an error occurs during runtime startup or pairing, an error banner is displayed with a retry button without leaving the onboarding view.

### US2 - Switching operating mode in Settings (P1)

- **US2.1** Given an application currently operating in Local mode, when the user navigates to Settings -> Services, a prominent badge displays "Mode actif : Compagnon Local" and a "Basculer vers le mode Fleet (Agent)" section is visible with the pairing form and existing agent attachment option.
- **US2.2** Given an application currently operating in Fleet mode, when the user navigates to Settings -> Services, a prominent badge displays "Mode actif : Agent Fleet" and a "Basculer vers le mode Compagnon Local" action is visible.
- **US2.3** When the user initiates a mode switch, if any active task, discussion, or PTY terminal session is running, a confirmation modal warns the user ("Des exécutions sont actuellement en cours. Voulez-vous vraiment changer de mode ?") with "Annuler" and "Confirmer le changement".
- **US2.4** When the mode switch is confirmed, the application persists the change in `runtime-mode.json`, refreshes the runtime state, resets the workspace views, and reloads the active project list without requiring an application restart.

### US3 - Strict data isolation between modes (P1)

- **US3.1** Given an existing Local workspace with local projects and tickets in `generations/companion-v1`, when the user switches to Fleet mode and later switches back to Local mode, all local projects, trackers, and settings are fully restored.
- **US3.2** Given an existing Fleet pairing in `fleet/`, when the user switches to Local mode and later switches back to Fleet mode, the Fleet server connection and pairing credentials are retained.

### US4 - Background execution and conversation continuity (P1)

- **US4.1** Given an active conversation, task execution, or PTY terminal session, when the user closes the Desktop application window, the background companion/agent daemon continues running undisturbed.
- **US4.2** When the user reopens the Desktop application, it reconnects to the running daemon and restores the active sessions, conversation transcripts, and execution monitors.
