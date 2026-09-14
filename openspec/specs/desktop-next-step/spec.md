# desktop-next-step Specification

## Purpose
TBD - created by archiving change 66-terminal-next-step. Update Purpose after archive.
## Requirements
### Requirement: Contextual next step below the task console
The desktop SHALL show the selected task's current stage and next agentic step below its TTY, based on current server task state rather than the selected execution's skill.

#### Scenario: Actionable workflow stage
- **GIVEN** a selected task at new, clarified, specified or implemented stage
- **WHEN** current task and configured project skills are available
- **THEN** the footer offers Clarify, Specify, Implement or Review and create PR respectively.

#### Scenario: No action available
- **GIVEN** no selected task, a reviewed or finished task, or unavailable task/project metadata or required skill
- **WHEN** the footer renders
- **THEN** it explains the state without an enabled launch action
- **AND** reviewed tasks indicate that human merge is pending.

### Requirement: Safe next-step dispatch
The desktop SHALL dispatch one next skill for the selected task using existing server launch behavior and SHALL NOT change workflow stage or merge a pull request itself.

#### Scenario: Launch the selected task
- **GIVEN** an available next step with no active execution for the task
- **WHEN** the user activates the button
- **THEN** the desktop rechecks current task, project and execution state before launching that skill for the selected project and full task ID
- **AND** the button remains disabled during submission and the execution list refreshes on success.

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

### Requirement: Accessible console footer
The footer SHALL retain terminal space and provide a keyboard-operable action with a descriptive label and visible status.

#### Scenario: Narrow window
- **GIVEN** a narrow desktop window
- **WHEN** the status and action render
- **THEN** they wrap beneath the terminal without covering console content.

