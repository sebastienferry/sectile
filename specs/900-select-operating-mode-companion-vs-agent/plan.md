# Plan #900 - Desktop: Select operating mode (Companion vs Agent) at first launch and allow switching in Settings

## Stack

- Frontend: React 19, TypeScript, Vite in `desktop-tauri/src/`
- Tauri backend: Rust, Tauri 2 in `desktop-tauri/src-tauri/src/`
- Go services: `cmd/companion`, `cmd/agent`, `cmd/server` (bundled binaries in `src-tauri/binaries/`)

## Design

### 1. First-Launch Onboarding Component (`ModeOnboarding.tsx`)
Create a dedicated component `desktop-tauri/src/components/ModeOnboarding.tsx`:
- Rendered in `desktop-tauri/src/App.tsx` when `setup.mode === "unconfigured"` or `!setup.mode`.
- Displays a clean, two-card choice:
  1. **Mode Compagnon Local**
     - Description: "Exécution et gestion de tickets 100% autonomes sur votre poste. Base de données locale intégrée, sans serveur distant requis."
     - Action: Calls `api.enableLocalRuntime()`.
  2. **Mode Agent Fleet**
     - Description: "Connectez votre poste à un serveur central Sectile d'équipe. Partagez vos projets et déléguez l'orchestration tout en conservant l'exécution en local."
     - Action: Opens the inline pairing form (URL + Pairing Code) or "Attach existing agent".
- Includes clear status and error handling.

### 2. Mode Switching in Settings (`LocalSettings.tsx`)
Update `desktop-tauri/src/components/LocalSettings.tsx`:
- Under the `services` section:
  - Display the current active mode prominently with a status badge.
  - When in `Local` mode: Show an expandable or explicit section "Basculer vers le mode Fleet (Agent d'équipe)" with the pairing form (`pairFleetAgent`) and "Attach existing agent" (`attachFleetAgent`).
  - When in `Fleet` mode: Show an explicit action "Basculer vers le mode Compagnon local" which invokes `enableLocalRuntime()`.
  - Ensure the state refresh cleanly reloads projects and execution monitors after mode transition.

### 3. Tauri Runtime Handling (`local.rs`)
In `desktop-tauri/src-tauri/src/local.rs`:
- Verify `enable_local_runtime` properly stops or re-targets active connections when switching from Fleet to Local.
- Verify `pair_fleet_agent` and `attach_fleet_agent` properly update `runtime-mode.json` when switching from Local to Fleet.
- Ensure the background daemon lifecycle (`LocalRuntime`) is preserved and not abruptly SIGKILLed during UI window close events (`tauri::WindowEvent::CloseRequested`).

## Target Files

- `desktop-tauri/src/components/ModeOnboarding.tsx` (new component)
- `desktop-tauri/src/components/LocalSettings.tsx` (mode switcher enhancements)
- `desktop-tauri/src/App.tsx` (render onboarding when unconfigured)
- `desktop-tauri/src-tauri/src/local.rs` (runtime mode transition support)
- `desktop-tauri/tests/mode-selection.test.ts` (unit tests for onboarding & settings mode switching)
