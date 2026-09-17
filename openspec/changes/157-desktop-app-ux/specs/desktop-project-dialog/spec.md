## ADDED Requirements

### Requirement: Recognisable dialog close control
The project dialog SHALL close through a cross icon drawn with the desktop's shared icon style and coloured with the theme's red, rather than a text glyph.

#### Scenario: User opens the project dialog
- **GIVEN** the project dialog is open
- **WHEN** its close control renders
- **THEN** the control shows the desktop's cross icon in the theme red on a transparent background
- **AND** it stays anchored at the top of the dialog while its content scrolls.

#### Scenario: User closes the dialog
- **GIVEN** the project dialog is open
- **WHEN** the user activates the close control with a pointer or the keyboard
- **THEN** the dialog closes
- **AND** the control exposes its Close accessible name, its icon being hidden from assistive technology.
