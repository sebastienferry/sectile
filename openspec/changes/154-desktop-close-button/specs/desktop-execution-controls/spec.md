## ADDED Requirements

### Requirement: Execution stop control reads as a close action
The desktop execution toolbar SHALL present its stop control with a cross glyph rather than a square, while keeping its accessible name and tooltip describing the stop action.

#### Scenario: User views a running execution
- **GIVEN** the desktop workspace displays an execution that is queued, preparing, or running
- **WHEN** the toolbar is displayed
- **THEN** the stop control shows a cross glyph
- **AND** its accessible name and tooltip still describe stopping the execution

#### Scenario: User stops the execution
- **GIVEN** the stop control is enabled
- **WHEN** the user activates it
- **THEN** the execution is requested to stop exactly as before
- **AND** the subsequent closure offer is unchanged

### Requirement: Agent stop control is visually distinct from the execution control
The desktop header SHALL present its agent stop control with a disconnect glyph, distinct from the execution stop glyph, while keeping its accessible name and tooltip describing stopping the agent.

#### Scenario: User views a running local agent
- **GIVEN** the local agent is running and its stop control is visible
- **WHEN** the header is displayed
- **THEN** the agent stop control shows a disconnect glyph
- **AND** that glyph differs from the execution stop glyph
- **AND** its accessible name and tooltip still describe stopping the agent

#### Scenario: User stops the local agent
- **GIVEN** the agent stop control is visible
- **WHEN** the user activates it
- **THEN** the agent is stopped exactly as before

### Requirement: Remaining desktop icon controls are unchanged
The desktop SHALL keep the existing glyphs, sizes, colours, and focus treatment of its other icon controls.

#### Scenario: User views the header controls
- **GIVEN** the desktop workspace is open
- **WHEN** the header is displayed
- **THEN** the agent settings, start agent, restart agent, and profile controls keep their existing glyphs
- **AND** every icon control keeps its existing 36 by 36 button size, 20 by 20 glyph size, hover background, and focus outline
