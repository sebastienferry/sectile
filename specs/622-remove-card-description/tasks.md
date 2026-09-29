# Tasks #622 - Remove the description from the card view

Ordered checklist. Each group is one commit and leaves the tree buildable.

## 1. Card (FR1, FR2)

- [x] T1.1 `web/src/components/TaskCard.tsx`: delete the
  `{task.description && <p ... line-clamp-2 ...>}` block under the title,
  with its "Ligne 2" comment.

## 2. Test (AC1-AC3)

- [x] T2.1 `web/tests/condensed-card.browser.mjs`: in the `standard` /
  `comfortable` loop, wait for the full card to render, then assert
  `getByText('Hidden description')` has count 0 instead of waiting for it.
- [x] T2.2 Keep the compact-density assertions (lines 23 and 37) as they are.

## 3. Changelog (FR5, AC5)

- [x] T3.1 `CHANGELOG.md`, `## [Unreleased]` → `### Changed`: one line saying
  board cards no longer show a description excerpt and the list view keeps its
  own. (#622)

## 4. Verification

- [x] T4.1 `npx tsc -b` and `npx oxlint` in `web/` (symlink the main
  checkout's `node_modules` if the worktree has none, then remove it).
- [x] T4.2 `node --test web/tests/*.test.mjs`.
- [x] T4.3 `condensed-card.browser.mjs`, `pr-state.browser.mjs` and
  `card-model-menu.browser.mjs` with `PLAYWRIGHT_MODULE` from the main
  checkout.
- [x] T4.4 Check (AC4), by reading the diff rather than in a running app: the list view still shows its excerpt and the
  detail panel its description.
- [x] T4.5 `npx vite build` in `web/`; restore `webui/.gitkeep` if the build
  deleted it.
