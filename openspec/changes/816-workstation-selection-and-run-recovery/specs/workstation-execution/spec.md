## ADDED Requirements

### Requirement: Eligible workstation selection for task launches
Every Web task skill launch that requires an agent SHALL use an online workstation belonging to the caller that has explicitly reported the resolved execution project available. The user SHALL see readable workstation names with device ID fallback and disambiguation. Server-only execution SHALL retain its existing behavior.

#### Scenario: Several eligible workstations
- **GIVEN** two or more eligible online workstations and a resolved execution project
- **WHEN** the user launches a task skill from its dialog, details or card shortcut
- **THEN** an accessible picker requires an explicit choice before launching on that workstation
- **AND** canceling the picker launches nothing and no preference is remembered

#### Scenario: One eligible workstation
- **GIVEN** only one eligible workstation, even if other connected machines lack the project
- **WHEN** the user launches an agent task skill
- **THEN** that eligible workstation is selected automatically

#### Scenario: No eligible workstation
- **GIVEN** no eligible online workstation, including unknown or missing project inventory
- **WHEN** the user attempts an agent task launch
- **THEN** the launch is blocked with an explanation and no active execution is created
- **AND** other users' workstations and credentials are not disclosed

#### Scenario: Project choice precedes workstation choice
- **GIVEN** a task requires the user to choose its execution project
- **WHEN** the user resolves that choice or changes the project
- **THEN** workstation eligibility is evaluated for that project and any previous selection is discarded

### Requirement: Selection is enforced for retries and batches
Web retries of agent task executions SHALL use the same selection rules as new task launches. Task batches SHALL launch only on a workstation eligible for every selected task's execution project. Macro launches SHALL remain outside this change.

#### Scenario: Compatible batch
- **GIVEN** a batch of tasks with resolved execution projects and a workstation eligible for all of them
- **WHEN** the user launches that batch
- **THEN** one common workstation is selected using the zero/one/many rules and every member executes there

#### Scenario: Incompatible or stale batch
- **GIVEN** no common eligible workstation or a member whose project is no longer available
- **WHEN** the batch is submitted
- **THEN** the entire launch is refused before starting any member and the user sees why

#### Scenario: Activity retry
- **GIVEN** a finished or failed agent task execution
- **WHEN** the user retries it from Web
- **THEN** its execution project is resolved and a new run uses the selected eligible workstation
- **AND** attribution of the original historical run remains unchanged

### Requirement: No silent rerouting
A selected run SHALL remain assigned to its workstation. Eligibility SHALL be rechecked before execution and an unavailable or invalid target SHALL never cause automatic selection of a different workstation.

#### Scenario: Target disappears before launch is accepted
- **GIVEN** the selected workstation disconnects or loses project availability after discovery
- **WHEN** the launch is submitted
- **THEN** it is refused without creating an active run and the user can refresh and choose again

#### Scenario: Target disappears after admission
- **GIVEN** an accepted execution has not yet dispatched and its target becomes unavailable
- **WHEN** dispatch cannot reach the selected workstation through the existing bounded reconnect behavior
- **THEN** the undispatched execution ends through the normal failure/handback path, releases its task lock and is not sent elsewhere

### Requirement: Durable readable workstation attribution
Task cards, task details and activity summaries SHALL display the assigned/executing workstation for known assignments. Attribution SHALL survive reload, server restart, reconnect and historical display. Unknown legacy attribution SHALL not fabricate a workstation.

#### Scenario: Assigned active run
- **GIVEN** an accepted run assigned to a workstation with a readable label
- **WHEN** the user views its task card, details or activity summary
- **THEN** each surface shows that workstation label, with device ID fallback when needed
- **AND** a disconnected active workstation is visibly unavailable without claiming the process ended

#### Scenario: Historical attribution
- **GIVEN** an assigned run has finished and the device is renamed, disconnected or removed later
- **WHEN** the activity is viewed after reload or restart
- **THEN** its assignment label remains readable and is not replaced by another workstation

#### Scenario: Legacy and multiple runs
- **GIVEN** a legacy run has no trustworthy assignment, or multiple visible runs have different assignments
- **WHEN** task and activity information is rendered
- **THEN** unknown attribution is left unknown and known assignments remain associated with their individual runs

