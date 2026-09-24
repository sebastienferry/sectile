# Tasks — priority colour dot in the task edit forms

1. [ ] `web/src/lib/priority.ts`: `PRIORITY_LEVELS`, `PRIORITY_COLORS`,
       `priorityColor`.
2. [ ] `web/tests/priority.test.mjs`: the level order; the pinned colour per
       level (guards the card/form agreement); an unknown or empty value falls
       back to the muted colour.
3. [ ] `TaskCard.tsx`, `TaskFilters.tsx`, `ListView.tsx`, `PinnedBar.tsx`: read
       the colours from `priority.ts`; no visual change.
4. [ ] `web/src/components/PrioritySelect.tsx`.
5. [ ] `TaskDetailModal.tsx`, `QuickAddModal.tsx`, `CloneTaskModal.tsx`: replace
       the priority `<select>` by `PrioritySelect`, keeping each form's classes.
6. [ ] `web/tests/priority-select.browser.mjs`: the dot matches the value's
       colour, follows a change, is `aria-hidden`, and the select keeps its four
       options in order.
7. [ ] `CHANGELOG.md`: one `Changed` line under `[Unreleased]`.
8. [ ] Gate, in `web/`: `npm test`, `npx tsc --noEmit`, `npx oxlint src`,
       `npx vite build`; then the new browser suite if Playwright is available.
