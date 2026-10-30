# Tasks #900 - Desktop: Select operating mode (Companion vs Agent) at first launch and allow switching in Settings

## Phase 1: Onboarding View for Mode Selection
- [ ] 1.1 Create `desktop-tauri/src/components/ModeOnboarding.tsx` with two cards: Local Companion vs Fleet Agent.
- [ ] 1.2 Integrate `ModeOnboarding` into `desktop-tauri/src/App.tsx` when `mode === "unconfigured"` or no mode is set.
- [ ] 1.3 Add visual styling and responsiveness for the onboarding screen.

## Phase 2: Mode Switching in Settings
- [ ] 2.1 Refactor the `services` section in `desktop-tauri/src/components/LocalSettings.tsx` to always show mode switching options regardless of the current mode.
- [ ] 2.2 Add "Basculer vers le mode Compagnon local" when in Fleet mode.
- [ ] 2.3 Add "Basculer vers le mode Fleet" when in Local mode.
- [ ] 2.4 Verify and handle confirmation or warning dialogs when switching modes.

## Phase 3: Runtime Transition and Daemon Preservation
- [ ] 3.1 Validate that `local.rs` cleanly transitions `runtime-mode.json` between `Local` and `Fleet`.
- [ ] 3.2 Ensure background companion daemon remains alive when UI window is closed.
- [ ] 3.3 Ensure reopening the app reconnects to the existing local daemon or fleet agent.

## Phase 4: Testing & Verification
- [ ] 4.1 Write frontend component tests in `desktop-tauri/tests/` verifying mode selection and settings transitions.
- [ ] 4.2 Run `npm run check` and `npm run test:ui` in `desktop-tauri`.
- [ ] 4.3 Run `cargo check --manifest-path src-tauri/Cargo.toml` and Rust tests.
- [ ] 4.4 Manual end-to-end verification launching the app with `npx tauri dev`.
