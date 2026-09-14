# Condensed board cards

## Why
GitHub #39 requests “Ticket ID, title”. The existing Compact density reduces spacing but leaves full card metadata and action rows visible. A condensed card will let users scan more tickets without losing access to ticket operations.

## What Changes
- Use the existing Compact density to show the public ticket reference, a single-line title and an always-visible Actions menu.
- Keep detailed cards in Standard and Comfortable densities.
- Move hidden shortcuts into the condensed card menu and preserve their availability rules, tracker navigation, detail opening and drag-and-drop.
- Apply the presentation to workflow, operational/tracker and unassigned board columns.

## Capabilities
### New Capabilities
- `condensed-board-cards`: minimal board presentation with equivalent access to existing operations.

### Modified Capabilities
None.

## Impact
Frontend card rendering and menu composition; existing density settings and task contracts remain intact. No backend changes, migrations, new preferences or dependencies.

## Out of Scope
Other views, column sizing rules, sorting, filtering rules, workflow transitions, global zoom, project-specific preferences and a separate board density switch.

## Decision Source
`docs/clarifications/39.md`, also published on issue #39. Its explicitly adopted autonomous recommendations are the settled scope. No open product requirements remain.
