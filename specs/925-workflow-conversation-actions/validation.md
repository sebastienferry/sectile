# Implementation validation — #925

## Preserved state

Branch: `feat/925`. Accepted clarification and specification artifacts are preserved. Integrated `origin/feat/tauri-monorepo` at `fda2c8dd`, including #922 compact header styling. No backend, workflow resolution, launch lifecycle, draft ownership or stop operation changed. No PR exists for this branch as confirmed by successful GitHub lookup. No workflow transition was submitted; the task remains specified.

## Changes

- `desktop-tauri/src/components/ProjectPrompt.tsx`: shares header next-step rendering with eligible read-only history, adds an adjacent action slot, and moves active Stop to the strip while retaining Archive and Start over.
- `desktop-tauri/src/App.css`: groups Workflow and its action, supports wrapping long labels, and relocates Stop colors.
- `desktop-tauri/tests/prompt.ui.test.tsx`: covers both action locations, workflow/PR recovery eligibility, task-wide activity, archived history, keyboard Enter/Space, draft retention, free busy/idle Stop, disabled states, refresh, callback and failure retry.
- Root and Tauri README/changelog: explains the user-visible placement and lifecycle rules.

## Actual final check results

- `npm --prefix desktop-tauri run check`: exit 0 (`tsc --noEmit`).
- `npm --prefix desktop-tauri run test:ui`: `Test Files 32 passed (32)`, `Tests 300 passed (300)`.
- `npm --prefix desktop-tauri run build`: exit 0, `built in 149ms`; existing bundle-size warning above 500 kB remains.
- `npm --prefix desktop-tauri test`: `tests 106`, `pass 105`, `fail 1`; unchanged `tests/local-update.test.ts` crashes inside Node 26.7.0 with `Assertion failed: (env_->execution_async_id()) == (0)`. Its tests create loopback servers. This implementation does not change that test or its source. An attempted Node 24 PATH selection still resolved Node 26 through the installed npm; it did not establish independent runtime evidence. Do not claim a passing unit suite or a proven baseline failure.
- `npm --prefix desktop-tauri run test:rust`: `41 passed; 7 failed`; each failure reports `PermissionDenied, message: "Operation not permitted"` at loopback listener setup in unchanged Rust sources.
- `git diff --check`: clean.

## Permission blockers and next action

Visual inspection could not start: Vite reported `listen EPERM: operation not permitted 127.0.0.1:1422`; PinchTab reported `open /Users/sferry/.pinchtab/server.log: operation not permitted`. No alternate route was attempted after these denials. An optional process inspection also reported `zsh:1: operation not permitted: ps` and was abandoned.

Per AGENTS.md, obtain explicit authorization before retrying denied intentions. Then rerun unit and Rust checks with loopback permissions, inspect narrow/wide layout, long labels, focus and disabled states, and complete attachment/folder preservation verification (T009/T011). Resolve any actual failures, finish checklist review, fetch and verify the base, push the preserved branch, create/reuse a draft PR targeting `feat/tauri-monorepo`, verify its head and URL, and submit implemented with that URL. Never advance to reviewed as part of this recovery.


## Authorized recovery — 2026-11-18

The owner explicitly authorized retrying validation with required permissions. A new code-issue run was started; the completed launcher run was not reused. All accepted artifacts and production code were preserved.

- Unit suite with loopback access: `tests 114`, `pass 114`, `fail 0` on installed Node 26.7.0. The prior native crash no longer reproduces with those permissions; no test or runtime workaround was needed.
- Rust suite with loopback access: `48 passed; 0 failed`; main and doc-test targets also pass. Existing objc macro cfg warnings remain.
- Final TypeScript: `tsc --noEmit`, exit 0.
- Final UI suite after added preservation assertions: `Test Files 32 passed (32)`, `Tests 300 passed (300)`.
- Final production build: exit 0, `built in 159ms`; existing bundle-size warning remains.
- Focused prompt UI: 35 passed. Added Tab from Workflow to contextual action, Enter/Space activation, unchanged draft text with a file reference, unchanged folder tray, source conversation context, and folder availability after read-only transition.
- PinchTab fixture inspected at 800 and 1280 px wide with 600 px height. At 800 px the Workflow/action group wraps together below activity and execution; at 1280 px it stays on the same row. Both controls remain reachable and aligned. Contextual next-step is 32 by 32 px; keyboard Tab reaches it with a clear accent outline. Disconnected next-step controls are disabled and retain their positions. The longest normal label, Workflow Implemented, fits with the adjacent next-step. Active task Stop is 32 by 32 px beside Workflow and absent from the header. No redundant Stop appears in read-only history.
- Fixture JSON and transport mocks restored byte-for-byte after visual checks; temporary frame and screenshots removed. No fixture changes are delivered.
- Re-read production diff against acceptance scenarios: no backend, workflow resolver, launch callback, draft owner, archive/start-over or stop lifecycle changes. No ADR needed for a placement/action-slot change.
- Remote base `origin/feat/tauri-monorepo` was fetched and is an ancestor of this branch. Successful GitHub lookup confirmed no existing open feat/925 PR before creation.

PR publication and final stage report follow these checks. This invocation must stop at implemented, never reviewed.
