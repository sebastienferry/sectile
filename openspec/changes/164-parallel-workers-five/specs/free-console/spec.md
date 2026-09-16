## MODIFIED Requirements

### Requirement: Reuse of the local execution platform
Free consoles SHALL use authenticated local IPC/HTTP admission and SHALL reuse the existing execution queue, shared-checkout serialization, process supervision, stop, replay, and history cleanup. A free console SHALL NOT consume background worker capacity: it SHALL NOT be counted in a project's active executions, and its own admission SHALL NOT be subject to the project's parallelism limit.

#### Scenario: Shared checkout serialization
- **GIVEN** an execution that works in the project's mapped checkout rather than in its own worktree
- **WHEN** a free console is requested on that project
- **THEN** it waits in the existing queue rather than running concurrently in that checkout.

#### Scenario: Worktree-isolated executions do not hold the console back
- **GIVEN** executions running in their own worktrees
- **WHEN** a free console is requested on that project
- **THEN** it is admitted, because it works in the mapped checkout that those executions do not use.

#### Scenario: Two consoles
- **GIVEN** a running free console on a project
- **WHEN** a second free console is requested on the same project
- **THEN** both run concurrently, since the user opened each one deliberately.

#### Scenario: Supervised stop
- **GIVEN** a running free console
- **WHEN** the user stops it
- **THEN** the supervised process terminates and its capacity is released.

#### Scenario: Console admitted on a saturated project
- **GIVEN** a project running as many background executions as its parallelism allows, in their own worktrees
- **WHEN** a free console is requested on that project
- **THEN** it is admitted without waiting for a background execution to finish.

#### Scenario: Console does not consume a worker slot
- **GIVEN** a running free console on a project
- **WHEN** background executions are admitted on that project
- **THEN** the console is not counted among the active executions
- **AND** the project may still run its full number of background workers.
