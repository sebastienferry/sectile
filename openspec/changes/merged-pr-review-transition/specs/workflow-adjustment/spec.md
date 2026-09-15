## MODIFIED Requirements

### Requirement: Adjustment requires an existing matching PR
Adjustment SHALL require an existing PR for the task's repository and branch that is
either open or already merged, SHALL NOT create a PR, and SHALL NOT push onto a
merged PR.

#### Scenario: PR is discoverable but unlinked
- **GIVEN** an open PR exists for the task branch but its URL is not yet recorded
- **WHEN** adjustment checks its prerequisite
- **THEN** it records and reuses that PR without creating another.

#### Scenario: PR was merged before the review stage was recorded
- **GIVEN** the task's recorded PR for the branch has been merged by the human and its head commit is the agent checkout commit
- **WHEN** the reviewed transition is recorded
- **THEN** the merged PR is accepted as adjustment evidence and the task becomes reviewed
- **AND** no PR is created, no commit is pushed onto the merged branch, and the task is not merged, approved or closed.

#### Scenario: An open PR outranks a merged one
- **GIVEN** the task branch has both a merged PR and a later open PR
- **WHEN** adjustment looks up its PR
- **THEN** the open PR is selected and the usual readiness gate applies to it.

#### Scenario: PR is missing or invalid
- **GIVEN** an implemented task has no matching open or merged PR, or its recorded PR is for another branch or was closed without merging
- **WHEN** Adjust is requested
- **THEN** adjustment reports the unmet prerequisite before modifying the branch, creates no PR, and leaves the task implemented
- **AND** directs recovery to the configured earlier stage.

#### Scenario: Merged PR does not match the checkout
- **GIVEN** a merged PR for the branch whose head commit is not the agent checkout commit
- **WHEN** the reviewed transition is evaluated
- **THEN** it is rejected as missing evidence and the task stays implemented.

#### Scenario: Forge is unavailable
- **GIVEN** PR existence or review feedback cannot be verified because forge access fails
- **WHEN** adjustment attempts verification
- **THEN** the failure is reported as retryable or blocked as appropriate, no replacement PR is created, and no successful review transition occurs.

#### Scenario: Earlier-stage recovery
- **GIVEN** an implemented task is missing its required PR
- **WHEN** the missing-PR action successfully completes PR setup through the configured earlier stage
- **THEN** accepted work and the implemented stage are preserved, the PR URL is recorded, and Adjust becomes available
- **AND** recovery does not itself mark the task reviewed.
