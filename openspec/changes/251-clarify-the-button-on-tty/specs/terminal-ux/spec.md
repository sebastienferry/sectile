## Purpose

Functional specification and acceptance criteria for interactive terminal button actions, session control hierarchy, and explicit terminology in TaskFlow UI.

## ADDED Requirements

### Requirement: Isolation between UI Panel Closing and PTY Session Termination
The application SHALL enforce strict isolation between hiding the terminal panel UI and terminating a background PTY session.

#### Scenario: Hiding the terminal panel UI without affecting active sessions
- **GIVEN** the terminal panel opened with one or more active PTY sessions (or a running agent)
- **WHEN** the user clicks the `X` button in the `WorkspaceTerminalPanel` header
- **THEN** the panel UI SHALL hide (`isTerminalPanelOpen = false`)
- **AND** the underlying PTY session and any associated background processes (agents, shell) SHALL continue executing without interruption.

#### Scenario: Explicit termination of a PTY session
- **GIVEN** a PTY session selected in the panel session dropdown
- **WHEN** the user clicks the explicit "Terminer la session" action for that session
- **THEN** the system SHALL terminate the corresponding PTY session (`resetTerminalSession(sessionId)`)
- **AND** the active session entry and client connection indicators SHALL update immediately.

---

### Requirement: Explicit Terminal Action Terminology
The application SHALL provide clear and unambiguous labels and tooltips for screen clearing and shell process restart actions.

#### Scenario: Clearing screen scrollback text
- **GIVEN** an active terminal session with text output displayed in the xterm viewport
- **WHEN** the user clicks the `Clear` button
- **THEN** the xterm viewport text buffer SHALL be cleared and the `clear` command sent to the terminal
- **AND** the active shell process SHALL NOT be restarted.

#### Scenario: Restarting a clean shell session
- **GIVEN** an active terminal session
- **WHEN** the user hovers over the "Relancer le shell" button
- **THEN** the tooltip SHALL display "Redémarrer une nouvelle session shell vierge"
- **WHEN** the user clicks "Relancer le shell"
- **THEN** the shell process SHALL be restarted and re-opened clean in its working directory.

---

### Requirement: Toolbar Consolidation and Duplicate Control Removal
The application SHALL consolidate control hierarchy between the container panel (`WorkspaceTerminalPanel`) and the inner terminal view (`InteractiveTerminal`).

#### Scenario: Control hierarchy in embedded mode
- **GIVEN** the terminal rendered within the `WorkspaceTerminalPanel` container
- **WHEN** the panel is rendered
- **THEN** container-level controls (session picker, dock positioning, fullscreen toggle, panel close) SHALL be displayed exclusively in the panel header
- **AND** the `InteractiveTerminal` header SHALL display exclusively PTY connection status, in-terminal quick actions (`Ctrl+C`, `Clear`, `Relancer le shell`), and the agent/skill action bar.
