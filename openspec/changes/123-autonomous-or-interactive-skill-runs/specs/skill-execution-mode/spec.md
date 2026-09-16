## ADDED Requirements

### Requirement: A skill run has an execution mode
A skill launched on a task SHALL run in one of two modes. In **interactive** mode the provider
CLI runs in a terminal window the user can answer, and the workflow stage moves only when the
user confirms the session is over. In **autonomous** mode the CLI runs headless with no
terminal window, its output is streamed into the run activity, and the workflow stage is posted
by the worker without any user confirmation.

#### Scenario: An interactive run waits for the user
- **GIVEN** a task whose resolved mode is interactive
- **WHEN** the user starts the next step from the card
- **THEN** a terminal window opens with the provider CLI and the task prompt
- **AND** the stage does not move until the user confirms the session is over

#### Scenario: An autonomous run needs nobody
- **GIVEN** a task whose resolved mode is autonomous
- **WHEN** the user starts the next step from the card
- **THEN** no terminal window is opened
- **AND** the CLI output is recorded on the run activity
- **AND** the stage is posted by the worker when the run ends

#### Scenario: An autonomous run that fails
- **GIVEN** an autonomous run whose CLI exits with a failure
- **WHEN** the run ends
- **THEN** the run is recorded as failed with the captured output
- **AND** the task stays at the stage it was at

### Requirement: The mode is resolved per launch with a fixed precedence
The mode of a given launch SHALL be resolved in this order, the first defined value winning: the
one-off override chosen for that launch, then the skill's own setting, then the project default,
then interactive.

#### Scenario: A one-off override wins
- **GIVEN** a skill set to interactive on a project defaulting to autonomous
- **WHEN** the user launches it with an autonomous override
- **THEN** the run is autonomous
- **AND** neither the skill setting nor the project default is modified

#### Scenario: The skill setting wins over the project default
- **GIVEN** a project defaulting to autonomous and a skill set to interactive
- **WHEN** the user launches that skill with no override
- **THEN** the run is interactive

#### Scenario: The project default applies
- **GIVEN** a skill with no mode of its own on a project defaulting to autonomous
- **WHEN** the user launches that skill with no override
- **THEN** the run is autonomous

#### Scenario: Nothing is configured
- **GIVEN** a project with no default and a skill with no mode of its own
- **WHEN** the user launches that skill with no override
- **THEN** the run is interactive

### Requirement: The project carries a default skill mode
A project SHALL expose a default skill mode, `interactive` or `autonomous`, editable in the
project settings and defaulting to `interactive`. An absent or unrecognised value SHALL be read
as `interactive`.

#### Scenario: The default is editable
- **WHEN** the user sets the project default to autonomous and saves
- **THEN** the setting is persisted on the project
- **AND** later launches with no override and no skill setting are autonomous

#### Scenario: An existing project keeps today's behaviour
- **GIVEN** a project saved before this setting existed
- **WHEN** a skill is launched on one of its tasks
- **THEN** the run is interactive

### Requirement: A skill carries its own mode
Each skill SHALL carry its own execution mode, editable in the skill editor, which overrides the
project default for that skill. A skill with no mode of its own SHALL fall through to the
project default.

#### Scenario: The skill mode is editable
- **WHEN** the user changes a skill's mode in the skill editor and saves
- **THEN** the setting is persisted for that skill
- **AND** later launches of that skill with no override use it

#### Scenario: A skill with no opinion
- **WHEN** the user clears a skill's mode in the skill editor and saves
- **THEN** later launches of that skill with no override use the project default

### Requirement: Every explicit launch surface offers a one-off mode choice
Each surface on which the user explicitly triggers a skill SHALL let the user launch it in
either mode for that launch only, without changing any persisted setting. Those surfaces are
the task card's `...` menu and the task detail modal's skill launcher on the web, and the
"Launch" and "Relaunch" dialogs in the desktop app.

#### Scenario: Launching in the other mode from the card menu
- **GIVEN** a task whose resolved mode is interactive
- **WHEN** the user picks the autonomous launch from the `...` menu
- **THEN** that run is autonomous
- **AND** the next launch with no override is interactive again

#### Scenario: Launching in the other mode from the task detail modal
- **GIVEN** a task whose resolved mode is interactive
- **WHEN** the user launches a skill from the detail modal with the autonomous choice
- **THEN** that run is autonomous
- **AND** no skill or project setting is modified

#### Scenario: Launching in the other mode from the desktop app
- **GIVEN** a task whose resolved mode is interactive
- **WHEN** the user launches it from the desktop "Launch" dialog with the autonomous choice
- **THEN** that run is autonomous
- **AND** the choice does not apply to the next launch from the same dialog

#### Scenario: Relaunching in the other mode from the desktop app
- **GIVEN** a finished run that was interactive
- **WHEN** the user relaunches it from the desktop "Relaunch" dialog with the autonomous choice
- **THEN** the new run is autonomous

### Requirement: The desktop next-step button uses the resolved mode
The desktop app's next-step button SHALL launch the next step in one click, with no mode choice,
using the mode the precedence resolves for that skill.

#### Scenario: One click launches the resolved mode
- **GIVEN** a task whose resolved mode is autonomous
- **WHEN** the user presses the next-step button in the desktop app
- **THEN** the run starts autonomous without asking anything
- **AND** no dialog is opened

### Requirement: A desktop autonomous run stays visible in the app
An autonomous run started from the desktop app SHALL keep its row in the run list and an output
pane in the app, streaming the captured CLI output read-only rather than a terminal the user can
type into.

#### Scenario: Watching an autonomous run from the desktop app
- **GIVEN** an autonomous run started from the desktop app
- **WHEN** the user selects it in the run list
- **THEN** its output is displayed as it is produced
- **AND** the pane accepts no input
- **AND** the run can still be stopped and its log exported

### Requirement: A provider without a headless mode refuses an autonomous launch
An autonomous launch SHALL be refused, with an error naming the provider and the reason,
when the configured provider has no attested headless invocation, or when a custom AI command
template is configured without a mode placeholder. The launch SHALL NOT silently fall back to
interactive.

#### Scenario: A provider with no headless mode
- **GIVEN** a project configured with a provider that has no headless invocation
- **WHEN** an autonomous launch is requested
- **THEN** the launch is refused with an error naming the provider
- **AND** no terminal window is opened

#### Scenario: A custom template without a mode placeholder
- **GIVEN** a project configured with a custom AI command template carrying no mode placeholder
- **WHEN** an autonomous launch is requested
- **THEN** the launch is refused with an error saying the template decides the mode

#### Scenario: A provider with a headless mode
- **GIVEN** a project configured with a provider that has a headless invocation
- **WHEN** an autonomous launch is requested
- **THEN** the CLI is invoked in its headless form with the task prompt

#### Scenario: A refusal is readable on the surface that launched it
- **GIVEN** a project configured with a provider that has no headless invocation
- **WHEN** an autonomous launch is requested from the web or from the desktop app
- **THEN** the refusal is displayed on the surface the user launched from

#### Scenario: A full chain run on a provider with no headless mode
- **GIVEN** a project configured with a provider that has no headless invocation
- **WHEN** the user starts a full chain run
- **THEN** the run is refused with the same explicit error
- **AND** no step is enqueued
