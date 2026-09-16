## MODIFIED Requirements

### Requirement: Safe next-step dispatch
The desktop SHALL dispatch one next skill for the selected task using existing server launch behavior and SHALL NOT change workflow stage or merge a pull request itself. The dispatch SHALL carry no execution-mode override: the step runs in the mode the precedence resolves for that skill, so the button stays a single interaction that asks nothing.

#### Scenario: Launch the selected task
- **GIVEN** an available next step with no active execution for the task
- **WHEN** the user activates the button
- **THEN** the desktop rechecks current task, project and execution state before launching that skill for the selected project and full task ID
- **AND** the button remains disabled during submission and the execution list refreshes on success.

#### Scenario: One click launches the resolved mode
- **GIVEN** a selected task whose resolved execution mode is autonomous
- **WHEN** the user activates the button
- **THEN** the step is submitted without a mode override and runs autonomous
- **AND** no dialog or mode choice is presented.

#### Scenario: Active or changed task
- **GIVEN** a queued, preparing or running execution, or a workflow stage changed since display
- **WHEN** the user views an older console or requests a next step
- **THEN** no conflicting or outdated step is launched
- **AND** the footer reflects the latest state.

#### Scenario: Selection and request failure
- **GIVEN** a metadata or launch request is pending
- **WHEN** selection changes or the request fails
- **THEN** a late response does not replace the newly selected task's action
- **AND** failures are visible with a recovery path.

#### Scenario: Refused autonomous dispatch
- **GIVEN** a selected task whose resolved execution mode is autonomous on a provider with no headless invocation
- **WHEN** the user activates the button
- **THEN** the refusal is displayed in the footer status with the provider named
- **AND** no execution is created.
