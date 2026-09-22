## ADDED Requirements

### Requirement: An unsupervised run shows what it is doing
A run launched with no terminal SHALL present, while it is running, the prose the engine writes and
the tools it calls, in the order they happen.

#### Scenario: Watching a run that has just started
- **GIVEN** an autonomous run launched on an engine whose reasoning stream is read
- **WHEN** the user selects that run in the desktop
- **THEN** the console pane shows the run's trace instead of a notice saying there is no terminal
- **AND** each new event appears as the engine produces it

#### Scenario: Attaching halfway through
- **GIVEN** an autonomous run that has been working for several minutes
- **WHEN** the user selects it for the first time
- **THEN** the pane shows what the run has already done, up to the retained limit
- **AND** it keeps showing the events that follow

#### Scenario: A tool call names what it was about
- **GIVEN** the engine calls a tool with arguments
- **WHEN** the call is shown in the trace
- **THEN** the line names the tool
- **AND** it carries the argument that says what the call was about, shortened to one readable line

#### Scenario: The run ends
- **GIVEN** an autonomous run whose trace the user is watching
- **WHEN** the run finishes
- **THEN** the trace stops and what it showed remains readable
- **AND** the run's final answer is on the task activity as before

### Requirement: The trace is read-only
A trace SHALL NOT offer any way to answer the run it describes.

#### Scenario: Typing into the pane
- **GIVEN** the user has an autonomous run's trace open
- **WHEN** they type in the pane
- **THEN** nothing is sent to the run
- **AND** the trace keeps streaming

### Requirement: Only an engine attested to stream is asked to
The reasoning stream SHALL be requested only on a headless launch of an engine known to produce one,
and never on an interactive launch or on a command the user configured.

#### Scenario: A headless launch on the attested engine
- **GIVEN** a project configured with the Claude engine and no command template
- **WHEN** a skill is launched autonomously
- **THEN** the command line carries the engine's streaming options

#### Scenario: Another engine
- **GIVEN** a project configured with an engine whose reasoning stream is not attested
- **WHEN** a skill is launched autonomously
- **THEN** its command line is exactly the one it had before
- **AND** the desktop presents its run as an autonomous run with no trace

#### Scenario: A configured command template
- **GIVEN** a project whose AI command template builds the command line itself
- **WHEN** a skill is launched autonomously
- **THEN** the template is expanded unchanged, with no option added to it

#### Scenario: An interactive launch
- **GIVEN** any project
- **WHEN** a skill is launched interactively
- **THEN** the command line carries no streaming option
- **AND** the session behaves as it does today

### Requirement: The task activity keeps recording the run's output
Reading the stream SHALL NOT change what a run records on its task.

#### Scenario: The engine's answer
- **GIVEN** an autonomous run whose stream is read
- **WHEN** the run completes
- **THEN** the task activity holds the engine's final answer, as it does for a run whose stream is not read
- **AND** it holds none of the stream's protocol lines

#### Scenario: A run that fails
- **GIVEN** an autonomous run whose engine prints a diagnostic that is not part of the stream
- **WHEN** that line is produced
- **THEN** it is recorded on the task activity
- **AND** the run's status reports the failure as before

### Requirement: The trace never compromises the run
No failure in producing, holding or delivering a trace SHALL affect the run it describes.

#### Scenario: An unreadable line
- **GIVEN** the engine prints a line the reader cannot interpret
- **WHEN** the line is read
- **THEN** it produces no trace event and no error
- **AND** the run continues

#### Scenario: Nobody is watching
- **GIVEN** an autonomous run with no desktop attached to it
- **WHEN** the run produces trace events
- **THEN** they are retained up to the limit
- **AND** the run completes normally

#### Scenario: A watcher that stops reading
- **GIVEN** a desktop attached to a trace stops consuming it
- **WHEN** new events are produced
- **THEN** that watcher is dropped
- **AND** the run and any other watcher are unaffected

#### Scenario: A run that produces a very long trace
- **GIVEN** an autonomous run producing far more events than the retained limit
- **WHEN** the user attaches
- **THEN** the most recent events are shown
- **AND** the agent's retained trace stays bounded

### Requirement: An agent that cannot trace is presented as one
A desktop SHALL keep presenting an autonomous run with no trace the way it does today, whatever the
version of the agent running it.

#### Scenario: An older agent
- **GIVEN** a desktop connected to an agent that does not report a trace on its runs
- **WHEN** the user selects an autonomous run
- **THEN** the pane shows the notice explaining that the run has no terminal and that its output is
  on the task activity
- **AND** nothing reports an error
