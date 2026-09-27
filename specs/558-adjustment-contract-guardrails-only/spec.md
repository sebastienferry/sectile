# Specification #558 - The adjustment contract carries guardrails only

- Ticket: https://github.com/sebastienferry/sectile/issues/558
- Branch: `feat/558`
- Clarification: `docs/clarifications/558.md` (round 1, no open question)
- Framework: Spec Kit

## Summary

Every adjustment run appends a fixed contract to its prompt. Today that
contract also prescribes the work (review, reconcile, check, commit, push),
which overrides a skill that deliberately does less. The contract keeps only
its guardrails and the feedback rule; the rest becomes the skill's call.

## Scope

In scope: the text of the adjustment contract, its tests, an ADR amending
ADR 0004, and the changelog.

Out of scope:

- The bundled `adjust-issue` skill fragments, which keep all the work.
- The built-in fallback prompt used when no skill is installed and no prompt
  is configured.
- The server-side stage gates (forge-confirmed PR, non-draft when open, head
  equal to the checkout commit, clean checkout).

## Definitions

- **Contract**: the text appended to every adjustment prompt, managed or
  native.
- **Guardrail**: a sentence that forbids or requires something regardless of
  the work the skill chooses to do.
- **Prescribed work**: a sentence that tells the agent what work to perform.

## User stories (prioritised)

### US1 - A custom adjustment skill that only reviews is obeyed (P1)

As a project owner whose adjustment skill reviews and reports without
correcting, I want the run to follow my skill, so that it does not fix, check
and push behind my back.

- **Given** a project whose adjustment skill forbids corrections,
  **When** an adjustment run is launched, managed or native,
  **Then** its prompt carries no instruction to review the branch, reconcile
  the default branch, run build/lint/test, commit, push or update the PR.

### US2 - The guardrails still hold for every skill (P1)

- **Given** any adjustment skill or custom prompt,
  **When** an adjustment run is launched, managed or native,
  **Then** its prompt still requires verifying the matching PR, forbids
  creating or replacing one and pushing onto a merged one, requires retrieving
  feedback with a disposition for each (retrieval failure blocks completion),
  requires preserving work on failure, and forbids merging, approving, closing
  the task and removing its worktree.

### US3 - The default behaviour does not change (P1)

- **Given** a project using the bundled skill or no skill at all,
  **When** an adjustment run is launched,
  **Then** the skill or the built-in fallback prompt still carries the full
  work: reconcile, review, fix, check, commit, push, verify readiness.

## Functional requirements

- **FR1** The contract keeps: PR verification (open, or merged by the human);
  never push onto a merged PR, report its final state; never create or replace
  a PR; missing-PR recovery belongs to the earlier creation stage; retrieve
  available feedback and record a disposition for each, retrieval failure
  blocks completion, no human feedback is required; preserve work on failure;
  never merge, approve, close the task or remove its worktree.
- **FR2** The contract carries no prescribed work: no branch review against
  the specification, no default-branch reconciliation, no build/lint/test, no
  commit, no push, no PR update or readiness verification.
- **FR3** The contract is appended exactly where it is today: every managed
  adjustment prompt and every native adjustment launch.
- **FR4** The built-in fallback prompt and the bundled skill fragments are
  unchanged.
- **FR5** A new ADR records the decision and ADR 0004 points to it.
- **FR6** `CHANGELOG.md` gains a `Changed` line under `## [Unreleased]`.

## Acceptance criteria

- **AC1** A native adjustment launch carries the guardrails of FR1 and none of
  the prescriptions of FR2 (test).
- **AC2** A managed adjustment prompt built from a configured prompt carries
  the guardrails and none of the prescriptions of the contract (test).
- **AC3** A managed adjustment prompt built from the built-in fallback still
  requires running build, lint and tests and pushing (test).
- **AC4** ADR 0038 exists and ADR 0004 references it.
- **AC5** `CHANGELOG.md` describes the change for users.
