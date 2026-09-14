# desktop-agent-logs Specification

## Purpose
TBD - created by archiving change 71-local-agent-logs. Update Purpose after archive.
## Requirements
### Requirement: Local diagnostic visibility

The desktop SHALL provide an Agent logs action that displays its existing agent diagnostic log as read-only literal text, identifies the source path, and explains that independently started agents may log elsewhere.

#### Scenario: Connected inspection
- **GIVEN** the desktop has a selected execution and a non-empty agent log
- **WHEN** the user opens Agent logs
- **THEN** the diagnostic contents and source are displayed without changing the selected execution or stopping it
- **AND** markup and terminal escape sequences are not executed

#### Scenario: Agent unavailable
- **GIVEN** the desktop is in setup or its agent is stopped or failed to start
- **WHEN** the user opens Agent logs
- **THEN** the same local diagnostic view remains available without an agent or server connection

### Requirement: Bounded snapshots and refresh

The viewer SHALL show at most the latest 256 KiB per snapshot, indicate omitted earlier content, and provide an explicit Refresh action.

#### Scenario: Large log
- **GIVEN** the log exceeds 256 KiB
- **WHEN** the user opens or refreshes the viewer
- **THEN** only the bounded recent tail is displayed with a truncation notice
- **AND** a UTF-8 character cut by the tail boundary is omitted rather than displayed as a broken prefix

#### Scenario: Refresh changed log
- **GIVEN** the log has been appended, truncated, or replaced since the displayed snapshot
- **WHEN** the user refreshes
- **THEN** the view replaces previous contents with a fresh snapshot and shows the newest output

### Requirement: Explicit unavailable states

The viewer SHALL distinguish missing files, empty files, and read failures, clear stale contents on a failed refresh, and allow retry.

#### Scenario: No output yet
- **GIVEN** the desktop log is absent or empty
- **WHEN** the user opens the viewer
- **THEN** it explains respectively that no desktop agent log exists yet or that the log is empty

#### Scenario: Read failure
- **GIVEN** the log is unreadable or is an unsupported file type
- **WHEN** the user refreshes
- **THEN** stale contents are cleared and an error is displayed with Refresh available for retry

### Requirement: Local fixed source

Diagnostic inspection SHALL remain on the workstation and SHALL only read the fixed desktop-owned log, without accepting arbitrary file paths or changing log contents.

#### Scenario: Unexpected source
- **GIVEN** the expected log is a symlink or non-regular file
- **WHEN** the user requests a snapshot
- **THEN** the viewer reports a read failure without reading the alternate source

