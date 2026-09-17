## ADDED Requirements

### Requirement: Distinguishable desktop alert per event
A Claude Code session SHALL raise a desktop alert when it starts waiting for the user, and a
different one when its turn ends, so the two are told apart without looking at a screen.

#### Scenario: An agent asks for a permission
- **GIVEN** the `Notification` hook is installed
- **WHEN** the agent requests a permission or waits for input
- **THEN** a desktop notification is raised
- **AND** it plays the sound reserved for waiting
- **AND** its title or body names the session

#### Scenario: A turn ends
- **GIVEN** the `Stop` hook is installed
- **WHEN** the agent finishes its turn
- **THEN** a desktop notification is raised with the sound reserved for a finished turn
- **AND** that sound differs from the waiting sound

#### Scenario: The session is named by its working directory
- **GIVEN** a hook payload whose `cwd` is a worktree directory
- **WHEN** the alert is raised
- **THEN** the alert names the session by the basename of that directory

#### Scenario: The payload carries no working directory
- **GIVEN** a hook payload whose `cwd` is absent or empty
- **WHEN** the alert is raised
- **THEN** a fallback session name is used
- **AND** no alert is suppressed for that reason alone

### Requirement: A hook never disturbs its session
A hook script SHALL terminate with exit code 0 whatever happens, and SHALL write nothing on its
standard output that the agent would read back.

#### Scenario: The notification channel is unavailable
- **GIVEN** the command that raises the notification is missing or fails
- **WHEN** the hook runs
- **THEN** the hook exits with code 0
- **AND** the agent session continues uninterrupted

#### Scenario: The payload is not valid JSON
- **GIVEN** stdin carries a payload the hook cannot parse
- **WHEN** the hook runs
- **THEN** the hook exits with code 0 without raising an error to the session

#### Scenario: The host is not macOS
- **GIVEN** the workstation provides no supported notification command
- **WHEN** the hook runs
- **THEN** it is a silent no-op and exits with code 0

### Requirement: User-level hook installation
Sectile SHALL install the hook scripts once per workstation and register them in the user-level
Claude settings, never as a per-project copy.

#### Scenario: Registering the hooks
- **GIVEN** a workstation whose Claude provider is being set up
- **WHEN** Sectile installs the agent configuration
- **THEN** the two scripts are written under the user Claude directory
- **AND** `~/.claude/settings.json` registers them for the `Notification` and `Stop` events
- **AND** the scripts are executable

#### Scenario: Existing settings are preserved
- **GIVEN** `~/.claude/settings.json` already holds unrelated keys and other hooks
- **WHEN** the registration is applied
- **THEN** every unrelated key and hook is left untouched
- **AND** only the Sectile-owned hook entries are added or updated

#### Scenario: Re-running the installation
- **GIVEN** the hooks are already registered
- **WHEN** the installation runs again
- **THEN** the settings file ends in the same state, with no duplicated entry

#### Scenario: Malformed settings file
- **GIVEN** `~/.claude/settings.json` cannot be parsed
- **WHEN** the registration is attempted
- **THEN** the file is left unchanged
- **AND** the failure is reported without aborting the rest of the agent setup

### Requirement: Waiting state reported to Sectile
A hook running inside a session Sectile launched SHALL report the waiting state to the local agent
loopback, and that report SHALL identify the run without inspecting the working directory.

#### Scenario: A Sectile-launched session starts waiting
- **GIVEN** the session environment carries a run identifier, the loopback URL and the loopback token
- **WHEN** the `Notification` hook runs
- **THEN** it posts a waiting report for that run to the loopback
- **AND** the run is marked as waiting from the moment of the report

#### Scenario: The waiting session resumes
- **GIVEN** a run currently marked as waiting
- **WHEN** the `Stop` hook runs for that session
- **THEN** it posts a resumed report
- **AND** the run is no longer marked as waiting

#### Scenario: A session Sectile did not launch
- **GIVEN** a session whose environment carries no run identifier
- **WHEN** either hook runs
- **THEN** no report is sent to the loopback
- **AND** the desktop alert is still raised

#### Scenario: The loopback does not answer
- **GIVEN** the local agent is not running or rejects the report
- **WHEN** a hook attempts to report
- **THEN** the attempt is abandoned within a bounded delay
- **AND** the hook exits with code 0

#### Scenario: A report without the loopback token
- **GIVEN** a report presented without the run's loopback credential
- **WHEN** the loopback receives it
- **THEN** it is rejected
- **AND** no run changes state

### Requirement: Waiting state cleared with the run
A run SHALL NOT remain marked as waiting once it has ended.

#### Scenario: A waiting run completes
- **GIVEN** a run marked as waiting
- **WHEN** the run reaches a terminal status
- **THEN** it is no longer marked as waiting

#### Scenario: A waiting run is cancelled
- **GIVEN** a run marked as waiting
- **WHEN** the run is cancelled
- **THEN** it is no longer marked as waiting
