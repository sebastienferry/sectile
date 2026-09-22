## ADDED Requirements

### Requirement: Closing offer after stopping a reviewed task
When a stop request issued from the desktop execution toolbar succeeds, the desktop SHALL read the stopped execution's current task and project state and, when the task is at the `reviewed` stage and the project exposes the closing skill, SHALL offer to close the task. The offer SHALL be an explicit, dismissible dialog, and the stop itself SHALL complete regardless of the offer.

#### Scenario: Reviewed task offers closure
- **GIVEN** a running execution whose task is at the `reviewed` stage on a configured project exposing the `handoff` skill
- **WHEN** the user stops that execution and the stop succeeds
- **THEN** a dialog offers to close the task
- **AND** the execution is stopped whether the user confirms or dismisses it.

#### Scenario: Any other stage stays silent
- **GIVEN** a running execution whose task is at the `new`, `clarified`, `specified`, `implemented` or `finished` stage
- **WHEN** the user stops that execution
- **THEN** no closing dialog is shown.

#### Scenario: No task behind the execution
- **GIVEN** a running free agent console
- **WHEN** the user stops it
- **THEN** no closing dialog is shown, because there is no task to close.

#### Scenario: Closing skill unavailable
- **GIVEN** a reviewed task on a project that is not configured, or whose server skill list has no closing skill
- **WHEN** the user stops its execution
- **THEN** no closing dialog is shown.

#### Scenario: Failed stop
- **GIVEN** a running execution whose task is at the `reviewed` stage
- **WHEN** the stop request fails
- **THEN** the failure is reported as before
- **AND** no closing dialog is shown.

#### Scenario: Unreadable task state
- **GIVEN** a stop request that succeeded
- **WHEN** the task or project state cannot be read from the server
- **THEN** no closing dialog is shown and no error is raised for the stop.

### Requirement: Closing runs the handoff skill
Accepting the closing offer SHALL launch the `handoff` skill for that task through the existing server launch path, which is what transitions the task to `finished`. The desktop SHALL NOT change the workflow stage itself.

#### Scenario: Confirmed closure
- **GIVEN** the closing dialog for a reviewed task
- **WHEN** the user confirms
- **THEN** the desktop launches the `handoff` skill for that project and full task ID
- **AND** the confirm action is disabled while the launch is in flight
- **AND** the dialog closes and the execution list refreshes on success.

#### Scenario: Dismissed closure
- **GIVEN** the closing dialog for a reviewed task
- **WHEN** the user dismisses it
- **THEN** no skill is launched and the task stays at `reviewed`.

#### Scenario: Launch failure
- **GIVEN** a confirmed closing offer
- **WHEN** the launch request fails
- **THEN** the dialog reports the failure and allows another attempt
- **AND** the task stays at `reviewed`.
