# Tasks — priority colour dot in the task edit forms

1. [x] `web/src/lib/priority.ts`: `PRIORITY_LEVELS`, `PRIORITY_COLORS`,
       `priorityColor`.
2. [x] `web/tests/priority.test.mjs`: the level order; the pinned colour per
       level (guards the card/form agreement); an unknown or empty value falls
       back to the muted colour.
3. [x] `TaskCard.tsx`, `TaskFilters.tsx`, `ListView.tsx`, `PinnedBar.tsx`: read
       the colours from `priority.ts`; no visual change.
4. [x] `web/src/components/PrioritySelect.tsx`.
5. [x] `TaskDetailModal.tsx`, `QuickAddModal.tsx`, `CloneTaskModal.tsx`: replace
       the priority `<select>` by `PrioritySelect`, keeping each form's classes.
6. [x] `web/tests/priority-select.browser.mjs`: the dot matches the value's
       colour, follows a change, is `aria-hidden`, and the select keeps its four
       options in order.
7. [x] `CHANGELOG.md`: one `Changed` line under `[Unreleased]`.
8. [x] Gate, in `web/`: `npm test`, `npx tsc --noEmit`, `npx oxlint src`,
       `npx vite build`; then the new browser suite if Playwright is available.
