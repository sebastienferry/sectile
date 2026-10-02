# #697: A task carries one pull request per repository it changed

Ticket: https://github.com/sebastienferry/sectile/issues/697
Branch: `feat/697`.
Clarification: [`docs/clarifications/697.md`](../../docs/clarifications/697.md) (confirmed in Round 2).

## Context

A task that changed several repositories (`changedRepositories`, #456) needs one
pull request per repository. Stage transitions already accept and verify them
(`prUrls`), but every other path that records a pull request reasons about a
single one, and every view shows a single one or a flat history. A merge
request opened in a deployment repository beside the application can therefore
stay invisible in Sectile.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

Out of scope: opening the secondary repositories' pull requests automatically,
merging or tracking reviews across repositories, and checking any merge on the
server at `finished`.

## Terms

- **Repository of a link**: the repository a pull request URL names, in the
  `host/path` identity form `changedRepositories` uses. A URL that is not a
  recognized GitHub pull request or GitLab merge request has none.
- **Primary repository**: the repository a task runs in (its pin, else the
  project's own repository, under any of the identities the project declares).
- **Secondary repository**: a repository the task changed other than the
  primary one.
- **Current pull request of a repository**: its last recorded link.
- **Allowed repositories** of a task: its primary repository, every repository
  of its project, and every repository it changed.

## Decisions being specified

From the clarification, restated:

1. The repository of a link is derived from its URL and exposed read-only as
   `repository` on each link. Nothing new is stored for it; `prLinks` keeps its
   shape.
2. `prLinks` keeps the primary repository's current pull request last, so
   `prUrl` keeps naming the primary one.
3. The substitution guard applies within one repository. A link for another
   allowed repository is an addition; a link for a repository outside the
   allowed set is refused.
4. A new MCP tool, `record_pull_request`, records a pull request on a task
   without changing its stage.
5. The board card shows the primary pull request and a `+N` badge.
6. No server merge check at `finished`.
7. Pull request discovery backfills a changed repository with no recorded pull
   request, by branch.
8. A pull request whose forge has no token for the acting user shows "state
   unknown" with a tooltip naming the missing token; the sync raises no warning.

## User stories

### US1 (P1): Every repository's pull request is recorded

As the owner of a task that changed two repositories, I want both pull requests
recorded on the task, whichever skill or path opened them, so that Sectile knows
all of the task's work.

**Acceptance**

- **Given** a task whose primary pull request is recorded, **when** a pull
  request of a repository it changed is recorded through any path (stage
  transition, managed post-back, `record_pull_request`, discovery, task update),
  **then** it is appended and the primary pull request stays the task's `prUrl`.
- **Given** a task with a pull request in repository A, **when** a pull request
  of repository B on another branch is recorded, **then** it is accepted: the
  branch of A's links does not veto B.
- **Given** a recorded pull request of repository A on branch `feat/1`, **when**
  a pull request of A on an unrelated branch is recorded by a skill, adjustment
  or discovery, **then** it is refused as a substitution, naming the recorded
  branch.
- **Given** a task, **when** a pull request of a repository that is neither the
  primary one, a project repository, nor a changed one is given to
  `record_pull_request`, an adjustment or discovery, **then** it is refused,
  naming the allowed repositories.
- **Given** a link whose URL is not a recognized pull request, **then** the
  guard of today applies to it over the whole set.
- **Given** `record_pull_request(taskKey, url)` succeeds, **then** the task's
  stage, labels and tracker status are unchanged, and the answer lists the
  task's links.
- **Given** the `adjust-issue`, `create-pr`, `pickup-issue` and `pickup-issues`
  skills, **then** they tell the agent to record the pull request of every
  secondary repository it pushed (`prUrls` at a transition, `record_pull_request`
  outside one).

### US2 (P1): Every view lists every repository's pull request

As a user, I want to see each repository's pull request with its state, so that
an open merge request in a deployment repository cannot be overlooked.

**Acceptance**

- **Given** a task with pull requests in two repositories, **when** the web
  task detail is open, **then** it shows one group per repository, primary
  first, each naming its repository and showing its current pull request with
  its state, with that repository's earlier links as its history.
- **Given** the same task, **when** it is selected in the desktop, **then** the
  toolbar shows one indicator per repository, primary first, each with its
  state and opening its own pull request.
- **Given** the same task, **then** its board card shows the primary pull
  request's indicator and a `+1` badge whose tooltip names the other
  repository's pull request and state.
- **Given** a task with a single repository, **then** every view is unchanged:
  no repository label, no badge.

### US3 (P2): A missing secondary pull request is found again

As the owner of a task in the observed state (a changed repository with no
recorded pull request), I want the next synchronization to find it.

**Acceptance**

- **Given** a task past its pull request creation stage, not finished, with a
  branch, that changed a repository with no recorded pull request, **when** the
  project is synchronized, **then** the open or merged pull request of that
  repository on the task's branch is looked up and recorded, under the rules of
  US1.
- **Given** no pull request exists there, or the lookup fails, **then** nothing
  is recorded and the synchronization does not fail.

### US4 (P2): A pull request on a forge without a token says why its state is unknown

**Acceptance**

- **Given** a pull request on a forge for which the acting user has no token,
  **when** the project is synchronized, **then** no warning is added to the
  sync for that forge, and the link is marked with the forge whose token is
  missing.
- **Given** that mark, **then** the web and desktop indicators show "State
  unknown" with a tooltip naming the missing token (for example "no GitLab
  token").
- **Given** the token is configured afterwards, **when** the next refresh reads
  the state, **then** the mark disappears.

## Functional requirements

- **FR1** Each link of `prLinks` returned by the API carries `repository`, empty
  for an unrecognized URL. It is not stored.
- **FR2** Every path that appends a link keeps the primary repository's last
  link at the end of `prLinks`.
- **FR3** The per-repository guard (US1) is shared by the server and the agent
  through `internal/models`.
- **FR4** `record_pull_request` is part of the MCP catalog and of the agent
  bridge's whitelist.
- **FR5** Discovery of secondary pull requests is bounded to the tasks of US3
  and to the changed repositories with no recorded link.
- **FR6** A link carries `missingToken` (the forge) while its state could not be
  read for want of a token.
- **FR7** `CHANGELOG.md` has an `Added` line under `[Unreleased]`.

## Open points

None: the clarification settled every product question.
