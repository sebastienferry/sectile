# Plan #808 - Web UI: Roadmap, hide a macro

Spec: [spec.md](spec.md)

## Stack

Web client only: React + TypeScript (`web/`), Tailwind classes, browser tests
on Playwright through Vite (`web/tests/*.browser.mjs`). No server, API, schema,
tracker or Desktop change.

## Current state

- `web/src/components/RoadmapView.tsx`
  - `persist(key, { horizon })` (around line 1094) saves through
    `saveMacroMeta`, merges the stored macro into `macroMeta`, then
    `setTab(patch.horizon)` and `setSelectedKey(saved.key)`.
  - `renderMacroRow` (around line 1460) maps `['now', 'next', 'later',
    'hidden']` to buttons styled from `HORIZON_META`, with the suggestion
    (`!row.horizon && row.suggested === h`) tinted and suffixed ` ?`, and the
    `strings.suggestedHorizon` / `strings.classifyAs` tooltips.
  - `renderCondensedRow` (around line 1563) maps `CONDENSED_HORIZONS` the same
    way, with an `aria-label`.
  - The panel header (around line 2045) renders `selected.key` then, when
    `selected.horizon` is set, a read-only `<span>` badge.
  - `condensedHere` (line 818) already renders the Hidden tab unfolded.
- `web/src/lib/roadmapDisplayMode.ts`: `CONDENSED_HORIZONS` and its comment,
  which already promises the panel offers HIDDEN.
- `web/src/lib/roadmap.ts`: `HORIZON_META` (labels `NOW`, `NEXT`, `LATER`,
  `HIDDEN`, colours, `bg`, `border`).

## Design

1. Replace the read-only badge in the panel header with a horizon selector:
   a `role="group"` container holding four `<button type="button">`, one per
   horizon in the order NOW, NEXT, LATER, HIDDEN.
2. Style each button exactly like the unfolded row's (same classes, same
   `color` / `background` / `borderColor` rules from `HORIZON_META`, same
   ` ?` suffix and tooltip choice), computed from `selected` instead of `row`.
   `aria-pressed={active}` on each button.
3. `onClick={() => persist(selected.key, { horizon: h })}`. No
   `stopPropagation` is needed: the panel header has no row click handler.
4. To avoid a third copy of the button markup, extract a small local renderer
   used by the unfolded row and the panel, for example
   `renderHorizonButton(row, h, onClick)` inside `RoadmapView`, or a
   `HorizonButtons` component taking `row`, `horizons` and `onPick`. The
   condensed row keeps its own compact markup (it uses short labels and an
   `aria-label`); it may adopt the helper only if the rendered DOM stays
   identical. The extraction must not change the unfolded row's DOM.
5. Keep the selector inside the existing `flex items-center gap-2 mb-1.5
   flex-wrap` header line, between the key and the `ml-auto` action group, so
   it wraps on a narrow panel instead of overflowing.

No new string: labels come from `HORIZON_META[h].label`, tooltips from
`strings.classifyAs` and `strings.suggestedHorizon`. If an accessible name for
the group is wanted, reuse an existing string; adding one is not required by
the spec.

### Rejected alternatives

- Add HIDDEN to the condensed row (option B): reverses the #345 decision to
  keep hiding out of one-click reach in a dense list. Rejected by the owner.
- A per-row "Hide" action (option C): adds a control to every row for a rare
  gesture. Rejected by the owner.
- Restore the exact pre-`3263a5a2` markup (segmented control with muted
  inactive buttons and no suggestion): it reads differently from the rows and
  carries a hard-coded French tooltip. The settled scope asks for the row
  styling.

## Data contracts

Unchanged. `saveMacroMeta(projectId, key, { horizon })` as used by the rows.

## Target files

| File | Change |
| --- | --- |
| `web/src/components/RoadmapView.tsx` | panel header selector; optional shared button renderer |
| `web/tests/roadmap-hide-from-panel.browser.mjs` (new) or `web/tests/roadmap-view.browser.mjs` | regression test |
| `CHANGELOG.md` | `Fixed` line under `## [Unreleased]` |

`web/src/lib/roadmapDisplayMode.ts` is read, not changed.

## Test plan

New browser test, built on the `roadmap-view.browser.mjs` harness (real
`RoadmapView`, mocked `useApp`), with a `saveMacroMeta` mock that stores the
patch and returns the updated macro, as `roadmap-grouping.browser.mjs` does:

1. Seed `localStorage` `sectile_roadmap_display_mode` with the condensed
   value (read the exact value from `roadmapDisplayMode.ts`), macros in NOW
   and LATER.
2. Select a NOW macro; assert the condensed row has no `HIDDEN` button and the
   panel header has four buttons, `NOW` pressed.
3. Click the panel's `HIDDEN`: assert one `saveMacroMeta` call with
   `{ horizon: 'hidden' }`, the Hidden tab active, the macro listed there and
   still selected, and the panel's `HIDDEN` pressed.
4. Click the panel's `NOW`: assert the save, the NOW tab active and the macro
   listed and selected there.
5. An unclassified macro with a suggested horizon shows the suggestion
   highlighted with ` ?` in the panel.
6. No page error.

Scope locators to the panel (`aside`) to tell its buttons from the rows'.
Check that the `activeTab()` and `tabButton()` helpers of
`roadmap-view.browser.mjs` still find the tab buttons first; they match on
`^NOW \d+$` and on DOM order, and the panel comes after the list.

Also run: `node --test web/tests/roadmap*.test.mjs`, the existing
`web/tests/roadmap-*.browser.mjs` suites, `tsc` and `oxlint` on `web/`.
