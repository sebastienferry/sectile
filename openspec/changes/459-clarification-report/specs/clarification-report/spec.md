## ADDED Requirements

### Requirement: Readable round comments
Each clarification round SHALL publish its complete Markdown section on the ticket and retain the report history.

#### Scenario: Intermediate round
- **GIVEN** an interactive or unattended standalone clarification round with open questions
- **WHEN** its report is saved
- **THEN** the complete initial report or new follow-up section is posted as a comment, split into numbered parts when necessary

#### Scenario: Final round
- **GIVEN** a standalone round with no open product questions
- **WHEN** clarification completes
- **THEN** its full section is the transition note, with no duplicate add_comment

#### Scenario: Managed reporting
- **GIVEN** a managed run
- **WHEN** a round completes
- **THEN** its result note contains the round section and the agent calls no comment or transition tool

### Requirement: Optional publication
Projects SHALL expose a persistent pushStageCommits boolean, defaulting to false, in project settings, agent configuration and MCP project context.

#### Scenario: Enabled publication
- **GIVEN** pushStageCommits is true
- **WHEN** clarify or specify commits a stage artifact
- **THEN** the assigned branch is pushed normally, with upstream set for its first publication, without forcing

#### Scenario: Disabled or missing option
- **GIVEN** the option is false or absent
- **WHEN** a stage artifact is committed
- **THEN** optional publication does not occur

#### Scenario: Push refusal
- **GIVEN** optional publication is enabled
- **WHEN** the push is refused
- **THEN** the refusal is reported and the stage may complete

### Requirement: User documentation
The changelog SHALL include one Changed entry for round comments and one Added entry for optional publication.

#### Scenario: Release notes
- **WHEN** the change is delivered
- **THEN** both user-visible changes appear under Unreleased
