# Specification #678 - A context repository prepared but left unchanged blocks the implemented transition

- Ticket: https://github.com/sebastienferry/sectile/issues/678
- Branch: `feat/678`
- Clarification: `docs/clarifications/678.md` (rounds 1 and 2, every
  recommendation accepted by the owner)
- Framework: Spec Kit

## Summary

A task that called `prepare_repository_worktree` for a secondary repository
records that repository as changed, for good. Today every stage that requires
pull request evidence then requires a pull request in that repository too,
even when the task never committed anything there: the transition fails and
nothing on the task can undo it. After this change, a secondary repository
whose task branch carries no commit of its own no longer requires a pull
request. The transition succeeds with the pull requests of the repositories
that did change, and its report names each skipped repository as "prepared,
unchanged".

## Scope

In scope:

- every stage check of secondary pull requests: the transitions that require
  pull request evidence (implemented, and clarified or specified when the
  project opens its pull request at that stage), whether reported through
  `transition_stage` or through a managed run's result, and the adjustment
  prerequisite;
- the local agent's answer about a secondary repository's task branch;
- the stage report, and the changelog.

Out of scope:

- a task that changed no repository at all, primary included (#584 covers
  it);
- the primary repository: it is never exempted here;
- any relaxation for a secondary branch that has commits ahead of its default
  branch;
- removing a repository from a task's changed repositories (no new MCP tool,
  no API, no migration);
- a new task state, or any web or Desktop change.

## Vocabulary

- **Secondary repository**: a repository the task recorded as changed through
  `prepare_repository_worktree`, other than its primary repository.
- **Task branch**: the branch the transition names, the task's work branch.
- **Default branch**: the branch the repository's `origin` names as its head,
  as the repository's own checkout knows it.
- **Commits ahead**: commits reachable from a ref of the task branch and not
  from the default branch.
- **Unchanged**: a secondary repository whose checkout the agent finds on the
  workstation and in which the task branch has zero commits ahead of the
  default branch, or does not exist at all, neither locally nor on `origin`.
- **Failed lookup**: any answer that does not prove the repository unchanged:
  no checkout found, a default branch that cannot be resolved, a Git or
  network failure, an agent too old to understand the question, an answer
  for another repository.

## User stories

### US1 (P1) - A prepared repository left untouched does not block the stage

As the owner of a task that prepared a worktree in a secondary repository and
then found nothing to change there, I want the implemented transition to
succeed with the primary pull request only, so that the task is not stuck on
a pull request that cannot exist.

- **Given** a task with a primary pull request on its branch and one
  secondary repository whose task branch exists with 0 commits ahead of the
  default branch, **when** the task is transitioned to implemented with the
  primary `prUrl` only, **then** the transition succeeds, the task records the
  primary pull request only, and the report names the secondary repository as
  prepared, unchanged.
- **Given** the same task where the secondary worktree and local branch were
  removed and the branch was never pushed, **when** the same transition is
  made, **then** it succeeds the same way.
- **Given** the task branch was pushed to `origin` with 0 commits ahead,
  **when** the same transition is made, **then** it succeeds the same way:
  being pushed changes nothing.

### US2 (P1) - A repository that did change still needs its pull request

As the owner, I want a secondary repository with real commits to keep
requiring its pull request, so that no work escapes review.

- **Given** the secondary task branch has at least one commit ahead of the
  default branch, locally or on `origin`, and no pull request, **when** the
  transition is made, **then** it fails and names that repository, as today.
- **Given** the commits were squash-merged elsewhere but the branch still has
  commits ahead, **when** the transition is made, **then** the pull request
  is still required.

### US3 (P1) - A lookup that cannot answer is never read as "unchanged"

As the owner, I want an unanswerable question to keep the pull request
required, so that an outage or an old agent never waives review.

- **Given** the agent finds no checkout of the secondary repository, or
  cannot resolve its default branch, or fails on Git or on the remote, or is
  too old to know the question, or answers for another repository, **when**
  the transition is made without that repository's pull request, **then** it
  fails as it does today.

