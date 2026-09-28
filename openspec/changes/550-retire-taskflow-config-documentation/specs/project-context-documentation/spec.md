## ADDED Requirements

### Requirement: Current documentation identifies the live project-context source

Current operational documentation SHALL state that an agent receives project
and remote-tracker context through its authenticated Sectile MCP connection and
dispatch context, rather than from the retired per-checkout
`.taskflow/config.json` file.

#### Scenario: A maintainer follows repository workflow instructions

- **GIVEN** a maintainer needs the project or remote-tracker context for a
  repository task
- **WHEN** they read the current repository workflow instructions
- **THEN** the instructions identify the live Sectile MCP project-context flow
- **AND** they do not identify `.taskflow/config.json` as a source of truth

#### Scenario: A maintainer reads the agent architecture and protocol contract

- **GIVEN** a maintainer reads current architecture or server-agent contract
  documentation
- **WHEN** they determine what configuration artifacts the agent writes or
  consumes
- **THEN** the documentation does not describe `.taskflow/config.json` as an
  active agent-written or configuration artifact
- **AND** it preserves the documented roles of `.taskflow/agent.json` and
  `.taskflow/remote-config.json`

#### Scenario: A maintainer investigates the retirement history

- **GIVEN** a maintainer reads archived specifications or clarification reports
- **WHEN** those historical records refer to `.taskflow/config.json`
- **THEN** the historical references remain available as records of the retired
  behavior
