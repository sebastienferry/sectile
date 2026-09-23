# #364 — Verify stage pull requests on GitLab merge requests

Ticket: https://github.com/sebastienferry/sectile/issues/364
Branch: `feat/364`.
Clarification: [`docs/clarifications/364.md`](../../docs/clarifications/364.md) (confirmed in Round 2).

## Context

The stage transitions that require a pull request as evidence (`specified` when the
PR creation owner is specify, `implemented`, `reviewed`, the agent post-back of the
same skills, and the launch prerequisite of `adjust`) only ever ask GitHub. A project
whose code lives on GitLab therefore can never pass them: every transition fails with
`configure an explicit GitHub owner/repository`, even with a pushed branch and an open
merge request whose head is the agent checkout.

This file states behaviour and acceptance criteria only. Implementation choices are in
[`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

Out of scope: the GitLab tracker adapter and the stored GitLab settings (#251), merge
request creation, a server-side GitLab API client, and any change to how GitHub
projects are verified.

## Decisions being specified

D1–D7 and Q1 of the clarification, restated:

1. The merge request is looked up by the local agent, on the task's resolved checkout,
   with the forge CLI the workstation is already logged into.
2. The forge is chosen from the project's code remote, never from its issue tracker.
3. The evidence rules are the GitHub ones, unchanged.
4. A merged merge request is evidence; a closed-unmerged one is not.
5. Refusals name GitLab and "merge request" for a GitLab lookup; GitHub wording is unchanged.
6. A lookup failure is reported as a failure, never as "no merge request".
7. Several open merge requests on the branch are refused as ambiguous, by the server
   and by the agent's own adjust pre-check alike.

## User stories

### US1 (P1) — A GitLab-hosted project reaches `implemented`

As the owner of a project whose code remote is GitLab (whatever its issue tracker),
I want `transition_stage` to find the branch's merge request, so that the task can
progress through `implemented` and `reviewed` and records the merge request as its PR.

**Acceptance**

- **Given** a Jira project whose remote is `git@gitlab.com:group/app.git`, a task on
  branch `feat/x`, and one open merge request from `feat/x` whose head is the agent
  checkout commit, **when** `transition_stage` records `implemented` (with or without
  `prUrl`), **then** it succeeds and the task PR is the merge request `web_url`.
- **Given** the same project and a draft merge request, **when** `implemented` is
  recorded, **then** it succeeds; **when** `reviewed` (adjust) is recorded, **then**
  it is refused because the merge request is still a draft.
- **Given** the merge request was merged by the human and its head is the checkout,
  **when** `reviewed` is recorded, **then** it succeeds.
- **Given** a `prUrl` that is not the merge request found for the branch, **then**
  the transition is refused.

### US2 (P1) — Wrong or missing evidence is refused, in GitLab terms

- **Given** no open or merged merge request on the branch (none at all, or only
  closed ones), **then** the transition is refused with a message saying no matching
  merge request exists.
- **Given** a merge request on another source branch, **then** it is refused.
- **Given** a merge request whose head differs from the agent checkout, **then** it is
  refused with a message saying the merge request does not contain the checkout commit.
- **Given** two or more open merge requests on the branch, **then** it is refused as
  ambiguous, and the agent refuses to launch `adjust` for the same reason.
- Every refusal of a GitLab lookup names GitLab or "merge request", never GitHub.

### US3 (P1) — A lookup failure is never absence

- **Given** no connected agent, an agent that does not know the lookup operation,
  `glab` missing or not authenticated, a timeout or a network error, **then** the
  transition is refused with a message saying the merge request lookup failed and why.

### US4 (P1) — GitHub projects are unchanged

- **Given** a project whose remote is a GitHub remote, or which has no remote and a
  GitHub `owner/repo`, **then** the pull request is looked up exactly as before, with
  the same messages.

## Functional requirements

- **FR1** The forge used for stage evidence is derived from the project's code remote:
  a remote naming GitHub uses the existing GitHub lookup; a remote naming GitLab uses
  the agent lookup, even when a GitHub repository is also configured. A remote that
  names neither, or no remote, keeps the GitHub lookup when a GitHub repository is
  configured and uses the agent lookup otherwise. The issue tracker plays no part.
- **FR2** The agent lookup reports, for the task's checkout and branch: the merge
  request URL, source branch, head SHA, open, draft and merged flags, and the forge
  that answered.
- **FR3** The agent lookup lists open and merged merge requests. Several open ones are
  ambiguous. With none open, the most recently merged one is the evidence. Closed
  unmerged ones are ignored.
- **FR4** The existing evidence rules apply unchanged to the agent answer: open or
  merged, same source branch, URL equal to the given `prUrl`, accepted against the
  task's recorded links, not a draft where readiness is required, head SHA equal to
  the agent checkout, clean checkout where required.
- **FR5** Refusal messages of a GitLab lookup say "GitLab" / "merge request".
- **FR6** A successful listing without a usable merge request and a failed lookup
  produce distinct messages; neither is ever treated as permission to proceed.
- **FR7** All three server entry points (stage transition, agent post-back, adjust
  launch prerequisite) use the same lookup.

## Success criteria

- The GODE-98 situation (Jira tracker, GitLab remote, draft MR on the checkout commit)
  transitions to `implemented` and records the MR URL.
- The existing GitHub tests pass unchanged.
