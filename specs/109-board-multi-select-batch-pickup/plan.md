# #109 — Technical plan

## Stack

No new dependency. React 19 + TypeScript in `web/`, built by `tsc -b && vite build`, linted by
`oxlint`, unit-tested by `node --test tests/*.test.mjs` over plain `.ts` modules in `web/src/lib/`.
No backend change: the `pickup_issues` skill, its catalog entry and `POST /tasks/{id}/run-skill`
already exist.

## Architecture

### D1 — The rules live in one pure module, `web/src/lib/boardSelection.ts`

The board has no unit tests, and its column logic is inline in `BoardView`. Every rule that
decides the behaviour of the selection is pulled out into DOM-free functions that `node --test`
can cover:

```ts
export const SELECTABLE_STAGES: readonly WorkflowStage[]              // ['new', 'clarified']
export const isSelectableStage = (stage: WorkflowStage): boolean
export const isSelectionClick = (e: { ctrlKey: boolean; metaKey: boolean }): boolean
export const toggleSelected = (selected: ReadonlySet<string>, id: string): Set<string>
// Keeps the ids that are still selectable on screen. Returns the same Set when nothing drops
// out, so that pruning on every render never re-renders the board for nothing.
export const pruneSelection = (selected: ReadonlySet<string>, selectableIds: readonly string[]): ReadonlySet<string>
// The ids of the selection in board order (the order of `boardOrder`), for the batch prompt.
export const orderSelection = (selected: ReadonlySet<string>, boardOrder: readonly string[]): string[]
export const shouldEscapeClearSelection = (ctx: EscapeContext): boolean
```

`isSelectableStage` is applied to `resolveTaskStage(task, currentProject)`, the resolver the
workflow columns already use. So the rule is the same in the three groupings (US3).

### D2 — `BoardView` owns the selection, and derives the board order from its own columns

The state is a `Set<string>` of task ids in `BoardView`, like `ListView`'s `selectedTaskIds`. It
is not stored in `AppContext`: nothing else reads it, and the clarification rules it local to the
board.

The board order (US5) and "on screen" (US7) must match what is rendered, including the collapsed
finished column, the hidden tracker columns and the "Non classé" column. So the column lists are
computed once per render, into `workflowColumnTasks` and `statusColumnTasks` (one list per column id). `boardCards` concatenates them in the order the columns are rendered:

- workflow grouping: `workflowColumns` in order, minus the collapsed `finished` column when
  `hideDone` is on;
- status grouping: `effectiveStatusColumns` in order, minus the collapsed closed columns when
  `hideDone` is on, then `unassignedTasks` when it is not empty.

Rendering reads the same lists, so the order can never drift from what is shown. `boardOrder` is
`boardCards` mapped to ids and deduplicated (two tracker columns can claim one status), and `selectableIds` is the subset whose stage is selectable.

The render prunes the selection with `pruneSelection(selected, selectableIds)` and stores the
result when it differs. Because `pruneSelection` returns the same Set when nothing changes, this
settles in one pass. A card that drops out is not remembered, so it does not come back selected
(US7). The render also keeps the project id the selection was made in, and clears the selection
when `currentProject?.id` differs (US6).

*Changed during implementation:* the plan first used two `useEffect`s. `oxlint` flags a
synchronous `setState` in an effect (`react(set-state-in-effect)`), and an effect would paint the
stale selection for one frame. React's "adjust state while rendering" pattern avoids both.

### D3 — `TaskCard` receives the selection as props, and keeps its gestures

`TaskCard` gains four optional props. Its other callers, if any appear, keep today's behaviour:

```ts
selectable?: boolean       // the card may show a checkbox and react to Ctrl/Cmd+click
selected?: boolean
selectionActive?: boolean  // at least one card is selected: show the checkbox without hover
onToggleSelect?: () => void
```

- **Checkbox.** A `<button type="button" role="checkbox" aria-checked>`. It sits after the priority
  dot on line 1 of a full card, and before the run badge of a condensed card. *Changed during
  implementation:* the plan first put it on the left, before the key. The box keeps its room while
  hidden, so every selectable card would have shown an empty gap in front of its key. On the
  condensed card it also must not break the `badge → model → menu` row that
  `skillLaunchModel.test.mjs` pins. It is always in the DOM when `selectable`,
  hidden with `opacity-0` and revealed by `group-hover`, `group-focus-within` and
  `selectionActive`, so that `Tab` reaches it and focusing it reveals it (US1). Its `onClick`
  stops propagation so that it never opens the detail. A button already answers `Space` and
  `Enter`.
- **Ctrl/Cmd+click.** The card root's `onClick`, and the condensed title button's `onClick` which
  stops propagation, both check `selectable && isSelectionClick(e)`. If true, they call
  `onToggleSelect()` and return before `setSelectedTask`. Otherwise they do what they do today.
  So a non-selectable card treats Ctrl/Cmd+click as a plain click (US2).
