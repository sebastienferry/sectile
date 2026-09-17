## ADDED Requirements

### Requirement: The card mode override is independent of the board display mode
The task card's `...` menu SHALL offer both one-off execution modes whatever the board display
mode, condensed or expanded. The override SHALL apply to that launch only and SHALL persist
nothing, in either display mode. The two entries SHALL come from one definition shared by both
shapes, so that neither shape can lose them on its own.

#### Scenario: Launching in the other mode from an expanded card
- **GIVEN** a board in expanded display mode
- **AND** a task whose resolved mode is interactive
- **WHEN** the user opens the card's `...` menu and picks the autonomous launch
- **THEN** that run is autonomous
- **AND** no skill setting and no project default is modified
- **AND** the next launch with no override is interactive again

#### Scenario: The condensed menu is unchanged
- **GIVEN** a board in condensed display mode
- **WHEN** the user opens a card's `...` menu
- **THEN** it offers the same entries in the same order as before: pin, advance, the two mode
  entries, the full chain, the parent filter when the task has a parent, and the pull request
  when the task has one

#### Scenario: The expanded menu gains only the mode entries
- **GIVEN** a board in expanded display mode
- **WHEN** the user opens a card's `...` menu
- **THEN** it offers the two mode entries followed by the entries it already had
- **AND** it does not gain the pin, parent filter or pull request entries, which the expanded
  card already shows inline

#### Scenario: The inline chevrons carry no override
- **GIVEN** a board in expanded display mode
- **WHEN** the user advances a task with the card's inline chevron rather than the `...` menu
- **THEN** the run uses the mode resolved from the skill setting and the project default

#### Scenario: A provider without a headless mode refuses from either shape
- **GIVEN** a project whose agent provider has no headless mode
- **WHEN** the user picks the autonomous launch from the `...` menu of a condensed card, and
  from the `...` menu of an expanded card
- **THEN** both launches are refused with an error naming the provider
