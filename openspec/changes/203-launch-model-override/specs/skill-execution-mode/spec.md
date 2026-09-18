## ADDED Requirements

### Requirement: The task detail launcher offers a per-run model
The task detail view's skill launcher SHALL offer a model field beside its execution mode selector. The field SHALL be empty by default and its placeholder SHALL show the model the project and global settings resolve for the task's project and the skill being launched, stating that a workstation override may still apply. The field SHALL accept the same free text and suggestions as the configuration field, and SHALL show the same notices when a command template governs the command line or when the provider takes no model. A launch from any of the skill row's controls, interactive, autonomous or configured mode, SHALL carry the field's value. An untouched field SHALL send no override.

#### Scenario: Placeholder shows the configured resolution
- **GIVEN** a project whose per-skill map names `implement -> claude-sonnet-5` and whose bare model is `claude-opus-5`
- **WHEN** the user opens the launcher on a task of that project
- **THEN** the model field is empty
- **AND** its placeholder names `claude-sonnet-5` for `implement` and `claude-opus-5` for the other skills

#### Scenario: Launching with the field untouched
- **GIVEN** the model field is empty
- **WHEN** the user launches a skill from any of its row's controls
- **THEN** the launch request carries no model
- **AND** the run uses the model the configured levels resolve

#### Scenario: Launching with a model
- **GIVEN** the user types `claude-opus-5` in the model field
- **WHEN** they launch a skill autonomously from its row
- **THEN** the launch request carries `claude-opus-5` and the autonomous mode
- **AND** no project or global setting is modified

#### Scenario: Field carries over to the next launch in the same view
- **GIVEN** the user typed a model and launched one skill
- **WHEN** they launch another skill from the same open detail view
- **THEN** the same model is carried, until they clear the field or close the view

#### Scenario: Template governs the line
- **GIVEN** a project whose command template contains no `{model}`
- **WHEN** the user opens the launcher
- **THEN** the field states that the template governs the command line, as the settings do

#### Scenario: Provider without a model flag
- **GIVEN** a project on a provider that takes no model
- **WHEN** the user opens the launcher
- **THEN** the field states that this provider ignores the model, as the settings do

#### Scenario: Card menu, copy menu and desktop next step are unchanged
- **GIVEN** a task card's `...` menu, its "Copy command" menu and the desktop next-step control
- **WHEN** the user launches from any of them
- **THEN** no model is offered and the run uses the model the configured levels resolve
