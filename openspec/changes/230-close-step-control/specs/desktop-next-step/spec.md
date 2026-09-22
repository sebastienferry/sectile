## ADDED Requirements

### Requirement: Step-closing control presented as a workflow step
The desktop SHALL present the control that ends the selected task's running execution as the closing step of the current stage, not as a destructive action, and SHALL NOT change what that control does: it ends the running execution and nothing else.

#### Scenario: Appearance of the closing control
- **GIVEN** a selected task execution in the toolbar
- **WHEN** the closing control renders
- **THEN** it uses the closing accent colour rather than the destructive red
- **AND** it shows a glyph that reads as closing a step, or the text `Close` when no glyph does
- **AND** its accessible name and tooltip state the action actually performed on the execution.

#### Scenario: Closing unblocks the next step
- **GIVEN** an active execution on the selected task, with `Next: <skill>` disabled because of it
- **WHEN** the user activates the closing control and the execution ends
- **THEN** the execution is ended through the existing stop behaviour
- **AND** `Next: <skill>` becomes available for that task once no execution is active
- **AND** the desktop does not transition the task's workflow stage itself.

#### Scenario: Reviewed task is not closed by the control
- **GIVEN** a reviewed task whose project exposes the handoff skill
- **WHEN** its execution is ended through the closing control
- **THEN** nothing but the execution ends, and the task stays at reviewed
- **AND** `Next: Handoff` remains the one way to run its closing step.

### Requirement: Toolbar order and colour separate closing from continuing
The toolbar SHALL place the step-closing control before the `Next: <skill>` control and SHALL colour them distinctly, so that ending the current step and launching the next one are never mistaken for one another.

#### Scenario: Order in the toolbar
- **GIVEN** the task toolbar with its export, closing and next-step controls
- **WHEN** it renders
- **THEN** the closing control appears before `Next: <skill>` in both visual and focus order.

#### Scenario: Distinct colours
- **GIVEN** the closing control and `Next: <skill>` both visible
- **WHEN** they render
- **THEN** the closing control uses the green closing accent and `Next: <skill>` uses a distinct blue accent
- **AND** each keeps its visible focus outline.

#### Scenario: Only one control applies at a time
- **GIVEN** the selected task
- **WHEN** an execution is queued, preparing or running
- **THEN** the closing control is enabled and `Next: <skill>` is disabled
- **AND** when no execution is active the closing control is disabled and `Next: <skill>` follows its existing availability rules.

#### Scenario: Narrow window
- **GIVEN** a narrow supported desktop window
- **WHEN** the toolbar renders both controls
- **THEN** they stay inside the window without overlapping the task title, and remain keyboard-operable.

## MODIFIED Requirements

### Requirement: Contextual next step in the execution toolbar
The desktop SHALL show the selected task's current stage and next agentic step for its console, based on current server task state rather than the selected execution's skill. The launch action SHALL sit in the execution toolbar immediately after the control that ends the execution, so the toolbar reads in the order the user acts, and its status text SHALL remain below the TTY.

#### Scenario: Actionable workflow stage
- **GIVEN** a selected task at new, clarified, specified or implemented stage
- **WHEN** current task and configured project skills are available
- **THEN** the toolbar offers Clarify, Specify, Implement or Review and create PR respectively, right after the control that ends the execution
- **AND** the footer shows the task key, its stage and its readiness message.

#### Scenario: No action available
- **GIVEN** no selected task, a reviewed or finished task, or unavailable task/project metadata or required skill
- **WHEN** the console renders
- **THEN** the footer explains the state without an enabled launch action in the toolbar
- **AND** reviewed tasks indicate that human merge is pending.
