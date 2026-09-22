## ADDED Requirements

### Requirement: Selected execution state in the header
The desktop header SHALL show the selected execution's run state beside the task identity, using the shared run-state definition that the sidebar row and the desktop notification already read, and SHALL spell out that state's label. No selected execution SHALL leave no state shown.

#### Scenario: Running execution
- **GIVEN** a selected execution the agent reports as running
- **WHEN** the header is displayed
- **THEN** it shows the shared running glyph and the state's own label beside the title
- **AND** that label is also available on hover and to assistive technology.

#### Scenario: Ended execution
- **GIVEN** a selected execution that completed, failed or was cancelled
- **WHEN** the header is displayed
- **THEN** it shows that outcome's shared glyph and label, and not a progress state.

#### Scenario: No execution selected
- **GIVEN** no execution is selected
- **WHEN** the header is displayed
- **THEN** no run state is shown.

### Requirement: Copyable worktree path
The desktop header SHALL present the selected execution's checkout directory as a control that copies the displayed path to the workstation clipboard on activation, confirms the copy, and keeps the path selectable for a manual copy. It SHALL copy exactly the path shown and send it nowhere else.

#### Scenario: Copy the path
- **GIVEN** a selected execution with a recorded checkout directory
- **WHEN** the user activates the path control
- **THEN** the clipboard holds exactly that path
- **AND** a confirmation is announced through a live region and then withdrawn.

#### Scenario: Copy fails
- **GIVEN** the copy cannot be performed
- **WHEN** the user activates the path control
- **THEN** no confirmation is shown and the failure is reported through the existing error surface.

#### Scenario: Manual copy and keyboard use
- **GIVEN** the path control is displayed
- **WHEN** the user selects its text with the pointer, or reaches it with the keyboard
- **THEN** the text can be selected and copied by hand
- **AND** the control shows a visible focus outline and activates from the keyboard.

#### Scenario: No path to show
- **GIVEN** no selected execution, or a selected execution without a recorded directory
- **WHEN** the header is displayed
- **THEN** no path control is shown and any previous confirmation is cleared.

### Requirement: Skill result reports only what the run state cannot
The skill-result indicator, wherever it is shown, SHALL report what the server knows about the launched skill and SHALL NOT restate the process state that the run state already carries. It MAY additionally report a requested stop that has not taken effect, which the run state does not express. With nothing to report it SHALL show nothing rather than an echo, and an indicator whose reported result no longer holds SHALL be cleared.

#### Scenario: Execution still in flight
- **GIVEN** a queued, preparing or running execution for which the server reports no verdict on its skill
- **WHEN** the header and the task row render
- **THEN** no skill-result indicator is shown
- **AND** the run state alone reports that the execution is queued or running.

#### Scenario: Server verdict available
- **GIVEN** the server reports the launched skill of that exact execution as completed, failed or cancelled
- **WHEN** the header and the task row render
- **THEN** the indicator reports that verdict, or that stage validation is still awaited
- **AND** it does so whether or not the console process has exited.

#### Scenario: Ended without a verdict
- **GIVEN** an execution whose process ended
- **WHEN** it completed and the server confirmed no skill
- **THEN** the indicator reports that the skill completion is unconfirmed
- **AND** when it failed or was cancelled instead, no indicator is shown, because the run state already reports that outcome.

#### Scenario: Requested stop
- **GIVEN** a stop has been requested for an execution that has not ended yet
- **WHEN** the header and the task row render
- **THEN** the indicator reports that the execution is stopping
- **AND** once the execution has ended, that report is withdrawn.

#### Scenario: Free console
- **GIVEN** a free agent console, which runs no skill
- **WHEN** the header and the task row render
- **THEN** no skill-result indicator is shown, except while a requested stop has not taken effect.

#### Scenario: Withdrawn result
- **GIVEN** a task row or header showing a skill result
- **WHEN** that result stops applying, because the verdict was withdrawn or the execution moved on
- **THEN** the indicator is cleared rather than left showing the previous result.

## MODIFIED Requirements

### Requirement: Readable and safe header presentation
The header SHALL display task-derived content as literal text. Long titles SHALL remain on one visible line with truncation, expose their full header text on hover and to assistive technology, and preserve toolbar control usability at supported narrow window widths. Toolbar controls whose action does not depend on the workflow stage — relaunch, log export, the console/changes view switch, and the linked pull request — SHALL be icon-led rather than labelled, and SHALL keep their wording as accessible name and tooltip; the pull-request control SHALL keep its identifying number. Controls that launch or close a workflow step SHALL keep their text label.

#### Scenario: Long title with toolbar controls
- **GIVEN** a long title, a narrow supported desktop window, and visible execution-history, PR, relaunch, export, and stop controls as applicable
- **WHEN** the header renders
- **THEN** the title truncates without overlapping or pushing controls outside the window
- **AND** the full header text is available on hover and to assistive technology
- **AND** the controls remain keyboard-operable.

#### Scenario: Title contains markup-like content
- **GIVEN** a task title contains HTML-like text
- **WHEN** the header renders
- **THEN** that content appears as literal text and does not create markup or execute code.

#### Scenario: Icon-led utility controls
- **GIVEN** the relaunch, log export and console/changes controls are displayed
- **WHEN** the header renders
- **THEN** each shows a glyph and no text label
- **AND** each keeps its former wording as accessible name and tooltip.

#### Scenario: Pull request control keeps its number
- **GIVEN** the selected task has a valid linked pull request
- **WHEN** the header renders
- **THEN** the control shows the shared pull-request glyph followed by that pull request's number.

#### Scenario: Workflow actions stay labelled
- **GIVEN** a next step, a review declaration, a retry or a forced launch is offered
- **WHEN** the header renders
- **THEN** each of those controls shows its text label.
