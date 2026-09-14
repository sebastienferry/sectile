## ADDED Requirements

### Requirement: Free agent console entry point
Sectile Desktop SHALL offer an Open agent console action on a configured project, letting the user choose between the Codex and Claude providers. The action SHALL be placed after the existing task actions so that established keyboard navigation is preserved.

#### Scenario: Opening a console from a configured project
- **GIVEN** a project connected to a local repository mapping
- **WHEN** the user triggers Open agent console and selects a provider
- **THEN** Desktop opens a console for that provider in the mapped repository
- **AND** no task, skill, or workflow stage is selected.

#### Scenario: Provider selection is required
- **GIVEN** the agent selector is open
- **WHEN** no provider has been chosen
- **THEN** no execution is started.

### Requirement: Prompt-free launch
A free console SHALL invoke only the selected provider executable, with no arguments, in the project's mapped repository, and SHALL preserve that CLI's normal interactive permission defaults. Workflow command templates and task instructions SHALL NOT be applied, and any inherited task context SHALL be cleared.

#### Scenario: Launch carries no prompt
- **GIVEN** a provider has been selected
- **WHEN** the console launches
- **THEN** the command line contains the executable alone
- **AND** no prompt, skill, or workflow template contributes arguments.

#### Scenario: No branch or workflow scaffolding
- **GIVEN** a free console is admitted
- **WHEN** it is queued and launched
- **THEN** no branch is created or changed
- **AND** no workflow artifact is scaffolded.

### Requirement: Independent console identity
Each free console launch SHALL have a unique local identity and SHALL carry no task, skill, prompt, workflow stage, or pull request association. The renderer SHALL group free consoles by run identifier rather than by task.

#### Scenario: Two consoles on the same project
- **GIVEN** a project with one running free console
- **WHEN** the user opens a second console on the same project
- **THEN** both appear as separate entries with their own terminals
- **AND** stopping one leaves the other running.

### Requirement: Reuse of the local execution platform
Free consoles SHALL use authenticated local IPC/HTTP admission and SHALL reuse the existing execution queue, shared-checkout serialization, process supervision, stop, replay, and history cleanup.

#### Scenario: Shared checkout serialization
- **GIVEN** another execution already holds the project's shared checkout
- **WHEN** a free console is requested
- **THEN** it waits in the existing queue rather than running concurrently in that checkout.

#### Scenario: Supervised stop
- **GIVEN** a running free console
- **WHEN** the user stops it
- **THEN** the supervised process terminates and its capacity is released.

### Requirement: Console survives the desktop session
A free console SHALL keep running when the desktop application is closed, and SHALL be rediscovered from the daemon's run list when the application is reopened.

#### Scenario: Reopening the desktop application
- **GIVEN** a running free console and a closed desktop application
- **WHEN** the user reopens Desktop
- **THEN** the console is listed again from the daemon
- **AND** its terminal history is replayed.

### Requirement: Admission rejection and capacity release
Admission SHALL be rejected for an invalid provider, a disconnected project, a missing repository mapping, or a daemon shutdown. Queue cancellation and launch failure SHALL release the reserved capacity.

#### Scenario: Invalid or unavailable target
- **GIVEN** an unknown provider, a disconnected project, or a missing mapping
- **WHEN** a console launch is requested
- **THEN** admission is rejected with an explanation
- **AND** no process is started.

#### Scenario: Launch failure releases capacity
- **GIVEN** an admitted console whose executable fails to start
- **WHEN** the failure is observed
- **THEN** the reserved execution capacity is released
- **AND** the user can retry the launch.

### Requirement: Isolation from task-based execution
Task-based execution behavior SHALL remain intact. A free console SHALL display process status without requesting task metadata and SHALL NOT post workflow completion to the server.

#### Scenario: No task traffic for a free console
- **GIVEN** a running free console
- **WHEN** it runs and then exits
- **THEN** no task metadata or task result request is issued
- **AND** no task is created or updated on the tracker.
