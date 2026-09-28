## ADDED Requirements

### Requirement: A run answer clears only its named wait
When an owner answers a question in an agent run's console, Sectile SHALL clear the wait only when its stored mark equals the answered mark at the instant of the write. The comparison and clear SHALL be one atomic database operation, including when different server instances handle the answer and a new wait.

#### Scenario: Answer to the current question
- **GIVEN** a running agent run owned by the answering user has a session wait
- **WHEN** its owner answers with the current wait mark
- **THEN** Sectile clears that wait and reports the change to wait listeners.

#### Scenario: New question overlaps an old answer
- **GIVEN** an agent has received the mark for an earlier question
- **WHEN** a new question is declared while that earlier answer is processed by another server instance
- **THEN** the earlier answer does not clear the new question
- **AND** the new wait remains visible as the current wait.

#### Scenario: Answer does not qualify
- **GIVEN** the mark is stale, the user is not the owner, the run is not a running agent run, or the wait is for a repository
- **WHEN** the answer arrives
- **THEN** the wait is unchanged and no clear notification is emitted.

#### Scenario: SQLite run
- **GIVEN** Sectile uses SQLite
- **WHEN** an owner answers a current or stale wait
- **THEN** the same current-only clearing behavior applies.
