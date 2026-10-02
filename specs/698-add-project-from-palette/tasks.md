# Tasks #698 - Add a project from the command palette

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 1. The command (FR1 to FR6)

- [x] T1.1 `desktop/src/main.js`: turn the anonymous `#add-project` handler into
  `async function openAddProject()`, body unchanged, and assign it to the
  button's `onclick`.
- [x] T1.2 Add `{label:'Add project',run:()=>openAddProject()}` as the last row
  of `COMMANDS`.

## 2. UI test

- [x] T2.1 `desktop/tests/palette-add-project.ui.cjs`: "tasks" hides **Add
  project**; "project" + Enter opens the "Add project" dialog listing both
  server projects, the added one disabled with "· Already added"; with the
  sidebar collapsed, clicking the command opens the dialog and leaves the
  sidebar collapsed; the `+` button still opens the same dialog.
- [x] T2.2 `project-open-tasks.ui.cjs` passes unchanged.

## 3. Documentation

- [x] T3.1 `desktop/README.md`: name **Add project** among the palette's
  commands.
- [x] T3.2 `CHANGELOG.md`: the `Added` line of the plan under `[Unreleased]`.

## 4. Gates

- [x] T4.1 `cd desktop && npx vite build`, then `node --test tests/` (sandbox
  off for the Electron suites); restore `internal/webui/.gitkeep` if a build
  removed it.
- [x] T4.2 `git diff origin/main --stat` reviewed: only the files of the plan.

## Notes from the implementation

- T4.1: `npm test` passes 170/170 and `npm run test:ui` 69/70. The failure,
  `workstation-settings.ui.cjs` "the skill settings save through the agent and
  reset to their defaults" (a settings tabpanel intercepts the click on a reset
  button), also fails with the `main.js` of `HEAD` before this change, so it
  predates it.
