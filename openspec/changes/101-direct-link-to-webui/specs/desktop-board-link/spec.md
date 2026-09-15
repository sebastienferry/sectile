## ADDED Requirements

### Requirement: Connected server links to the board
The desktop header SHALL expose the connected server address as a keyboard-accessible link that opens its board in the default browser without navigating the desktop window.

#### Scenario: Mouse or keyboard activation
- **GIVEN** a connected server with a valid HTTP(S) address
- **WHEN** the user clicks the header link or focuses it and presses Enter
- **THEN** the current server board opens in the default browser
- **AND** the configured base path is preserved without selecting a specific task
- **AND** the desktop console remains available.

#### Scenario: Status refresh and server changes
- **GIVEN** a focused connection link
- **WHEN** connection status refreshes
- **THEN** keyboard focus remains on the link while connected
- **AND** a changed server address updates the displayed destination.

### Requirement: Unavailable destinations remain safe
The desktop SHALL show plain connection status when disconnected or stopped and SHALL report failed browser opening without navigating the desktop window.

#### Scenario: Server disconnects or local agent stops
- **GIVEN** a previously connected server
- **WHEN** the server disconnects or the local agent stops
- **THEN** the header no longer exposes an actionable board link.

#### Scenario: Invalid destination or browser failure
- **GIVEN** a server destination that is invalid, uses a non-HTTP(S) scheme, contains credentials, or is no longer connected
- **WHEN** board opening is requested
- **THEN** no external destination is opened and an error is reported.
- **GIVEN** a valid connected destination
- **WHEN** the default browser fails to open
- **THEN** an error is reported and the desktop remains usable.
