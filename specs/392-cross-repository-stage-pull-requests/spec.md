# #392: Stage pull requests without an origin remote or in another repository

Ticket: https://github.com/sebastienferry/sectile/issues/392
Branch: `feat/392`.
Clarification: [`docs/clarifications/392.md`](../../docs/clarifications/392.md) (confirmed in Round 2).

## Context

The stage transitions that require a pull request as evidence (`specified` when the
PR creation owner is specify, `implemented`, `reviewed`, the agent post-back of the
same skills, and the launch prerequisite of `adjust`) look the pull request up by
branch in the project checkout, then compare the checkout HEAD with the pull
request head. Two kinds of project can therefore never pass them:

- a project whose checkout has no `origin` remote: the lookup fails with an opaque
  `read repository remote: exit status 2`;
- a coordination project whose code changes land in another repository (SFE and
  `argocd-arch`): the pull request is never in the project repository, and the
  project checkout never holds its head.

This file states behaviour and acceptance criteria only. Implementation choices are
in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

Out of scope: creating pull requests in another repository, a settings UI or an
allowlist of repositories per project, the PR state refresh (it already reads the
forge and repository from each link), forges other than GitHub and GitLab, and
replaying SFE-360 as part of the pull request.

## Terms

- **Project repository**: the repository named by the project's code remote
  (`gitRemoteUrl`, else `githubRepo`). A project may have none.
- **Foreign pull request**: a `prUrl` whose repository differs from the project
  repository, or any `prUrl` when the project has no project repository.
- **Verified checkout**: a local checkout the agent found among the task's pinned
  `repoPath`, the project's `repoPath` and the project's `repoPaths`, whose own `origin` names the foreign
  pull request's repository, and which has the task branch checked out.

## Decisions being specified

From the clarification, restated:

1. A foreign pull request is accepted only when the project has no project
   repository or is not mono-repo (`monoRepo=false`). A mono-repo project with a
   project repository keeps the same-repository rule.
2. A foreign pull request's head is checked on a verified checkout when one exists.
   Without one, the forge evidence alone is accepted, and the transition says the
   head was not verified on a local checkout.
3. A missing `origin` is reported by name, with the fix, and stays a lookup failure.
4. The existing evidence rules are unchanged: open or merged, same source branch as
   the task branch, a URL equal to `prUrl`, accepted against the task's recorded
   links, no draft where readiness is required, and a lookup failure is never absence.
5. Replaying SFE-360 is a manual check by the owner after the merge.

## User stories

### US1 (P1): A project without a remote gets an explicit error

As the owner of a project whose checkout has no `origin`, I want the error to name
the missing remote and the way out, so that I or a skill can fix the setup instead
of stopping on an opaque integration failure.

**Acceptance**

- **Given** a project with no project repository whose checkout has no `origin`,
  **when** a transition requiring PR evidence is recorded **without** `prUrl`,
  **then** it is refused with a message saying the project checkout has no `origin`
  remote and that the project repository must be configured or the `prUrl` of the
  repository carrying the pull request must be given.
- The message is presented as a failed lookup, never as "no matching pull request",
  and never as permission to create one.

### US2 (P1): A coordination project records a pull request from another repository

As the owner of a project whose code lands elsewhere, I want `transition_stage` to
verify the `prUrl` against the repository it names, so that the task moves forward
and records that pull request.

**Acceptance**

- **Given** the SFE shape (Jira tracker, no project repository, checkout without
  `origin`), a task on `feature/SFE-360-remove-arch-api`, and an open, ready GitLab
  merge request from that branch in `gitlab.com/smartadserver/private/arch/argocd-arch`,
  **when** `implemented` is recorded with that merge request as `prUrl` and that
  branch, **then** it succeeds and the task PR is that merge request.
- **Given** a project with a GitHub project repository and `monoRepo=false`, **when**
  `implemented` is recorded with an open pull request of another GitHub repository
  from the task branch, **then** it succeeds.
- **Given** the foreign merge request is a draft, **when** `implemented` is recorded,
  **then** it succeeds; **when** `reviewed` is recorded, **then** it is refused as a
  draft.
- **Given** the foreign merge request was merged by the human, **when** `reviewed`
  is recorded, **then** it succeeds.

