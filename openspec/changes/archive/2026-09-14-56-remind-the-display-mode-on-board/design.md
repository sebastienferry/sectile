# Design — Remind board display mode

## Context & Architecture
TaskFlow uses a client-side React single-page application (`web/src/`). Board cards support two presentation formats in `TaskCard.tsx`:
- Condensed: single-line format (`compact={true}`), showing key, title and actions menu.
- Expanded: multi-line format (`compact={false}`), showing key, title, priority, parent, labels, branch, PR, assignee, dates, and direct action buttons.

In `BoardView.tsx`, the card presentation mode was controlled by local ephemeral state:
```tsx
const [showSimplifiedCards, setShowSimplifiedCards] = useState(true)
```
Because this state was not persisted to browser storage, refreshing the page or switching views caused it to reset to the initial value (`true` / condensed).

## Technical Decisions

### 1. Storage Key & Values
Following TaskFlow's convention for view-level preferences (such as `taskflow_board_grouping`, `taskflow_hide_done`, `taskflow_sprint_display_mode`):
- Primary storage key: `taskflow_board_display_mode`
- Legacy fallback storage key: `taskacao_board_display_mode`
- Values: `'condensed'` | `'expanded'`
- Default value: `'condensed'`

A helper parser in `web/src/lib/boardDisplayMode.ts` will parse stored values resiliently:
- `'expanded'` or `'false'` -> `'expanded'`
- `'condensed'` or `'true'` -> `'condensed'`
- null, empty, or unrecognized -> `'condensed'` (default)

### 2. State Management via AppContext
To ensure consistency across the application and avoid state loss when `BoardView` unmounts (e.g. user navigates between Board and Backlog views):
- Expose `boardCardDisplayMode: BoardCardDisplayMode` and `toggleBoardCardDisplayMode: () => void` (or `setBoardCardDisplayMode`) in `AppContextType`.
- Initialize from `loadBoardCardDisplayMode()` on app startup.
- Update both React state and `localStorage` on change.
- In `BoardView.tsx`, derive `const isCondensed = boardCardDisplayMode === 'condensed'` and pass `compact={isCondensed}` to `TaskCard`.

### 3. Accessible Toggle Button
In `BoardView.tsx`:
- When `isCondensed` is true:
  - Icon: `<List size={14} />`
  - aria-pressed: true
  - title / aria-label: "Afficher les cartes détaillées"
- When `isCondensed` is false:
  - Icon: `<Kanban size={14} />`
  - aria-pressed: false
  - title / aria-label: "Afficher les cartes sur une ligne"

### 4. Rejected Alternatives
- *Backend database persistence in `UserSettings`*: Rejected because view-level toggle preferences (grouping mode, hide-done, sprint display mode) are consistently managed client-side in localStorage across TaskFlow without requiring database migrations or network requests.
- *Component-local persistence in `BoardView.tsx` only*: Rejected because switching between views causes unnecessary re-reads and doesn't allow centralized preference management.
