## MODIFIED Requirements

### Requirement: Preserve existing desktop behavior
Ordering SHALL preserve alphabetic project order, one row per task, existing archive filtering, chronological execution history, and selection by execution identity.

#### Scenario: Refresh changes task ordering
- **GIVEN** an execution is selected and another task changes state
- **WHEN** a refresh reorders rows
- **THEN** the same execution stays selected and its history remains in submission order

#### Scenario: Archived execution
- **GIVEN** an archived finished execution and visible executions
- **WHEN** rows are ordered
- **THEN** archived finished executions do not influence visible task representatives, anchors or order

## ADDED Requirements

### Requirement: Order by immutable submission anchor
The sidebar SHALL order task rows newest first within each state group using an immutable group anchor: the earliest submission time among the task's visible executions. Actual execution start SHALL NOT influence row order. Groups whose executions carry no usable submission time SHALL sort last within their state group. Equal anchors SHALL use ascending task and execution identities for deterministic order.

#### Scenario: Execution starts
- **GIVEN** a queued execution positioned between two other rows
- **WHEN** it starts and reports an actual start time later than its neighbours
- **THEN** its position within its new state group derives from its submission anchor, not from the actual start time

#### Scenario: Relaunch on a listed task
- **GIVEN** a finished task row positioned below other finished rows
- **WHEN** a new execution is launched on that task
- **THEN** the row moves only because its state group changed, and it does not move to the top of the list on submission time

#### Scenario: Legacy timestamps and ties
- **GIVEN** records with missing or invalid submission times, equivalent instants expressed with different timezone offsets, or no usable times
- **WHEN** the agent returns those records in a different order
- **THEN** unknown-time placement and identity ties produce the same row order

### Requirement: Hold the sidebar order while the user interacts with it
The desktop sidebar SHALL NOT reorder or rebuild its task list in response to a background refresh while the pointer is over the task list or keyboard focus is inside it. Per-execution status indicators SHALL keep updating during that hold. The pending order SHALL be applied as soon as the pointer leaves the list and focus is outside it, and a render requested by a user action SHALL apply immediately.

#### Scenario: State change under the pointer
- **GIVEN** the pointer rests over the task list
- **WHEN** a refresh reports a state change that would reorder rows
- **THEN** the row sequence is unchanged and the affected row's status indicator is updated

#### Scenario: Pointer leaves the list
- **GIVEN** a reorder was withheld while the pointer was over the task list
- **WHEN** the pointer leaves the list and no row holds focus
- **THEN** the withheld order is applied

#### Scenario: User action during the hold
- **GIVEN** the pointer is over the task list and a reorder is pending
- **WHEN** the user selects a row or collapses a project
- **THEN** the sidebar renders immediately with the current order

## REMOVED Requirements

### Requirement: Order by actual execution start
**Reason**: Actual execution start is a volatile sort key: a row jumped the moment its run started (#69). Replaced by `Order by immutable submission anchor`.
**Migration**: No data migration. The local agent run contract still exposes the actual start timestamp; only the desktop ordering stops reading it.
