## MODIFIED Requirements

### Requirement: Project open-task entry point
Each desktop sidebar project row SHALL offer an accessible icon action for browsing that project's open tasks, independently of project expansion and existing actions. Activating it SHALL open that project's tickets pane in the content area, in place of the selected execution's console, and SHALL NOT change the project's collapsed state or launch an execution.

#### Scenario: Pointer or keyboard discovery
- **GIVEN** an expanded or collapsed project row
- **WHEN** the user hovers the row or focuses its controls
- **THEN** the open-task icon is visible and keyboard reachable
- **AND** activating it opens the project's tickets pane without changing its collapsed state or launching an execution.

#### Scenario: Second entry point
- **GIVEN** a project's **+** menu
- **WHEN** the user chooses to run an existing ticket
- **THEN** the same tickets pane opens for that project.

#### Scenario: Command palette entry point
- **GIVEN** the command palette
- **WHEN** the user runs its tickets-list action
- **THEN** the pane opens for the selected project, or for the only configured project when none is selected
- **AND** when several projects could apply and none is selected, the user is asked which project to browse instead of one being guessed.

#### Scenario: Closing the pane
- **GIVEN** the tickets pane is open
- **WHEN** the user activates its close control or presses Escape with no dialog open
- **THEN** the pane closes, the previously selected execution view is shown again
- **AND** keyboard focus returns to the control that opened the pane, or to a stable control of the app when that opener no longer exists.

#### Scenario: Selecting an execution leaves the pane
- **GIVEN** the tickets pane is open
- **WHEN** the user selects an execution, or opens a console, from outside the pane
- **THEN** the pane closes and that execution's console is shown, so no selection is made behind a hidden view.

#### Scenario: The local agent becomes unavailable
- **GIVEN** the tickets pane is open
- **WHEN** the local agent stops
- **THEN** the pane closes, since its contents come from that agent
- **AND** opening it while no agent is connected explains that instead of replacing the connection screen.

#### Scenario: One replacement view at a time
- **GIVEN** the agent-log pane is open
- **WHEN** the user opens a project's tickets pane
- **THEN** the log pane closes and the tickets pane is shown, and the reverse holds when the log pane is opened over the tickets pane.

### Requirement: Browse and search open tasks
The tickets pane SHALL load the selected project's unfinished tasks without requiring search text and SHALL present them as a table with one row per task showing its key, title, current workflow stage, priority and, when one is linked, its pull request.

#### Scenario: Initial list and search
- **GIVEN** a project with open and finished tasks
- **WHEN** the user opens its tickets pane
- **THEN** only that project's open tasks are listed, one row each
- **AND** searching narrows the rows and clearing the search restores the open-task list.

#### Scenario: Loading, empty and failed requests
- **GIVEN** a task-list request
- **WHEN** it is pending, succeeds without matches, or fails
- **THEN** the pane shows a loading message, an empty message, or a recoverable error respectively
- **AND** an older response cannot overwrite newer results or another project's pane.

#### Scenario: Row contents
- **GIVEN** a listed task with a priority and a linked pull request
- **WHEN** its row renders
- **THEN** the row shows the task key, its title on one line with the full title available on hover and to assistive technology, its workflow stage label, its priority
- **AND** a pull request icon that opens the pull request externally without changing the selected execution.

#### Scenario: Open a listed task in Sectile
- **GIVEN** a listed task
- **WHEN** the user activates its key, by pointer or keyboard
- **THEN** that task opens in Sectile externally
- **AND** no execution is submitted and the pane keeps its rows and selection.

### Requirement: Explicit task pickup
Each listed task SHALL offer a primary action that launches the task's next workflow step and a secondary menu that launches any other server-provided skill, a discussion console or custom instructions, all through the existing execution flow. Opening the pane, sorting or searching alone SHALL submit no execution.

#### Scenario: Run the next step
- **GIVEN** a listed task whose next workflow step resolves to an available skill on a configured project
- **WHEN** the user activates the row's primary action
- **THEN** exactly one execution of that skill is submitted for that task with the project's configured execution mode
- **AND** the pane stays open and reports the submission.

#### Scenario: Next step unavailable
- **GIVEN** a listed task whose next step cannot be resolved (finished, skill missing on the project, or project unconfigured)
- **WHEN** its row renders
- **THEN** the primary action is disabled and explains why where a pointer can reach the explanation, since a disabled control receives no pointer events.

#### Scenario: Reviewed task offers its closing step
- **GIVEN** a listed task awaiting human merge on a project that exposes the closing skill
- **WHEN** its row renders
- **THEN** the primary action offers that closing step rather than being disabled
- **AND** it is disabled with an explanation when the project does not expose that skill.

