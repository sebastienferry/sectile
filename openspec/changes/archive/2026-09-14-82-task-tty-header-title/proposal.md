# Task title in the desktop TTY header

## Why
The selected console header displays the task key and execution skill but omits the task title already available to the sidebar. Users must look elsewhere to identify the work, especially when browsing historical executions.

## What Changes
- Display task identity, effective task title, and selected execution skill in the desktop TTY header.
- Respect local task names before tracker titles, matching the sidebar convention.
- Refresh the title when metadata arrives or changes, when selection changes, and after local rename.
- Keep useful identity/skill fallbacks and make long titles readable without displacing toolbar controls.

## Scope
Desktop header presentation, focused desktop UI regression coverage, and desktop usage documentation. This implements the clarification recorded on [ticket #82](https://github.com/sebastienferry/taskflow/issues/82).

## Non-goals
No restored web terminal, external terminal window title changes, task editing feature, new API, database migration, or terminal lifecycle change.

## Impact
Affected capability: `desktop-task-header`. Expected implementation files are `desktop/src/main.js`, `desktop/src/style.css`, focused tests under `desktop/tests/`, and `desktop/README.md`. No unresolved product requirements or external dependencies remain.
