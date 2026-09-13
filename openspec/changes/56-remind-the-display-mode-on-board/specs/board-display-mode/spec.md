## ADDED Requirements

### Requirement: Board card display mode is remembered across reloads and view changes
The application SHALL persist the user's board card display mode preference (`condensed` or `expanded`) in browser storage, and restore it when the board view is loaded or reloaded.

#### Scenario: Default display mode when unconfigured
- **GIVEN** a user with no saved board display mode preference in browser storage
- **WHEN** the board view is opened
- **THEN** cards are displayed in condensed (single-line) mode
- **AND** the toggle button indicates that condensed mode is active.

#### Scenario: Toggle to expanded mode and persist
- **GIVEN** the board is in condensed mode
- **WHEN** the user clicks the card display mode toggle button
- **THEN** cards switch to expanded (detailed) mode
- **AND** the preference `expanded` is persisted to `localStorage`.

#### Scenario: Reload or re-mount preserves selected display mode
- **GIVEN** the user has toggled the board card display mode to `expanded`
- **WHEN** the user switches to another view (e.g. Backlog or Roadmap) and returns to the board, or reloads the browser
- **THEN** cards remain displayed in expanded mode without resetting to condensed.

#### Scenario: Toggle back to condensed mode and persist
- **GIVEN** the board is in expanded mode
- **WHEN** the user clicks the card display mode toggle button
- **THEN** cards switch back to condensed (single-line) mode
- **AND** the preference `condensed` is persisted to `localStorage`.
