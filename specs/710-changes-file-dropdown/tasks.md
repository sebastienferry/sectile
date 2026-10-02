# Tasks #710 - Pick the changed file from a drop-down

Ordered checklist. Each group leaves the tree buildable and the tests green.

## 1. Tests first

- [ ] T1.1 `desktop/tests/git-diff.ui.cjs`: pick `binary.dat` with
  `selectOption` on `select[aria-label="Changed file"]`; after Refresh, assert
  the select's value is still `binary.dat`; assert it lists two options in the
  agent's order; after the error and the empty refreshes, assert it is hidden
  and has no option.
- [ ] T1.2 `desktop/tests/markdown-view.ui.cjs`: `pick` selects the option by
  its label through the select.
- [ ] T1.3 Assert the diff starts at the panel's content left edge (no list
  beside it) in the split view.

## 2. The drop-down (FR1 to FR6)

- [ ] T2.1 `gitDiff.js`: markup, `render`, `showFile`, `clear`, `change`
  listener, as the plan says.

## 3. Layout (FR5, FR7)

- [ ] T3.1 `style.css`: remove the grid and list rules, style the select.

## 4. Changelog and checks (FR8)

- [ ] T4.1 `CHANGELOG.md`: `Changed` line under `[Unreleased]`.
- [ ] T4.2 `npx vite build` in `desktop/`, restore
  `internal/webui/dist/.gitkeep` if removed, run both UI suites unsandboxed
  and the desktop unit tests.
