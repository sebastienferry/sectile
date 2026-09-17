## ADDED Requirements

### Requirement: One mapping from a run to its state
The mapping from a run or an activity to its run state SHALL have a single implementation, shared by
every surface that displays that state.

#### Scenario: A blocked run
- **GIVEN** a run whose status is running and which carries a start of wait
- **WHEN** its state is resolved
- **THEN** it is the waiting state
- **AND** every surface resolving it reaches the same answer

#### Scenario: A finished run keeps its outcome
- **GIVEN** a run whose status is completed, failed or cancelled
- **WHEN** its state is resolved
- **THEN** it is that outcome
- **AND** a leftover start of wait does not make it read as waiting

#### Scenario: The queued statuses
- **GIVEN** a run whose status is queued, preparing or pending
- **WHEN** its state is resolved
- **THEN** it is the queued state

#### Scenario: Anything else is running
- **GIVEN** a run whose status names none of the above
- **WHEN** its state is resolved
- **THEN** it is the running state

### Requirement: Every surface renders from the shared definition
A surface showing a run state SHALL take its glyph and its label from the shared definition, and
SHALL render a state the definition adds without being edited itself.

#### Scenario: The activities view shows a waiting badge
- **GIVEN** an activity that is running and waiting for the user
- **WHEN** its badge is rendered in the activities view
- **THEN** the badge reads as waiting
- **AND** it carries the waiting glyph of the shared definition

#### Scenario: The waiting filter and the badge agree
- **GIVEN** the activities view filtered on the waiting state
- **WHEN** the listed activities are displayed
- **THEN** every badge shown reads as waiting
- **AND** none of them reads as running

#### Scenario: The desktop list agrees with the banner it raised
- **GIVEN** a desktop session blocked on a permission prompt
- **WHEN** the sidebar row for that session is displayed
- **THEN** it reads as waiting
- **AND** it says what the notification raised for that session says

#### Scenario: A seventh state is added
- **GIVEN** a state added to the shared definition
- **WHEN** a run in that state is displayed in the activities view or in the desktop sidebar
- **THEN** its glyph and its label are rendered
- **AND** neither surface had to be edited for it

#### Scenario: A localised surface keeps its own wording
- **GIVEN** a state whose label the interface translates
- **WHEN** its badge is rendered
- **THEN** the translated wording is shown
- **AND** a state the translation does not know falls back to the shared label

### Requirement: The state is readable without colour or glyph
A run state SHALL be conveyed by text as well as by its glyph and its colour.

#### Scenario: Reading the state in the desktop sidebar
- **GIVEN** a sidebar row whose glyph cannot be interpreted by the user or their assistive technology
- **WHEN** the row is read
- **THEN** the state is reported as text