### US3 (P1): The foreign head is checked where a checkout exists

**Acceptance**

- **Given** a foreign pull request and a verified checkout, **when** its branch head
  equals the pull request head, **then** the transition succeeds and says nothing
  about verification.
- **Given** a verified checkout whose head differs from the pull request head,
  **then** the transition is refused with a message saying the pull request does not
  contain the checkout commit.
- **Given** a verified checkout with uncommitted changes, **when** `reviewed`
  (adjust) or a pickup is recorded, **then** it is refused as for a project checkout.
- **Given** no verified checkout (no pinned path, no known path, a candidate whose
  `origin` names another repository, or no candidate with the branch checked out),
  **then** the transition succeeds on forge evidence, and the recorded transition
  says the head was not verified on a local checkout, naming the repository.
- A path the server knows is only a candidate: a candidate whose own `origin` does
  not name the pull request's repository is never used.

### US4 (P1): The trust boundary holds

**Acceptance**

- **Given** a mono-repo project with a project repository, **when** a transition is
  recorded with a `prUrl` of another repository, **then** it is refused with a
  message saying the pull request is not in the project repository and that only a
  project without a code remote or not mono-repo may record one from elsewhere.
- **Given** a foreign pull request whose source branch is not the task branch, or
  whose branch is unrelated to the task's recorded links, **then** it is refused as
  today.
- **Given** a `prUrl` that is neither a GitHub pull request nor a GitLab merge
  request link, **then** it is never treated as a foreign pull request: it keeps
  the project path, where the forge answer never matches it and the evidence
  check refuses it (or the lookup fails when the project has no repository).

### US5 (P1): Lookup failures stay failures

**Acceptance**

- **Given** a foreign pull request, **when** the forge cannot be reached, the
  credential or CLI login cannot read the foreign repository, no agent is connected,
  or the agent is too old to look up another repository, **then** the transition is
  refused with a message saying the lookup failed and why. None of these is read as
  "no pull request" or as "no verified checkout".

### US6 (P1): Same-repository projects are unchanged

**Acceptance**

- **Given** a project with an `origin` and either no `prUrl` or a `prUrl` in the
  project repository, **then** the pull request is looked up and verified exactly as
  before, with the same messages, including the head check on the project checkout.

## Functional requirements

- **FR1** A `prUrl` is a foreign pull request when its forge host and repository
  path differ from the project repository's, compared case-insensitively and without
  a `.git` suffix, or when the project has no project repository.
- **FR2** A foreign pull request is refused on a mono-repo project that has a
  project repository (US4). The refusal is a refusal, not a lookup failure.
- **FR3** The forge of a foreign pull request comes from its URL: GitHub for a
  GitHub pull request link, GitLab for a merge request link (`/-/merge_requests/<n>`)
  on any host. Other links keep the project path (see US4).
- **FR4** A foreign pull request is looked up in the repository its URL names, for
  the task branch, with the same selection rules as a project lookup (one open
  request, else the latest merged one, several open ones ambiguous, closed ignored).
- **FR5** The evidence rules of decision 4 apply unchanged to the foreign answer.
- **FR6** The head check of a foreign pull request uses a verified checkout (Terms)
  when one exists; cleanliness is required there where it is required today.
  Without one, the transition proceeds on forge evidence and its recorded outcome
  states that the head was not verified on a local checkout of the named repository.
- **FR7** When no `prUrl` is given and the checkout has no `origin`, the lookup
  fails with a message naming the missing remote and the two fixes (US1).
- **FR8** The three server entry points (stage transition, agent post-back, adjust
  launch prerequisite) and the agent's own adjust launch pre-check apply the same
  rules. For the adjust entry points, the foreign repository is the one of the
  task's current PR.
- **FR9** An agent that cannot answer for another repository is a lookup failure
  (US5), never a silent fallback to the project checkout.

## Success criteria

- The SFE-360 shape (no remote, GitLab merge request in another repository, branch
  `feature/SFE-360-remove-arch-api`) passes `implemented` in automated tests.
- The existing GitHub and GitLab stage evidence tests pass unchanged.
- Manual, after the merge and the agent update: the owner re-runs an SFE-360-like
  `implemented` transition with MR !97 and it goes through.

## Open requirements

None. Every product question was settled in the clarification.
