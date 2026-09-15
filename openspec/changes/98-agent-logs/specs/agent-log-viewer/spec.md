## ADDED Requirements

### Requirement: Workspace diagnostics
The desktop SHALL show Agent logs throughout the available content area while retaining usable project sidebar navigation and its current width and collapse preference.

#### Scenario: Open and navigate connected logs
- **GIVEN** an execution is selected
- **WHEN** the user opens Agent logs
- **THEN** diagnostics occupy the workspace beside the sidebar without changing the selected execution
- **AND** selecting an execution from the sidebar returns to that execution

#### Scenario: Close diagnostics
- **GIVEN** Agent logs is open
- **WHEN** the user closes logs or presses Escape without a settings dialog open
- **THEN** the previous execution view or offline setup is restored and focus returns to Agent logs

#### Scenario: Offline diagnostics
- **GIVEN** the local agent is unavailable
- **WHEN** Agent logs is opened or refreshed
- **THEN** local diagnostics remain accessible, with explicit missing, empty, truncated or read-error feedback

### Requirement: Readable plain text
The diagnostics viewer SHALL omit terminal control sequences while preserving printable Unicode, line breaks, tabs and literal HTML-like text. Reading and refreshing SHALL NOT alter the stored log.

#### Scenario: Terminal output
- **GIVEN** a snapshot contains ANSI colors, erase/cursor controls, OSC titles or hyperlinks, and printable messages
- **WHEN** diagnostics are displayed
- **THEN** only readable message text is shown, without control payloads or interpreted HTML

#### Scenario: Incomplete control sequence
- **GIVEN** a snapshot ends within a terminal control sequence
- **WHEN** it is displayed
- **THEN** the incomplete sequence is omitted

#### Scenario: Refresh and navigation race
- **GIVEN** a snapshot read is pending
- **WHEN** the user leaves logs and opens another pane or dialog
- **THEN** the late result does not replace the newly opened content
