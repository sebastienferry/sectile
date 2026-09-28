# Tasks #525 - Lighter execution history drop-down in the desktop toolbar

Ordered checklist. Merge `origin/main` first (the branch is already pushed:
merge, do not rebase).

## 1. Ghost style (US1, US2, FR1-FR7)

- [x] T1.1 `desktop/src/style.css`: `#toolbar #execution-history` block with
  `appearance:none`, no border, transparent background colour, gradient
  chevron, right padding, ellipsis (plan decisions 1-3).
- [x] T1.2 Option colours (plan decision 4).
- [x] T1.3 Hover and `:focus-visible` rules (plan decision 5).

## 2. Tests

- [x] T2.1 `desktop/tests/execution-history.ui.cjs`: rest, hover, focus,
  other select unchanged, selection and visibility, light theme (plan Tests).
- [x] T2.2 `npx vite build`, then run the new test and the six UI tests that
  touch the control, plus `npm test` in `desktop/`.

## 3. Changelog

- [x] T3.1 `CHANGELOG.md`: one `Changed` line under `## [Unreleased]` (FR8).
