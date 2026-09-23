# #392: Plan

Spec: [`spec.md`](spec.md). Stack: Go server (`internal/db`, `internal/trackerapi`),
Go local agent (`internal/agent`, `internal/runner`), WebSocket agent operations
(`agentprotocol.Operation`), shared pure helpers (`internal/models`).

## Architecture

```
TransitionTaskStageBy / PostBackTask / adjustmentPrerequisite
        │  (task, actor, branch, prUrl)
        ▼
validateStagePR ──► resolveStagePRTarget(project, prUrl)
        │              ├── same repository / no prUrl ──► today's path (unchanged)
        │              ├── foreign + mono-repo with remote ──► refusal (FR2)
        │              └── foreign ──► target{Forge, Repository}
        ▼
lookupStagePR(task, actor, repo, branch, prUrl)
        ├── foreign GitHub ──► trackerapi.Client.BranchPullRequest(foreign owner/repo, branch)
        ├── foreign GitLab ──► agent "pr_evidence" {Repository} ──► Runner.RepositoryPullRequest (glab -R)
        └── otherwise ──► today's routing (stagePRForge)
        ▼
validatePullRequestEvidence (unchanged rules)
        ▼
head check
        ├── same repository ──► agent "git_evidence" on the project checkout (unchanged)
        └── foreign ──► agent "git_evidence" {Repository, Branch}
                          ├── verified checkout found ──► SHA (+ clean) compared as today
                          └── none ──► accepted, notice "head not verified locally"
```

## Target files

| File | Change |
| --- | --- |
| `internal/models/pullrequests.go` | `PullRequestLink{Forge, Host, Repository, Number}` and `ParsePullRequestLink(url)`; `RepositoryIdentity(remote)` (moved from `internal/agent/agent_config.go`, which then calls it). Pure, shared by server and agent. |
| `internal/db/adjustment.go` | `stagePRTarget`; `resolveStagePRTarget(project, prUrl)` (FR1 to FR3); `lookupStagePR` gains `prURL` and routes foreign requests; `validateStagePR` returns `(url, notice string, err error)` and performs the foreign head check; `adjustmentPrerequisite` passes the task's current PR as `prURL`. |
| `internal/db/db.go` | `prEvidenceLookup` widened to `func(repo, branch, prURL string)`. |
| `internal/db/stage.go` | Appends the notice to the transition note. |
| `internal/db/postback.go` | Records the notice as a step of the post-back activity. |
| `internal/agentprotocol/operations.go` | `Repository string \`json:"repository,omitempty"\`` on `Operation`. |
| `internal/runner/adjustment.go` | `ErrNoOriginRemote` and the explicit message (FR7); `RepositoryPullRequest(repository, branch)` using `-R`, reusing `parsePullRequestEvidence` and `gitLabEvidencePages`. |
| `internal/agent/agent_operations.go` | `pr_evidence` with `op.Repository` calls `RepositoryPullRequest`; `git_evidence` with `op.Repository` searches a verified checkout; both echo `repository` in their answer. |
| `internal/agent/agent.go` | Adjust launch pre-check uses `RepositoryPullRequest` when the task's current PR is foreign to the local checkout. |
| `internal/*/..._test.go` | See the test plan in [`tasks.md`](tasks.md). |
| `CHANGELOG.md` | `Fixed` entry under `[Unreleased]`. |

## Data contracts

### Project repository identity

`host/path`, lowercased, no scheme, no user, no `.git`, no trailing slash, from
`gitRemoteUrl` through `models.RepositoryIdentity`; else `github.com/<githubRepo>`;
else none. A pull request link's identity is `host/<repository>` from
`ParsePullRequestLink`. Foreign means the two identities differ or the project has
none.

### `ParsePullRequestLink(raw)`

- `https://<host>/<owner>/<repo>/pull/<n>` with a host naming GitHub, or equal to the
  configured GitHub host → `Forge: "github"`, `Repository: "<owner>/<repo>"`.
- `https://<host>/<group>/.../<project>/-/merge_requests/<n>` → `Forge: "gitlab"`,
  `Repository: "<group>/.../<project>"`, whatever the host (the `/-/merge_requests/`
  path identifies GitLab, self-hosted included).
