# Clean up task line in sidebar

## Why
Issue #57 requests reorganizing the task line in the desktop application sidebar into a clean inline sequence: `ID (clickage) Title Status Icon PR icon ...`. Previously, pull request indicators were rendered as block buttons underneath each row in the project group container, creating vertical gaps and misaligned lists. Consolidating the elements onto a single inline row improves scanning efficiency, visual rhythm, and clarity.

## What Changes
- Reorganize each `.local-task` row in `desktop/src/main.js` so that elements flow sequentially on a single line:
  1. Task ID button (`context`, `.task-number`), clicking opens the task in Sectile.
  2. Execution selection button (`.run`), containing the task title (`strong`) and status icon (`.status`).
  3. PR icon button (when `pullRequests.get(run.taskId)` exists), displaying a Git Pull Request SVG icon inline, with full title and accessible name (`Open PR ... for ...`), opening the PR externally on click.
  4. Archive button (`.task-archive`), visible on hover/focus.
  5. Menu button (`.task-menu`), visible on hover/focus.
- Adjust `desktop/src/style.css` to style the inline PR icon button consistently with the line typography, without block margins or wrapping.
- Ensure all accessible names, click actions, and existing UI tests in `desktop/tests/console.ui.cjs` continue to pass.

## Capabilities
### New Capabilities
- `desktop-sidebar-task-row`: inline task row composition in the desktop sidebar with integrated PR indicator and action controls.

### Modified Capabilities
None.

## Impact
Changes are confined to `desktop/src/main.js` and `desktop/src/style.css`. No backend APIs, database schemas, or web app components are affected.

## Out of Scope
Changes to the remote web interface sidebar, server APIs, and backend execution orchestration.

## Decision Source
Autonomous clarification on issue #57 following the pre-existing desktop architecture and Playwright UI test contracts.
