# Autonomous Desktop Tickets pickup

## Why
The Tickets menu's full-chain Pickup action currently follows configured execution mode and can open an interactive session, unlike the other full-chain pickup controls.

## What changes
- Explicitly request autonomous mode for the dedicated Tickets Pickup action.
- Preserve configured-mode behavior for other Tickets skills and generic launch controls.
- Add UI regression coverage and a user-facing changelog entry.

## Scope
Desktop renderer and its UI tests only. Server mode resolution, provider commands, and run lifecycle remain unchanged.
