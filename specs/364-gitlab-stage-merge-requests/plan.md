# #364 — Plan

Spec: [`spec.md`](spec.md). Stack: Go server (`internal/db`), Go local agent
(`internal/agent`, `internal/runner`), WebSocket agent operations
(`agentprotocol.Operation`).

## Architecture

```
TransitionTaskStageBy / postback / enqueue adjust
        │
        ▼
validateStagePR / adjustmentPrerequisite
        │
        ▼
lookupStagePR(task, actor, branch)
        ├── stagePRForge(project) == github ──► trackerapi.Client.BranchPullRequest (unchanged)
        └── otherwise ──► callAgent("pr_evidence") ──► Runner.BranchPullRequest (glab / gh)
```

## Target files

| File | Change |
| --- | --- |
| `internal/runner/adjustment.go` | `PullRequestEvidence.Forge`; `glab mr list --all`; ambiguity on several open MRs; latest `merged_at` wins; sentinel errors `ErrNoMatchingPullRequest` / `ErrAmbiguousPullRequest` separating refusals from lookup failures. |
| `internal/agent/agent_operations.go` | New `pr_evidence` operation: runs `Runner.BranchPullRequest` on the resolved checkout for `op.Branch` (else the task branch); a refusal is returned as data (`refusal`), a lookup failure as an operation error. |
| `internal/trackerapi/github.go` | `PullRequest.Forge`: `gitlab` for a merge request read by the agent, empty for GitHub, so the GitHub client is untouched. |
| `internal/db/adjustment.go` | `stagePRForge`; `lookupStagePR` takes the task and routes; `agentBranchPullRequest`; forge-aware wording in `validatePullRequestEvidence` / `validateStagePR`. |
| `internal/db/adjustment_test.go` | Forge selection, GitLab rules through `prEvidenceLookup`, agent route through `SetAgentOperations`. |
| `internal/runner/adjustment_test.go` | `parsePullRequestEvidence` GitLab merged / ambiguous / closed / readiness cases. |

## Data contract — `pr_evidence`

Request: `Operation{Action: "pr_evidence", ProjectID, TaskID, Branch, UserID}`.
Result (JSON):

```json
{"forge":"gitlab","url":"https://gitlab.com/g/app/-/merge_requests/9","branch":"feat/x","sha":"…","open":true,"draft":false,"merged":false,"refusal":""}
```

`refusal` is non-empty when the forge answered but holds no usable MR (none, or
ambiguous); the evidence fields are then empty. An operation error means the lookup
failed. Timeout: the default 45 s budget (the lookup reaches the network; the runner
has its own 25 s limit).

## Decisions

- **Forge selection** (`stagePRForge`): the remote *host* (URL or scp-like form)
  containing `github` → GitHub; containing `gitlab` → agent; otherwise GitHub when
  `GithubRepo` is set, else agent. Reading the host, not the whole remote, keeps a
  `github.com:acme/gitlab-mirror` remote on GitHub.
  The third rule refines D2 so that a GitHub project whose remote hostname names no
  forge keeps its exact current path (US4).
- **Hook**: `prEvidenceLookup(repo, branch)` stays the injection point replacing the
  forge answer on either route; the answer carries `Forge`, so GitLab wording is
  testable through it. The agent route itself is tested with a fake agent
  (`SetAgentOperations`) answering `pr_evidence`.
- **Refusal wording on the agent**: the runner's refusals are typed
  (`ErrNoMatchingPullRequest`, `ErrAmbiguousPullRequest`) but keep a message in the
  forge's terms; the server prefixes a GitLab refusal with `GitLab: `.
- **Wording**: GitHub keeps "forge … PR"; GitLab says "GitLab … merge request". The
  agent-route failure says "GitLab merge request lookup failed on the local agent"
  (or "pull request lookup" when the remote does not name GitLab).
- **Pagination**: `--all` selects MR states only. Read every page with explicit
  `--per-page 100 --page N` under the shared lookup deadline before selecting
  evidence. A failed later page invalidates the entire lookup.
- **Ordering of merged MRs**: by `merged_at` when present, else the listing order
  (glab lists newest first).

## Rejected alternatives

- A `trackerapi` GitLab client using `gitlab_url` / `gitlab_token`: depends on the
  settings #251 may remove and on a server-side credential the workstation already has.
- Choosing the forge from the issue tracker: a Jira project can live on either forge.
