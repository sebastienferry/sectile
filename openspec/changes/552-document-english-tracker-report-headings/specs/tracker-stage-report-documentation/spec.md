## ADDED Requirements

### Requirement: Release note for English tracker report headings

The Unreleased changelog SHALL describe that newly generated tracker reports for all five completed workflow stages have English headings and SHALL reference PR #549.

#### Scenario: Reader checks the upcoming release

- **GIVEN** PR #549 has delivered English headings for clarification, specification, implementation, review, and closure reports
- **WHEN** a reader opens the Unreleased Fixed section
- **THEN** one entry describes that visible change and cites PR #549.

### Requirement: Accurate historical scope

Clarification #548 SHALL preserve its original Round 1 text and append a dated account of the final five-stage scope. Its design rationale SHALL describe five headings.

#### Scenario: Reader reviews the decision history

- **GIVEN** the original clarification described two headings
- **WHEN** a reader reviews clarification #548 and its design
- **THEN** Round 1 remains intact
- **AND** a later dated section records the five headings delivered in PR #549
- **AND** the design rationale no longer describes only two headings.
