# Specification #584 - A task that changed no repository can be recorded as implemented

- Ticket: https://github.com/sebastienferry/sectile/issues/584
- Branch: `feat/batch-393-497-498-584-647-648`
- Clarification: `docs/clarifications/584.md` (rounds 1 and 2, option A)
- Framework: Spec Kit

## Summary

A skill that finished a task whose work changed no repository says so when it
records the stage. The server accepts the statement in place of a pull
request, refuses it when the task shows that it did change a repository, and
the stage report says that no pull request was expected.

## Scope

In scope: the `transition_stage` input, the server's validation, the stage
report line, the skills' instructions, tests and the changelog.

Out of scope: a check of the branch's commits on the agent, a stored flag or
a card display, the board's manual move, and tasks with code.

## User stories

### US1 (P1) - A task without code reaches implemented and reviewed

1. Given a project that opens its pull request at `implemented` and a task
   with no recorded pull request and no changed repository, when a skill
   calls `transition_stage(implemented, noRepositoryChange: true)` with a
   note, then the task moves to `implemented` and the report ends with "No
   pull request: this task changed no repository."
2. Given the same task at `implemented`, when the skill records `reviewed`
   with the statement, then the task moves to `reviewed` with the same line.
3. Given a project whose checkout has no `origin` remote, when the statement
   is given, then no pull request lookup is made and the transition succeeds.

### US2 (P1) - The guard still holds for tasks with code

1. Given the same task, when `implemented` is recorded without the statement
   and without `prUrl`, then it fails with the current explicit error.
2. Given the statement together with `prUrl` or `prUrls`, then the call is
   refused.
3. Given a task that records a pull request on its branch, when the
   statement is given, then it is refused.
4. Given a task with a repository recorded as changed (through
   `prepare_repository_worktree`), when the statement is given, then it is
   refused.

### US3 (P2) - Skills know the path

1. Given a skill that ends a task without a code change, when it reads its
   instructions, then they tell it to pass `noRepositoryChange: true` with a
   justification in the note, and never to forge a pull request.

## Functional requirements

- FR1. `transition_stage` accepts `noRepositoryChange` (boolean, default
  false) on the stages that validate pull request evidence.
- FR2. With the statement, the server skips the pull request lookup, after
  refusing the cases of US2.2 to US2.4.
- FR3. The stage report carries the line "No pull request: this task changed
  no repository."
- FR4. Without the statement, nothing changes.

## Acceptance criteria

- AC1. US1 and US2 are covered by tests through `prEvidenceLookup`.
- AC2. The skills' instructions name the statement.
- AC3. A `Fixed` line in `CHANGELOG.md`.
- AC4. SFE-367 is moved to `implemented` with the statement once the server
  is deployed (owner step).

## Open points

None. The commit-ahead check of the ticket is dropped by decision.
