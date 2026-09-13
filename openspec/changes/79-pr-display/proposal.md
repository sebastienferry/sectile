# Inline desktop pull request icon

## Why
Issue #79 reports that the PR occupies a separate line below its task. The current desktop renderer still appends a text button outside the task row, despite the layout described by #57.

## What Changes
- Show an icon-only PR control after the task title/status and before archive/menu on the same sidebar row.
- Preserve accessible naming, the URL tooltip, external opening, and title truncation.
- Cover row geometry and independent activation with the existing Electron UI test framework.

## Capabilities
### New Capabilities
- `desktop-sidebar-task-row`: enforce the inline PR behavior already described in the unarchived #57 change.

## Impact
Desktop renderer, stylesheet, UI regression tests, and desktop usage documentation only. No API, data, dependency, web UI, or selected-task toolbar changes.

## Scope source
The persisted clarification for #79 identifies the desktop sidebar as the intended surface. This change follows #57's existing layout decisions; its finished tracker state does not match the current rendering behavior.
