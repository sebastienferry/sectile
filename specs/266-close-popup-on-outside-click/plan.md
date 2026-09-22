# #266 — Technical plan

## Stack

No new dependency. React 19 + TypeScript in `web/`, built by `tsc -b && vite build`, linted by
`oxlint`, unit-tested by `node --test tests/*.test.mjs` over plain `.ts` modules in
`web/src/lib/`, with a Playwright harness (`tests/*.browser.mjs`) for anything that needs a DOM.

## Architecture

### D1 — The dismissal is local to each backdrop, never document-level

The handler lives on the dialog's own backdrop element as React props, not as a
`document.addEventListener`. Three reasons, in order of weight:

1. **Layering (US3).** An event on the top backdrop never reaches the one underneath, so one
   click closes one layer, for free and without a z-index registry.
2. **Board safety (US6).** No new listener exists while no dialog is open, so @dnd-kit's
   pointer sequence is untouched.
3. **It is already the house idiom.** `AdminModal`, `ChangelogModal`, `ProfileModal` and
   `ProjectModal` do exactly this.

Rejected: a shared `useDismissOnOutsideClick(ref)` installing a document listener while the
dialog is open. It reintroduces the layering problem (both dialogs' listeners fire on one
click) and puts a listener in the path of every board drag.

### D2 — The gesture is a press *and* a release on the backdrop

A bare `onClick` with `e.target === e.currentTarget` is not enough: `click` fires on the
nearest common ancestor of `mousedown` and `mouseup`, which for a selection dragged out of the
dialog *is* the backdrop. The dialog would close on a gesture that never meant to close it
(US2).

So the backdrop carries two props: `onMouseDown` records whether the press landed on the
backdrop itself, and `onClick` closes only if that flag is set *and* the click target is the
backdrop. The flag is cleared on every press, so a press inside followed by a press outside
behaves as expected.

The state is a `useRef`, not `useState` — it must never cause a render, and it is read
synchronously in the click that follows the press.

### D3 — One hook, one pure predicate

- `web/src/lib/backdropDismiss.ts` — the decision, pure and DOM-free enough to unit-test:

  ```ts
  export const pressLandedOnBackdrop = (target: EventTarget | null, backdrop: EventTarget | null): boolean
  export const shouldDismiss = (pressedBackdrop: boolean, clickTarget: EventTarget | null, backdrop: EventTarget | null): boolean
  ```

  Both are total functions over `null`, since a press outside the window can leave a click with
  no usable target.

- `web/src/hooks/useBackdropDismiss.ts` — a hook returning the two props to spread on the
  backdrop element:

  ```ts
  const backdrop = useBackdropDismiss(onClose)
  // <div className="fixed …" {...backdrop}>
  ```

  It holds the `useRef` of D2 and nothing else. Spreading keeps the call sites to one line and
  makes it impossible to wire `onClick` while forgetting `onMouseDown`.

`onClose` is called as given. No dialog's close path is rewritten, which is what keeps US4
true: `TaskDetailModal` passes its `handleClose`, so the autosave-on-close still runs.

### D4 — `Escape` follows the existing per-dialog pattern

The five dialogs without `Escape` get the same `useEffect` + `window.addEventListener('keydown')`
that their eleven siblings already use, guarded by the same "am I open?" condition. No shared
hook for this: the guards differ (`isOpen` flags, nullable state objects, nested layers), and
factoring them would cost more than it saves.

For the nested cases the topmost handler wins because its dialog is the only one mounted with
that state — `TrackerSetup` and the spec reader already sit behind their own condition, and
`TaskDetailModal` already ranks its layers inside a single handler.

### D5 — Anchored menus: mechanical deduplication only

`web/src/hooks/useClickOutside.ts` holds the effect five call sites repeat verbatim: install a
`mousedown` listener while enabled, close when the press is outside every given ref.

```ts
useClickOutside(ref | ref[], onOutside, enabled = true)
```

It takes several refs because `LookupField` has two (its anchor and its portal panel). It is a
like-for-like replacement: same event, same `contains` test, same enable condition.

Not migrated, and deliberately so:
- `TaskCard` — its listener is bundled with `scroll`, `resize` and a two-level `Escape`.
- `McpSessions` — one effect installs both `mousedown` and `keydown`; splitting it to save four
  lines is churn.

### D6 — Portal panels are safe by construction

`LookupField` renders its panel through a portal at `document.body`. A click on it has a target
outside the backdrop's subtree, so neither the press check nor the click check matches, and the
dialog stays open (US1, second Given). No portal-awareness is needed anywhere.

## Data contracts

None. No API, no persisted state, no migration. The change is entirely in the browser's event
handling.

## Target files

| File | Change |
|---|---|
| `web/src/lib/backdropDismiss.ts` | new — the two predicates of D3 |
| `web/src/hooks/useBackdropDismiss.ts` | new — the hook of D3 |
| `web/src/hooks/useClickOutside.ts` | new — the shared effect of D5 |
| `web/src/components/CloneTaskModal.tsx` | backdrop dismissal |
| `web/src/components/QuickAddModal.tsx` | backdrop dismissal |
| `web/src/components/CommandPalette.tsx` | backdrop dismissal |
| `web/src/components/TaskDetailModal.tsx` | backdrop dismissal on the centered mode and the spec reader; panel mode moves to the hook |
| `web/src/components/RoadmapView.tsx` | backdrop dismissal + `Escape` on three dialogs |
| `web/src/components/SprintTimelineView.tsx` | backdrop dismissal + `Escape` |
| `web/src/components/TrackerSetup.tsx` | backdrop dismissal + `Escape` |
| `web/src/components/AdminModal.tsx` | moves to the hook |
| `web/src/components/ChangelogModal.tsx` | moves to the hook |
| `web/src/components/ProfileModal.tsx` | moves to the hook |
| `web/src/components/ProjectModal.tsx` | moves to the hook |
| `web/src/components/LookupField.tsx` | moves to `useClickOutside` |
| `web/src/components/TaskFilters.tsx` | two effects move to `useClickOutside` |
| `web/src/components/Sidebar.tsx` | moves to `useClickOutside` |
| `web/src/components/ListView.tsx` | moves to `useClickOutside` |
| `web/tests/backdropDismiss.test.mjs` | new — the predicate's truth table |
| `web/tests/outside-click.browser.mjs` | new — the gesture, in a real browser |

## Test plan

- **Unit (`node --test`)** — `backdropDismiss.test.mjs` over the pure predicates: press on the
  backdrop then click on it closes; press inside then release on the backdrop does not; press
  on the backdrop then release inside does not; a `null` target never closes.
- **Browser (Playwright, run by hand)** — `outside-click.browser.mjs` mounts a dialog wired
  with `useBackdropDismiss` over the real stylesheet and drives the four gestures of US1/US2
  plus the two-layer case of US3, asserting the close callback count each time. It follows the
  `condensed-card.browser.mjs` harness: a Vite dev server, a fixture module, Chrome headless.
  It is not in `yarn test`, which only runs `tests/*.test.mjs`.
- **Whole build** — `yarn lint` and `yarn build` (`tsc -b`) cover the fifteen edited
  components, which have no unit tests of their own.
