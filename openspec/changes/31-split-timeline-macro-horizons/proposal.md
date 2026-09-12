## Why

Currently, the roadmap feature in TaskFlow combines two distinct functional views—Sprint Timeline planning (`SprintTimelineView`) and Macro/Horizons management (`RoadmapView`)—under a single `roadmap` view mode with an internal sub-toggle switch (`roadmapMode: 'sprints' | 'macros'`). This causes visual clutter, unnecessary tab switches inside the view header, and poor ergonomics when users want to switch directly to sprint timeline planning. 

Separating these views into two top-level `ViewMode` values (`'roadmap'` and `'timeline'`) provides a cleaner separation of concerns, dedicated navigation entries, distinct command palette shortcuts, and simplified component state.

## What Changes

- Extend `ViewMode` union type in `web/src/types/index.ts` to include `'timeline'`.
- Register `'timeline'` mode in `web/src/context/AppContext.tsx` (`VIEW_MODES`).
- Render `<SprintTimelineView />` directly in `web/src/App.tsx` when `activeView === 'timeline'`.
- Remove the internal `roadmapMode` state toggle (`'sprints' | 'macros'`) and sub-toggle header controls from `web/src/components/RoadmapView.tsx`, making `RoadmapView` focus exclusively on Macro Horizons (NOW / NEXT / LATER / Unclassified / Hidden).
- Add dedicated sidebar navigation entries and command palette shortcuts for Timeline (`'timeline'`) and Roadmap (`'roadmap'`) in `Sidebar.tsx`, `Header.tsx`, and `CommandPalette.tsx`.

## Capabilities

### New Capabilities
- `timeline-view`: Top-level view mode (`'timeline'`) dedicated to Sprint Timeline visualization and drag-and-drop sprint planning.

### Modified Capabilities
- `roadmap-view`: Refactored top-level view mode (`'roadmap'`) focusing exclusively on Macro Horizons management (NOW / NEXT / LATER / UNCLASSIFIED) without embedded sprint timeline toggles.

## Impact

- **Frontend Types**: `web/src/types/index.ts` (`ViewMode` union).
- **Frontend Context**: `web/src/context/AppContext.tsx` (`VIEW_MODES` list and navigation handlers).
- **Frontend Views**: `web/src/App.tsx`, `web/src/components/RoadmapView.tsx`, `web/src/components/SprintTimelineView.tsx`.
- **Frontend Navigation & Commands**: `web/src/components/Sidebar.tsx`, `web/src/components/Header.tsx`, `web/src/components/CommandPalette.tsx`.
