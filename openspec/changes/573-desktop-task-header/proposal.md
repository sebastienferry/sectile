# Desktop task header and split execution views

## Why
The task header mixes identity, execution history, navigation, and actions in a wrapping group. Console and Changes currently replace one another, making it impossible to inspect output beside a diff.

## What Changes
- Arrange task identity and execution history in a first row, with the worktree path and actions in a second row.
- Make Console and Changes independent toggles with a resizable side-by-side view when both are shown.
- Preserve existing task actions, console attachment, diff loading, and footer status.

## Impact
Desktop renderer, styles, and UI tests. No agent or server contract changes.
