## Why
Selected desktop tasks currently have an accent border while a more specific compact-row rule hides their background highlight. Ticket #65 requests selection through background alone.

## What Changes
- Highlight the selected desktop execution button with the existing selection background.
- Remove the selection-specific border color while retaining stable dimensions and keyboard focus indication.
- Preserve task selection, execution history, status, and task actions.

## Impact
Desktop sidebar styling only. Web views and execution behavior are out of scope.
