## MODIFIED Requirements

### Requirement: Full-chain pickup from Desktop Tickets runs autonomously
The Desktop Tickets menu SHALL explicitly request autonomous execution when the user selects Pickup (full chain), regardless of the configured project or pickup skill mode.

#### Scenario: Configured mode is interactive
- **GIVEN** a task whose project or pickup skill defaults to interactive execution
- **WHEN** the user selects Pickup (full chain) in the Tickets menu
- **THEN** the launch request identifies the pickup skill and carries autonomous mode

### Requirement: Other launch controls preserve their mode behavior
The Desktop SHALL preserve the existing execution-mode behavior of other Tickets skills, the console pickup button, and generic custom-instruction and relaunch controls.

#### Scenario: Another Tickets skill is launched
- **GIVEN** a configured interactive default
- **WHEN** the user selects a non-pickup skill in the Tickets menu
- **THEN** the request carries no mode override

#### Scenario: Other full-chain and generic controls are used
- **GIVEN** the user launches full pickup from the console toolbar or web action, or uses a generic mode-selection dialog
- **WHEN** the launch request is sent
- **THEN** the full-pickup controls retain autonomous mode and the generic controls retain the user's explicit choice or configured-mode default
