# Plan — priority colour dot in the task edit forms

## Stack

Web front end only: React 19 + TypeScript + Tailwind under `web/`. Unit tests
are `node --test` suites under `web/tests/*.test.mjs` importing `.ts` sources
directly; component behaviour is checked by an opt-in Playwright suite
`web/tests/*.browser.mjs`. Gate in `web/`: `npm test`, `npx tsc --noEmit`,
`npx oxlint src`, `npx vite build`.

## Single source for the colours

`web/src/lib/priority.ts`, pure and free of React:

- `PRIORITY_LEVELS: readonly Priority[]` — `['urgent', 'high', 'medium', 'low']`,
  the order every priority list uses.
- `PRIORITY_COLORS: Record<Priority, string>` — the CSS variable per level,
  identical to the four existing copies.
- `priorityColor(priority)` — the colour, or `var(--text-muted)` for a value
  outside the four levels (a tracker can send one), which is what `PinnedBar`
  already falls back to.

`TaskCard.tsx`, `TaskFilters.tsx`, `ListView.tsx` and `PinnedBar.tsx` replace
their local colour map by this module; their labels still come from
`t.priority`.

## Component

`web/src/components/PrioritySelect.tsx`:

```ts
interface PrioritySelectProps {
  value: Priority
  onChange: (priority: Priority) => void
  className?: string   // the form's existing select classes
}
```

- A `relative` wrapper holding a native `<select>` (options from
  `PRIORITY_LEVELS`, labels from `useApp().t.priority`) with left padding, and
  an absolutely positioned `w-2 h-2 rounded-full` span centred vertically on
  its left edge, `pointer-events-none`, `aria-hidden="true"`, background from
  `priorityColor(value)`, with the card's `ring-1 ring-black/10`.
- The caller keeps its label element; the component does not render one, so
  the accessible name and layout stay as they are.

Rejected: a custom listbox with coloured options. It would re-implement
keyboard navigation, typeahead and mobile pickers for a decorative gain.

## Target files

- `web/src/lib/priority.ts` (new)
- `web/src/components/PrioritySelect.tsx` (new)
- `web/src/components/TaskDetailModal.tsx`, `QuickAddModal.tsx`,
  `CloneTaskModal.tsx` — use `PrioritySelect`
- `web/src/components/TaskCard.tsx`, `TaskFilters.tsx`, `ListView.tsx`,
  `PinnedBar.tsx` — read `priority.ts`
- `web/tests/priority.test.mjs` (new), `web/tests/priority-select.browser.mjs`
  (new)
- `CHANGELOG.md`
