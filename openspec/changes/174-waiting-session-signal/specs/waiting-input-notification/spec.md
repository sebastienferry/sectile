## ADDED Requirements

### Requirement: The alert is an operating-system notification
A session waiting for the user SHALL be announced by the operating system's own notification
facility, raised by the Sectile desktop application, so the banner is attributed to Sectile and
carries its identity.

#### Scenario: An agent asks for a permission
- **GIVEN** the hooks are installed and the desktop application is running
- **WHEN** the agent requests a permission or waits for input
- **THEN** a system notification is raised by the desktop application
- **AND** it names the session
- **AND** it is attributed to Sectile rather than to a scripting host

#### Scenario: A turn ends
- **GIVEN** a run that was executing
- **WHEN** it reaches a terminal status
- **THEN** a system notification is raised reporting the turn ended
- **AND** it is distinguishable from the waiting notification

#### Scenario: The same state is not announced twice
- **GIVEN** a run already announced as waiting
- **WHEN** its state is observed again unchanged
- **THEN** no further notification is raised

#### Scenario: Another platform
- **GIVEN** the desktop application runs on a platform other than macOS
- **WHEN** a session starts waiting
- **THEN** the notification is raised through that platform's own facility
- **AND** no platform-specific external command is required

#### Scenario: Notifications are unavailable or denied
- **GIVEN** the operating system does not permit the application to notify
- **WHEN** a session starts waiting
- **THEN** no error is surfaced to the user beyond the application's own indication
- **AND** the waiting state is still shown in the sessions list

#### Scenario: The desktop application is not running
- **GIVEN** no desktop application is open
- **WHEN** a session starts waiting
- **THEN** no notification is raised
- **AND** the waiting state is still recorded and still shown in the sessions list

### Requirement: The notification icon is the task list icon
The glyph identifying a state on a notification SHALL be the glyph identifying that same state in
the task list, read from one shared definition.

#### Scenario: Waiting
- **GIVEN** a session that starts waiting
- **WHEN** the notification is raised and the task row is displayed
- **THEN** both show the glyph defined for the waiting state

#### Scenario: A finished turn
- **GIVEN** a run that reaches a terminal status
- **WHEN** the notification is raised and the task row is displayed
- **THEN** both show the glyph defined for that state

#### Scenario: A state has only one definition
- **GIVEN** the shared definition of run states
- **WHEN** a state's glyph is changed in it
- **THEN** the notification and the task list both change
- **AND** neither carries a glyph the definition does not name

### Requirement: The session is named in the alert
A notification SHALL name the session it concerns, so the waiting session is identifiable without
opening it.

#### Scenario: A session in a worktree
- **GIVEN** a hook payload whose `cwd` is a worktree directory
- **WHEN** the notification is raised
- **THEN** the session is named by the basename of that directory

#### Scenario: A Sectile execution
- **GIVEN** a run Sectile launched for a known task
- **WHEN** the notification is raised
- **THEN** it names that task

#### Scenario: The payload carries no working directory
- **GIVEN** a hook payload whose `cwd` is absent or empty
- **WHEN** the notification is raised
- **THEN** a fallback session name is used
- **AND** no notification is suppressed for that reason alone

### Requirement: A hook never disturbs its session
A hook script SHALL terminate with exit code 0 whatever happens, and SHALL write nothing on its
standard output that the agent would read back.

#### Scenario: The report cannot be delivered
- **GIVEN** the local agent is not running or rejects the report
- **WHEN** the hook runs
- **THEN** the attempt is abandoned within a bounded delay
- **AND** the hook exits with code 0
- **AND** the agent session continues uninterrupted

#### Scenario: The payload is not valid JSON
- **GIVEN** stdin carries a payload the hook cannot parse
- **WHEN** the hook runs
- **THEN** the hook exits with code 0 without raising an error to the session

#### Scenario: The workstation was never paired
- **GIVEN** no agent connection is available to the hook
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

#### Scenario: A report without the loopback credential
- **GIVEN** a report presented without the workstation credential
- **WHEN** the loopback receives it
- **THEN** it is rejected
- **AND** no run changes state

### Requirement: A session Sectile did not launch is still announced
A hook running in a session that carries no run identifier SHALL still have its state announced,
identified by its working directory.

#### Scenario: A plain Claude Code session waits
- **GIVEN** a session whose environment carries no run identifier
- **AND** the workstation publishes an agent connection
- **WHEN** the `Notification` hook runs
- **THEN** a session-level report naming that session is delivered to the local agent
- **AND** a notification is raised for it
- **AND** no run changes state

#### Scenario: Reports are not accumulated without end
- **GIVEN** session-level reports arriving while nothing collects them
- **WHEN** their number exceeds what is retained
- **THEN** the oldest are dropped
- **AND** the local agent's memory does not grow with them

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
