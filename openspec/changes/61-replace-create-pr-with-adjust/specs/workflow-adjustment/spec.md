## ADDED Requirements

### Requirement: Adjustment replaces the PR creation workflow action
The system SHALL offer Adjust between implemented and reviewed while preserving all existing workflow states and human merge/handoff responsibilities.

#### Scenario: Follow the workflow
- **GIVEN** a task at each workflow stage
- **WHEN** its next action is presented or executed
- **THEN** new offers Clarify, clarified offers Specify, specified offers Code, implemented offers Adjust, and reviewed offers Handoff
- **AND** finished offers no next development action
- **AND** PR creation is not presented as the review workflow step.

### Requirement: PR creation belongs to the configured earlier stage
The system SHALL create or reuse the task's PR during specification or implementation according to project policy and SHALL retain the default implementation timing.

#### Scenario: Specification timing
- **GIVEN** specification-time PR creation is configured
- **WHEN** specification completes successfully
- **THEN** a PR for the task branch exists with its URL recorded, newly created PRs are drafts, and the task reaches specified
- **AND** subsequent implementation updates that same PR without marking the task reviewed.

#### Scenario: Default implementation timing
- **GIVEN** implementation-time creation is configured or no timing override exists
- **WHEN** specification completes and implementation subsequently succeeds
- **THEN** specification has not created a PR and implementation has created or reused and recorded the branch PR before reaching implemented
- **AND** a newly created PR remains draft until successful adjustment.

#### Scenario: Creation cannot complete
- **GIVEN** the responsible earlier stage cannot push, verify, create, or record its required PR
- **WHEN** completion is evaluated
- **THEN** the stage is not reported successfully completed and the concrete failure is reported with work preserved for retry.

### Requirement: Adjustment requires an existing matching PR
Adjustment SHALL require an existing open PR for the task's repository and branch and SHALL NOT create a PR.

#### Scenario: PR is discoverable but unlinked
- **GIVEN** an open PR exists for the task branch but its URL is not yet recorded
- **WHEN** adjustment checks its prerequisite
- **THEN** it records and reuses that PR without creating another.

#### Scenario: PR is missing or invalid
- **GIVEN** an implemented task has no matching open PR, or its recorded PR is for another branch, closed, or merged
- **WHEN** Adjust is requested
- **THEN** adjustment reports the unmet prerequisite before modifying the branch, creates no PR, and leaves the task implemented
- **AND** directs recovery to the configured earlier stage.

#### Scenario: Forge is unavailable
- **GIVEN** PR existence or review feedback cannot be verified because forge access fails
- **WHEN** adjustment attempts verification
- **THEN** the failure is reported as retryable or blocked as appropriate, no replacement PR is created, and no successful review transition occurs.

#### Scenario: Earlier-stage recovery
- **GIVEN** an implemented task is missing its required PR
- **WHEN** the missing-PR action successfully completes PR setup through the configured earlier stage
- **THEN** accepted work and the implemented stage are preserved, the PR URL is recorded, and Adjust becomes available
- **AND** recovery does not itself mark the task reviewed.

### Requirement: Adjustment performs complete review and corrective work
Adjustment SHALL review the complete branch against the specification, reconcile the current default branch, address findings and available PR feedback, and verify the final changes before declaring success.

#### Scenario: Review with feedback
- **GIVEN** a matching PR has actionable feedback and the branch contains review findings
- **WHEN** adjustment succeeds
- **THEN** the complete diff has been reviewed, findings corrected, feedback addressed or explicitly dispositioned, and final build/lint/test evidence reported
- **AND** the existing PR contains the pushed corrections and updated review information and is ready for review
- **AND** the task becomes reviewed without merge, approval, ticket closure, or workspace cleanup.

#### Scenario: No human feedback
- **GIVEN** the matching PR has no human review feedback
- **WHEN** adjustment runs
- **THEN** it still performs full branch review and checks and may succeed without waiting for a human comment.

#### Scenario: Quality gate fails
- **GIVEN** unresolved findings, failed required checks, uncommitted final changes, a push failure, or an unverified PR readiness update
- **WHEN** adjustment completion is evaluated
- **THEN** it reports the failure and remaining work and does not advance an implemented task to reviewed
- **AND** branch and PR remain available for retry.

#### Scenario: Repeated adjustment
- **GIVEN** a reviewed task still has an open PR and receives additional feedback
- **WHEN** the user repeats Adjust
- **THEN** it applies the same complete review gate to the same branch and PR, reports its outcome, and does not merge or create another PR.

### Requirement: Completion requires evidence in every execution mode
The system SHALL preserve stage verification for standalone and managed runs and SHALL NOT treat PR existence alone as successful adjustment.

#### Scenario: Invalid completion evidence
- **GIVEN** adjustment reports a successful exit but lacks required checks or provides a mismatching or draft PR
- **WHEN** TaskFlow evaluates completion
- **THEN** it rejects the successful review transition and reports the missing or inconsistent evidence.

#### Scenario: Managed ownership
- **GIVEN** a managed workflow run is active
- **WHEN** a standalone transition attempts to bypass its result validation
- **THEN** the transition is rejected and the managed completion contract remains authoritative.

### Requirement: Compatibility preserves existing work and customization
The system SHALL accept legacy review invocations with adjustment behavior, preserve saved customization and history, and surface incompatible customization before it can restore PR creation during adjustment.

#### Scenario: Legacy invocation or queued job
- **GIVEN** an existing invocation or queued job requests the former Create PR or Review action
- **WHEN** it executes after upgrade
- **THEN** it runs adjustment with the same existing-PR prerequisite and quality gates as the new action
- **AND** existing activity history remains readable.

#### Scenario: Legacy customization requires reconciliation
- **GIVEN** a project has saved legacy review overrides or a custom creation prompt
- **WHEN** the replacement skill is presented or automatically launched
- **THEN** the customization remains available with its origin and any conflicts visible
- **AND** inherited legacy customization requires explicit reconciliation before automatic execution rather than being silently discarded or allowed to create a PR.

#### Scenario: Divergent installed command
- **GIVEN** an installed legacy skill has local edits
- **WHEN** replacement skills are installed
- **THEN** the divergence is reported and the local edits are not silently overwritten.

### Requirement: Composite workflows share adjustment semantics
Single and batch pickup SHALL create PRs at the configured earlier stage, perform adjustment, and stop at reviewed for human merge.

#### Scenario: Single pickup
- **GIVEN** a task is processed autonomously from new
- **WHEN** pickup completes
- **THEN** clarification, specification, implementation, and adjustment have succeeded in order, with PR creation completed before adjustment
- **AND** execution stops at reviewed.

#### Scenario: Combined batch
- **GIVEN** selected tasks share a batch branch and PR
- **WHEN** batch pickup performs adjustment
- **THEN** it reviews the complete combined diff, updates the same PR, and links that PR to applicable completed tickets
- **AND** unfinished tickets are not marked reviewed merely because the shared PR exists.
