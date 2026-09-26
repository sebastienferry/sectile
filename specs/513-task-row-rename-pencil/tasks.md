# Tasks #513 - Pencil rename instead of the task row "…" menu

Ordered checklist. Each group leaves the desktop buildable. Before starting,
`git fetch origin` and integrate `origin/main` (merge, the branch is pushed).

## 1. Inline rename in the row (FR-2 to FR-9) - `feat(desktop)`

- [x] T1.1 `desktop/src/main.js`: `displayedName(run)` helper, used by the
  row title and the new pencil label.
- [x] T1.2 `renaming` state (`{key,draft,fresh}`), and in `render()` the
  `.task-rename` input in place of the `.run` button for the edited row;
  focus, selection on a fresh edition, caret at the end on a rebuild; clear
  `renaming` when its row is no longer rendered.
- [x] T1.3 `finishRename(run,commit)`: trimmed, non-empty, changed value
  saved into `localTasks`; Enter and blur commit, Escape cancels and stops
  propagation; rebuild guard against blur during `replaceChildren()`; focus
  back on the row's pencil (`data-task-key` on `.local-task`).
- [x] T1.4 Pencil button `.task-rename-button` after the archive button, on
  every row, `Rename <name>` label and tooltip, line SVG icon; it does not
  select the run.

## 2. Remove the "…" menu (FR-1, FR-10) - `feat(desktop)`

- [x] T2.1 Delete the `.task-menu` button and `taskMenu(run)`; keep
  `requestArchive` and `archiveTask`.
- [x] T2.2 `desktop/src/style.css`: pencil visibility shares the
  `.task-archive` rules (hover, focus-within, `hover:none`); `.task-rename`
  field styled with existing tokens only; delete the `.task-menu` rules.

## 3. Tests - `test(desktop)`

- [x] T3.1 `console.ui.cjs`: rename through the pencil and Enter; drop the
  dialog's Detach check (the toolbar check stays).
- [x] T3.2 `task-header.ui.cjs`: rename through the pencil; tracker title
  change and restart checks unchanged.
- [x] T3.3 `pr-display.ui.cjs`: `.task-menu` → `.task-rename-button`.
- [x] T3.4 Check that the `Relaunch` clicks of `console.ui.cjs`,
  `free-console.ui.cjs` and `skill-mode.ui.cjs` still resolve to the toolbar.
- [x] T3.5 New `desktop/tests/task-rename.ui.cjs` covering the ten cases of
  the plan (no "…", pencil visibility, Enter, blur, Escape with the ticket
  pane open, blank value, edition surviving a render, focus return, 120
  characters, free console and macro run).
- [x] T3.6 `grep -rn "task-menu\|taskMenu\|Actions for #" desktop/` returns
  nothing but project and ticket menus.
- [x] T3.7 `cd desktop && npx vite build && npm test && npm run test:ui`
  green (restore `webui/.gitkeep` if the build deleted it).

## 4. Documentation (FR-11) - `docs`

- [x] T4.1 `CHANGELOG.md`: one `Changed` line under `[Unreleased]` (#513).
- [x] T4.2 `desktop/README.md`: rewrite lines 53 and 521, which describe the
  removed task "…" menu.
