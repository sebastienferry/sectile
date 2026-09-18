## ADDED Requirements

### Requirement: The registered hook runs on every supported platform
The command Sectile registers as a Claude Code hook SHALL be directly executable
by the shell of every platform Sectile supports, without depending on a file
association, an interpreter discovered at run time, or any tool outside the
Sectile installation.

#### Scenario: A workstation running Windows
- **GIVEN** a workstation whose host shell is `cmd.exe`
- **WHEN** Sectile registers its Claude Code hooks
- **THEN** the registered command names the installed agent executable
- **AND** it carries the hook subcommand as its argument
- **AND** running it requires no shell, no file association and no external tool

#### Scenario: A workstation running macOS or Linux
- **GIVEN** a workstation whose host shell is POSIX
- **WHEN** Sectile registers its Claude Code hooks
- **THEN** the registered command names the same agent executable and the same
  subcommand as on Windows
- **AND** the two registrations differ only in how the executable path is quoted

#### Scenario: The executable path contains a space
- **GIVEN** an agent installed under a path containing a space
- **WHEN** the hook command is registered
- **THEN** the executable path is quoted for the shell that will read the command
- **AND** the subcommand remains a separate argument

### Requirement: A session Sectile launched reports its run state
A hook invocation carrying a run identity, a loopback address and a workstation
credential SHALL report that run as waiting or as working, and SHALL make no
other report.

#### Scenario: The session starts waiting
- **GIVEN** a launched session whose environment names its run and the local agent
- **WHEN** the hook receives an event that means the agent awaits the user
- **THEN** the run is reported as waiting to the local agent

#### Scenario: The session resumes work
- **GIVEN** the same launched session
- **WHEN** the hook receives an event that only occurs while the agent works
- **THEN** the run is reported as no longer waiting

#### Scenario: A launched session files no session alert
- **GIVEN** a launched session on a workstation that also published a connection file
- **WHEN** the hook makes its run report
- **THEN** no session alert is filed
- **AND** the session is announced once, not twice

### Requirement: A session Sectile did not launch announces itself by name
A hook invocation with no run identity SHALL announce the session through the
connection file the local agent publishes, naming the session by the last
element of its working directory.

#### Scenario: A working directory in Windows form
- **GIVEN** an unlaunched session whose payload reports the working directory
  `C:\git\sectile`
- **WHEN** the hook announces the session on a Windows workstation
- **THEN** the announced name is `sectile`

#### Scenario: A working directory in POSIX form
- **GIVEN** an unlaunched session whose payload reports the working directory
  `/Users/x/worktrees/#174`
- **WHEN** the hook announces the session
- **THEN** the announced name is `#174`

#### Scenario: A name containing characters that would break the message
- **GIVEN** a working directory whose last element contains a quote or a backslash
- **WHEN** the hook announces the session
- **THEN** the message received by the local agent is well formed
- **AND** the name is carried whole, with no character silently dropped

#### Scenario: No working directory in the payload
- **GIVEN** a payload naming no working directory
- **WHEN** the hook announces the session
- **THEN** the session is announced under a generic fallback name

#### Scenario: No connection file
- **GIVEN** a workstation where the local agent published no connection file
- **WHEN** an unlaunched session triggers the hook
- **THEN** no report is attempted
- **AND** the session is not interrupted

#### Scenario: The home directory is resolved without the shell
- **GIVEN** a host whose shell sets no `HOME` variable
- **WHEN** the hook looks for the connection file
- **THEN** the home directory is resolved from the operating system's own account
  information

### Requirement: The hook never interrupts the session it reports on
A hook invocation SHALL always succeed, SHALL write nothing on standard output,
and SHALL never make the session wait on the network.

#### Scenario: The local agent is not running
- **GIVEN** a loopback address that refuses the connection
- **WHEN** the hook attempts its report
- **THEN** the invocation succeeds
- **AND** nothing is written on standard output

#### Scenario: The local agent does not answer
- **GIVEN** a local agent that accepts the connection and never replies
- **WHEN** the hook attempts its report
- **THEN** the attempt is abandoned after a bounded delay
- **AND** the invocation succeeds

#### Scenario: The payload cannot be read
- **GIVEN** a payload that is empty, or not JSON, or names no event
- **WHEN** the hook runs
- **THEN** no report is made
- **AND** the invocation succeeds with nothing on standard output

#### Scenario: An event the hook does not answer
- **GIVEN** a payload naming an event outside the set the hook reports on
- **WHEN** the hook runs
- **THEN** no report is made
- **AND** the invocation succeeds

### Requirement: An installation carrying the shell script is migrated
Upgrading a workstation that carries the previously installed hook script SHALL
remove that script and replace its registration, leaving no second entry and no
registration pointing at a file that no longer exists.

#### Scenario: A workstation upgrading from the script
- **GIVEN** a workstation whose Claude settings register the installed
  `sectile-hook.sh` on every hook event
- **WHEN** Sectile refreshes the workstation configuration
- **THEN** each of those events registers the agent subcommand instead
- **AND** no event registers the script any more
- **AND** the script file is removed

#### Scenario: A registration written before the installation moved
- **GIVEN** a registration naming a Sectile hook under a former home directory or
  a former agent location
- **WHEN** Sectile refreshes the workstation configuration
- **THEN** that registration is replaced in place
- **AND** no second Sectile registration is added for the same event

#### Scenario: A script the user modified
- **GIVEN** a workstation where the installed hook script was edited by the user
- **WHEN** Sectile refreshes the workstation configuration
- **THEN** the file is left as the user wrote it
- **AND** its registration is still replaced by the agent subcommand

#### Scenario: Hooks the user wrote themselves
- **GIVEN** Claude settings carrying third-party hooks, some on events Sectile
  also answers
- **WHEN** Sectile refreshes the registration
- **THEN** every third-party hook is preserved with its matcher
- **AND** every key Sectile does not own is preserved
- **AND** a second refresh changes nothing

### Requirement: The hook behaviour is verifiable on any host
The automated tests covering the hook SHALL run on every platform the project
builds for, and the platform-specific part of the registration SHALL be
assertable without running on that platform.

#### Scenario: Running the suite on Windows
- **GIVEN** a Windows host with no POSIX shell on its path
- **WHEN** the project's Go test suite runs
- **THEN** the hook reporting tests execute rather than being skipped or failing

#### Scenario: Asserting a registration for another platform
- **GIVEN** a test host of any platform
- **WHEN** the hook command is built for a named target platform
- **THEN** the command produced for that target can be asserted
- **AND** no test depends on the platform it happens to run on
