# Tasks #900 - Desktop: Select operating mode (Companion vs Agent) at first launch and allow switching in Settings

## Phase 1: Mode Onboarding Component (`ModeOnboarding.tsx`)
- [x] 1.1 Create `desktop-tauri/src/components/ModeOnboarding.tsx` with two cards: Local Companion vs Agent Fleet.
- [x] 1.2 Implement Local Companion initialization handler invoking `api.enableLocalRuntime()`.
- [x] 1.3 Implement Agent Fleet pairing form and submission invoking `api.pairFleetAgent()` and `api.attachFleetAgent()`.
- [x] 1.4 Add styling in `desktop-tauri/src/App.css` for onboarding container, cards, hover states, and responsive layout.
- [x] 1.5 Integrate `ModeOnboarding` into `desktop-tauri/src/App.tsx` when `setup.mode === "unconfigured"` or absent.

## Phase 2: Mode Switching in Settings (`LocalSettings.tsx`)
- [x] 2.1 Refactor the `services` tab in `LocalSettings.tsx` to prominently show active mode badge.
- [x] 2.2 Add bi-directional switching controls:
  - When in Local mode: provide "Basculer vers le mode Fleet (Agent)" pairing form.
  - When in Fleet mode: provide "Basculer vers le mode Compagnon local" action.
- [x] 2.3 Implement the active executions guard: prompt for confirmation before switching modes if runs or tasks are active.
- [x] 2.4 Ensure post-switch refresh reloads projects and execution monitors cleanly.

## Phase 3: Tauri Backend Validation (`local.rs`)
- [x] 3.1 Verify `enable_local_runtime` and `pair_fleet_agent` transition logic in `desktop-tauri/src-tauri/src/local.rs`.
- [x] 3.2 Ensure background process management preserves active daemons across window close events.

## Phase 4: Automated Testing & Verification
- [x] 4.1 Write component tests in `desktop-tauri/tests/mode-onboarding.ui.test.tsx` verifying card selection, submissions, and error states.
- [x] 4.2 Write tests for settings mode switcher and confirmation guard in `desktop-tauri/tests/mode-switching.ui.test.tsx`.
- [x] 4.3 Run `npm run check` and `npm run test:ui` in `desktop-tauri`.
- [x] 4.4 Run `cargo check --manifest-path src-tauri/Cargo.toml` and Rust tests.
- [x] 4.5 Perform manual end-to-end launch verification with `npx tauri dev`.
