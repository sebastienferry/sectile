## MODIFIED Requirements

### Requirement: Execution stop control reads as a close action
The desktop execution toolbar SHALL present its stop control with a completion mark rather than a cross, in the affirmative accent rather than the destructive one, while keeping its accessible name and tooltip describing the stop action.

#### Scenario: User views a running execution
- **GIVEN** the desktop workspace displays an execution that is queued, preparing, or running
- **WHEN** the toolbar is displayed
- **THEN** the stop control shows a completion mark in the affirmative accent
- **AND** its accessible name and tooltip still describe stopping the execution
- **AND** that glyph still differs from the agent stop control's disconnect glyph.

#### Scenario: User stops the execution
- **GIVEN** the stop control is enabled
- **WHEN** the user activates it
- **THEN** the execution is requested to stop exactly as before
- **AND** nothing else is proposed or launched by that activation
