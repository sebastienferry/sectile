# Unify the workflow / status grouping toggle

## Why
Issue #290 reports that the control switching between the status grouping and the agentic
workflow grouping is not the same in the Board view and in the Backlog view. Each view
renders its own copy of the buttons, with a different order, different labels, different icon
sizes and different tooltips, so the same action looks like two unrelated controls and the
duplicated markup drifts further apart with every change.

## What Changes
- Add `web/src/lib/boardGrouping.ts`, a pure module exposing the ordered grouping options
  (`workflow` first, then `status`) with their label keys, icon names and tooltips.
- Add `web/src/components/BoardGroupingToggle.tsx`, the single presentational control reading
  those options and the shared `boardGrouping` / `setBoardGrouping` from `AppContext`, with a
  `size` prop (`sm` | `md`) as its only variation.
- Replace the inline toggle markup in `web/src/components/BoardView.tsx` (size `md`) and
  `web/src/components/ListView.tsx` (size `sm`) with the shared component.
- Add the `Workflow` / `Statuts` labels and tooltips to `web/src/locales/translations.ts`.
- Add `web/tests/boardGrouping.test.mjs` covering the option order and descriptors.

## Capabilities
### New Capabilities
- `board-grouping-toggle`: one shared control for switching between the status grouping and
  the agentic workflow grouping, identical across every view that offers it.

### Modified Capabilities
None.

## Impact
Confined to the web UI: two components, one new component, one new lib module, the
translations file and one test file. No API, no persistence, no Go code. The stored
`boardGrouping` value and its semantics are unchanged.

## Out of Scope
Grouping semantics, workflow stages, tracker column configuration, sidebar behaviour, and
adding the toggle to views that do not have it today.

## Decision Source
`docs/clarifications/290.md`, round 1 (unattended, no open product question).
