## Purpose

Decouples the sprint timeline visualization (`SprintTimelineView`) and macro horizon management (`RoadmapView`) into two independent top-level view modes (`timeline` and `roadmap`) with dedicated navigation, sidebar entries, and command palette shortcuts.

## ADDED Requirements

### Requirement: Top-level Timeline View Mode
The application SHALL support `'timeline'` as an independent top-level `ViewMode`.

#### Scenario: Navigating to Timeline view
- **WHEN** the user selects the Timeline navigation item or executes the `switch_timeline` command
- **THEN** `activeView` transitions to `'timeline'` and the application renders `<SprintTimelineView />` directly within the main workspace area.

#### Scenario: Navigating to Roadmap view
- **WHEN** the user selects the Roadmap navigation item or executes the `switch_roadmap` command
- **THEN** `activeView` transitions to `'roadmap'` and the application renders `<RoadmapView />` displaying exclusively macro horizons (NOW, NEXT, LATER, Non classés, Masqués).

### Requirement: Decoupled Roadmap View Header
The `RoadmapView` component SHALL NOT contain internal sub-toggle tabs switching between Sprints timeline and Macros horizons.

#### Scenario: Inspecting RoadmapView header
- **WHEN** `RoadmapView` is rendered under `activeView === 'roadmap'`
- **THEN** the view header displays macro horizon tabs (NOW, NEXT, LATER, Non classés, Masqués) without an embedded "Sprints" / "Macros" switcher bar.

### Requirement: Sidebar and Command Palette Navigation
The `Sidebar`, `Header`, and `CommandPalette` components SHALL provide dedicated navigation shortcuts for both Timeline and Roadmap views.

#### Scenario: Sidebar items list
- **WHEN** the user inspects the navigation sidebar
- **THEN** separate links exist for "Timeline" (routing to `'timeline'`) and "Roadmap" (routing to `'roadmap'`).

#### Scenario: Command Palette execution
- **WHEN** the user opens the command palette and selects "Vue Timeline Sprints"
- **THEN** the command palette closes and `activeView` updates to `'timeline'`.
