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

#### Scenario: Copy menu and desktop next step are unchanged
- **GIVEN** a task card's "Copy command" menu and the desktop next-step control
- **WHEN** the user launches from either of them
- **THEN** no model is offered and the run uses the model the configured levels resolve

### Requirement: The card menu offers the next step under a chosen model
The task card's `...` menu SHALL offer, in both board display modes, an entry that opens a submenu listing the models suggested for the task project's provider, with the model the configured levels resolve listed first and marked as current. Picking a listed model SHALL launch the task's next step under that model in the configured execution mode, for that launch only, and SHALL persist nothing. Picking the current model SHALL send no override. The submenu SHALL be a list to pick from and SHALL NOT offer free-text entry. The entry SHALL NOT be rendered when the provider has no suggested model list. The entry SHALL come from the definition both card shapes already share for the mode entries.

#### Scenario: Launching the next step under another model
- **GIVEN** a task on a project whose provider is `claude` and whose configured levels resolve `claude-sonnet-5`
- **WHEN** the user opens the card's `...` menu, opens the model submenu and picks `claude-opus-5`
- **THEN** the next step is launched with model `claude-opus-5` in the configured execution mode
- **AND** the project and global settings are unchanged
- **AND** the next launch without a model uses `claude-sonnet-5` again

#### Scenario: The current model is listed first and sends nothing
- **GIVEN** the same task
- **WHEN** the user opens the model submenu
- **THEN** `claude-sonnet-5` is the first row and is marked as current
- **AND** picking it launches the next step with no model override

#### Scenario: Both card shapes offer the entry
- **GIVEN** a board in condensed display mode and a board in expanded display mode
- **WHEN** the user opens a card's `...` menu in each
- **THEN** both menus offer the model entry next to the mode entries

#### Scenario: Provider without a suggestion list
- **GIVEN** a task on a project whose provider is `agy`, `vibe` or `custom`
- **WHEN** the user opens the card's `...` menu
- **THEN** no model entry is shown
- **AND** the detail view's model field remains available for that task

#### Scenario: Keyboard operation
- **GIVEN** the card's `...` menu is open with focus on the model entry
- **WHEN** the user presses the right arrow key
- **THEN** the submenu opens with focus on its first row
- **AND** the left arrow or escape key closes the submenu and returns focus to the entry
- **AND** activating a row closes the whole menu

#### Scenario: Submenu near the viewport edge
- **GIVEN** a card whose `...` menu opens near the right edge of the viewport
- **WHEN** the user opens the model submenu
- **THEN** the list is placed on the side where it fits entirely within the viewport
