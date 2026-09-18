## ADDED Requirements

### Requirement: The launch surfaces offer the configured models, not free text
The surfaces on which a skill is launched on a task SHALL let the user pick the model that run uses from the models configured for the task project's provider, and SHALL NOT accept a free-text identifier. The model the configured levels resolve for that task and skill SHALL be the default choice on every such surface and SHALL be presented as the current one; picking it SHALL send no override, so a launch nobody touched reproduces the command line built before this change. Picking any other listed model SHALL apply to that launch only and SHALL persist nothing. A surface SHALL NOT offer a model when the task project's provider has no configured models.

#### Scenario: Default is the configured model and sends nothing
- **GIVEN** a task whose configured levels resolve `claude-sonnet-5` for the skill being launched
- **WHEN** the user launches it without changing the model
- **THEN** the launch request carries no model
- **AND** the run uses `claude-sonnet-5`

#### Scenario: Picking another configured model
- **GIVEN** the same task, on a provider whose configured list contains `claude-opus-5`
- **WHEN** the user picks `claude-opus-5` and launches
- **THEN** the launch request carries `claude-opus-5`
- **AND** no project or global setting is modified
- **AND** the next launch left untouched uses `claude-sonnet-5` again

#### Scenario: No free-text entry
- **GIVEN** any launch surface offering the model
- **WHEN** the user opens it
- **THEN** it offers only the configured models to pick from
- **AND** it exposes no field in which an identifier can be typed

#### Scenario: Provider with no configured models
- **GIVEN** a task whose project provider has no configured model list
- **WHEN** the user opens the launch surfaces
- **THEN** no model choice is offered
- **AND** launching uses the model the configured levels resolve

### Requirement: The task detail launcher offers a per-run model
The task detail view's skill launcher SHALL offer a model selector beside its execution mode selector, listing the models configured for the task project's provider with the configured resolution as its default. Its value SHALL apply to every launch control of that view, interactive, autonomous and configured mode alike. The selector SHALL show the same notices as the configuration field when a command template governs the command line or when the provider takes no model.

#### Scenario: One choice serves every control of the view
- **GIVEN** the user picked a model in the launcher
- **WHEN** they launch a skill from its interactive, autonomous or configured-mode control
- **THEN** each launch carries that model
- **AND** the execution mode of each control is unchanged

#### Scenario: Choice carries over to the next launch in the same view
- **GIVEN** the user picked a model and launched one skill
- **WHEN** they launch another skill from the same open detail view
- **THEN** the same model is carried, until they select the current model again or close the view

#### Scenario: Template governs the line
- **GIVEN** a project whose command template contains no `{model}`
- **WHEN** the user opens the launcher
- **THEN** it states that the template governs the command line, as the settings do

#### Scenario: Provider that takes no model flag
- **GIVEN** a project on a provider that takes no model but has configured models
- **WHEN** the user opens the launcher
- **THEN** it states that this provider ignores the model, as the settings do

### Requirement: The card menu offers the next step under a chosen model
The task card's `...` menu SHALL offer, in both board display modes, an entry that opens a submenu listing the models configured for the task project's provider, with the model the configured levels resolve first and marked as current. Picking a listed model SHALL launch the task's next step under that model in the configured execution mode. The submenu SHALL be a list to pick from and SHALL NOT offer free-text entry. The entry SHALL come from the definition both card shapes already share for the mode entries.

#### Scenario: Launching the next step under another model
- **GIVEN** a task on a project whose provider is `claude` and whose configured levels resolve `claude-sonnet-5`
- **WHEN** the user opens the card's `...` menu, opens the model submenu and picks `claude-opus-5`
- **THEN** the next step is launched with model `claude-opus-5` in the configured execution mode
- **AND** the project and global settings are unchanged

#### Scenario: The current model is listed first and sends nothing
- **GIVEN** the same task
- **WHEN** the user opens the model submenu
- **THEN** `claude-sonnet-5` is the first row and is marked as current
- **AND** picking it launches the next step with no model override

#### Scenario: Both card shapes offer the entry
- **GIVEN** a board in condensed display mode and a board in expanded display mode
- **WHEN** the user opens a card's `...` menu in each
- **THEN** both menus offer the model entry next to the mode entries

#### Scenario: Keyboard operation
- **GIVEN** the card's `...` menu is open with focus on the model entry
- **WHEN** the user presses the right arrow key
- **THEN** the submenu opens with focus on its first row
- **AND** the left arrow or escape key closes the submenu and returns focus to the entry
- **AND** activating a row closes the whole menu

#### Scenario: Submenu near the viewport edge
- **GIVEN** a card whose `...` menu opens near an edge of the viewport
- **WHEN** the user opens the model submenu
- **THEN** the list stays within the menu's own bounds and scrolls with it
- **AND** no part of it is rendered outside the viewport

#### Scenario: Copy menu and desktop next step are unchanged
- **GIVEN** a task card's "Copy command" menu and the desktop next-step control
- **WHEN** the user launches from either of them
- **THEN** no model is offered and the run uses the model the configured levels resolve
