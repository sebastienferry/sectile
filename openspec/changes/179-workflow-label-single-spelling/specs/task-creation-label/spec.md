## ADDED Requirements

### Requirement: One spelling for every workflow label
Whatever path posts it, a workflow label SHALL be written `#<stage>` in lowercase. The function
that sets it SHALL accept a stage name with or without a leading `#`, in any casing, and SHALL
normalise it. A caller SHALL NOT be able to decide the spelling.

#### Scenario: A caller passes a bare stage name
- **GIVEN** a workflow label is set with the target `new`
- **WHEN** the resulting labels are read back
- **THEN** the workflow label is `#new`

#### Scenario: A caller passes a prefixed or mixed-case stage name
- **GIVEN** a workflow label is set with the target `#New`
- **WHEN** the resulting labels are read back
- **THEN** the workflow label is `#new`

#### Scenario: Non-workflow labels keep their own spelling
- **GIVEN** a task carrying `CustomerCase` and `#TeamTag` alongside a workflow label
- **WHEN** its workflow label is set
- **THEN** `CustomerCase` and `#TeamTag` are returned unchanged, casing and prefix included

#### Scenario: A stage transition and a creation agree
- **GIVEN** one task created through the API and one moved by a stage transition
- **WHEN** both their workflow labels are read
- **THEN** both are spelled `#<stage>`, with no bare variant among them

### Requirement: Existing bare workflow labels keep working
A task already carrying a bare workflow label SHALL keep resolving to its stage, and SHALL have
that label removed at its next transition. This change SHALL NOT rewrite existing task labels
and SHALL NOT alter labels already posted on the tracker.

#### Scenario: Read a task labelled with the bare form
- **GIVEN** an existing task carrying the label `new`
- **WHEN** its stage is resolved
- **THEN** it resolves to the `new` stage

#### Scenario: Transition a task labelled with the bare form
- **GIVEN** an existing task carrying the label `new`
- **WHEN** it is moved to another stage
- **THEN** the bare label is removed, locally and on the tracker
- **AND** the new workflow label is `#<target stage>`

## MODIFIED Requirements

### Requirement: Lowercase add-form default
The task add form SHALL present its default workflow label as `#new` and SHALL submit `#new` when
that default is left unchanged. The form SHALL style a workflow badge identically whether the
label it displays carries the `#` or not.

#### Scenario: Open and reopen the add form
- **GIVEN** the task add form is closed
- **WHEN** the user opens it, including after a previous close
- **THEN** its default workflow label is displayed as `#new`, with exactly one hash and lowercase letters.

#### Scenario: Submit the default label
- **GIVEN** an add form with a valid title and its default label unchanged
- **WHEN** the user submits the form
- **THEN** the creation request includes `#new` as the default workflow label.

#### Scenario: Badge a label carrying the prefix
- **GIVEN** the add form displaying the labels `#new` and `new`
- **WHEN** their badges are rendered
- **THEN** both use the workflow badge styling of the `new` stage.

### Requirement: Lowercase newly created task label
Newly created tasks SHALL return and persist exactly one workflow label, `#new`, whether they
were created through `POST /api/tasks`, the MCP `create_task` tool, or the add form. Custom
labels SHALL keep their existing behavior.

#### Scenario: Create without supplied labels
- **GIVEN** a valid new-task request without labels
- **WHEN** the task is created and subsequently reloaded
- **THEN** both results contain `#new` as their sole workflow label
- **AND** the label posted on the tracker is `#new`.

#### Scenario: Replace legacy workflow variants and retain custom labels
- **GIVEN** a valid creation request containing `New`, `#New`, `#Specified`, `CustomerCase`, and `#TeamTag`
- **WHEN** the task is created and reloaded
- **THEN** `#new` is its only workflow label
- **AND** `CustomerCase` and `#TeamTag` retain their casing and prefix conventions.

### Requirement: Lowercase cloned task label
A cloned task SHALL receive the default workflow label `#new`, regardless of whether source
labels are included. Cloning SHALL retain existing custom-label inclusion semantics and SHALL
NOT modify the source task.

#### Scenario: Clone with source labels included
- **GIVEN** a source task with a legacy workflow label and custom label `CustomerCase`
- **WHEN** it is cloned with source labels included, either by default or explicitly
- **THEN** the returned and reloaded clone has `#new` as its only workflow label and retains `CustomerCase`
- **AND** the source task remains unchanged.

#### Scenario: Clone without source labels
- **GIVEN** a source task with workflow and custom labels
- **WHEN** it is cloned with source labels excluded
- **THEN** the returned and reloaded clone has only `#new` in its labels
- **AND** the source task remains unchanged.

### Requirement: Preserve status and historical task behavior
The label correction SHALL preserve creation and cloning status selection and SHALL NOT migrate existing task labels.

#### Scenario: Default and explicit status
- **GIVEN** a valid creation or clone request
- **WHEN** its status is omitted or explicitly supplied
- **THEN** the omitted status defaults to `to_clarify`, or the explicit status is retained, respectively
- **AND** the new task receives workflow label `#new` in either case.

#### Scenario: Read an existing task
- **GIVEN** an existing task labeled `New`
- **WHEN** it is read without a task-changing operation
- **THEN** this creation-default correction does not rewrite its label.
