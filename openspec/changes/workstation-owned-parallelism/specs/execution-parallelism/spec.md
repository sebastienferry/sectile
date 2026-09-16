## MODIFIED Requirements

### Requirement: Configurable worker ceiling
A workstation SHALL accept a background execution parallelism between 1 and 5
workers for a project. Any stored or overridden value outside that range SHALL be
clamped to it, a project without a workstation value SHALL run a single
execution, and a project without worktrees SHALL remain limited to a single
execution.

#### Scenario: Selecting the maximum
- **GIVEN** a project using worktrees
- **WHEN** the user selects five parallel executions in the desktop app
- **THEN** the setting is accepted and five background executions may run concurrently

#### Scenario: Out-of-range value
- **GIVEN** a stored or overridden parallelism of 0, of 9, or absent
- **WHEN** the effective limit is computed
- **THEN** it is 1 for a value below the range, 5 for a value above it, and 1 when absent

#### Scenario: Shared checkout
- **GIVEN** a project that does not use worktrees
- **WHEN** its effective limit is computed
- **THEN** it is 1 regardless of the configured parallelism

### Requirement: Consistent ceiling across surfaces
Parallelism SHALL be workstation-owned. The server SHALL NOT store it on a
project, SHALL NOT expose it in the project API and SHALL NOT carry it in the
agent configuration payload; its own skill-job queue SHALL run one worker per
project. The desktop project mapping and the local agent override SHALL enforce
the same ceiling, and an override outside the range SHALL be rejected rather than
silently stored.

#### Scenario: Picker range
- **GIVEN** the desktop parallelism picker
- **WHEN** it is displayed for a project using worktrees
- **THEN** it offers every value from one to the ceiling, with no inheritance control

#### Scenario: Rejected override
- **GIVEN** a local agent mapping request
- **WHEN** it carries a parallelism outside the range
- **THEN** the request is rejected and no override is stored

#### Scenario: No server-side setting
- **GIVEN** a project read from the server API or an agent configuration download
- **WHEN** its payload is inspected
- **THEN** it carries no parallelism field, and the workstation value alone decides the limit