#### Scenario: Launch another skill from the menu
- **GIVEN** a listed task on a configured project
- **WHEN** the user opens the row's secondary menu and chooses the full-chain pickup, another server skill or the discussion console
- **THEN** exactly one execution of that choice is submitted for that task with the configured execution mode.

#### Scenario: Custom instructions with an execution mode
- **GIVEN** the user chose custom instructions from a row's secondary menu
- **WHEN** they enter instructions, optionally pick a one-off execution mode and launch
- **THEN** one execution carrying those instructions and that mode override is submitted
- **AND** an empty instruction text is refused with a message and nothing is submitted.

#### Scenario: Instructions survive a reordering
- **GIVEN** a row's custom instructions are open with text entered and an execution mode chosen
- **WHEN** the user sorts or searches, which rebuilds the rows
- **THEN** that text and that mode are still there, on that task's row.

#### Scenario: Unconfigured project or failed launch
- **GIVEN** a missing local repository mapping or a launch failure
- **WHEN** the user views or launches a task
- **THEN** a missing mapping disables every launch control with an explanatory notice and a failed submission remains visible for retry.

## ADDED Requirements

### Requirement: Default and column ordering of the tickets list
The tickets list SHALL order rows by priority descending (urgent, high, medium, low, then unknown) and, within equal priority, by task identity ascending using natural numeric comparison of the task key, falling back to the task id. Key, Title, Stage and Priority headers SHALL be sortable; a first activation sorts ascending except Priority which sorts descending, a second activation on the same column reverses the direction, and task identity ascending SHALL remain the final tie-break in every ordering. The active column and direction SHALL be visible and announced to assistive technology. The chosen ordering SHALL last for the window session only and SHALL reset to the default when the pane is reopened.

#### Scenario: Default order
- **GIVEN** open tasks `#9` (medium), `#100` (medium), `#12` (urgent) and `#7` (no priority)
- **WHEN** the pane opens
- **THEN** rows appear in the order `#12`, `#9`, `#100`, `#7`.

#### Scenario: Natural identity order across trackers
- **GIVEN** tasks keyed `PROJ-10` and `PROJ-9`, or tasks without a key
- **WHEN** identity decides the order
- **THEN** `PROJ-9` precedes `PROJ-10` and keyless tasks are ordered by their id the same way.

#### Scenario: Sort by a column and reverse
- **GIVEN** the pane with its default order
- **WHEN** the user activates the Title header once, then again
- **THEN** rows are ordered by title ascending, then by title descending
- **AND** rows with equal titles keep identity ascending in both cases
- **AND** the Title header reports the current direction to assistive technology while the other headers report none.

#### Scenario: Priority header starts descending
- **GIVEN** the user has sorted by Key
- **WHEN** they activate the Priority header
- **THEN** rows return to priority descending, then identity ascending.

#### Scenario: No persistence
- **GIVEN** the user sorted by Stage descending
- **WHEN** they close and reopen the pane, or restart the desktop
- **THEN** the default order applies again.

### Requirement: Tickets list reflects local execution state
A row whose task has a local execution SHALL show the shared run-state glyph and label for that task's current execution. While an execution for the task is queued, preparing or running, the row's primary action SHALL be disabled with an explanation and its secondary menu SHALL remain available. These indications SHALL update with the desktop refresh without reloading the list, reordering rows, or disturbing keyboard focus or an open menu.

#### Scenario: Active execution on a listed task
- **GIVEN** a listed task with a running local execution
- **WHEN** its row renders
- **THEN** the running glyph and label are shown on the row
- **AND** the primary action is disabled and explains that an execution is active
- **AND** the secondary menu can still be opened.

#### Scenario: Execution finishes while the pane is open
- **GIVEN** the row above
- **WHEN** the execution completes and the desktop refreshes
- **THEN** the row shows the completed state glyph, the primary action becomes available again
- **AND** the row order and any open menu are unchanged, and a refresh that disables the focused primary action moves focus to that row's secondary menu rather than losing it.

#### Scenario: Refresh deferred by the sidebar
- **GIVEN** the pointer or keyboard focus rests on a sidebar task row, which defers the sidebar's own re-render
- **WHEN** a listed task's execution changes state
- **THEN** the tickets rows still reflect that change.

#### Scenario: Executions the user archived
- **GIVEN** a listed task whose only execution has been archived
- **WHEN** its row renders
- **THEN** no run state is shown for it, as in the sidebar that hides it.
