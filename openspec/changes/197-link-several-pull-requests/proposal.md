# A task links several pull requests

## Why
A task carries one `pr_url`. One ticket routinely produces several pull requests: a first one
merged, then a follow-up pushed on the same branch after a later clarification pass. Nothing in
the model holds the second one, and nothing in the web UI corrects the first.

Ticket #132 is stranded by exactly that. `pr_url` holds PR #181, merged; the follow-up work sits
in PR #195 on the same branch `feat/132`. `validateStagePR` (`internal/db/adjustment.go:69`)
promotes the recorded `pr_url` to `expected`, and `validatePullRequestEvidence`
(`internal/db/adjustment.go:61`) rejects the branch PR with `adjustment replaced the original PR`.
Every PR-bearing stage is affected — `create_pr`, `adjust`, `pickup`, `implement`, and `specify`
when it owns creation — so the task cannot leave `#specified` without forging evidence or writing
straight to the database.

A second wall sits behind the first. `trackerapi.BranchPullRequest` returns one PR per branch: a
branch whose only PRs are two merged ones fails with `expected one matching open or merged pull
request, got 0 open and 2 merged`. That is precisely the shape `feat/132` now has, so the
acceptance criterion "#132 can be unblocked by this path alone" is unreachable without it.

## What Changes
- A task holds an **ordered set of pull request links**, each keeping its URL and the branch it
  was opened from, in a new `pr_links` JSON column on `tasks`. Order is append order, oldest
  first; the set is deduplicated by URL.
- `pr_url` survives as the **current** PR, always the last entry of the set, so tracker
  synchronization, the `transition_stage` contract, post-back payloads and the existing read
  sites keep working unchanged.
- Existing rows are migrated: a non-empty `pr_url` becomes the single entry
  `{url: pr_url, branch: branch_name}`.
- The stage validators reason over the set. A recorded PR no longer vetoes a newer PR that shares
  a branch with a recorded one. The swap guard stays: a found PR whose branch matches no recorded
  link is refused.
- `BranchPullRequest` selects the most recently merged PR when a branch has several merged PRs and
  none open. More than one *open* PR remains an error.
- The web UI lets a human add, correct and detach a task's PR links from the task detail modal,
  without touching the database.
- The rule is stated in `docs/contracts/server-agent-v1.md` and in the transition prompts
  (`internal/db/skilltemplates.go`).

## Impact
`internal/models/models.go` (the `TaskPullRequest` type, `Task.PrLinks`, `UpdateTaskRequest`),
`internal/db/db.go` (schema, migration + backfill, task reads and writes, `UpdateTask`),
`internal/db/adjustment.go` (set-aware validation, recording), `internal/db/stage.go`,
`internal/db/postback.go`, `internal/trackerapi/github.go`,
`web/src/types/index.ts`, `web/src/components/TaskDetailModal.tsx`,
`docs/contracts/server-agent-v1.md`, `docs/API_AND_DATA_SPEC.md`.

The column is additive and defaulted, so an older binary still opens a database written by this
one, as every previous `tasks` migration does.

## Out of scope
- Who may merge: merging stays the human's call.
- Accepting a merged PR as review evidence (#153, landed in `merged-pr-review-transition`).
- Showing more than the current PR on cards and list rows; `TaskCard` and `ListView` keep
  linking `prUrl`.
- Any per-PR state cached locally (open, draft, merged): the forge stays the authority.
