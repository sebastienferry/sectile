## ADDED Requirements

### Requirement: Select existing tickets in Board and Backlog
Desktop SHALL offer accessible selection checkboxes in both project Board and Backlog and a visible selected count, clear action and batch preparation action. A batch SHALL contain at least two distinct existing tickets belonging to the active project. Selection SHALL survive filtering, refresh and switching between these surfaces in the same project, and SHALL clear when the project changes or the ticket workspace closes.

#### Scenario: Prepare from either surface
- **GIVEN** a configured project with at least two existing tickets
- **WHEN** the user selects two ticket checkboxes in Board or Backlog
- **THEN** the count reflects both selections and batch preparation becomes available without opening tickets or changing their stages.

#### Scenario: Preserve selection during display changes
- **GIVEN** selected tickets in an active project
- **WHEN** the user filters, refreshes or switches Board and Backlog
- **THEN** the same selection remains and hidden selected tickets remain visible in preparation.

#### Scenario: Insufficient or unresolved selection
- **GIVEN** fewer than two distinct tickets or a selected ticket that cannot be resolved
- **WHEN** the user attempts to confirm a batch
- **THEN** launch is blocked with an explanation and no execution starts.

### Requirement: Review execution order and shared worktree name
Desktop SHALL present every selected ticket with its key and title in a preparation dialog, allow changing execution order and the suggested worktree name, and require explicit confirmation. The name SHALL be trimmed, contain 1–80 characters, begin with an ASCII letter or digit and otherwise contain only ASCII letters, digits, hyphens or underscores. Cancellation SHALL launch nothing and preserve selection.

#### Scenario: Reorder and confirm
- **GIVEN** a prepared batch of selected tickets
- **WHEN** the user moves a ticket to the first position, edits a valid name and confirms
- **THEN** that ticket leads the batch and the execution follows the displayed order with the chosen shared worktree name.

#### Scenario: Invalid name and cancellation
- **GIVEN** an empty, oversized or path-like name
- **WHEN** the user attempts confirmation
- **THEN** an accessible validation message blocks launch; cancel remains available and retains selection.

### Requirement: Launch one sequential pickup batch
Desktop SHALL launch the existing pickup batch workflow once, with explicit active project identity and ordered full ticket IDs, using existing execution configuration and console behavior. The batch SHALL request sequential processing in one dedicated worktree and one combined PR while preserving existing work. The first member SHALL be the lead. Server membership, eligibility and busy checks SHALL remain authoritative; a batch SHALL offer no forced launch.

#### Scenario: Accepted launch
- **GIVEN** at least two eligible, distinct tickets in the active project and a valid preparation
- **WHEN** confirmation is accepted
- **THEN** one batch execution starts with the displayed members and order, existing console behavior applies, and preparation and selection clear.

#### Scenario: Multi-project ticket
- **GIVEN** a selected ticket belongs to the active project and also another project
- **WHEN** the batch launches
- **THEN** it runs for the explicitly selected active project rather than guessing from the ticket.

### Requirement: Preserve preparation on launch failure
Desktop SHALL show launch refusal without discarding selection, execution order or worktree name. Confirmation SHALL be disabled while its request is pending. Failure SHALL not trigger an automatic replay or independent member launches.

#### Scenario: Busy or invalid member
- **GIVEN** preparation is complete but a member becomes busy, disappears or leaves the project before launch
- **WHEN** the server refuses the batch
- **THEN** Desktop displays the refusal, retains preparation for correction and retry, and offers no launch-anyway bypass.

#### Scenario: Repeated confirmation
- **GIVEN** a batch launch request is pending
- **WHEN** the user clicks confirmation again
- **THEN** no second request is sent.

#### Scenario: Transport failure
- **GIVEN** the launch response cannot be obtained
- **WHEN** Desktop reports the error
- **THEN** preparation remains available and no automatic retry occurs.

### Requirement: Preserve existing Desktop interactions
Selection SHALL be keyboard-operable and SHALL not trigger ticket opening, card dragging, stage movement or row menus. Single-ticket launch behavior SHALL remain available. An agent unable to carry batch membership SHALL block batch launch with an update explanation.

#### Scenario: Unsupported agent
- **GIVEN** the connected agent does not advertise batch launch support
- **WHEN** a batch is prepared
- **THEN** confirmation is unavailable with an explanation and no single-ticket fallback is launched.

#### Scenario: Existing single-ticket action
- **GIVEN** a ticket shown in either surface
- **WHEN** the user invokes its existing single-ticket launch action
- **THEN** its established launch behavior remains unchanged.

### Requirement: Document the user-visible addition
Implementation SHALL add an English entry under CHANGELOG.md Unreleased describing batch preparation and launch from Desktop Board and Backlog.

#### Scenario: Release documentation
- **GIVEN** the feature has been implemented
- **WHEN** its changes are submitted for review
- **THEN** the changelog names the user-visible addition without requiring the reader to inspect the code.
