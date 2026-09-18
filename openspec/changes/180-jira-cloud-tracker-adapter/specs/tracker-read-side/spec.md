# tracker-read-side

## Purpose
A tracker tells the server what its projects are made of: boards, columns,
sprints, statuses, issue types, epics, teams and their members, and the fields
a creation requires. The server asks the resolved tracker and never names one.

## ADDED Requirements

### Requirement: The ticketing abstraction exposes a read side
`TicketingSystem` SHALL offer operations to list a project's boards, a board's
columns with their statuses, a board's sprints, a project's statuses, a
project's issue types, a project's epics, the fields a creation requires for an
issue type, to search teams by name and to read a team's members. Boards,
columns, statuses and issue types SHALL be gated by a board capability;
sprints, epics and teams SHALL be gated by the existing sprint, epic and team
capabilities.

#### Scenario: A tracker without boards
- **GIVEN** the `github` adapter
- **WHEN** the server asks for a project's boards
- **THEN** it receives an unsupported-capability error naming `github` and the board capability
- **AND** the interface shows it as a limit, not a failure.

#### Scenario: A tracker with boards
- **GIVEN** the `jira` adapter
- **WHEN** the server asks for a project's boards
- **THEN** it receives the project's boards with id, name and type.

### Requirement: Existing adapters are unaffected
Adding the read side SHALL NOT require any change to the `github` or `local`
adapters beyond what the shared base provides; they SHALL keep every behaviour
they have.

#### Scenario: GitHub sync after the change
- **GIVEN** a GitHub project
- **WHEN** a sync runs
- **THEN** it imports issues exactly as before the change.

### Requirement: The server routes project structure through the resolved tracker
The server SHALL resolve the project's tracker, check the capability and call
the read side when listing a project's tracker boards, issue types and
statuses, importing a board's columns, searching teams or refreshing a team's
members. No tracker name SHALL be tested in the database layer for these
operations.

#### Scenario: Import board columns on a Jira project
- **GIVEN** a Jira project with a selected board
- **WHEN** the user imports the board columns
- **THEN** the columns come from the tracker's read side
- **AND** statuses assigned by hand and hidden columns are preserved.

#### Scenario: Refresh team members on a local project
- **GIVEN** a local project
- **WHEN** the user asks to refresh a team's members
- **THEN** the answer is an unsupported-capability error naming `local`.

### Requirement: Creation asks the tracker what it requires
Before creating a work item of a given type, the server SHALL be able to ask
the tracker which fields the site makes mandatory beyond summary, type and
project, with their allowed values when the site enumerates them.

#### Scenario: An issue type with a mandatory select field
- **GIVEN** a Jira issue type requiring "Epic Type" with values "Feature" and "Tech"
- **WHEN** the server asks the required creation fields
- **THEN** it receives "Epic Type" with the two values.
