## ADDED Requirements

### Requirement: Existing density selects condensed board cards
The application SHALL display condensed cards when the user selects Compact density, using the existing saved density preference. Standard and Comfortable SHALL retain detailed cards, and no new density preference or board switch SHALL be introduced.

#### Scenario: Select and retain Compact
- **GIVEN** the board is displaying detailed cards
- **WHEN** the user saves Compact density through the existing settings
- **THEN** displayed board cards become condensed
- **AND** reloading the application retains the condensed presentation through the saved preference.

#### Scenario: Return to detailed presentation
- **GIVEN** Compact density is selected
- **WHEN** the user saves Standard or Comfortable density
- **THEN** cards show their existing detailed content and shortcuts again.

### Requirement: Minimal visible content and readable titles
A condensed card SHALL show the public ticket identifier without truncation, a title occupying one line, and an always-visible Actions control. It SHALL hide parent metadata, priority, type, description, labels, branch/PR metadata, assignee, dates and the separate shortcut row without reserving their space.

#### Scenario: Ticket with extensive metadata
- **GIVEN** a ticket containing all supported metadata
- **WHEN** its condensed card is displayed
- **THEN** its public identifier, title and Actions control are visible
- **AND** the hidden metadata and shortcut rows consume no card height.

#### Scenario: Long title
- **GIVEN** a title too long for the available card width
- **WHEN** its condensed card is displayed
- **THEN** the title stays on one line and ends with an ellipsis
- **AND** the full title is available on hover and in ticket details
- **AND** the title control has the full title as its accessible name.

### Requirement: Details and tracker navigation remain available
Users SHALL be able to open details from the condensed card by pointer or keyboard. The public identifier SHALL retain its existing tracker destination when one exists.

#### Scenario: Detail opening
- **GIVEN** a condensed card
- **WHEN** the user clicks its title or body, or focuses the title and presses Enter or Space
- **THEN** details for that ticket open once.

#### Scenario: Tracker and local references
- **GIVEN** a ticket with an external tracker destination
- **WHEN** the user activates its public identifier
- **THEN** the tracker opens with the existing navigation behavior without also opening details
- **AND** tickets without an external destination display their identifier without a fabricated link.

### Requirement: Actions remain accessible with existing rules
The condensed Actions menu SHALL preserve access to operations available on the detailed card, including hidden metadata links and shortcuts. Existing availability, disabled states, confirmations and operation semantics SHALL remain unchanged.

#### Scenario: Hidden shortcuts and metadata operations
- **GIVEN** a condensed ticket for which the corresponding operations are available
- **WHEN** the user opens Actions
- **THEN** pin/unpin, single-step and automatic advancement, terminals, details, parent filtering and existing PR/MR navigation remain accessible alongside existing menu operations
- **AND** selecting an operation affects only the intended ticket or existing filter and does not also open details.

#### Scenario: Guarded operations
- **GIVEN** a finished ticket, an advancement already in progress, or a skill action disabled by the current execution state
- **WHEN** the user opens its condensed menu
- **THEN** corresponding actions retain the detailed card's existing disabled or availability rules
- **AND** switching density does not bypass those rules.

#### Scenario: Keyboard and constrained menu space
- **GIVEN** a condensed card at the top or bottom of a scrollable column
- **WHEN** the user reaches Actions by keyboard and opens it
- **THEN** the menu is visible, its available controls are keyboard reachable, and its contents can be scrolled when needed
- **AND** Escape dismisses it and returns keyboard focus to Actions.

### Requirement: Board behavior and activity remain consistent
Condensed presentation SHALL apply across workflow, operational/tracker and unassigned columns, for all projects and ticket sources. Existing sorting, filtering, drag-and-drop and activity border cues SHALL remain intact.

#### Scenario: All board paths
- **GIVEN** Compact density and tickets in workflow, operational/tracker or unassigned columns
- **WHEN** the user views each supported board path
- **THEN** each ticket uses the same condensed presentation and existing order/filter rules.

#### Scenario: Move a condensed card
- **GIVEN** a condensed ticket and a valid destination column
- **WHEN** the user drags and drops the card into that column
- **THEN** the existing transition occurs once with the same rules as a detailed card.

#### Scenario: Activity states
- **GIVEN** a ticket with running or queued activity
- **WHEN** its condensed card is displayed
- **THEN** the existing activity border cue remains visible without a separate Live/Queued row
- **AND** activity details remain accessible through the terminal action.