- **Highlight.** A selected card gets `ring-2 ring-[var(--accent-color)]`, the accent treatment
  the board already uses for a drop target, and `border-[var(--accent-color)]`. The running and
  queued border colours keep precedence on the border, and the ring shows the selection.
- **Drag.** `draggable`, `onDragStart` and `handleDragStartInternal` are untouched: the payload is
  still the dragged card's id only (US8). The drop handlers in `BoardView` are unchanged.

### D4 — The selection bar copies ListView's floating bar and the batch button of the other views

The bar is rendered by `BoardView` when the selection is not empty. It is `fixed bottom-6
left-1/2 -translate-x-1/2 z-50`, the container classes of `ListView`'s bulk bar. It holds:

1. the counter pill and "sélectionnée"/"sélectionnées", as in `ListView`;
2. the batch button, with the label, the `Sparkles` icon, the purple classes and the title of
   `TriageView`'s button, disabled while `launching`;
3. "Désélectionner tout", which clears the selection.

UI strings stay in French, as they are in the three other views.

### D5 — `startBatchPickup` reports whether the launch was accepted

Today `startBatchPickup` returns `Promise<void>`, so a caller cannot tell an accepted launch from a
failed one. `runSkill` already returns `null` on failure, after showing the error toast. The
signature becomes `Promise<boolean>`: `false` for an empty batch, for the cross-project refusal
and for a failed `runSkill`, and `true` otherwise. The three existing callers ignore the result,
so nothing changes for them. The board clears the selection only on `true` (US4).

The board passes `orderSelection(selected, boardOrder)`. `startBatchPickup` filters `tasks` by
`taskIds.includes`, which keeps the order of `tasks`, not the order of `taskIds`. So it is changed
to map over `taskIds` instead. That way the prompt lists the ids in the order the caller gave, and
the other views, which already pass their row order, get the order they meant to send (US5).

### D6 — `Escape` clears the selection only when nothing else takes the key

`AppContext` ranks `Escape` on `window` for the surfaces it owns (command palette, quick add, task
detail, activity, admin, profile, search). Other dialogs catch it on `document` in the capture
phase (`useEscapeKey`) or on `window`, and the card menu handles it on `document` and calls
`preventDefault()`.

`BoardView` installs a `window` `keydown` listener only while the selection is not empty.
`shouldEscapeClearSelection` returns true only when all of these hold:

- the key is `Escape` and the event is not `defaultPrevented` (a card menu has already spent it);
- none of `isCommandPaletteOpen`, `isQuickAddOpen`, `selectedTask`, `selectedActivity`,
  `isAdminOpen`, `isProfileOpen` is set, and `searchQuery` is empty. These are the states the
  `AppContext` handler would act on, so the two handlers never act on the same press, whatever
  order they run in;
- the focus is not in an `input`, `textarea`, `select` or terminal, which is the same test as
  `AppContext`;
- no `[aria-modal="true"]` element is in the document.

A dialog caught by `useEscapeKey` stops the event before it reaches `window`, so it never gets
this far.

Rejected: a `document` capture listener that stops the event. It would take the key away from
the card menu and from every dialog opened over the board.

## Target files

| File | Change |
|------|--------|
| `web/src/lib/boardSelection.ts` | New: the pure rules of D1 and D6. |
| `web/tests/boardSelection.test.mjs` | New: unit tests of every function of D1 and D6. |
| `web/src/components/BoardView.tsx` | Selection state, per-column task lists and `boardCards`, pruning and project reset while rendering, `Escape` listener, selection bar, props passed to `TaskCard`. |
| `web/src/components/TaskCard.tsx` | Four optional props, checkbox, Ctrl/Cmd+click, highlight. |
| `web/src/context/AppContext.tsx` | `startBatchPickup` returns `Promise<boolean>` and keeps the caller's order. |
| `openspec/specs/batch-issue-pickup/spec.md` | Say that the board selects `new`/`clarified` cards and passes them in board order. |
| `CHANGELOG.md` | One `Added` line under `[Unreleased]`. |

## Data contracts

None change. The launch is still `POST /api/tasks/{firstId}/run-skill` with
`{ skillId: 'pickup_issues', prompt: '/pickup-issues <id> <id> …' }`. Only the order of the ids
in the prompt is now guaranteed.

## Test plan

- **Unit** (`yarn test`): `boardSelection.test.mjs` covers the selectable stages, the modifier
  test, toggling, pruning (identity kept when nothing drops out, drops the ids that are off screen
  or no longer selectable), ordering (board order whatever the insertion order, ids off the board
  ignored) and each clause of `shouldEscapeClearSelection`.
- **Build and lint**: `yarn lint` and `yarn build` (`tsc -b`) cover `BoardView`, `TaskCard` and
  `AppContext`, which have no unit tests.
- **Manual, in the dev server**: the replay checklist in `tasks.md`.
