## ADDED Requirements

### Requirement: One mapping from a run to its displayed state
The mapping from a run or an activity to its run state SHALL have a single implementation, held with
the shared run-state definition, and every surface that shows a run state SHALL derive it there.

#### Scenario: A blocked run
- **GIVEN** a run whose status is running and which carries a wait mark
- **WHEN** its state is derived
- **THEN** the state is waiting

#### Scenario: A finished run carrying a stale wait mark
- **GIVEN** a run whose status is completed, failed or canceled and which still carries a wait mark
- **WHEN** its state is derived
- **THEN** the state is that terminal status, not waiting

#### Scenario: A run not yet started
- **GIVEN** a run whose status is queued, preparing or pending
- **WHEN** its state is derived
- **THEN** the state is queued

#### Scenario: The notification layer reads the same mapping
- **GIVEN** the desktop notification layer decides which banner to raise
- **WHEN** it resolves a run's state
- **THEN** it resolves it through the shared mapping, and not a copy of it

### Requirement: The activities view badge reads the derived state
The activities view SHALL render each activity's badge from the shared run-state definition, keyed by
the derived state, and SHALL show the state in text as well as in colour and glyph.

#### Scenario: Filtering on waiting
- **GIVEN** activities including one running activity that carries a wait mark
- **WHEN** the view is filtered on waiting
- **THEN** each listed activity shows a badge that reads waiting
- **AND** none of them reads running

#### Scenario: The state is stated once
- **GIVEN** a running activity that carries a wait mark
- **WHEN** its row is displayed
- **THEN** the waiting state appears once, on the badge
- **AND** no second pill repeats it

#### Scenario: A queued or pending activity
- **GIVEN** an activity whose status is queued or pending
- **WHEN** its row is displayed
- **THEN** the badge reads queued

#### Scenario: The badge is localised
- **GIVEN** the interface language is French
- **WHEN** a badge is displayed for a state the translation table knows
- **THEN** its text is the French label

### Requirement: The desktop sidebar shows the derived state
The desktop sidebar SHALL show each run's derived state, as the shared glyph together with a text
label, on the task row and in the execution queue, instead of the raw status string alone.

#### Scenario: A session blocked on a prompt
- **GIVEN** a local session whose run is waiting for the user
- **WHEN** the sidebar lists it
- **THEN** its row shows the waiting state
- **AND** that is the state the banner raised for the same run announces

#### Scenario: The state is readable without colour or glyph
- **GIVEN** any run shown in the sidebar
- **WHEN** its row is displayed
- **THEN** the state is available as text to a screen reader

#### Scenario: The test selector is unchanged
- **GIVEN** a run shown on a task row
- **WHEN** the row is inspected
- **THEN** its status data attribute still carries the raw run status

### Requirement: A new state renders without editing the surfaces
Adding a state to the shared definition SHALL make it render on the activities view and on the
desktop sidebar with no edit to either surface.

#### Scenario: A seventh state
- **GIVEN** a seventh state is added to the shared definition
- **WHEN** a run in that state is displayed on either surface
- **THEN** its shared glyph and label are shown
- **AND** neither surface had to be edited for it

#### Scenario: A state the translation table does not know
- **GIVEN** a state with no entry in the translation table
- **WHEN** its badge is displayed
- **THEN** the label from the shared definition is shown
