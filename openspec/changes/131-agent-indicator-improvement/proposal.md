# Reduce the remote run indicator to a single actionable icon

## Why
A task carrying a remote execution renders two verbose text badges ("Remote execution", "Queued") with a separate "Stop" button. On condensed board cards and list rows this consumes horizontal space, and a cancelled run disappears without ever telling the user whether the cancellation took effect.

## What Changes
- Show a single icon for a task's remote runs instead of per-state text chips, with the state precedence running > queued > recently cancelled.
- Expose the state, the skill names and the run count through the tooltip and the accessible label rather than visible text.
- Let the user cancel from the icon itself: hovering or focusing a cancellable icon turns it into a stop control.
- Keep a cancelled run visible as a neutral icon for a short window after it ends, so the outcome of a cancellation is observable.

## Impact
Only the web UI presentation changes: `web/src/components/RemoteRunBadge.tsx` and a new pure state-derivation helper under `web/src/lib/`. The cancellation endpoint, the activity model and the activities view are unchanged.
