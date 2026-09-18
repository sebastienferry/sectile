## ADDED Requirements

### Requirement: A launch override outranks every configured level for that run
A skill launch on a task SHALL accept an optional model identifier. When one is given, that run SHALL use it in place of the model resolved from the workstation override, the project and the global settings, including any per-skill entry those levels carry. The override SHALL apply to that run only and SHALL NOT be written to the project, to the global settings or to the workstation configuration. When none is given, the run SHALL use the model the configured levels resolve, exactly as before.

#### Scenario: Override wins over a per-skill project entry
- **GIVEN** a project whose per-skill map names `implement -> claude-sonnet-5`
- **WHEN** the user launches `implement` on one of its tasks with the model `claude-opus-5`
- **THEN** the command line of that run carries `claude-opus-5`
- **AND** the project's per-skill map still names `claude-sonnet-5` afterwards

#### Scenario: Override wins over the workstation override
- **GIVEN** a workstation whose `.taskflow/agent.json` sets a model for the project
- **WHEN** the user launches a skill with another model
- **THEN** the command line of that run carries the launched model
- **AND** the workstation file is unchanged

#### Scenario: No override reproduces the configured command line
- **GIVEN** a task whose configured levels resolve model `M` for skill `S`
- **WHEN** the user launches `S` without giving a model
- **THEN** the command line is byte for byte the one built before this change, carrying `M`

#### Scenario: No override and nothing configured
- **GIVEN** a task with no model configured at any level
- **WHEN** the user launches a skill without giving a model
- **THEN** the command line carries no model flag, exactly as before

#### Scenario: The next launch is not affected
- **GIVEN** a run launched with a model override has finished
- **WHEN** the user launches the same skill on the same task without a model
- **THEN** the run uses the model the configured levels resolve

### Requirement: The override applies in both execution modes and on both launch paths
A launch override SHALL reach the command line whether the run is interactive or autonomous, and whether the launch is dispatched directly to a connected agent or filed through the server queue.

#### Scenario: Autonomous launch with an override
- **GIVEN** a task launched autonomously with model `M`
- **WHEN** the agent builds the headless command line
- **THEN** it carries `M`

#### Scenario: Interactive launch with an override
- **GIVEN** a task launched interactively with model `M`
- **WHEN** the agent opens the session
- **THEN** the session command line carries `M`

#### Scenario: Queued launch keeps the override
- **GIVEN** a launch that the server files as a job rather than dispatching directly
- **WHEN** the worker hands the job to the agent
- **THEN** the model given at launch is carried unchanged to the agent

### Requirement: The launch override obeys the template and provider rules
A launch override SHALL enter the command line only through the rules that already govern a configured model: through the `{model}` placeholder when a command template is in charge, and as a `--model` flag only for providers that accept it. A provider that takes no model SHALL ignore the override without failing.

#### Scenario: Template with the placeholder
- **GIVEN** a project whose command template contains `{model}`
- **WHEN** a skill is launched with model `M`
- **THEN** `{model}` expands to `M` and no extra flag is injected

#### Scenario: Template without the placeholder
- **GIVEN** a project whose command template contains no `{model}`
- **WHEN** a skill is launched with a model
- **THEN** the template runs unchanged and the launch is not refused

#### Scenario: Provider without a model flag
- **GIVEN** a project on a provider that takes no model
- **WHEN** a skill is launched with a model
- **THEN** the command line carries no model and the launch is not refused

### Requirement: A launch override is validated on shape at every boundary
A launch override SHALL be checked with the same shape rule as the configured model in the web interface, in the server handlers and in the local agent. A value the rule rejects SHALL be refused before any run is recorded or any command line is built, with an error naming the value. An empty value SHALL mean "no override".

#### Scenario: Malformed identifier at the web interface
- **GIVEN** the user types a model containing a space or a shell metacharacter
- **WHEN** the launch surface renders
- **THEN** the field is marked invalid with the same hint as the configuration field
- **AND** the launch controls of that surface are disabled

#### Scenario: Malformed identifier reaching the server
- **GIVEN** a launch request whose model the shape rule rejects
- **WHEN** the server receives it
- **THEN** it answers with a client error naming the value
- **AND** no run is recorded and nothing is dispatched

#### Scenario: Malformed identifier reaching the agent
- **GIVEN** a dispatch whose model the shape rule rejects
- **WHEN** the agent prepares the launch
- **THEN** it refuses the launch with an error naming the value
- **AND** no command line is built

#### Scenario: Well-formed but unknown identifier
- **GIVEN** a model identifier the rule accepts that no suggestion list contains
- **WHEN** it is given at launch
- **THEN** it is accepted and reaches the command line unchanged

### Requirement: A run record names the engine and model it used
Each run record SHALL carry the provider and the model the run ran against. At launch the record SHALL hold the value the server resolves, including the launch override when one was given. Once the agent has built the command line it SHALL report the provider and model actually placed on it, and the record SHALL be updated with that report. Runs recorded before this capability SHALL read as unknown rather than as any particular model.

#### Scenario: Finished run shows its engine and model
- **GIVEN** a run launched with model `M` on provider `P` that has finished
- **WHEN** the user opens the task's activity list or the run badge
- **THEN** the run names `P` and `M` beside the skill name

#### Scenario: Agent report corrects the server's resolution
- **GIVEN** a run launched without an override on a workstation whose local override names `W`, while the server resolves `M`
- **WHEN** the agent reports the engine it actually launched
- **THEN** the run record names `W`

#### Scenario: No model at all
- **GIVEN** a run with no model configured or given, on a provider that would accept one
- **WHEN** the run is displayed
- **THEN** it names the provider and no model

#### Scenario: Older run
- **GIVEN** a run recorded before this capability existed
- **WHEN** it is displayed
- **THEN** no provider or model is shown for it

#### Scenario: Report on a finished run is refused
- **GIVEN** a run that is no longer running
- **WHEN** an engine report arrives for it
- **THEN** the report is refused and the record is unchanged

#### Scenario: Desktop run list
- **GIVEN** a task run that carries a provider and a model
- **WHEN** the desktop app lists it
- **THEN** its label names the skill, the provider and the model
- **AND** a free console's label is unchanged
