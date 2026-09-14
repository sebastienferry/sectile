# Desktop terminal next step

## Why
After a task execution ends, users must find the next workflow skill elsewhere. The console needs a contextual action beneath the TTY.

## What Changes
- Show the selected task's current workflow stage and next skill in the desktop footer.
- Launch one next step through the existing server skill dispatch.
- Handle active executions, missing metadata, launch failures and terminal workflow stages explicitly.

## Scope
Desktop renderer, behavior tests and desktop usage documentation. No automatic merge, workflow transitions, new API or web UI changes.

## Clarification
Ticket #66 has no description or comments. The desktop owns the task TTY, so this targets its footer. Server task stage is authoritative, including when viewing historical executions. Reviewed tasks await human merge; finished tasks cannot advance. Existing project skill definitions and local mapping govern availability. No unresolved questions or external dependencies.
