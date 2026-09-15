## ADDED Requirements

### Requirement: A skill run has an execution mode
A skill launched on a task SHALL run in one of two modes. In **interactive** mode the provider
CLI runs in a terminal window the user can answer, and the workflow stage moves only when the
user confirms the session is over. In **non interactive** mode the CLI runs headless with no
terminal window, its output is streamed into the run activity, and the workflow stage is posted
by the worker without any user confirmation.

#### Scenario: An interactive run waits for the user
- **GIVEN** a task whose resolved mode is interactive
- **WHEN** the user starts the next step from the card
- **THEN** a terminal window opens with the provider CLI and the task prompt
- **AND** the stage does not move until the user confirms the session is over

#### Scenario: A non-interactive run needs nobody
- **GIVEN** a task whose resolved mode is non interactive
- **WHEN** the user starts the next step from the card
- **THEN** no terminal window is opened
- **AND** the CLI output is recorded on the run activity
- **AND** the stage is posted by the worker when the run ends

#### Scenario: A non-interactive run that fails
- **GIVEN** a non-interactive run whose CLI exits with a failure
- **WHEN** the run ends
- **THEN** the run is recorded as failed with the captured output
- **AND** the task stays at the stage it was at

### Requirement: The mode is resolved per launch with a fixed precedence
The mode of a given launch SHALL be resolved in this order, the first defined value winning: the
one-off override chosen for that launch, then the skill's own setting, then the project default,
then interactive.

#### Scenario: A one-off override wins
- **GIVEN** a skill set to interactive on a project defaulting to non interactive
- **WHEN** the user launches it with a non-interactive override from the card menu
- **THEN** the run is non interactive
- **AND** neither the skill setting nor the project default is modified

#### Scenario: The skill setting wins over the project default
- **GIVEN** a project defaulting to non interactive and a skill set to interactive
- **WHEN** the user launches that skill with no override
- **THEN** the run is interactive

#### Scenario: The project default applies
- **GIVEN** a skill with no mode of its own on a project defaulting to non interactive
- **WHEN** the user launches that skill with no override
- **THEN** the run is non interactive

#### Scenario: Nothing is configured
- **GIVEN** a project with no default and a skill with no mode of its own
- **WHEN** the user launches that skill with no override
- **THEN** the run is interactive

### Requirement: The project carries a default skill mode
A project SHALL expose a default skill mode, `interactive` or `non_interactive`, editable in the
project settings and defaulting to `interactive`. An absent or unrecognised value SHALL be read
as `interactive`.

#### Scenario: The default is editable
- **WHEN** the user sets the project default to non interactive and saves
- **THEN** the setting is persisted on the project
- **AND** later launches with no override and no skill setting are non interactive

#### Scenario: An existing project keeps today's behaviour
- **GIVEN** a project saved before this setting existed
- **WHEN** a skill is launched on one of its tasks
- **THEN** the run is interactive

### Requirement: A skill carries its own mode
Each skill SHALL carry its own execution mode, editable in the skill editor, which overrides the
project default for that skill.

#### Scenario: The skill mode is editable
- **WHEN** the user changes a skill's mode in the skill editor and saves
- **THEN** the setting is persisted for that skill
- **AND** later launches of that skill with no override use it

### Requirement: The card menu offers a one-off mode choice
The card's `...` menu SHALL let the user launch the next step in either mode for that launch
only, without changing any persisted setting.

#### Scenario: Launching in the other mode from the menu
- **GIVEN** a task whose resolved mode is interactive
- **WHEN** the user picks the non-interactive launch from the `...` menu
- **THEN** that run is non interactive
- **AND** the next launch with no override is interactive again

### Requirement: A provider without a headless mode refuses a non-interactive launch
A non-interactive launch SHALL be refused, with an error naming the provider and the reason,
when the configured provider has no attested headless invocation, or when a custom AI command
template is configured without a mode placeholder. The launch SHALL NOT silently fall back to
interactive.

#### Scenario: A provider with no headless mode
- **GIVEN** a project configured with a provider that has no headless invocation
- **WHEN** a non-interactive launch is requested
- **THEN** the launch is refused with an error naming the provider
- **AND** no terminal window is opened

#### Scenario: A custom template without a mode placeholder
- **GIVEN** a project configured with a custom AI command template carrying no mode placeholder
- **WHEN** a non-interactive launch is requested
- **THEN** the launch is refused with an error saying the template decides the mode

#### Scenario: A provider with a headless mode
- **GIVEN** a project configured with a provider that has a headless invocation
- **WHEN** a non-interactive launch is requested
- **THEN** the CLI is invoked in its headless form with the task prompt

#### Scenario: An autonomous run on a provider with no headless mode
- **GIVEN** a project configured with a provider that has no headless invocation
- **WHEN** the user starts an autonomous run
- **THEN** the run is refused with the same explicit error
- **AND** no step is enqueued
