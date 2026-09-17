## ADDED Requirements

### Requirement: A task is active while it carries a live remote run
A task SHALL count as active while at least one of its remote runs is waiting, running or queued, and
SHALL stop counting as active as soon as none is. The set of active tasks SHALL be derived from the
same runs the run indicator reduces, so the filter and the badge cannot disagree.

#### Scenario: A task whose run is executing
- **GIVEN** a task carrying a remote run whose state is running or queued
- **WHEN** the active tasks are derived
- **THEN** the task is among them

#### Scenario: A task whose run is blocked on the user
- **GIVEN** a task carrying a remote run that reported it is waiting for the user
- **WHEN** the active tasks are derived
- **THEN** the task is among them

#### Scenario: A task whose run has just been canceled
- **GIVEN** a task whose only remote run was canceled moments ago and still shows its cancellation
- **WHEN** the active tasks are derived
- **THEN** the task is not among them

#### Scenario: A task whose runs have all finished
- **GIVEN** a task carrying only completed, failed or canceled remote runs
- **WHEN** the active tasks are derived
- **THEN** the task is not among them

#### Scenario: A task with no run at all
- **GIVEN** a task that has never been executed
- **WHEN** the active tasks are derived
- **THEN** the task is not among them

### Requirement: The board and the backlog can be narrowed to the active tasks
The board and the backlog SHALL offer a filter that restricts the tasks they display to the active
ones. The filter SHALL be offered in the toolbar both views share, and SHALL apply to both.

#### Scenario: Narrowing the board
- **GIVEN** a board showing tasks of which two carry a live remote run
- **WHEN** the filter is turned on
- **THEN** only those two tasks are displayed

#### Scenario: Narrowing the backlog
- **GIVEN** the backlog showing the same tasks
- **WHEN** the filter is turned on
- **THEN** only the tasks carrying a live remote run are listed

#### Scenario: The filter survives the change of view
- **GIVEN** the filter is on in the board
- **WHEN** the user switches to the backlog
- **THEN** the filter is still on

#### Scenario: The board keeps its shape
- **GIVEN** the filter is on and a column holds no active task
- **WHEN** the board is displayed
- **THEN** the column is still shown, empty

#### Scenario: Turning the filter off
- **GIVEN** the filter is on
- **WHEN** it is turned off
- **THEN** every task the other filters allow is displayed again

### Requirement: The filter composes with the other filters
The filter SHALL narrow the tasks the other filters already allow, and SHALL NOT widen them.

#### Scenario: Combined with a priority
- **GIVEN** the priority filter is set to high and the active filter is on
- **WHEN** the board is displayed
- **THEN** only the high-priority tasks carrying a live remote run are displayed

#### Scenario: An active task the other filters exclude
- **GIVEN** a task carrying a live remote run which the current search excludes
- **WHEN** the active filter is turned on
- **THEN** that task is still not displayed

### Requirement: The filter states itself and is reachable
The filter SHALL show whether it is on, SHALL appear among the clearable filters of the header while
it is on, and SHALL be remembered per project between sessions.

#### Scenario: The filter is visible in the header
- **GIVEN** the filter is on
- **WHEN** the header is displayed
- **THEN** it lists the filter among the active ones
- **AND** the filter can be cleared from there

#### Scenario: The filter is remembered
- **GIVEN** the filter was left on for a project
- **WHEN** the project is opened again
- **THEN** the filter is still on

#### Scenario: Each project keeps its own
- **GIVEN** the filter is on for one project
- **WHEN** another project is selected
- **THEN** that project's own remembered value applies

#### Scenario: Nothing is running
- **GIVEN** no task carries a live remote run
- **WHEN** the filter is on
- **THEN** the view states that no task is running rather than showing an unexplained emptiness

### Requirement: The filter follows the runs as they change
The displayed set SHALL follow the runs: a task SHALL appear as its run starts and SHALL leave as its
run ends, without the user reloading or toggling the filter.

#### Scenario: A run starts
- **GIVEN** the filter is on and a task is not displayed
- **WHEN** a remote run starts on that task
- **THEN** it appears among the displayed tasks

#### Scenario: A run ends
- **GIVEN** the filter is on and a task is displayed because of its run
- **WHEN** that run completes, fails or is canceled
- **THEN** the task leaves the displayed tasks
