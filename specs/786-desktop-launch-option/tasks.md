# Tasks #786 - Desktop: Launch dialog with an AI engine

Order matters: each step leaves the tree buildable. Tests go with the step
they cover.

## 1. Engine helper

- [x] T1.1 Add `launchEngineChange(view,taskId,chosenId)` to
      `desktop/src/engines.mjs` (FR2).
- Tests (`desktop/tests/engines.test.mjs`): same as stored engine, same as
  project default with no stored engine, another engine, back to the project
  default from a switched task, unknown or empty choice (US2.3-US2.5).

## 2. Launch dialog

- [x] T2.1 Extract `openLaunchDialog` from the `#rerun` handler, rename the
      button, title, controls and submit (US1, FR1, FR4).
- [x] T2.2 Add the `AI engine` select behind `taskEnginesAvailable()`, store
      the engine before the launch, stop on a refusal, refresh the open
      ticket table's engines (US2).
- Tests: rename in `console.ui.cjs`, `conversation.ui.cjs`,
  `discussion-header.ui.cjs`, `free-console.ui.cjs`, `skill-mode.ui.cjs`
  (US1). `task-engine.ui.cjs`: select contents and default, a changed engine
  stored before the launch, a refusal launches nothing, no select without the
  capability (US2).

## 3. Row entry

- [x] T3.1 Add `Launch…` to the row `…` menu (US3).
- Tests (`task-engine.ui.cjs`): the entry opens the dialog with the next skill
  preselected, and with **Discussion (no skill)** when there is none.

## 4. Documentation

- [x] T4.1 `CHANGELOG.md` `[Unreleased]` line (US4.1).
- [x] T4.2 `docs/CAPABILITIES.md` table row (US4.2).

## 5. Checks

- [x] T5.1 `git grep Relaunch -- desktop/src desktop/tests` is empty.
- [x] T5.2 Desktop unit tests, `npx vite build`, the UI suites listed above.
