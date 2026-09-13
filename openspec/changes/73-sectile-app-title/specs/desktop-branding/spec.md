## ADDED Requirements

### Requirement: Sectile desktop app title
The desktop application SHALL display Sectile Local as its document title, native window title and visible application header, with S as the adjacent brand badge.

#### Scenario: Desktop application opens
- **GIVEN** the desktop application is installed
- **WHEN** the user opens its window
- **THEN** the document and native window titles are Sectile Local
- **AND** the application header displays Sectile Local with the S badge

#### Scenario: User connects to an agent
- **GIVEN** the desktop application displays its connection setup
- **WHEN** the user connects to an agent and opens the workspace
- **THEN** the application title and header remain Sectile Local
