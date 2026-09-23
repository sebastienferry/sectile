# #312 — A GitHub call with a locked personal credential falls back silently on the server token

Ticket: https://github.com/sebastienferry/sectile/issues/312
Type: Technical debt, split from #310.
Branch: `feat/312`.
Clarification: [`docs/clarifications/312.md`](../../docs/clarifications/312.md).

## Context

#328 (ADR 0018) made the background synchronisation read as the project's owner, and the Jira adapter refuses a
locked or missing personal credential. The GitHub adapter does not: when the acting user's sealed GitHub token is
locked, it resolves the project or server token instead. A background pass then reads with a credential that is not
the owner's while its activity records the owner in `user_id`, and a person's write goes out under the server account
while they believe they act as themselves. ADR 0018 states the opposite: "a locked credential still fails, loudly".

Out of scope: `trackerAs` in `internal/db/trackercredentials.go` (none of its callers is a background sync path), the
`task_activities` schema (#310), and agent or MCP credential resolution (ADR 0019 / 0020).

This file states behaviour only. Implementation choices are in [`plan.md`](plan.md); the checklist is in
[`tasks.md`](tasks.md).

## Decisions being specified

Settled in round 2 of the clarification, each in line with the recommendation.

1. The ticket is scoped down to the GitHub residue; its original premise is covered by ADR 0018.
2. A locked personal GitHub credential refuses the call instead of falling back.
3. An acting user with no personal GitHub token keeps the project or server token. This fallback is a decision, and
   ADR 0018 records it.
4. The refusal applies to every call made through the GitHub tracker adapter that carries an actor, background pass or
   person alike. The calls resolved through `trackerAs` (branch pull request lookup, GitHub GraphQL reads) still fall
   back and are out of scope.

## User stories

### US1 — A locked credential refuses the call (P1)

As a person who sealed their GitHub token, I want a call made in my name to fail while the token is locked, so that
nothing is read or written under an account I did not choose, and the activity never names me for a credential that
was not used.

- **Given** a GitHub project and an acting user whose sealed GitHub token is locked,
  **When** any GitHub tracker operation runs with that actor (read, sync, create, update, delete, comment, labels, pull
  request discovery),
  **Then** the operation returns an error, and no request reaches GitHub.
- **Given** a project whose owner's sealed GitHub token is locked,
  **When** the background pass synchronises it,
  **Then** the pass fails, and its activity keeps the owner in `user_id`.

### US2 — No personal token keeps the project credential (P1)

As an operator of a deployment configured through `SECTILE_GITHUB_TOKEN` or the project's GitHub token, I want calls
by people who stored no personal GitHub token to keep working.

- **Given** an acting user with no personal GitHub token,
  **When** a GitHub operation runs with that actor,
  **Then** it uses the project token when one is stored, the server token otherwise.
- **Given** an acting user with an unlocked personal GitHub token,
  **When** a GitHub operation runs with that actor,
  **Then** it uses that personal token.

### US3 — Work that names nobody is unchanged (P2)

- **Given** a GitHub operation with no acting user (an ownerless project, unattended work),
  **When** it runs,
  **Then** it uses the project or server token, as before.

## Functional requirements

- **FR-1** A GitHub operation whose actor's personal credential cannot be resolved returns that resolution error
  instead of resolving the project or server credential.
- **FR-2** The refusal happens before any HTTP request to GitHub.
- **FR-3** An actor without a personal GitHub token, and a call without an actor, resolve the same credential as
  before this change.
- **FR-4** ADR 0018 records the two remaining fallbacks on the project or server GitHub credential (ownerless project,
  actor without a personal GitHub token) as decisions.

## Acceptance criterion

A call made through the GitHub tracker adapter that carries an actor whose sealed credential is locked fails instead
of reading or writing with the project or server token, whether it comes from a background pass or from a person. A failed background pass records the
owner in `user_id`. The two remaining fallbacks are recorded as decisions in ADR 0018.

## Open requirements

None.
