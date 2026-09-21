## ADDED Requirements

### Requirement: An autonomous preset emits readable output as the run proceeds
Every shipped autonomous command preset SHALL produce readable text progressively, while the
provider CLI is still running. A preset SHALL NOT buffer the whole run before emitting its first
line, and SHALL NOT discard the events that precede the final answer.

#### Scenario: Progress reaches the surface before the run ends
- **GIVEN** an autonomous run started from a shipped preset
- **WHEN** the provider CLI has emitted events but has not exited
- **THEN** the text rendered from those events is already recorded on the run
- **AND** it is recorded within one flush interval of the events being emitted

#### Scenario: A run that dies mid-stream still shows what it did
- **GIVEN** an autonomous run started from a shipped preset
- **WHEN** the provider CLI exits abnormally before emitting a terminal result event
- **THEN** the run holds the text rendered from every event received before the exit

#### Scenario: The final answer is distinguishable
- **GIVEN** an autonomous run that completes normally
- **WHEN** its output is read
- **THEN** the provider's final answer appears in full
- **AND** it is set apart from the progress lines that precede it

#### Scenario: No preset slurps the stream
- **GIVEN** the shipped autonomous presets for the supported providers
- **WHEN** their command lines are inspected
- **THEN** none of them invokes a filter in slurp mode

### Requirement: A preset tolerates output that is not a JSON event
A shipped autonomous preset SHALL pass through a line that is not a parsable provider event
rather than failing, so a diagnostic written to the merged output stream stays visible.

#### Scenario: A warning printed by the CLI
- **GIVEN** an autonomous run whose CLI writes a plain-text warning to its error stream
- **WHEN** that line reaches the preset's filter
- **THEN** the line appears verbatim in the run output
- **AND** the events printed after it are still rendered

#### Scenario: An event shape the filter does not recognise
- **GIVEN** a provider emits a JSON event whose fields the filter does not know
- **WHEN** that event is rendered
- **THEN** something representing the event appears in the output
- **AND** the run is not left with an empty output panel

### Requirement: The command previews match the commands that run
The command line shown in the settings previews SHALL be the command line the agent builds for
the same provider, model and mode.

#### Scenario: The previewed autonomous command
- **GIVEN** a provider whose shipped autonomous preset has changed
- **WHEN** the web preview and the desktop preview are rendered for that provider
- **THEN** both show the same command line the agent would run

### Requirement: A configured command is never rewritten
Changing a shipped preset SHALL NOT alter a command a user has already configured for a profile
or a project.

#### Scenario: An existing custom autonomous command
- **GIVEN** a project whose autonomous command was configured before this change
- **WHEN** an autonomous run is launched for that project
- **THEN** the command that runs is the configured one, unchanged

### Requirement: A missing pipeline prerequisite is stated, not silent
A headless run SHALL fail with a message naming the missing tool when its command line depends
on a tool that is not available on the workstation, rather than completing or leaving an empty
output.

#### Scenario: jq is not installed
- **GIVEN** a workstation whose PATH has no `jq`
- **WHEN** an autonomous run is launched with a command line that pipes through `jq`
- **THEN** the run is reported as failed
- **AND** its output names `jq` as the missing prerequisite

#### Scenario: A stage of the pipeline fails
- **GIVEN** an autonomous run whose provider CLI exits non-zero while the last stage of its
  pipeline exits zero
- **WHEN** the run finishes
- **THEN** the run is reported as failed, not completed
