## ADDED Requirements

### Requirement: Lowercase add-form default
The task add form SHALL present its default workflow label as `#new` and SHALL submit lowercase `new` when that default is left unchanged.

#### Scenario: Open and reopen the add form
- **GIVEN** the task add form is closed
- **WHEN** the user opens it, including after a previous close
- **THEN** its default workflow label is displayed as `#new`, with exactly one hash and lowercase letters.

#### Scenario: Submit the default label
- **GIVEN** an add form with a valid title and its default label unchanged
- **WHEN** the user submits the form
- **THEN** the creation request includes `new` as the default workflow label.

### Requirement: Lowercase newly created task label
Newly created tasks SHALL return and persist exactly one workflow label, lowercase `new`, while retaining the existing unprefixed creation-label convention and custom-label behavior. Where task labels are displayed, the default SHALL appear as `#new`.

#### Scenario: Create without supplied labels
- **GIVEN** a valid new-task request without labels
- **WHEN** the task is created and subsequently reloaded
- **THEN** both results contain `new` as their sole workflow label
- **AND** a view displaying its labels shows `#new`.

#### Scenario: Replace legacy workflow variants and retain custom labels
- **GIVEN** a valid creation request containing `New`, `#New`, `#Specified`, `CustomerCase`, and `#TeamTag`
- **WHEN** the task is created and reloaded
- **THEN** `new` is its only workflow label
- **AND** `CustomerCase` and `#TeamTag` retain their casing and prefix conventions.

### Requirement: Lowercase cloned task label
A cloned task SHALL receive the lowercase default workflow label `new`, regardless of whether source labels are included. Cloning SHALL retain existing custom-label inclusion semantics and SHALL NOT modify the source task.

#### Scenario: Clone with source labels included
- **GIVEN** a source task with a legacy workflow label and custom label `CustomerCase`
- **WHEN** it is cloned with source labels included, either by default or explicitly
- **THEN** the returned and reloaded clone has `new` as its only workflow label and retains `CustomerCase`
- **AND** the source task remains unchanged.

#### Scenario: Clone without source labels
- **GIVEN** a source task with workflow and custom labels
- **WHEN** it is cloned with source labels excluded
- **THEN** the returned and reloaded clone has only `new` in its labels
- **AND** the source task remains unchanged.

### Requirement: Preserve status and historical task behavior
The label correction SHALL preserve creation and cloning status selection and SHALL NOT migrate existing task labels.

#### Scenario: Default and explicit status
- **GIVEN** a valid creation or clone request
- **WHEN** its status is omitted or explicitly supplied
- **THEN** the omitted status defaults to `to_clarify`, or the explicit status is retained, respectively
- **AND** the new task receives workflow label `new` in either case.

#### Scenario: Read an existing task
- **GIVEN** an existing task labeled `New`
- **WHEN** it is read without a task-changing operation
- **THEN** this creation-default correction does not rewrite its label.
