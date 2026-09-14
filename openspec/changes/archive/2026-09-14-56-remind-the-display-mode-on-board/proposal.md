# Remind board display mode

## Why
When viewing the Kanban board, users can toggle between a condensed (single-line) presentation and an expanded (detailed) card presentation. Currently, this toggle is held in ephemeral component state (`useState(true)` in `BoardView.tsx`). As reported in GitHub issue #56 ("Condensed or expanded is always reset to condensed"), navigating away from the board or reloading the browser page resets the view back to condensed mode, causing friction for users who prefer the expanded card view.

## What Changes
- Persist the board card display mode (`condensed` vs `expanded`) in browser `localStorage` under `taskflow_board_display_mode` (with fallback to `taskacao_board_display_mode`).
- Initialize the display mode state from persisted storage, defaulting to `condensed` when no saved preference exists.
- Provide state management in `AppContext` (or resilient helper) so that navigating between views (e.g., Board to Backlog and back) preserves the user's selected mode without resetting.
- Keep the toggle button in `BoardView.tsx` synchronised with this persistent preference.

## Capabilities
### New Capabilities
- `board-display-mode`: persistent board card display mode across page reloads and view navigation.

### Modified Capabilities
None.

## Impact
Frontend only (`web/src/`). No backend database migrations, no API schema changes, no new network endpoints or dependencies.

## Out of Scope
- Modifying other views (Backlog, Roadmap, Sprint Timeline, Activities).
- Introducing additional card presentation variants beyond the existing condensed (single-line) and expanded (detailed) modes.
- Modifying card layouts, metadata display rules, or drag-and-drop mechanics.

## Decision Source
`docs/clarifications/56.md`, following issue #56.
