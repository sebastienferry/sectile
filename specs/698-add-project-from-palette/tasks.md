# Tasks #698 - Add a project from the command palette

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 1. The command (FR1 to FR6)

- [ ] T1.1 `desktop/src/main.js`: turn the anonymous `#add-project` handler into
  `async function openAddProject()`, body unchanged, and assign it to the
  button's `onclick`.
- [ ] T1.2 Add `{label:'Add project',run:()=>openAddProject()}` as the last row
  of `COMMANDS`.

## 2. UI test

- [ ] T2.1 `desktop/tests/palette-add-project.ui.cjs`: "tasks" hides **Add
  project**; "project" + Enter opens the "Add project" dialog listing both
  server projects, the added one disabled with "· Already added"; with the
  sidebar collapsed, clicking the command opens the dialog and leaves the
  sidebar collapsed; the `+` button still opens the same dialog.
- [ ] T2.2 `project-open-tasks.ui.cjs` passes unchanged.

## 3. Documentation

- [ ] T3.1 `desktop/README.md`: name **Add project** among the palette's
  commands.
- [ ] T3.2 `CHANGELOG.md`: the `Added` line of the plan under `[Unreleased]`.

## 4. Gates

- [ ] T4.1 `cd desktop && npx vite build`, then `node --test tests/` (sandbox
  off for the Electron suites); restore `internal/webui/.gitkeep` if a build
  removed it.
- [ ] T4.2 `git diff origin/main --stat` reviewed: only the files of the plan.
