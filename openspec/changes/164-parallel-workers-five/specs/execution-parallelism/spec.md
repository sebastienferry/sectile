## ADDED Requirements

### Requirement: Configurable worker ceiling
A project SHALL accept a background execution parallelism between 1 and 5 workers.
Any configured, stored or overridden value outside that range SHALL be clamped to
it, and a project without worktrees SHALL remain limited to a single execution.

#### Scenario: Selecting the maximum
- **GIVEN** a project using worktrees
- **WHEN** the user selects five parallel executions
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
Every surface SHALL enforce the same ceiling: the web project settings, the
desktop project mapping, the local agent override and the server-side job
limiter. A local agent override outside the range SHALL be rejected rather than
silently stored.

#### Scenario: Picker range
- **GIVEN** the web or desktop parallelism picker
- **WHEN** it is displayed for a project using worktrees
- **THEN** it offers every value from one to the ceiling

#### Scenario: Rejected override
- **GIVEN** a local agent mapping request
- **WHEN** it carries a parallelism outside the range
- **THEN** the request is rejected and no override is stored
