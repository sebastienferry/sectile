# User guide

## ADDED Requirements

### Requirement: A discoverable task-oriented guide

The repository SHALL provide a user guide under `/docs`, linked from both the root README and the `/docs` index, that leads a new user through the supported Sectile workflow.

#### Scenario: A new user starts from either documentation index

- **GIVEN** a user opens the root README or the `/docs` index
- **WHEN** they look for instructions to use Sectile
- **THEN** they can open the user guide directly
- **AND** the guide presents the prerequisites and task steps in a usable order

### Requirement: Browser and tracker setup

The guide SHALL explain browser sign-in, adding a personal Jira credential with site URL, email and token, optional sealing and unlocking, and the distinction between personal and unattended server credentials.

#### Scenario: A user configures Jira access

- **GIVEN** a user can sign in to a Sectile server and has Jira credentials
- **WHEN** they follow the guide
- **THEN** they can locate the personal tracker credential controls and understand the optional sealing passphrase
- **AND** they know how a locked credential affects their tracker operations

### Requirement: Project and workstation setup

The guide SHALL explain how to use an existing project, add a new project, pair a workstation, start or connect an agent, and map the project to local folders in Desktop.

#### Scenario: A user prepares local execution

- **GIVEN** a user has access to a project and a local repository
- **WHEN** they follow the guide
- **THEN** they can distinguish the server project from the local folder mapping
- **AND** they can identify the steps to pair and configure the workstation

### Requirement: Coding and Desktop workflow

The guide SHALL explain using a Sectile skill prompt in Claude Code, launching a full autonomous pickup, following progress in Desktop, and reaching a pull request for human review.

#### Scenario: A user works on a ticket

- **GIVEN** a ticket in a configured project
- **WHEN** the user follows either the manual Claude Code path or the autonomous pickup path
- **THEN** they understand how to launch the task, inspect execution progress and results, and where human merge occurs
- **AND** the guide does not imply a free-form prompt alone updates Sectile stages