- Anything else (user info, query, fragment, `..` segments, non-positive number,
  other shapes) → not a link; the transition refuses it as an unsupported forge.

### `pr_evidence` (extended)

Request: `Operation{Action: "pr_evidence", ProjectID, TaskID, Branch, UserID, Repository}`.
`Repository` is `host/path` and is only sent for a foreign GitLab merge request.
Result: today's fields plus `"repository": "<echo of the request>"`.
The server treats a missing or different echo, when it sent `Repository`, as a
lookup failure: `local agent is too old to look up a pull request in another
repository; update it` (FR9, US5).

### `git_evidence` (extended)

Request: `Operation{Action: "git_evidence", ProjectID, TaskID, UserID, Repository, Branch}`.
With `Repository`, the answer is:

```json
{"repository":"gitlab.com/g/app","found":true,"path":"/abs/checkout","sha":"…","branch":"feat/x","clean":true}
```

`found: false` (with empty evidence fields) means no verified checkout. The same echo
rule as `pr_evidence` applies. Without `Repository`, the operation is unchanged.

### Verified checkout search (agent)

1. Candidates, in order, de-duplicated: the task's `repoPath` (already read from
   `/api/tasks/<id>`), then the project's `repoPaths` read from
   `GET /api/projects/<id>`. They are hints: nothing else about them is trusted.
2. A candidate is kept only if it is an absolute, existing directory and
   `git -C <c> remote get-url origin`, normalised by `RepositoryIdentity`, equals
   `op.Repository`.
3. In a kept candidate, `git -C <c> worktree list --porcelain` gives the worktree
   whose `branch` is `refs/heads/<branch>`; the main checkout counts as one of them.
   The first match is the verified checkout; its `rev-parse HEAD` and
   `status --porcelain` give `sha` and `clean`.
4. No match anywhere → `found: false`. A git failure on a kept candidate is an
   operation error (lookup failure), not `found: false`.

### Notice

Exact text, appended to the transition note after a blank line, and added as a step
of the post-back activity:

`Head commit not verified on a local checkout: no checkout of <repository> on <branch> is known to the agent (pin the task's repository to verify it).`

## Decisions

- **Foreign lookup by branch in the named repository**, not `mr view <url>`: the
  selection rules (ambiguity, latest merge, closed ignored) and parsers stay the
  ones already tested, and the existing `URL == prUrl` check still decides. This
  refines the Round 1 wording "look the PR up by URL" without changing its effect.
- **GitHub foreign requests stay on the server** client, as GitHub same-repository
  requests do; the credential used is the same (`trackerAs(actor, "github",
  project)`). A 404 or 403 on the foreign repository is a lookup failure.
- **GitLab foreign requests go to the agent** with `glab ... -R https://<host>/<path>`,
  run from the project root, so no `origin` is needed.
- **Trust boundary** (FR2) is enforced on the server only, in
  `resolveStagePRTarget`, before any lookup. The agent pre-check reads, it does not
  decide.
- **Missing origin** is detected before calling a forge CLI: `git -C <path> remote`
  does not list `origin`. The runner returns `ErrNoOriginRemote` wrapping the
  message of FR7; the server passes it through as the lookup failure text.
- **Hook**: `prEvidenceLookup(repo, branch, prURL)` replaces the forge answer on
  every route; for a foreign request `repo` is the foreign identity. The head check
  is still exercised through `SetAgentOperations`.
- **Protocol compatibility**: new fields are optional; an older server sends none
  and gets today's behaviour; an older agent ignoring `Repository` is caught by the
  missing echo.
- **Adjust entry points**: the foreign repository is taken from
  `models.CurrentPullRequest(task.PrLinks)`; with no recorded PR, the adjust
  prerequisite keeps today's project lookup.

## Rejected alternatives

- **Scanning the disk for clones** of the foreign repository: unbounded, surprising,
  and outside Q2's decision (pinned and known paths only).
- **Sending candidate paths from the server in the operation**: the agent contract
  deliberately carries no server filesystem path; the agent reads them as hints
  from the API it already uses and proves them locally.
- **Forge-only evidence without echo detection**: an older agent would silently
  answer for the project checkout.
- **A per-project allowlist**: rejected in Q1 (new setting, migration and UI).
