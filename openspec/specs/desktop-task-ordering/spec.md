# desktop-task-ordering Specification

## Purpose
TBD - created by archiving change 72-project-tasks-list. Update Purpose after archive.
## Requirements
### Requirement: Order task rows by execution state
The desktop sidebar SHALL show active tasks before queued tasks before finished tasks within each project. Running and preparing executions SHALL be active; completed, failed, and canceled executions SHALL be finished. A task SHALL be represented by its highest-priority visible execution state, choosing the newest effective time in that state.

#### Scenario: Mixed executions on one task
- **GIVEN** a task with an older running execution and a newer queued execution
- **WHEN** the sidebar renders
- **THEN** one task row represents the running execution and precedes queued-only tasks

#### Scenario: All execution states
- **GIVEN** tasks with running, preparing, queued, completed, failed, and canceled executions
- **WHEN** the sidebar renders
- **THEN** running/preparing tasks precede queued tasks, which precede all finished tasks

### Requirement: Order by actual execution start
The sidebar SHALL order rows newest first within each state group using actual execution start when available. Queued/preparing runs SHALL use submission time. Older records with missing or invalid start time SHALL fall back to submission time. Records with no usable timestamp SHALL sort last within their group. Equal times SHALL use ascending task and execution identities for deterministic order.

#### Scenario: Queue delay changes chronological order
- **GIVEN** two running executions whose start order differs from submission order
- **WHEN** their rows render
- **THEN** the more recently started execution appears first

#### Scenario: Legacy timestamps and ties
- **GIVEN** records with missing or invalid start times, equivalent instants expressed with different timezone offsets, or no usable times
- **WHEN** the agent returns those records in a different order
- **THEN** submission fallbacks, unknown-time placement, and identity ties produce the same row order

### Requirement: Expose actual start without changing submission time
The local desktop run contract SHALL expose an optional actual execution start timestamp for successfully started executions and preserve it when they finish. Runs that have not started SHALL have no actual start timestamp; submission timestamps SHALL remain unchanged.

#### Scenario: Queued execution starts and finishes
- **GIVEN** an execution submitted before a console is available
- **WHEN** it successfully starts and later finishes
- **THEN** its actual start reflects successful command launch, differs from its earlier submission, and remains available after completion

#### Scenario: Execution never starts
- **GIVEN** a queued or preparing execution that is canceled or fails before launch
- **WHEN** desktop runs are requested
- **THEN** it has no actual start timestamp

### Requirement: Preserve existing desktop behavior
Ordering SHALL preserve alphabetic project order, one row per task, existing archive filtering, chronological execution history, and selection by execution identity.

#### Scenario: Refresh changes task ordering
- **GIVEN** an execution is selected and another task changes state or start time
- **WHEN** a refresh reorders rows
- **THEN** the same execution stays selected and its history remains in submission order

#### Scenario: Archived execution
- **GIVEN** an archived finished execution and visible executions
- **WHEN** rows are ordered
- **THEN** archived finished executions do not influence visible task representatives or order

