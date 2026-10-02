# Tasks #703 - Keep settings controls clear of the macOS overlay scrollbar

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 1. Test first (acceptance criteria)

- [x] T1.1 In `desktop/tests/workstation-settings.ui.cjs`, test "the skill
  settings save through the agent and reset to their defaults": before the
  first reset click, scroll the reset button's `section[id^="settings-panel-"]`
  ancestor to its bottom.
- [x] T1.2 For both reset buttons, assert the gap between the button's right
  edge and the scroller's content right edge (`left+clientLeft+clientWidth`)
  is at least 15 px, with a message giving the measured gap.
- [x] T1.3 For both reset buttons, assert `document.elementFromPoint` at the
  button's rightmost pixel, vertical centre, is the button or inside it.
- [x] T1.4 Build the desktop (`npx vite build` in `desktop/`), restore
  `internal/webui/dist/.gitkeep` if removed, run the suite unsandboxed and confirm
  T1.2 fails on the current CSS (gap near 0 px).

## 2. The gutter (FR1 to FR7)

- [x] T2.1 `desktop/src/style.css:411`: add `padding-right:16px` to
  `.settings-content.stretch>section:not(#settings-panel-Logs)`, with an
  English comment on the overlay scrollbar and on `scrollbar-gutter` not
  covering it.
- [x] T2.2 Leave `.settings-content` (6 px, overridden), the project view's
  24 px and `#settings-panel-Logs` untouched.
- [x] T2.3 Rerun `workstation-settings.ui.cjs`: T1.2 and T1.3 pass, and on
  macOS with overlay scrollbars the plain reset clicks no longer time out.

## 3. Changelog (FR8)

- [x] T3.1 One `Fixed` line under `## [Unreleased]` in `CHANGELOG.md`, as
  worded in `plan.md` §4.

## 4. Verification

- [x] T4.1 Run the settings UI suites unsandboxed and serially:
  `workstation-settings`, `settings-version`, `sidebar-shortcut`.
- [ ] T4.2 Manual check on macOS ("Show scroll bars: Automatic", trackpad):
  scroll Execution defaults, click the right half of each reset icon; open AI
  engines and Profile, check no control sits under the scrollbar; open project
  settings, check nothing moved.
- [ ] T4.3 Manual check at a window width of 720 px or less: the gutter is
  still there.
- [x] T4.4 `git diff origin/main --stat` lists only the three target files
  plus this specification and the clarification.

## Test plan

| Requirement | Covered by |
| --- | --- |
| FR1, FR2 | T1.2, T1.3 (every OS), T4.2 (macOS) |
| FR3 | T2.1 review: outer padding unchanged |
| FR4 | T2.2, T4.2 |
| FR5 | T2.2, existing Logs tests in `workstation-settings.ui.cjs` |
| FR6 | T4.3 |
| FR7 | T4.1 suites unchanged |
| FR8 | T3.1 |
