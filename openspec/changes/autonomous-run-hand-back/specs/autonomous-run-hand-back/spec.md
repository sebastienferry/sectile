## ADDED Requirements

### Requirement: A headless launch can use the tools it was sent to use
An autonomous launch SHALL invoke the provider CLI in a form that does not require a human to
approve a tool, for every provider whose non-interactive approval mode is attested here. An
interactive launch SHALL NOT carry it. A provider whose approval mode is not attested SHALL keep
its current headless invocation rather than carry a guessed flag.

#### Scenario: An autonomous run reaches the board
- **GIVEN** a project whose provider has an attested headless invocation and approval mode
- **WHEN** a skill is launched autonomously
- **THEN** the command line carries that provider's non-interactive approval mode
- **AND** the run can call the Sectile MCP tools without a human answering a prompt

#### Scenario: An interactive run still asks
- **GIVEN** the same project
- **WHEN** the same skill is launched interactively
- **THEN** the command line carries no approval bypass

### Requirement: A live provider session is never run headless
A launch that opens a live provider session with no prompt of its own — a discussion, or a bare
terminal — SHALL be interactive whatever mode the project, the skill or the launch resolved to.

#### Scenario: A discussion on a project defaulting to autonomous
- **GIVEN** a project whose default skill mode is autonomous
- **WHEN** the user opens a discussion on a task
- **THEN** the session opens in a terminal
- **AND** no headless process is started for it

#### Scenario: A stage skill on the same project
- **GIVEN** the same project
- **WHEN** a workflow step is launched with no override
- **THEN** it runs headless as the project default says

### Requirement: A run records how it was launched
A run SHALL record the execution mode it was resolved to, the workflow stage of its task at the
moment it started and, when it is one step of a full chain, the stage that chain stops at. A run
recorded before those values existed SHALL read as unknown and SHALL NOT be judged on them.

#### Scenario: An autonomous launch
- **WHEN** a skill is launched autonomously on a task
- **THEN** the run carries the autonomous mode and the stage the task was on

#### Scenario: A run from an older record
- **GIVEN** a run recorded before a launch carried those values
- **WHEN** it finishes
- **THEN** nothing is reported about what it handed back

### Requirement: An autonomous run says whether it handed the workflow back
When an autonomous run of a skill whose purpose is to reach the next stage finishes without the
task having left the stage it started from, the run SHALL state that on its own activity,
alongside the reason the process gave for ending. The server SHALL NOT transition the task on
the run's behalf. A skill that owns no workflow stage SHALL NOT be reported this way.

#### Scenario: A run that moved nothing
- **GIVEN** an autonomous run of a stage skill on a task at a given stage
- **WHEN** the run ends with the task still at that stage
- **THEN** the run says it ended without moving the task, and names the stage
- **AND** the reason the process gave for ending is preserved
- **AND** the task's stage is unchanged

#### Scenario: A run that did the work
- **GIVEN** an autonomous run of a stage skill
- **WHEN** it transitions the task before ending
- **THEN** nothing is added to what the run reported

#### Scenario: A skill that owns no stage
- **GIVEN** an autonomous run of a skill that is not a workflow step
- **WHEN** it ends with the task's stage unchanged
- **THEN** it is not reported as having moved nothing

#### Scenario: An interactive run
- **GIVEN** an interactive run of a stage skill
- **WHEN** it ends with the task's stage unchanged
- **THEN** it is not reported as having moved nothing

### Requirement: A full chain enqueues its next step until the stop stage
Each step of a full chain run SHALL enqueue the step that follows it, in autonomous mode, when
it completes having advanced the task's stage and the task has not reached the chain's stop
stage. The chain SHALL stop, on a reason recorded on the run that ended, when the stop stage is
reached, when a step ends with any status other than completed, when a step completes without
advancing the stage, or when no step follows the stage reached. A repeated completion report for
the same run SHALL NOT enqueue a second step.

#### Scenario: A step that advanced the stage
- **GIVEN** a full chain step that completes and leaves the task one stage further
- **WHEN** that stage is before the chain's stop stage
- **THEN** the step that follows it is enqueued in autonomous mode

#### Scenario: The stop stage is reached
- **GIVEN** a full chain step that completes and leaves the task at the chain's stop stage
- **WHEN** it finishes
- **THEN** no further step is enqueued
- **AND** the run says the chain reached its stop stage

#### Scenario: A step that moved nothing
- **GIVEN** a full chain step that completes with the task still at the stage it started from
- **WHEN** it finishes
- **THEN** no further step is enqueued
- **AND** the run says the chain stops there

#### Scenario: A step that failed
- **GIVEN** a full chain step that ends as failed or canceled
- **WHEN** it finishes
- **THEN** no further step is enqueued
- **AND** the run says the chain stops there, naming the status

#### Scenario: The same run reports twice
- **GIVEN** a full chain step whose completion has already enqueued the next step
- **WHEN** the same run is reported finished again
- **THEN** no second step is enqueued
