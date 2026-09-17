# Desktop next-step placement and dialog close affordance

## Why
The next-step action sits alone in the task footer, far from the execution controls the user is already looking at, while the project dialog closes with a bare `×` text glyph that matches no other icon button in the desktop.

## What Changes
- Move the next-step action and its retry companion out of the task footer into the execution toolbar, immediately next to Stop execution.
- Keep the next-step status text in the footer, where it stays readable and keeps its live-region role.
- Replace the project dialog's `×` glyph with the repository's cross icon, rendered in the existing red used by destructive controls.

## Impact
Desktop presentation only: `desktop/src/main.js`, `desktop/src/style.css` and the affected UI tests. No server API, workflow, persisted state or web application change. The task closing dialog and the web application are out of scope.
