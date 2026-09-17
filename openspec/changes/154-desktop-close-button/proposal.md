# Distinguish the desktop stop controls by icon

## Why
The desktop toolbar button that ends an execution and the header button that stops the local agent both render a square. They look like a media stop and like each other, so the user cannot tell the two actions apart at a glance.

## What Changes
- Replace the execution stop glyph with a cross, so the control reads as closing the running execution.
- Replace the agent stop glyph with a disconnect mark, so the header control no longer mirrors the execution control.
- Keep both actions, their labels, tooltips, accessible names, colours, and sizes unchanged.

## Impact
Only the desktop icon glyphs change. No behaviour, API, configuration, data, or web surface change is required. The web stop control, the cancellation flow, and the archive confirmation are out of scope.
