# Plan #900 - Desktop: Select operating mode (Companion vs Agent) at first launch and allow switching in Settings

## Stack

- Frontend: React 19, TypeScript, Vite in `desktop-tauri/src/`
- Tauri backend: Rust, Tauri 2 in `desktop-tauri/src-tauri/src/`
- Go services: `cmd/companion`, `cmd/agent`, `cmd/server` (bundled binaries in `src-tauri/binaries/`)

## Architecture & Data Contracts

### 1. Runtime State & Detection
In `desktop-tauri/src/lib/local.ts` and `desktop-tauri/src-tauri/src/local.rs`:
- `LocalSetup` interface:
  ```typescript
  export interface LocalSetup {
    enabled: boolean;
    mode?: "unconfigured" | "local" | "fleet";
    fleetServer?: string;
    backendError?: string;
    updatePreparing?: boolean;
    ready?: boolean;
    signedIn?: boolean;
    agentReady?: boolean;
    apiSetupAvailable?: boolean;
    dataDirectory?: string;
    profile?: { email: string; displayName: string; role?: string };
    trackers?: { id: string; name: string; provider: string; site: string; scope: string }[];
    credentials?: { credentials: { tracker: string; siteUrl?: string; account?: string; kind?: string; disconnected?: boolean }[] };
    projects?: { id: string; name: string; trackers?: { trackerId: string }[] }[];
  }
  ```
- Unconfigured condition in `App.tsx`:
  When `setup && (!setup.mode || setup.mode === "unconfigured")`, instead of falling back to `{ view: "settings" }`, render `<ModeOnboarding onConfigured={...} />`.

### 2. Components Design

#### A. `ModeOnboarding.tsx`
- Location: `desktop-tauri/src/components/ModeOnboarding.tsx`
- Props: `onConfigured: () => void`
- State:
  - `selectedMode`: `"local" | "fleet" | null`
  - `fleetUrl`, `fleetCode`
  - `pending`: boolean, `error`: string
- Layout:
  - Header: Sectile logo mark + Title "Bienvenue dans Sectile Desktop" + Subtitle "Choisissez votre mode d'utilisation pour commencer".
  - Two cards:
    1. **Mode Compagnon Local**:
       - Icon: `laptop` / `folder`
       - Description: "100% autonome sur votre poste avec base de données SQLite locale embarquée. Idéal pour un usage individuel et sans serveur d'équipe."
       - Button: "Démarrer en mode Compagnon" -> triggers `api.enableLocalRuntime()`.
    2. **Mode Agent Fleet**:
       - Icon: `cloud` / `link`
       - Description: "Connectez votre poste à une instance centrale Sectile. Les tickets et l'orchestration sont partagés avec votre équipe, l'exécution reste locale."
       - Button / Form: Opens URL and Pairing Code inputs -> triggers `api.pairFleetAgent(url, code)` or `api.attachFleetAgent()`.

#### B. Mode Switching in `LocalSettings.tsx`
- Location: `desktop-tauri/src/components/LocalSettings.tsx`
- Refactor the `services` section:
  - Prominent badge displaying the active mode (`Mode Compagnon Local` or `Mode Agent Fleet`).
  - Section "Changer de mode de fonctionnement" :
    - If current mode is `Local`: Provide button or disclosure "Basculer vers le mode Fleet", revealing the Fleet pairing form.
    - If current mode is `Fleet`: Provide button "Basculer vers le mode Compagnon local".
  - Guard modal: If `activeExecutions > 0` (via `useExecutionMonitor()` or `activeTrackerSyncs`), display an accessible confirmation dialog before invoking `enableLocalRuntime` or `pairFleetAgent`.

### 3. Tauri Rust Lifecycle (`src-tauri/src/local.rs` & `lib.rs`)
- Verify that `enable_local_runtime` properly stops existing fleet child connections if switching from Fleet to Local.
- Verify `pair_fleet_agent` and `attach_fleet_agent` cleanly shut down the local companion if switching from Local to Fleet.
- Confirm window close behavior leaves background processes alive, and `resume` on setup reconnects cleanly.

## Target Files

- `desktop-tauri/src/components/ModeOnboarding.tsx`: New onboarding component.
- `desktop-tauri/src/components/LocalSettings.tsx`: Mode switching controls and confirmation guard.
- `desktop-tauri/src/App.tsx`: App-level routing for unconfigured onboarding state.
- `desktop-tauri/src/App.css`: Onboarding card layout and responsive styles.
- `desktop-tauri/tests/mode-onboarding.test.tsx`: Unit tests for onboarding choices and error states.
- `desktop-tauri/tests/mode-switching.test.tsx`: Unit tests for settings mode switcher and active runs guard.
