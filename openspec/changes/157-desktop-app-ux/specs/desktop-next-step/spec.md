## RENAMED Requirements
- FROM: `### Requirement: Contextual next step below the task console`
- TO: `### Requirement: Contextual next step in the execution toolbar`
- FROM: `### Requirement: Accessible console footer`
- TO: `### Requirement: Accessible next-step controls`

## MODIFIED Requirements
### Requirement: Contextual next step in the execution toolbar
The desktop SHALL show the selected task's current stage and next agentic step for its console, based on current server task state rather than the selected execution's skill. The launch action SHALL sit in the execution toolbar immediately before the stop control, and its status text SHALL remain below the TTY.

#### Scenario: Actionable workflow stage
- **GIVEN** a selected task at new, clarified, specified or implemented stage
- **WHEN** current task and configured project skills are available
- **THEN** the toolbar offers Clarify, Specify, Implement or Review and create PR respectively, next to the stop control
- **AND** the footer shows the task key, its stage and its readiness message.

#### Scenario: No action available
- **GIVEN** no selected task, a reviewed or finished task, or unavailable task/project metadata or required skill
- **WHEN** the console renders
- **THEN** the footer explains the state without an enabled launch action in the toolbar
- **AND** reviewed tasks indicate that human merge is pending.

### Requirement: Accessible next-step controls
The next-step launch and retry controls SHALL be keyboard-operable with descriptive labels, SHALL not displace the existing toolbar controls, and their status SHALL stay visible without covering console content.

#### Scenario: Narrow window
- **GIVEN** a narrow desktop window
- **WHEN** the toolbar and the status render
- **THEN** the toolbar controls remain reachable and the status text wraps beneath the terminal without covering console content.

#### Scenario: Keyboard use
- **GIVEN** the next step is available
- **WHEN** the user reaches the toolbar with the keyboard
- **THEN** the launch action and, when shown, its retry companion are focusable and carry their descriptive labels.