### US4 (P2) - Later stages follow the same rule

As the owner, I want the adjustment of an implemented task to apply the same
rule, so that a stage after implemented is not blocked where implemented was
not.

- **Given** an implemented task whose secondary repository is unchanged and
  has no pull request, **when** the adjustment prerequisite is checked, **then**
  that repository is skipped.
- **Given** a later stage committed in that repository, **when** the next
  transition is made, **then** its pull request is required again.

## Functional requirements

- **FR1** For each secondary repository of the task that has no pull request
  given in `prUrls` and none recorded on the task branch, the server asks the
  local agent, before any forge lookup, whether the task branch is unchanged
  in that repository.
- **FR2** A repository the agent proves unchanged is skipped: no forge lookup,
  no pull request recorded for it, no refusal.
- **FR3** Unchanged holds when the agent finds a checkout of that repository
  whose own `origin` names it and either (a) the task branch exists neither
  locally, nor as a remote-tracking ref, nor on `origin`, or (b) every ref of
  the task branch it can see (local branch, remote-tracking ref, the head
  `origin` reports) has 0 commits ahead of the default branch.
- **FR4** Whether the branch was pushed plays no part in the decision.
- **FR5** Any failed lookup keeps the current rules: the pull request is
  looked up by branch and its absence refuses the transition with the current
  error.
- **FR6** A repository with a given or recorded pull request keeps the current
  rules unchanged (#392, #456); the question of FR1 is not asked for it.
- **FR7** The primary repository is never skipped.
- **FR8** The same rule applies to every path that checks secondary pull
  requests: a transition reported through `transition_stage`, a managed run's
  result, and the adjustment prerequisite.
- **FR9** The repository stays recorded as changed on the task; the question
  is asked again at every check, so a later commit there requires its pull
  request.
- **FR10** The report of a transition that skipped repositories carries one
  line per skipped repository naming it as prepared, unchanged, with the
  branch and the default branch it was compared to, in the stage notice the
  report already carries (as "Head commit not verified on a local checkout"
  is).
- **FR11** `CHANGELOG.md` gets one line under `### Fixed` in
  `## [Unreleased]`.

## Acceptance criteria

1. A two-repository task whose secondary branch has 0 commits ahead reaches
   implemented with the primary `prUrl` only; its report names the secondary
   repository as prepared, unchanged; no pull request is recorded for it.
2. The same holds when the secondary branch no longer exists locally or on
   `origin` while the checkout is found.
3. The same holds for a pushed branch with 0 commits ahead.
4. The transition still fails, naming the repository, when the secondary
   branch has commits ahead and no pull request.
5. The transition still fails when the agent finds no checkout, cannot
   resolve the default branch, fails, is too old, or answers for another
   repository.
6. A given or recorded secondary pull request is validated as today and the
   agent is not asked FR1's question for it.
7. The adjustment prerequisite skips an unchanged secondary repository and
   still refuses a changed one without a pull request.
8. Tests fake the forge through `prEvidenceLookup` or the agent's
   `pr_evidence` fake, and the agent through the existing agent fakes.
9. `CHANGELOG.md` carries the `Fixed` line.

## Edge cases

- A local branch exists with 0 ahead but `origin` holds the same branch with
  commits: changed (FR3 b looks at every ref).
- `origin` reports a head for the branch that the checkout does not have
  locally: its commits cannot be counted, so the lookup fails and the pull
  request stays required.
- A branch cut from an older default branch and never committed on: 0 ahead,
  unchanged. No fetch is needed.
- Several secondary repositories, some unchanged and some changed: the
  unchanged ones are skipped and named, the changed ones need their pull
  requests; the primary pull request stays the task's current one.
- Every secondary repository unchanged: the task behaves as a
  single-repository task with a report naming the skipped ones.
- The statement "changed no repository" of #584 keeps refusing a task with a
  recorded changed repository; this specification does not change it.

## Open points

None. Every product question was settled in round 2 of the clarification.
