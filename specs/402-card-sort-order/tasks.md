# #402: Implementation checklist

References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 0. Start

- [ ] T0.1 `git fetch origin`, merge `origin/main` into `feat/402` if it moved,
      and grep `git log --oneline origin/main` for `#402` and for
      `boardSort` / `BoardSortSelect` before writing code.

## 1. Sort module (FR2-FR6)

- [ ] T1.1 Add `web/src/lib/boardSort.ts`: `BoardSortField`, `BoardSort`,
      `BOARD_SORT_STORAGE_KEY`, `DEFAULT_BOARD_SORT`, `BOARD_SORT_OPTIONS`,
      `pickBoardSortField`, `sortTasks`, `loadBoardSort`, `saveBoardSort`.
- [ ] T1.2 Add `web/tests/boardSort.test.mjs` (see the test plan below).

## 2. Shared preference and selector (US1.1, US1.2, US1.6, US3.4, US4, FR1, FR5, FR6, FR9)

- [ ] T2.1 `board.sort` strings in `translations.ts`: interface, French,
      English.
- [ ] T2.2 `boardSort` / `setBoardSort` in `AppContext.tsx`, loaded and saved
      through `boardSort.ts`, exposed on the context.
- [ ] T2.3 Add `web/src/components/BoardSortSelect.tsx` (`sm` / `md`), native
      criterion select plus direction button, with the test ids of `plan.md`.

## 3. Board (US1.3-US1.7, US2, US3, US7.1, FR7)

- [ ] T3.1 Replace `byPriorityDesc` with `sortTasks(list, boardSort)` in
      `tasksForColumn`, `unassignedTasks` and `workflowColumnTasks`; delete
      `PRIORITY_RANK` and `byPriorityDesc` once nothing else uses them.
- [ ] T3.2 Put `<BoardSortSelect size="md" />` after the Workflow / Status
      toggle.

## 4. Backlog (US5, US7.2, FR8)

- [ ] T4.1 Replace `sortField` / `sortAsc` with a nullable `headerSort`; derive
      `selectorSortedTasks` (grouped view, grouped `batchRows`) and the flat
      table's list (header override or selector).
- [ ] T4.2 Rewrite `handleSort` per `plan.md`; clear `headerSort` when
      `boardSort` changes.
- [ ] T4.3 Replace the "Priorité" toolbar button with
      `<BoardSortSelect size="sm" />`; drop the now unused imports.

## 5. Changelog (FR10)

- [ ] T5.1 One line under `[Unreleased]` / `### Added` in `CHANGELOG.md`, with
      `(#402)`.

## 6. Verification

- [ ] T6.1 Add `web/tests/board-sort.browser.mjs` (see the test plan below).
- [ ] T6.2 `npm test`, `npm run lint`, `npm run build` in `web/`.
- [ ] T6.3 Run `board-sort.browser.mjs`, `board-views.browser.mjs`,
      `backlog-batch.browser.mjs` and `condensed-card.browser.mjs`; adjust a
      fixture only where its expected order relied on arrival order, and say so.
- [ ] T6.4 By hand in the web app: Epic mode on a project with epics, the
      direction toggle, the Backlog header override and its reset.
- [ ] T6.5 Re-read the diff against `spec.md`.

## Test plan

`boardSort.test.mjs` builds plain task objects; every case asserts the order of
keys returned by `sortTasks`.

| Case | Covers |
| --- | --- |
| Priority desc: urgent, high, medium, low; equal priority by key numeric (`#9` before `#42`) | US1.3, FR4 |
| Priority asc: low first; equal priority still key ascending | US3.1, FR3 |
| Key asc numeric (`#9`, `#42`, `#402`; `PROJ-9` before `PROJ-12`); Key desc | US1.4, US3.2 |
| Updated desc uses `trackerUpdatedAt`, falls back to `updatedAt`; no date last | US1.5 |
| Updated asc: oldest first, no date still last | US3.3, D12 |
| Epic desc: example of US2.2 (B urgent, A high, A low) | US2.1, US2.2 |
| Epic: equal top priority ordered by parent key numeric (`M-2` before `M-10`) | US2.3 |
| Epic: no-parent tickets last, by priority; `''` and `undefined` both mean no parent | US2.4 |
| Epic asc: groups reversed, inside-group order and no-parent position unchanged | US2.5, FR3 |
| Same tasks in two input orders give the same output | US6.1 |
| Two tasks with the same key and priority, different ids, in both input orders | US6.2 |
| `sortTasks` does not mutate its input | plan |
| `pickBoardSortField` natural directions for the four fields | US1.6, FR5 |
| `loadBoardSort`: absent, not JSON, `null`, unknown field, non-boolean `asc`, throwing storage, no storage give the default; a valid value round-trips through `saveBoardSort` | US4.3, US4.4, FR6 |

`board-sort.browser.mjs` runs the real App against an in-page fake API, as
`board-views.browser.mjs` does, with tasks of mixed priorities, parents and
dates in one column.

| Case | Covers |
| --- | --- |
| Board toolbar shows the selector next to Workflow / Status; default order is by priority | US1.1, D11 |
| Select Epic: column order matches US2.2 | US2.2 |
| Direction button on Priority: low first; its label names the direction | US3.1, US3.4 |
| Open the Backlog: selector shows the Board's choice, no "Priorité" button, grouped rows follow it | US1.2, US4.1, US5.1 |
| Change the sort in the Backlog, back to the Board: the Board follows | US4.2 |
| Reload: sort kept | US4.3 |
| Flat table: click the Key header, rows by key; click again, reversed; Board unaffected | US5.3, US5.4 |
| Flat table header-sorted, change the selector: table follows the selector | US5.5 |
