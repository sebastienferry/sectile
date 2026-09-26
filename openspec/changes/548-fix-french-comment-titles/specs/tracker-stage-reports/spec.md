## ADDED Requirements

### Requirement: English clarification report title

When Sectile posts a tracker report after a task reaches the clarified stage,
the report SHALL use the English heading
`### 💬 [Sectile] Clarification Report`.

#### Scenario: Clarified stage note is posted

- **GIVEN** a tracker-backed task transitions to `clarified` with a report note
- **WHEN** Sectile posts the generated stage report comment
- **THEN** the comment begins with `### 💬 [Sectile] Clarification Report`
- **AND** the report note follows the heading unchanged.

### Requirement: English specification report title

When Sectile posts a tracker report after a task reaches the specified stage,
the report SHALL use the English heading
`### 📋 [Sectile] Technical Specification & Implementation Plan`.

#### Scenario: Specified stage note is posted

- **GIVEN** a tracker-backed task transitions to `specified` with a report note
- **WHEN** Sectile posts the generated stage report comment
- **THEN** the comment begins with
  `### 📋 [Sectile] Technical Specification & Implementation Plan`
- **AND** the report note follows the heading unchanged.
