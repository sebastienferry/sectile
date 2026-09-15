## ADDED Requirements

### Requirement: A task runs in the host terminal on Windows
On Windows the system SHALL execute a dispatched task in the host terminal instead of an
embedded PTY, and SHALL NOT fail the dispatch because a pseudo-terminal is unavailable. On
platforms whose PTY is supported the system SHALL keep using the embedded console.

#### Scenario: A task is dispatched on Windows
- **GIVEN** an agent running on Windows with a mapped checkout
- **WHEN** the server dispatches a skill step for a task
- **THEN** the command runs in the host terminal in the task's working directory
- **AND** the dispatch is not reported as failed for want of a pseudo-terminal

#### Scenario: A task is dispatched on a platform with a working PTY
- **GIVEN** an agent running on macOS or Linux
- **WHEN** the server dispatches a skill step for a task
- **THEN** the command runs in the agent's embedded PTY session as before

#### Scenario: Windows Terminal is unavailable
- **GIVEN** an agent on Windows where `wt.exe` cannot be found
- **WHEN** a task is dispatched
- **THEN** the command runs in `cmd.exe` instead
- **AND** the run is still reported to the server

### Requirement: The launcher script matches the host shell
The system SHALL render the external terminal script in the language of the shell that will
run it, and SHALL reject an environment variable name the target shell cannot express.

#### Scenario: Rendering for Windows
- **GIVEN** a task with a working directory, environment variables and an initial command
- **WHEN** the launcher script is rendered for Windows
- **THEN** it is a batch script that sets each variable, changes to the working directory and
  runs the command
- **AND** it does not contain POSIX `export` or `exec "$SHELL"` syntax

#### Scenario: Rendering for a POSIX host
- **GIVEN** the same task
- **WHEN** the launcher script is rendered for macOS or Linux
- **THEN** it is the existing bash script

#### Scenario: An unusable variable name
- **GIVEN** an environment variable whose name is not a valid identifier
- **WHEN** the launcher script is rendered
- **THEN** rendering fails and names the offending variable

#### Scenario: The script does not outlive the run
- **GIVEN** a launcher script holding an agent token
- **WHEN** the launched command has finished
- **THEN** the script file has been removed

### Requirement: An externally launched run is supervised
A run launched in a host terminal SHALL be supervised exactly as an embedded one: the system
SHALL report when it starts and when it exits, and SHALL terminate the whole process tree when
the run is cancelled.

#### Scenario: The run finishes
- **GIVEN** a task launched in a host terminal on Windows
- **WHEN** the command exits
- **THEN** the run is reported finished with the command's exit status

#### Scenario: The run is cancelled
- **GIVEN** a task running in a host terminal on Windows
- **WHEN** the run is cancelled
- **THEN** the command and every process it started are terminated

#### Scenario: Supervision is no longer refused
- **GIVEN** an agent on Windows
- **WHEN** it starts a supervised command
- **THEN** the start is not refused as unsupported on this platform

### Requirement: The command line is quoted for the host shell
The system SHALL quote the supervised command line for the shell that will parse it, so that a
path containing spaces and a prompt containing quotes survive the round trip.

#### Scenario: A Windows path with spaces
- **GIVEN** an agent binary under a path containing spaces on Windows
- **WHEN** the supervised command line is built
- **THEN** the path is quoted so `cmd.exe` passes it as a single argument

#### Scenario: A POSIX path with a quote
- **GIVEN** a value containing a single quote on macOS or Linux
- **WHEN** the supervised command line is built
- **THEN** the existing POSIX quoting is used unchanged
