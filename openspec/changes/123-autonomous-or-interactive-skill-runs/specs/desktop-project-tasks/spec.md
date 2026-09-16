## MODIFIED Requirements

### Requirement: Explicit task pickup
Users SHALL be able to launch a listed task using a server-provided skill through the existing execution flow, and SHALL be able to choose the execution mode for that launch only. The mode control SHALL offer the configured mode, autonomous, and interactive, default to the configured mode, and change no persisted setting.

#### Scenario: Pick a task
- **GIVEN** a listed task and a configured local project
- **WHEN** the user selects a skill and activates Launch
- **THEN** the selected task identity, project and skill are submitted once to the existing launcher
- **AND** opening or searching the list alone submits no execution.

#### Scenario: Launch with the configured mode
- **GIVEN** a listed task and a mode control left on its default
- **WHEN** the user activates Launch
- **THEN** no mode override is submitted
- **AND** the run uses the mode the precedence resolves for that skill.

#### Scenario: Launch in the chosen mode
- **GIVEN** a listed task whose resolved mode is interactive
- **WHEN** the user picks autonomous in the mode control and activates Launch
- **THEN** that run is autonomous
- **AND** the next launch from the dialog starts again on the configured mode.

#### Scenario: Unconfigured project or failed launch
- **GIVEN** a missing local repository mapping or a launch failure
- **WHEN** the user views or launches a task
- **THEN** a missing mapping prevents submission and a failed submission remains visible for retry.

#### Scenario: Refused autonomous launch
- **GIVEN** a project whose provider has no headless invocation
- **WHEN** the user picks autonomous and activates Launch
- **THEN** the refusal is shown as the dialog's notice with the provider named
- **AND** the dialog stays open for retry in interactive mode.
