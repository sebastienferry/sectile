## MODIFIED Requirements

### Requirement: Consistent ceiling across surfaces
Parallelism SHALL be workstation-owned. The server SHALL NOT store it on a
project, SHALL NOT expose it in the project API and SHALL NOT carry it in the
agent configuration payload; its own skill-job queue SHALL run one worker per
project. The desktop project mapping and the workstation executable's
configuration command SHALL both write the workstation settings file, SHALL
enforce the same ceiling, and an override outside the range SHALL be rejected
rather than silently stored.

#### Scenario: Picker range
- **GIVEN** the desktop parallelism picker
- **WHEN** it is displayed for a project using worktrees
- **THEN** it offers every value from one to the ceiling, with no inheritance control

#### Scenario: Command-line writer
- **GIVEN** a workstation running the agent without the desktop app
- **WHEN** the parallelism of a project is set from the workstation executable's configuration command
- **THEN** the value is stored in the same workstation settings the desktop mapping writes, and the next execution admitted for that project uses it without restarting the agent

#### Scenario: Command-line reader
- **GIVEN** a project with no stored parallelism
- **WHEN** the configuration command reports the workstation settings
- **THEN** the project is shown as running a single execution by default rather than as having no value

#### Scenario: Rejected override
- **GIVEN** a local agent mapping request or a configuration command
- **WHEN** it carries a parallelism outside the range
- **THEN** the request is rejected and no override is stored

#### Scenario: No server-side setting
- **GIVEN** a project read from the server API or an agent configuration download
- **WHEN** its payload is inspected
- **THEN** it carries no parallelism field, and the workstation value alone decides the limit
