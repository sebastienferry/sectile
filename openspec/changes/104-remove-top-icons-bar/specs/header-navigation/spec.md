## ADDED Requirements

### Requirement: Header avoids duplicate view navigation
The application header SHALL omit the view icon bar beside Quick Add at all viewport sizes.

#### Scenario: User views the header
- **GIVEN** the application is open
- **WHEN** the header is displayed
- **THEN** no header view buttons for triage, backlog, board, roadmap, timeline, activities, or synchronization appear
- **AND** Quick Add, project settings, global search, and applicable active filter chips remain available

### Requirement: Existing navigation and task creation remain available
Users SHALL retain sidebar navigation and header task creation.

#### Scenario: User navigates through the sidebar
- **GIVEN** the sidebar is expanded or collapsed
- **WHEN** the user selects any of the seven view destinations
- **THEN** the corresponding view opens

#### Scenario: User creates a task from the header
- **GIVEN** the header is displayed
- **WHEN** the user selects Quick Add
- **THEN** the existing task creation interface opens
