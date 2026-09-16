## ADDED Requirements

### Requirement: A task runs in an embedded console on Windows
On Windows the system SHALL run a dispatched task in a pseudo-console it owns, as it does on
every other platform, and SHALL NOT fail the dispatch for want of a pseudo-terminal. The run
SHALL be attachable: the desktop and the board see its output and can type into it.

#### Scenario: A task is dispatched on Windows
- **GIVEN** an agent running on Windows with a mapped checkout
- **WHEN** the server dispatches a skill step for a task
- **THEN** the command runs in a console session the agent owns, in the task's working directory
- **AND** the dispatch is not reported as failed for want of a pseudo-terminal
- **AND** the run carries a session the companion can attach to

#### Scenario: A task is dispatched on a platform with a working PTY
- **GIVEN** an agent running on macOS or Linux
- **WHEN** the server dispatches a skill step for a task
- **THEN** the command runs in the agent's embedded session as before

### Requirement: A session runs the shell its user works in
The system SHALL start a console session in the host's own interactive shell, and SHALL NOT
impose one the user did not choose.

#### Scenario: A Windows host with PowerShell
- **GIVEN** a Windows host where `pwsh` or Windows PowerShell is installed
- **WHEN** a console session starts
- **THEN** the session runs that shell

#### Scenario: A Windows host without PowerShell
- **GIVEN** a Windows host where no PowerShell is found
- **WHEN** a console session starts
- **THEN** the session runs `cmd.exe`

#### Scenario: A POSIX host
- **GIVEN** a macOS or Linux host
- **WHEN** a console session starts
- **THEN** the session runs the login shell as before

### Requirement: A line typed into a session runs
When the system types a command line into a session, it SHALL end the line the way the host
console expects, so that the shell runs the command rather than echoing it.

#### Scenario: A line is typed into a Windows session
- **GIVEN** a console session on Windows
- **WHEN** the agent types a command line into it
- **THEN** the shell runs the command
- **AND** the shell is not left on a continuation prompt with the command unexecuted

#### Scenario: Bytes from a viewer's keyboard
- **GIVEN** a viewer attached to a session
- **WHEN** the viewer types
- **THEN** the bytes reach the shell unchanged

### Requirement: The session environment suits the host
The system SHALL build a session's environment for the platform it runs on: `PATH` joined with
the host's list separator, and POSIX-only variables set only on POSIX hosts.

#### Scenario: PATH on Windows
- **GIVEN** a console session starting on Windows
- **WHEN** its environment is built
- **THEN** the discovered tool directories precede the inherited `PATH`
- **AND** the entries are joined with the Windows list separator

### Requirement: A run is supervised on Windows
A run started on Windows SHALL be supervised exactly as a POSIX one: the system SHALL report
when it starts and when it exits, and SHALL terminate the whole process tree when the run is
cancelled.

#### Scenario: The run finishes
- **GIVEN** a task running on Windows
- **WHEN** the command exits
- **THEN** the run is reported finished with the command's exit status

#### Scenario: The run is cancelled
- **GIVEN** a task running on Windows
- **WHEN** the run is cancelled
- **THEN** the command and every process it started are terminated

#### Scenario: Supervision is no longer refused
- **GIVEN** an agent on Windows
- **WHEN** it starts a supervised command
- **THEN** the start is not refused as unsupported on this platform

#### Scenario: An autonomous run is cancelled
- **GIVEN** an autonomous run started on Windows without a terminal of its own
- **WHEN** the run is cancelled
- **THEN** the stop reaches that run's own process group
- **AND** it leaves the agent's console and the processes sharing it running

### Requirement: The command line is quoted for the session's shell
The system SHALL quote the supervised command line for the shell that will parse it, so that a
path containing spaces and a prompt containing quotes survive the round trip.

#### Scenario: A Windows path with spaces
- **GIVEN** an agent binary under a path containing spaces on Windows
- **WHEN** the supervised command line is built
- **THEN** the path is quoted so the session's shell passes it as a single argument

#### Scenario: A multi-line prompt
- **GIVEN** a skill prompt spanning several lines
- **WHEN** the supervised command line is built for a Windows shell
- **THEN** the command is encoded so no console line has to carry the newlines

#### Scenario: A POSIX path with a quote
- **GIVEN** a value containing a single quote on macOS or Linux
- **WHEN** the supervised command line is built
- **THEN** the existing POSIX quoting is used unchanged
