## ADDED Requirements

### Requirement: Open desktop configuration from the workspace
The focused desktop window SHALL open the existing Configuration page at General / User profile on Cmd+, on macOS and Ctrl+, on Windows/Linux.

#### Scenario: Shortcut from workspace content
Given no unrelated modal is open and a terminal or ordinary input has focus, when the user presses the platform shortcut, then Configuration opens at General / User profile and the intercepted chord does not reach the focused control.

### Requirement: Preserve an open configuration page
The desktop SHALL leave the current category and unsaved fields unchanged when the configuration shortcut is pressed while Configuration is open.

#### Scenario: Shortcut while editing configuration
Given any configuration category is open with an unsaved field value, when the user presses the platform shortcut, then the selected category and field value remain unchanged.

### Requirement: Ignore other combinations and contexts
The desktop SHALL ignore repeated shortcut events, chords with extra or wrong platform modifiers, and the shortcut while an unrelated modal is open.

#### Scenario: Ineligible key event
Given a repeated event, an extra modifier, the other platform modifier, or an unrelated modal, when the user presses comma, then the shortcut does not open or rebuild Configuration. Back and Escape continue to close Configuration as before.

### Requirement: Make the shortcut discoverable
The Settings control SHALL show its platform shortcut in its tooltip and accessible shortcut metadata, and the desktop user guide SHALL document both platform shortcuts.

#### Scenario: Inspect Settings control
Given the desktop Settings control is displayed, when a user inspects its tooltip or accessibility metadata, then the platform-specific shortcut is announced.
