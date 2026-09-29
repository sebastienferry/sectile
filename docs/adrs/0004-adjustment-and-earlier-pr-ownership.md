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
instructions.
*Amended by [ADR 0038](0038-the-adjustment-contract-carries-guardrails-only.md): the
contract keeps its guardrails only; reviewing, fixing and running checks belong to
the skill.* Forge identity, pushed commit, readiness and checkout validation are
separate from agent-reported check results.

## Consequences

Existing projects retain their PR timing and work. Missing PRs require recovery through
an earlier owner rather than creation during review. Historical customization remains
accessible but may block execution until reconciled. Forge access is required for a
successful PR-owning or adjustment stage. No new workflow state or service is added.

## Amendment (#580): clarification as the earliest owner

`prCreationStage` gains a third value, `clarified`. Clarification then owns
draft creation, in its final round only: intermediate rounds, which may stop
on open questions, publish nothing on the forge, so an unresolved
clarification never becomes a pull request. The clarified transition requires
the pull request, and so does the specified transition that follows, which
updates it. Dropped artefacts defer it to implementation at both stages. A
task that reaches implemented without a pull request is recovered through
implementation, never by re-running a clarification.

Agents stop rejecting a creation stage they do not know and fall back to
implementation, so the next value added cannot stop a lagging agent from
running a project. Agents built before this amendment still reject
`clarified`; the release note asks for a desktop update first.
