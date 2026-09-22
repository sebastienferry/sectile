# Design

## Decision: one presentational component, one pure options module
The option list (order, ids, icons, label keys, tooltip keys) lives in
`web/src/lib/boardGrouping.ts` so it can be unit-tested with `node --test`, the convention the
other `web/src/lib` modules already follow (`boardDisplayMode.ts`, `workflow.ts`). The
component `BoardGroupingToggle.tsx` only maps that list to buttons and reads the shared state
from `AppContext`. A component test harness does not exist in this repo, so the testable part
is deliberately pushed into the pure module.

## Decision: the Board toolbar is the reference rendering
Workflow first, then Statuts. The agentic pipeline is the primary grouping of the product and
the Board is the primary view; aligning the Backlog on the Board avoids re-teaching the
user a second reading order.

## Decision: `size` is the only prop
The Backlog toolbar is denser than the Board toolbar, which is a legitimate difference. It is
expressed as `size="sm"` (icon 12, `px-2 py-1`) versus `size="md"` (icon 15, `px-2 py-1.5`).
Everything else — order, labels, icons, inactive icon colours, tooltips, active state styling,
the `hidden md:inline` label — is shared and not configurable, so the two views cannot drift
again.

## Rejected alternative: keep both toggles and merely align the markup by hand
Cheaper now, but it leaves two copies of the same control and reproduces exactly the defect
reported. Rejected.

## Rejected alternative: move the toggle into a global toolbar rendered once
It would guarantee consistency, but the Board and the Backlog toolbars carry different
neighbouring controls and different layouts; hoisting the toggle would restructure both
headers, well beyond the reported bug. Rejected.

## Localisation
The labels come from `translations.ts` under `common.grouping` (`workflow`, `status`, and the
two tooltips) rather than from the existing `list.columns.status`, whose meaning is a table
column header, not a grouping mode.
