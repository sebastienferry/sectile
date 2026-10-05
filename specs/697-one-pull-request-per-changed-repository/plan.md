# #697: Plan

References: [`spec.md`](spec.md), [`tasks.md`](tasks.md).

## Stack

Go server and agent (`internal/models`, `internal/db`, `internal/taskmcp`,
`internal/agentmcp`, `internal/agent`, `internal/skills`), React web
(`web/src`), Electron desktop (`desktop/src`). No migration: `pr_links` is a
JSON column and keeps its shape; the one new stored field (`missingToken`) is an
optional JSON key.

## Shared model (`internal/models/pullrequests.go`)

- `TaskPullRequest` gains `Repository string \`json:"repository,omitempty"\`` and
  `MissingToken string \`json:"missingToken,omitempty"\``.
- `PullRequestRepository(url) string`: `ParsePullRequestLink(url, "").Identity()`,
  `""` when the URL is not recognized.
- `NormalizePullRequestLinks` fills `Repository` from the URL and keeps
  `MissingToken`. `encodePullRequestLinks` (db) clears `Repository` before
  writing, so the identity is never stored (decision 1).
- `AcceptRepositoryPullRequest(links, url, branch, allowed []string) error`:
  - URL unrecognized: `AcceptPullRequest` over the whole set (today's rule);
  - URL already recorded: accepted;
  - `allowed` non-empty and the URL's repository not in it: refused, naming
    the allowed repositories;
  - otherwise `AcceptPullRequest` over the links of the same repository plus
    the unrecognized ones (whose repository is unknown, so they keep vetoing).
- `KeepPrimaryLast(links, primary []string)`: moves the last link whose
  repository is in `primary` (or unrecognized) to the end. No-op when `primary`
  is empty or no such link exists. `AddPullRequestLink(links, url, branch,
  primary)` is `AppendPullRequestLink` followed by it.

`AcceptPullRequest`, `AppendPullRequestLink` and `CurrentPullRequest` keep their
meaning; `CurrentPullRequest` stays the primary repository's thanks to FR2.

## Server (`internal/db`)

- `taskPullRequestScope(project, task) (primary, allowed []string)` in
  `stageprs.go`. Primary: the pin when it is not one of the project's
  identities, else every identity of `projectRepositoryIdentities` plus
  `TaskPrimaryRepository`. Allowed: primary, the identities of
  `project.Repositories`, and `taskChangedRepositories`.
- Paths:
  - `adjustment.go`: `adjustmentPrerequisite` and `validatePullRequestEvidence`
    use `AcceptRepositoryPullRequest`; the appended link goes through
    `KeepPrimaryLast`. `validatePullRequestEvidence` gains the allowed set.
  - `stage.go` and `postback.go`: already keep the primary last when `prUrls`
    is given; the post-back append also applies `KeepPrimaryLast` when only
    `prUrl` is given (a secondary one must not displace the primary).
  - `db.go` `UpdateTask` (`PATCH prUrl`): appends through `AddPullRequestLink`.
    No refusal: the task detail is how a person corrects links, and an
    explicit `prLinks` set replaces the list as it does today.
  - `prdiscovery.go`: `applyDiscoveredPullRequests` uses the per-repository
    guard and `KeepPrimaryLast`; seeding a task with no link is unchanged.
- `RecordPullRequest(ctx, actorID, taskKey, url)` in a new `recordpr.go`:
  reads the task and its project, refuses an unrecognized URL, checks
  `AcceptRepositoryPullRequest` with the task branch, appends on the locked row
  with `KeepPrimaryLast`, clears `pr_links_detached`, refreshes the states
  (`refreshTaskPullRequestStates`) and returns the task. No stage, label or
  tracker change, no activity. The branch is the task's.
- Secondary discovery (`prdiscovery.go`), `discoverSecondaryPullRequests`: for
  each task at or past its PR creation stage, not finished, with a branch and a
  changed repository with no recorded link, look the pull request up with
  `lookupStagePR(task, actor, "", branch, repositoryTarget(identity))` (GitHub
  through the server, GitLab through the caller's agent), keep it when open or
  merged on the branch, and record it as discovery does. Errors are swallowed:
  an agent that is not connected or a forge that refuses is not a sync
  failure. It runs after the issue discovery in the same sync pass.
- State refresh (`prstates.go`): a forge client with no token
  (`trackerapi.Client.HasForgeToken`) adds no warning; every non-merged link of
  that forge gets `MissingToken = forge`. A refresh that reads a link's state
  clears its mark. Link order never changes.

## MCP

- `internal/taskmcp/server.go`: `record_pull_request` with `taskKey` and `url`,
  answering `{task, prLinks}`. Description: records a pull request outside a
  stage transition, one call per repository pushed.
- `internal/agentmcp/mcp.go`: whitelist and count (15 → 16).
- `internal/mcptest/contract.go`: catalog entry. Count tests follow.

## Agent (`internal/agent/agent.go`)

The adjust dispatch checks `AcceptRepositoryPullRequest(task.PrLinks, pr.URL,
pr.Branch, nil)`: the pull request is the one of the primary checkout, scoped to
its repository. The server re-checks with the allowed set.

## Skills (`internal/skills/fragments`)

- `adjust/steps.md`: on a task with changed repositories, push each secondary
  repository's branch from its worktree and give its pull request in
  `transition_stage` `prUrls`.
- `create_pr/steps.md`: record each pull request it created or reused with
  `record_pull_request`, one call per repository.
- `contracts/transition.md` or the pickup fragments: the pickups pass `prUrls`
  for every secondary repository at `implemented` and `reviewed`.
- Plugin testdata and catalog goldens regenerated (`UPDATE_GOLDEN=1`, and the
  plugin `-update` flag, unsandboxed).

## Web (`web/src`)

- `types.ts`: `PullRequestLink.repository?`, `missingToken?`.
- `lib/pullRequests.ts`: `repositoryPullRequests(task)` returns groups
  `{repository, current, history}`: the primary group (the last link's
  repository, unrecognized links folded into it) first, the others in order of
  first appearance. `pullRequestStateTitle(link)` words the tooltip, naming the
  missing token.
- `TaskDetailModal.tsx`: one block per group when there are several, the
  repository as its heading, the current link and its history; unchanged with a
  single group.
- `TaskCard.tsx`: the primary indicator plus a `+N` badge with a tooltip
  listing the other repositories' pull requests and states.

## Desktop (`desktop/src`)

- `pullRequests.mjs`: `repositoryPullRequests(task)` (same grouping), state
  label "State unknown: no GitLab token" when `missingToken` is set.
- `main.js`: the `pullRequests` map holds the current link of each repository,
  primary first. The toolbar renders `#selected-pr` for the primary and one
  extra indicator per secondary repository in a `#selected-pr-others` group;
  the sidebar row and the ticket list show the primary one and `+N`.

## Rejected alternatives

- Storing the repository in `pr_links`: redundant with the URL and needing a
  backfill of every existing link.
- Ordering `prLinks` primary first: every single-PR consumer reads the last
  link; changing that meaning would touch the workflow checks for nothing.
- Refusing unrelated repositories on `PATCH`: the task detail is the human
  correction path; refusing there would leave a wrong link with no way out.

## Test plan

- Go table tests for `AcceptRepositoryPullRequest`, `KeepPrimaryLast`,
  `NormalizePullRequestLinks` (repository filled), encode (repository dropped).
- DB tests: post-back with a secondary `prUrl` only keeps the primary current;
  discovery accepts a secondary on another branch and refuses an unrelated
  repository; `RecordPullRequest` appends, refuses, leaves the stage alone;
  secondary discovery records the missing pull request through the
  `prEvidenceLookup` hook and skips a finished task; state refresh marks
  `missingToken` with no warning and clears it.
- MCP: catalog contract, bridge count, a `record_pull_request` call.
- Web `node --test` on `pullRequests.ts` grouping; desktop `node --test` on
  `pullRequests.mjs`.
- Skill goldens.
