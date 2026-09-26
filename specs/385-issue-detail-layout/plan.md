# Implementation plan

## Stack and scope

React, TypeScript and Tailwind, using the existing shared Story renderer in `web/src/components/TaskDetailModal.tsx`. No data contracts or dependencies change.

## Design

Keep the title as the first full-width child. Move the complete description/editor/rewrite block into the first cell of a responsive grid; group metadata and labels in its second cell. Use a wider content column at large widths, `min-width: 0` containment, and two metadata fields per row where space allows. Collapse to one outer column on narrow screens. Place existing pull request controls after the grid. Preserve all handlers, state and conditional visibility.

Avoid separate panel/modal implementations and CSS visual reordering, which would create divergent behavior or keyboard order. A new architecture or ADR is not necessary for this local presentation change.

## Files and validation

- `web/src/components/TaskDetailModal.tsx`: markup grouping, placement and responsive classes only.
- `CHANGELOG.md`: reader-facing note.
- Browser regression under `web/tests`: real component and styles with mocked app context, following existing isolated browser fixtures. Verify wide/narrow panel and modal geometry plus representative editing and pull request actions.
- Run web build, lint and unit tests; inspect resulting diff and browser render before publishing.
