## ADDED Requirements

### Requirement: Provider-aware PTY invocation
The system SHALL invoke a skill by its plain name for Codex and preserve the slash syntax for other providers in both manual injection and workflow execution in an existing PTY agent.

#### Scenario: Codex manual action
- **GIVEN** a task configured for Codex and an agent running in its terminal
- **WHEN** the user selects clarify
- **THEN** the injected line starts with `clarify-issue` without `/` and retains the task key, title and tracker.

#### Scenario: Codex automatic action
- **GIVEN** a workflow step reusing a running Codex terminal
- **WHEN** it invokes a skill
- **THEN** the same plain-name invocation is used.

#### Scenario: Other provider
- **GIVEN** a terminal configured for Claude or another existing provider
- **WHEN** a skill is invoked
- **THEN** its command retains the existing leading slash.

### Requirement: Project settings and context preservation
The system SHALL prefer the project's provider over the global provider, honor project skill names, and preserve existing context formatting and no-agent guards.

#### Scenario: Renamed skill
- **GIVEN** a Codex project overriding clarify with `/clarify-workitem`
- **WHEN** the skill is invoked
- **THEN** its name is `clarify-workitem` and its title is flattened to a single line.

#### Scenario: Global provider fallback
- **GIVEN** a project without a provider override and a global Codex provider
- **WHEN** a skill is invoked
- **THEN** the Codex syntax is used.

#### Scenario: No running agent
- **GIVEN** no agent is running in the task terminal
- **WHEN** a manual skill action is requested
- **THEN** the action is rejected without injecting a shell command.

### Requirement: Accurate terminal actions
The terminal SHALL display the provider-appropriate skill name in button labels and tooltips, including project skill overrides.

#### Scenario: Codex action label
- **GIVEN** a Codex project with a custom skill name
- **WHEN** the terminal renders its actions
- **THEN** the button and tooltip show that name without a slash.

### Requirement: Interactive Codex launch
The system SHALL support launching Codex with the existing interactive binary resolution behavior.

#### Scenario: Codex executable available
- **GIVEN** Codex is configured and its executable is available
- **WHEN** the user launches the agent
- **THEN** the task terminal starts that executable using the existing launch path.
