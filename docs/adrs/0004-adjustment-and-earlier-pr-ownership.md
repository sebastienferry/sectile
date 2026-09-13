# ADR 0004: Adjustment and earlier pull request ownership

Status: Accepted for issue #61

## Context

The review workflow action previously created a PR although projects can create one
at specification time. This conflates publication with the final quality gate.

## Decision

Use Adjust for implemented-to-reviewed and require an existing open task-branch PR.
Specification or implementation owns draft creation according to the existing policy;
implementation remains the default. Recovery preserves already attained progress.
Adjustment reviews the complete diff and available feedback, fixes findings, runs
final checks, updates the same PR and verifies readiness. It never merges.

Normalize legacy invocation IDs at execution boundaries, retaining storage fields and
activity history. Resolve overrides in order: adjust, create_pr, review, built-in.
Inherited custom content requires explicit reconciliation; preserve losing entries
and divergent installed files. Keep the mandatory adjustment contract with customized
instructions. Forge identity, pushed commit, readiness and checkout validation are
separate from agent-reported check results.

## Consequences

Existing projects retain their PR timing and work. Missing PRs require recovery through
an earlier owner rather than creation during review. Historical customization remains
accessible but may block execution until reconciled. Forge access is required for a
successful PR-owning or adjustment stage. No new workflow state or service is added.