### Requirement: Authorized explicit cancellation across active states
The task detail panel and activity view SHALL provide explicit cancellation for queued, pending and running executions to their owner or an admin. Cancellation SHALL preserve workflow handback, completion validation and one-active-run protection, including batch lifecycle protection.

#### Scenario: Undispatched queued or pending cancellation
- **GIVEN** an owned queued or pending execution is proven not dispatched
- **WHEN** its owner or an admin cancels it from either surface
- **THEN** it is canceled, cannot subsequently be dispatched and no longer blocks a new launch

#### Scenario: Running process confirmed stopped
- **GIVEN** an active agent execution
- **WHEN** an authorized cancellation receives confirmation from its owning workstation that the process stopped
- **THEN** the record becomes canceled, the UI reflects the result and a new launch is allowed

#### Scenario: Cancellation races dispatch or completion
- **GIVEN** cancellation overlaps worker claim, dispatch or completion
- **WHEN** those operations finish
- **THEN** one coherent terminal outcome is retained, a possibly dispatched process is not reported stopped without evidence, and no terminal execution later starts or reopens

#### Scenario: Permission refusal
- **GIVEN** a caller is neither the owner nor an admin, including an active legacy run without an owner
- **WHEN** they attempt cancellation or force closure through either endpoint
- **THEN** the server refuses it without changing the queue, process, activity or workflow
- **AND** an admin's permitted cancellation targets the recorded owner's workstation

#### Scenario: Client and server execution compatibility
- **GIVEN** an execution is owned by an MCP client or a registered server job rather than a local agent process
- **WHEN** an authorized user cancels it
- **THEN** its existing session release or job cancellation and workflow handback semantics remain effective

### Requirement: Evidence-based disconnected run recovery
Disconnection, transport timeouts and absence alone SHALL NOT expire an unconfirmed active execution. Proven orphaned executions MAY be automatically closed. Unconfirmed executions SHALL remain recoverable by explicit authorized force closure, distinct from confirmed stopping.

#### Scenario: Disconnection without termination evidence
- **GIVEN** a workstation disconnects while its local process may still execute
- **WHEN** transport or reconnect grace expires
- **THEN** the run remains active, the workstation appears unavailable and recovery actions remain accessible

#### Scenario: Proven orphan
- **GIVEN** authoritative reconciliation establishes that no owning agent process holds the execution
- **WHEN** recovery closes that orphan
- **THEN** the record becomes terminal through existing handback semantics and the task can launch again
- **AND** a non-owner reply or missing workstation alone cannot establish orphanhood

#### Scenario: Unconfirmed cancellation and force closure
- **GIVEN** a stop cannot be confirmed
- **WHEN** the user attempts ordinary cancellation
- **THEN** the run stays active and both recovery surfaces explain the failure and offer a separate force-close confirmation
- **WHEN** the owner or admin explicitly confirms force closure
- **THEN** the blocking record is canceled with a note that the local process may continue, and subsequent launch is permitted

#### Scenario: Reconnect or late completion after force close
- **GIVEN** an old run was force-closed and a new run may exist on the task
- **WHEN** the old device reconnects, reports status or submits completion
- **THEN** the old run is not resurrected, the newer run is unchanged and existing authenticated completion validation is not bypassed
- **AND** force closure itself does not advance the task workflow stage

### Requirement: Localized user experience and release documentation
New runtime controls and feedback SHALL use each surface's existing locale and accessible interaction conventions. Implementation SHALL document the visible selector, attribution and recovery changes in English under CHANGELOG.md Unreleased.

#### Scenario: Keyboard and locale
- **GIVEN** the user navigates the picker and recovery controls by keyboard in a supported locale
- **WHEN** they select, cancel or confirm an action
- **THEN** accessible labels, focus behavior and localized feedback describe the actual selected workstation and cancellation outcome

#### Scenario: Implementation publication
- **GIVEN** the visible changes are implemented
- **WHEN** the implementation is prepared for review
- **THEN** English changelog entries describe the user-visible launch and recovery changes without altering existing runtime language incidentally
