# Tasks #900 - Desktop: Select operating mode (Companion vs Agent) at first launch and allow switching in Settings

## Phase 1: Mode Onboarding Component (`ModeOnboarding.tsx`)
- [ ] 1.1 Create `desktop-tauri/src/components/ModeOnboarding.tsx` with two cards: Local Companion vs Agent Fleet.
- [ ] 1.2 Implement Local Companion initialization handler invoking `api.enableLocalRuntime()`.
- [ ] 1.3 Implement Agent Fleet pairing form and submission invoking `api.pairFleetAgent()` and `api.attachFleetAgent()`.
- [ ] 1.4 Add styling in `desktop-tauri/src/App.css` for onboarding container, cards, hover states, and responsive layout.
- [ ] 1.5 Integrate `ModeOnboarding` into `desktop-tauri/src/App.tsx` when `setup.mode === "unconfigured"` or absent.

## Phase 2: Mode Switching in Settings (`LocalSettings.tsx`)
- [ ] 2.1 Refactor the `services` tab in `LocalSettings.tsx` to prominently show active mode badge.
- [ ] 2.2 Add bi-directional switching controls:
  - When in Local mode: provide "Basculer vers le mode Fleet (Agent)" pairing form.
  - When in Fleet mode: provide "Basculer vers le mode Compagnon local" action.
- [ ] 2.3 Implement the active executions guard: prompt for confirmation before switching modes if runs or tasks are active.
- [ ] 2.4 Ensure post-switch refresh reloads projects and execution monitors cleanly.

## Phase 3: Tauri Backend Validation (`local.rs`)
- [ ] 3.1 Verify `enable_local_runtime` and `pair_fleet_agent` transition logic in `desktop-tauri/src-tauri/src/local.rs`.
- [ ] 3.2 Ensure background process management preserves active daemons across window close events.

## Phase 4: Automated Testing & Verification
- [ ] 4.1 Write component tests in `desktop-tauri/tests/mode-onboarding.test.tsx` verifying card selection, submissions, and error states.
- [ ] 4.2 Write tests for settings mode switcher and confirmation guard in `desktop-tauri/tests/mode-switching.test.tsx`.
- [ ] 4.3 Run `npm run check` and `npm run test:ui` in `desktop-tauri`.
- [ ] 4.4 Run `cargo check --manifest-path src-tauri/Cargo.toml` and Rust tests.
- [ ] 4.5 Perform manual end-to-end launch verification with `npx tauri dev`.
