# agent-provider-setup Specification

## Purpose
TBD - created by archiving change 113-provider-scoped-agent-setup. Update Purpose after archive.
## Requirements
### Requirement: Skills install for the agents the project sets up
The agent SHALL install the managed skill set into the user-level configuration folder of each
agent the project sets up, and into no other provider's location. An agent with no supported skill
convention SHALL receive no managed skill files, and the dispatch SHALL proceed.

#### Scenario: Provider with a skill convention
- **GIVEN** a project whose selected AI provider supports managed skills
- **WHEN** a task is dispatched
- **THEN** the managed skills exist under that provider's user-level configuration folder
- **AND** no managed skill file is written under any other provider's location

#### Scenario: Provider without a skill convention
- **GIVEN** a project whose selected AI provider has no supported skill convention
- **WHEN** a task is dispatched
- **THEN** no managed skill file is written
- **AND** the dispatch proceeds and the agent is launched

#### Scenario: Selected provider changes
- **GIVEN** managed skills previously installed for one provider
- **WHEN** the project's selected provider changes and a task is dispatched
- **THEN** the new provider's location holds the current managed skills
- **AND** the previous provider's unedited managed skills are removed

### Requirement: A project chooses the agents to set up
A project SHALL declare the additional agents to set up, among those Sectile supports installing
for. The agent that runs the tasks SHALL always be set up whether or not it is declared, because it
cannot work without its MCP registration. An unsupported agent SHALL be rejected rather than
silently ignored. Unchecking an agent SHALL retire its installation like any other retirement.

#### Scenario: Set up several agents
- **GIVEN** a project that declares additional agents alongside the one running its tasks
- **WHEN** a task is dispatched
- **THEN** each declared agent and the running agent hold the managed skills and the MCP registration
- **AND** no other agent's configuration is created

#### Scenario: Stop setting up an agent
- **GIVEN** an agent previously set up and now no longer declared
- **WHEN** a task is dispatched
- **THEN** its unedited managed files are removed and the remaining agents keep theirs

#### Scenario: Unsupported agent declared
- **GIVEN** a project declaring an agent Sectile cannot set up
- **WHEN** a task is dispatched
- **THEN** the dispatch fails with an error naming that agent

### Requirement: MCP registers in user-level provider configuration
The agent SHALL register the Sectile MCP server in the user-level configuration of every agent the
project sets up, preserving unrelated registrations and settings, and SHALL NOT persist credentials.
A registration that cannot be written SHALL abort the dispatch with an actionable error and leave
the existing configuration unchanged.

#### Scenario: Register before launch
- **GIVEN** a supported provider and a running loopback agent gateway
- **WHEN** a task is dispatched
- **THEN** the provider's user-level configuration contains exactly one managed `sectile` registration
  pointing at the current agent executable and gateway
- **AND** no MCP configuration file is created inside the repository or the worktree

#### Scenario: Registration cannot be written
- **GIVEN** a provider whose registration target is unsupported, malformed or unwritable
- **WHEN** a task is dispatched
- **THEN** the dispatch fails with an error naming the provider and the target
- **AND** the existing configuration file is left unchanged and no agent process is launched

### Requirement: Checkouts receive no managed agent configuration
Preparing a dispatch SHALL NOT create or modify managed skill files, managed command files, MCP
registration files or the managed project-context block inside the repository or any worktree.
Local state the agent owns under `.taskflow/` remains permitted.

#### Scenario: Clean checkout after dispatch
- **GIVEN** a repository with no Sectile-managed files under version control
- **WHEN** a task is dispatched and the agent runs
- **THEN** the working tree shows no added or modified skill, command, MCP or project-context file

#### Scenario: Retire previously scaffolded copies
- **GIVEN** a checkout carrying managed files from an earlier release, recorded in the local manifest
- **WHEN** a task is dispatched
- **THEN** the copies whose content still matches the recorded managed content are backed up and removed
- **AND** copies the user has edited are left in place and reported as preserved

#### Scenario: Personal files are never touched
- **GIVEN** skill, command or MCP files the user authored at paths Sectile has never managed
- **WHEN** a task is dispatched
- **THEN** those files are unchanged

### Requirement: Managed skills carry no project-specific content
Managed skill content SHALL be identical for every project using the same provider and skill
catalogue. Project identity, tracker configuration and workflow settings SHALL reach the agent
through the dispatch prompt and the `get_project_context` MCP tool.

#### Scenario: Two projects on one workstation
- **GIVEN** two connected projects with different trackers sharing one selected provider
- **WHEN** a task is dispatched from each in turn
- **THEN** the installed skill content is identical after both dispatches
- **AND** each running agent resolves its own project identity through the MCP interface

#### Scenario: Per-skill override still applies
- **GIVEN** a workstation override supplying custom content for a skill
- **WHEN** a task is dispatched
- **THEN** the installed skill carries the override content

### Requirement: Installed skills remain readable by the desktop
The desktop SHALL read managed skill content back from the location where it was installed for the
selected provider, and SHALL report that no managed skills are installed when the provider has no
skill convention.

#### Scenario: Inspect installed skills
- **GIVEN** a project whose managed skills are installed for its selected provider
- **WHEN** the user opens the project's AI skills view
- **THEN** each skill's installed content and its actual path are shown

#### Scenario: Provider without installed skills
- **GIVEN** a project whose selected provider has no skill convention
- **WHEN** the user opens the project's AI skills view
- **THEN** the view states that no managed skills are installed for that provider, without error

