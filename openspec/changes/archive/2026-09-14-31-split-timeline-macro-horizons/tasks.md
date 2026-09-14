## 1. Type & Context Definitions

- [x] 1.1 Add `'timeline'` to `ViewMode` type union in `web/src/types/index.ts`.
- [x] 1.2 Update `VIEW_MODES` array and navigation helpers in `web/src/context/AppContext.tsx` to support `'timeline'`.

## 2. Component Decoupling & View Routing

- [x] 2.1 Import `<SprintTimelineView />` in `web/src/App.tsx` and render it when `activeView === 'timeline'`.
- [x] 2.2 Refactor `web/src/components/RoadmapView.tsx`: remove `roadmapMode` state, remove header sub-toggle tabs, and remove unused `SprintTimelineView` import.

## 3. Sidebar, Header & Command Palette Updates

- [x] 3.1 Add Timeline navigation item (with `Clock` / `CalendarDays` icon) to `web/src/components/Sidebar.tsx`.
- [x] 3.2 Update view title rendering and view switcher in `web/src/components/Header.tsx` to distinguish Timeline and Roadmap views.
- [x] 3.3 Add `switch_timeline` command entry ("⏱️ Vue Timeline Sprints") to `web/src/components/CommandPalette.tsx`.

## 4. Verification & Testing

- [x] 4.1 Run frontend build (`npm run build` or `cd web && npm run build` or `vite build`) to ensure type safety.
- [x] 4.2 Run existing tests to verify zero regression across frontend components.
