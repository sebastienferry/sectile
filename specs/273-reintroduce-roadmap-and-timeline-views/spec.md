# #273 — Reintroduce the Roadmap and Timeline views

## Context

In commit `0d93a04`, the Roadmap and Timeline navigation entries were dropped from the web interface alongside the experimental Triage view and secondary sidebar filters (macros and labels). 

However, macro planning across temporal horizons (Now, Next, Future) and sprint timeline scheduling remain core project planning features of Sectile:
1. **The view components are fully preserved and functional**: Both `RoadmapView.tsx` and `SprintTimelineView.tsx` exist, are active in `App.tsx`, and are typed as valid views in `ViewMode` (`'roadmap' | 'timeline'`).
2. **Access and discoverability are missing**: Users currently have no visible trigger in the primary sidebar navigation or in the Command Palette to navigate to these views.

This specification describes user-facing behaviour and acceptance criteria only. Architectural choices and implementation details are described in `plan.md`, and the ordered implementation checklist is in `tasks.md`.

## Decisions being specified

1. **Reintroduce Roadmap and Timeline triggers in the primary sidebar**:
   - Add a dedicated navigation button for Roadmap in `Sidebar.tsx` between "Board" and "Activités".
   - Add a dedicated navigation button for Timeline in `Sidebar.tsx` between "Roadmap" and "Activités".
   - Support both expanded and collapsed sidebar states with clear icon visual identity and accessible tooltips.

2. **Reintroduce Command Palette shortcuts and search keywords**:
   - Re-register the Roadmap command in `CommandPalette.tsx` with shortcut `R` and keywords for fast search (`roadmap`, `macros`, `macro`, `horizon`, `now`, `next`, `future`, `vue`, `plan`).
   - Re-register the Timeline command in `CommandPalette.tsx` with shortcut `TL` and keywords for fast search (`timeline`, `sprint`, `sprints`, `planning`, `vue`, `horizons`, `duree`, `chronologie`).

3. **Localized navigation strings**:
   - Avoid hardcoding navigation labels in component JSX. Define `roadmap` and `timeline` within `web/src/locales/translations.ts` in both French and English dictionaries.

4. **Preserve intentional exclusions (Settled in Clarification)**:
   - The previously removed Triage view remains excluded.
   - Secondary sidebar filters (macro list and tag list filters) remain excluded to maintain a streamlined sidebar design.

## User stories

### US1 — Access Roadmap view from the primary sidebar (P1)

As a product lead or developer using Sectile, I want to navigate to the Roadmap view directly from the sidebar, so that I can organise and inspect macro initiatives across the Now, Next, and Future horizons.

- **Given** I am on any view in Sectile (e.g., Board, Backlog, or Activities)
- **When** I inspect the "Vues" section of the primary sidebar navigation
- **Then** a "Roadmap" button is visible between "Board" and "Activités", displaying a map icon.
- **Given** the sidebar is expanded
- **When** I view the Roadmap navigation button
- **Then** the label "Roadmap" is displayed next to the icon, and a descriptive tooltip is available.
- **Given** the sidebar is collapsed
- **When** I view the Roadmap navigation button
- **Then** only the map icon is displayed, centered, with an accessible tooltip.
- **Given** I click the Roadmap navigation button
- **When** the navigation executes
- **Then** `activeView` transitions to `'roadmap'`, the `RoadmapView` component is mounted, and the Roadmap sidebar item displays the active accent styling (`bg-[var(--accent-light)] accent-text font-bold`).

### US2 — Access Sprint Timeline view from the primary sidebar (P1)

As a team member tracking sprint delivery, I want to navigate to the Timeline view directly from the sidebar, so that I can see the chronological schedule and progress of past, active, and upcoming sprints.

- **Given** I am on any view in Sectile
- **When** I inspect the "Vues" section of the primary sidebar navigation
- **Then** a "Timeline" button is visible directly following "Roadmap" and before "Activités", displaying a clock icon.
- **Given** the sidebar is expanded
- **When** I view the Timeline navigation button
- **Then** the label "Timeline" is displayed next to the icon, and a descriptive tooltip is available.
- **Given** the sidebar is collapsed
- **When** I view the Timeline navigation button
- **Then** only the clock icon is displayed, centered, with an accessible tooltip.
- **Given** I click the Timeline navigation button
- **When** the navigation executes
- **Then** `activeView` transitions to `'timeline'`, the `SprintTimelineView` component is mounted, and the Timeline sidebar item displays the active accent styling.

### US3 — Fast keyboard navigation via Command Palette (P1)

As a power user, I want to jump to the Roadmap or Timeline views using the Command Palette, so that I can switch contexts without using the mouse.

- **Given** I open the Command Palette (`Cmd+K` or `Ctrl+K`)
- **When** I search for "roadmap", "macro", "horizon", or press shortcut `R`
- **Then** the command palette displays the item "Vue Roadmap (Macros : NOW / NEXT / FUTURE)" with a map icon and shortcut badge.
- **When** I select the Roadmap command
- **Then** the Command Palette closes, and `activeView` switches immediately to `'roadmap'`.
- **Given** I open the Command Palette
- **When** I search for "timeline", "sprint", "planning", or press shortcut `TL`
- **Then** the command palette displays the item "Vue Timeline Sprints" with a clock icon and shortcut badge.
- **When** I select the Timeline command
- **Then** the Command Palette closes, and `activeView` switches immediately to `'timeline'`.

### US4 — Fully localized navigation entries (P2)

As a bilingual user, I want navigation labels and tooltips to match my chosen language, so that the user interface remains consistent.

- **Given** my profile language is set to French (`fr`)
- **When** I inspect the sidebar and view triggers
- **Then** the navigation labels and tooltips use the French dictionary values defined in `translations.fr.nav`.
- **Given** my profile language is set to English (`en`)
- **When** I inspect the sidebar and view triggers
- **Then** the navigation labels and tooltips use the English dictionary values defined in `translations.en.nav`.

### US5 — Non-regression on excluded views and filters (P2)

As an engineer maintaining Sectile, I want excluded legacy views and filters to remain absent, so that the scope of #273 does not reintroduce unwanted UI bloat.

- **Given** the application navigation in `Sidebar.tsx` and `CommandPalette.tsx`
- **When** I search for or inspect views
- **Then** the Triage view is not present.
- **When** I inspect the sidebar filter sections
- **Then** secondary macro lists and tag filter lists remain absent.
