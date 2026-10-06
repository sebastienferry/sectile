# Tasks #778 - Desktop: the Claude settings tab draws the Claude logo

Order matters: each step leaves the tree buildable. Tests go with the step
they cover.

## 1. Icon helper

- [x] T1.1 `settingsCategoryIcon(document, category)` in
      `desktop/src/claude-mark.mjs`.
- Tests (`desktop/tests/claude-mark.test.mjs`): a `mark: 'claude'` category
  gets the Claude mark (FR1, US1.4); another category gets the stroked outline
  with its icon content (FR2).

## 2. Settings navigation

- [x] T2.1 `mark: 'claude'` on both `Sandbox` categories, shield removed (FR4).
- [x] T2.2 The five tab-building sites go through `settingsCategoryIcon` (FR2).

## 3. Colour

- [x] T3.1 `--claude-brand` and the `.claude-mark` colour rule in
      `desktop/src/style.css` (FR3).
- Tests (`desktop/tests/conversation-mode.ui.cjs`): the workstation and project
  "Claude settings" tabs draw `svg.claude-mark`, orange, idle and selected
  (US1.1, US1.2, US2.1); the conversation mode row mark is orange (US2.2).

## 4. Documentation

- [x] T4.1 `CHANGELOG.md` `Changed` line (FR5).

## 5. Checks

- [x] T5.1 Desktop unit tests, `npx vite build`, the desktop UI suites touching
      settings.
