## ADDED Requirements

### Requirement: Next step after implementation depends on the pull request
For a task at the `implemented` stage, the desktop SHALL offer **Next: Adjust** when the task records a pull request and **Next: Create PR** when it records none. **Next: Create PR** SHALL launch the pull request creation owner configured for the project, and SHALL NOT launch the stage-neutral `create_pr` skill. The desktop SHALL read the pull request record and the creation owner from the task and project state the server returns, and SHALL NOT query a forge itself.

#### Scenario: Implemented task with a pull request
- **GIVEN** a selected task at the `implemented` stage whose record carries a pull request URL
- **WHEN** the project exposes the `adjust` skill
- **THEN** the toolbar offers **Next: Adjust**
- **AND** activating it launches the `adjust` skill for that task.

#### Scenario: Implemented task without a pull request, creation at implementation
- **GIVEN** a selected task at the `implemented` stage with no pull request recorded
- **AND** the project creates pull requests at the `implemented` stage, or states no creation stage
- **WHEN** the project exposes the `implement` skill
- **THEN** the toolbar offers **Next: Create PR**
- **AND** activating it launches the `implement` skill for that task with no prompt of its own.

#### Scenario: Implemented task without a pull request, creation at specification
- **GIVEN** a selected task at the `implemented` stage with no pull request recorded
- **AND** the project creates pull requests at the `specified` stage
- **WHEN** the project exposes the `specify` skill
- **THEN** the toolbar offers **Next: Create PR**
- **AND** activating it launches the `specify` skill for that task with no prompt of its own.

#### Scenario: Pull request recorded after recovery
- **GIVEN** an implemented task that offered **Next: Create PR**
- **WHEN** the recovery execution ends and the server task now records a pull request URL
- **THEN** the next refresh of the footer offers **Next: Adjust** for that task
- **AND** the desktop has not transitioned the stage or linked the pull request itself.

#### Scenario: Merged pull request
- **GIVEN** a selected task at the `implemented` stage whose recorded pull request is already merged
- **WHEN** the footer renders
- **THEN** the toolbar offers **Next: Adjust**, leaving the merged-state handling to the adjust skill.

#### Scenario: Resolved skill unavailable
- **GIVEN** a selected task at the `implemented` stage
- **WHEN** the skill the rule resolves to is not among the project's configured skills
- **THEN** the footer explains that the next skill is unavailable, naming **Adjust** or **Create PR**
- **AND** no launch action is enabled.

## MODIFIED Requirements

### Requirement: Contextual next step below the task console
The desktop SHALL show the selected task's current stage and next agentic step below its TTY, based on current server task state rather than the selected execution's skill.

#### Scenario: Actionable workflow stage
- **GIVEN** a selected task at new, clarified, specified or implemented stage
- **WHEN** current task and configured project skills are available
- **THEN** the footer offers Clarify, Specify or Implement for the first three stages
- **AND** for the implemented stage it offers Adjust when the task records a pull request and Create PR otherwise.

#### Scenario: No action available
- **GIVEN** no selected task, a reviewed or finished task, or unavailable task/project metadata or required skill
- **WHEN** the footer renders
- **THEN** it explains the state without an enabled launch action
- **AND** reviewed tasks indicate that human merge is pending.
