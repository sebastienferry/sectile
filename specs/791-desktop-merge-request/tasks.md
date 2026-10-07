# Tasks #791 - Desktop: Merge request

Order matters: each step leaves the tree building and green.

## 1. Helpers (US2)

- [ ] T1.1 Move `repositoryName` and `prLabel` from `main.js` to
      `desktop/src/pullRequests.mjs`, exported; import them in `main.js` (D3).
- [ ] T1.2 Add `pullRequestMenuEntries(links)` (D3).
- Tests: `pullRequestStates.test.mjs` covers the empty and single cases, order,
  names, `PR #n` / `MR !n` / `PR / MR` labels.

## 2. Shared menu (US3)

- [ ] T2.1 Extract `toolbarMenu(button,menu,items)` from the folders menu code
      and rewire the folders menu on it, with no behaviour change (D2).
- Tests: `folder-menu.ui.cjs`, `run-folders.ui.cjs`, `attached-folders.ui.cjs`
  and `spec-folder.ui.cjs` pass unchanged.

## 3. Pull request menu (US1, US2, US3, US4)

- [ ] T3.1 Replace `#selected-pr-others` by `#selected-pr-more` and
      `#selected-pr-menu` in the toolbar markup (D1).
- [ ] T3.2 Render the chevron and build the menu entries through
      `toolbarMenu`; close it on selection change or below two pull requests
      (D3, D4).
- [ ] T3.3 Styles: chevron, single-row menu entries, side-by-side rules
      removed (D5).
- Tests: rewrite `pr-repositories.ui.cjs` per the test plan (AC-1 to AC-6),
  including the single pull request case.

## 4. Documentation

- [ ] T4.1 `desktop/README.md`: the sentence on the `+N` chevron (D6).
- [ ] T4.2 `CHANGELOG.md`: one `Changed` line under `[Unreleased]` (AC-7).
