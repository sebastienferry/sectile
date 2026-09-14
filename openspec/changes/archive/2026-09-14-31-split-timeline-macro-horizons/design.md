## Context

The TaskFlow web frontend current combines sprint timeline management (`SprintTimelineView`) and epic/macro horizon management (`RoadmapView`) into a single composite view component (`RoadmapView.tsx`) gated by a local component state `roadmapMode: 'sprints' | 'macros'`.

This architecture introduces unnecessary complexity:
1. Navigating to the sprint timeline requires opening the Roadmap view and then clicking a sub-toggle button.
2. `RoadmapView.tsx` conditionally imports and renders `SprintTimelineView`, mixing sprint timeline state and header tabs with macro horizon filtering and CRUD modal operations.
3. Keyboard shortcuts and command palette items treat both views under a single `roadmap` identifier.

Ticket #31 requires decoupling these two views into distinct top-level `ViewMode` states: `'timeline'` and `'roadmap'`.

## Goals / Non-Goals

**Goals:**
- Add `'timeline'` as a top-level value in the `ViewMode` union type in `web/src/types/index.ts`.
- Update `AppContext.tsx` to handle `'timeline'` in `VIEW_MODES` and view navigation state.
- Render `<SprintTimelineView />` directly in `App.tsx` when `activeView === 'timeline'`.
- Clean up `RoadmapView.tsx` by removing `roadmapMode` state, removing internal sub-toggle tabs ("Timeline" vs "Macros & Horizons"), and removing the `SprintTimelineView` import.
- Update `Sidebar.tsx`, `Header.tsx`, and `CommandPalette.tsx` to display separate navigation items and command palette shortcuts for "Timeline" and "Roadmap".

**Non-Goals:**
- Modifying backend models or API endpoints.
- Modifying sprint calculation logic in `SprintTimelineView.tsx` or macro horizon placement logic in `lib/roadmap.ts`.
- Altering macro CRUD operations, horizon migration modals, or epic drag-and-drop features.

## Decisions

1. **Top-Level `ViewMode` Union Extension**:
   - `export type ViewMode = 'board' | 'list' | 'triage' | 'roadmap' | 'timeline' | 'activities' | 'sync' | 'digest' | 'skills' | 'team'`
   - `'timeline'` is placed after `'roadmap'` in `VIEW_MODES` list in `AppContext.tsx`.

2. **Decoupling `SprintTimelineView` from `RoadmapView`**:
   - `<SprintTimelineView />` is imported directly into `web/src/App.tsx`.
   - `RoadmapView.tsx` will exclusively render the Macro Horizons view (NOW, NEXT, LATER, Non classés, Masqués).

3. **Navigation & Command Palette Integration**:
   - `Sidebar.tsx`: Add a Timeline entry with icon `Clock` or `CalendarDays` and route to `timeline`.
   - `CommandPalette.tsx`: Add `switch_timeline` command ("⏱️ Vue Timeline Sprints") mapping to `setActiveView('timeline')`.

## Target Files

- `web/src/types/index.ts`
- `web/src/context/AppContext.tsx`
- `web/src/App.tsx`
- `web/src/components/RoadmapView.tsx`
- `web/src/components/Sidebar.tsx`
- `web/src/components/Header.tsx`
- `web/src/components/CommandPalette.tsx`

## Risks / Trade-offs

- **Risk**: Existing bookmarks or state relying on `activeView === 'roadmap'` expecting sprint timeline might open the macro horizons view.
  - *Mitigation*: Ensure sidebar clearly displays both "Timeline" and "Roadmap" items so users easily find the dedicated sprint timeline.
