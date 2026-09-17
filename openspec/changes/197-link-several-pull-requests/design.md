# Design

## Storage shape
A JSON column on `tasks`, not a child table:

```sql
ALTER TABLE tasks ADD COLUMN pr_links TEXT NOT NULL DEFAULT '[]';
```

`labels`, `stage_columns` and `ai_skill_models` already store their collections this way. A child
table would force every one of the six `SELECT ... FROM tasks` read paths in `internal/db/db.go`
to grow a join and a second scan for what is, in practice, one to three rows per task. The set is
read and written whole, never queried across tasks, so the JSON column is the honest shape.

Each entry is minimal:

```go
type TaskPullRequest struct {
    URL    string `json:"url"`
    Branch string `json:"branch,omitempty"`
}
```

No cached `open` / `merged` / `draft` flag. Those go stale the moment a human touches the forge,
and the validators already read them live through `lookupStagePR`.

## Why `pr_url` stays
`pr_url` becomes a derived value: the URL of the last entry of `pr_links`. It is not deprecated
and not dropped.

- `TrackerOp.PrURL` (`internal/db/trackerops.go:206,348`) writes the link into the tracker comment
  and the post-back payload.
- The `transition_stage` MCP contract carries one `prUrl`, and the agent contract documents it.
- `TaskCard.tsx`, `ListView.tsx` and the modal header link one PR — the current one.

Making all of those set-aware would be a far larger change than the ticket asks for, and every one
of them genuinely wants *the* current PR. The invariant is that `pr_url` never diverges: every
write to one goes through `recordTaskPullRequest`, which appends to the set and rewrites `pr_url`
from it in the same statement.

## Migration
Done in the additive-`ALTER TABLE` block of `runMigrations`, followed by a one-shot backfill:

```sql
UPDATE tasks
   SET pr_links = json_array(json_object('url', pr_url, 'branch', COALESCE(branch_name, '')))
 WHERE pr_links = '[]' AND pr_url IS NOT NULL AND TRIM(pr_url) != '';
```

`modernc.org/sqlite` ships the JSON1 functions (verified: `SELECT json_array(json_object(...))` returns `[{"url":"u","branch":"b"}]`), so this stays one statement. It is guarded by
`pr_links = '[]'` and therefore idempotent: a row already migrated is not rewritten, and a row
whose links were later detached from the UI is not resurrected.

## The swap guard, restated over the set
`validatePullRequestEvidence` keeps its first two checks unchanged — the forge PR must be open or
merged, must sit on the branch under validation, and must be the URL being recorded. Only the
`expected` check changes shape. Instead of one pinned URL, it takes the task's recorded links:

| Recorded links | Found PR | Verdict |
| --- | --- | --- |
| empty | any valid PR | accepted, first link |
| contains the found URL | the same PR | accepted, no new entry |
| some link shares the found PR's branch | a newer PR on that branch | accepted, appended |
| every link is on another branch | a PR on an unrelated branch | **refused** |

The last row is the guard the ticket asks to keep. It is why each entry stores its branch: without
it the set cannot tell a follow-up from a substitution.

A merged recorded PR is not a special case in the rule — it falls out of it. `#132` has PR #181
recorded on `feat/132` and PR #195 found on `feat/132`: same branch, so it is a follow-up. A PR
found on `feat/999` while every link is on `feat/132` is a substitution and is refused.

Re-branching a task legitimately therefore needs a human gesture: detach the stale links in the
modal. That is deliberate, and it is the reason the UI editor is in the same change rather than a
follow-up — the guard must never be a dead end.

## Several merged PRs on one branch
`BranchPullRequest` partitions the branch's PRs into open and merged and today accepts only a
single candidate in each bucket. The merged bucket becomes ordered:

- exactly one open PR → that one, as today;
- no open PR and one or more merged → the most recently merged, by `merged_at`;
- more than one open PR → still an error.

Two *open* PRs on one branch is a genuine ambiguity about which one is current, and guessing there
would let the swap guard be bypassed by whichever PR the forge happened to list first. Two merged
PRs is not ambiguous: the branch moved on, and the latest merge is the state of the branch.

The response already carries `merged_at`; it is parsed into the `PullRequest` value so the sort has
something to sort on. `Open` and `Merged` keep their meaning.

## Where links are recorded
The set rules are pure functions in `internal/models/pullrequests.go` — `AppendPullRequestLink`,
`CurrentPullRequest`, `NormalizePullRequestLinks`, `PullRequestBranches` and `AcceptPullRequest`.
They live in `models` rather than in `db` because the **agent** applies the same guard before it
dispatches an adjustment (`internal/agent/agent.go`), and that pre-check enforced the old
single-PR rule: left alone it would have refused the follow-up before the server ever saw it. Two
copies of the rule is exactly the kind of disagreement this ticket is about.

`internal/db/pullrequests.go` keeps only the storage concern — `decodePullRequestLinks`,
`encodePullRequestLinks`, `pullRequestURLValue` — and each existing writer persists `pr_url` and
`pr_links` in its own statement.

A single `recordTaskPullRequest(taskID, ...)` doing its own `UPDATE` was the first shape written
down, and it was dropped during implementation: `TransitionTaskStage` writes the PR inside a
transaction that also carries the status, labels and the stage activity, and a helper with its own
statement would have written the set outside that transaction. The invariant "`pr_url` is the last
link" is better served by computing both values from the same slice than by a second round trip.

The writers are:

- `stage.go` — inside the existing transaction of `TransitionTaskStage`, replacing the bare
  `pr_url = ?` assignment with `pr_url = ?, pr_links = ?`.
- `postback.go` — the payload's `prURL` is appended to the set before the row is written.
- `adjustment.go` — `adjustmentPrerequisite`, where a discovered-but-unlinked PR is recorded, and
  only when the set actually grows.
- `UpdateTask` — the human editor path below.
- `internal/agent/agent.go` — the agent's adjustment pre-check, which records a discovered PR
  through the existing `PATCH /api/tasks/<id>` and now compares against the set.

`validateStagePR` loses its `expected` parameter rather than gaining a set-shaped one: both call
sites passed `""`, so the pin was already dead code, and the recorded set now reaches the
validator through the task it already receives.

## The UI editor
`TaskDetailModal` already holds a `prUrl` state (line 146) that no input renders — the link is
read-only in practice. It becomes a list editor in the task metadata block:

- one row per link: the URL as an external link, its branch, and a detach button;
- an input plus an Add button, which normalizes the URL and defaults the branch to the task's
  `branchName`;
- editing a row's URL in place is the "correct it" path.

The state is a `TaskPullRequest[]`, sent through the existing `UpdateTaskRequest` as `prLinks`.
`UpdateTask` recomputes `pr_url` from the last entry, and clears it when the set is emptied, so the
invariant holds on the human path too. The unsaved-changes guard (line 377) compares the
serialized list.

## Rejected alternatives
- **A `task_pull_requests` child table.** Correct normalization, wrong cost: six read paths, a
  second write path, and cascade handling, for a collection that is always read whole.
- **Dropping `pr_url`.** It is the contract with the tracker, the MCP `transition_stage` argument
  and three UI call sites. Removing it is a breaking change the ticket does not ask for.
- **Caching the merged state of each link.** The validators would then decide on a snapshot, which
  is the class of bug this ticket is about.
- **Letting the newest PR win unconditionally.** That deletes the swap guard the ticket explicitly
  asks to keep.
