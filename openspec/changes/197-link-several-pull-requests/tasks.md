# Tasks

## 1. Model and storage
- [x] 1.1 Add `models.TaskPullRequest{URL, Branch}` and `Task.PrLinks []TaskPullRequest`
      (`prLinks`) in `internal/models/models.go`, plus `PrLinks *[]TaskPullRequest` on
      `UpdateTaskRequest`.
- [x] 1.2 Add the `pr_links TEXT NOT NULL DEFAULT '[]'` column to the `tasks` creation statement
      and to the additive migration block in `internal/db/db.go`.
- [x] 1.3 Backfill existing rows from `pr_url` + `branch_name`, guarded by `pr_links = '[]'` so the
      statement is idempotent.
- [x] 1.4 Read and scan `pr_links` in every `SELECT ... FROM tasks` path, and carry it through the
      task `INSERT` / `UPDATE` statements.

## 2. Recording
- [x] 2.1 Add `recordTaskPullRequest` in `internal/db/adjustment.go`: append when absent,
      deduplicate by URL, rewrite `pr_url` from the last entry, in one statement.
- [x] 2.2 Call it from `TransitionTaskStage` (`internal/db/stage.go`), inside the existing
      transaction, in place of the bare `pr_url = ?` assignment.
- [x] 2.3 Call it from `PostBackTask` (`internal/db/postback.go`) once `validateStagePR` returns a
      verified URL.
- [x] 2.4 Call it from `adjustmentPrerequisite` where a discovered-but-unlinked PR is recorded.
- [x] 2.5 In `UpdateTask` (`internal/db/db.go`), accept `prLinks`, normalize it and recompute
      `pr_url` from its last entry — clearing it when the set is emptied.

## 3. Validation
- [x] 3.1 Rework `validatePullRequestEvidence` so the `expected` argument becomes the task's
      recorded links: accept an empty set, a set containing the found URL, or a set with a link on
      the found PR's branch; refuse otherwise with an error naming the recorded branch.
- [x] 3.2 Update `validateStagePR` to pass the set instead of promoting `task.PrURL` to `expected`.
- [x] 3.3 Update `adjustmentPrerequisite` to accept a branch follow-up instead of failing on
      `recorded PR does not match the task-branch PR`.
- [x] 3.4 In `trackerapi.BranchPullRequest`, parse `merged_at`, and select the most recently merged
      PR when no PR is open; keep the error for more than one open PR and report the counts.

## 4. Web UI
- [x] 4.1 Add `prLinks?: PullRequestLink[]` to `Task` and the update payload in
      `web/src/types/index.ts`.
- [x] 4.2 Replace the unrendered `prUrl` state in `web/src/components/TaskDetailModal.tsx` with a
      link list editor: one row per link (external link, branch, detach), an add input defaulting
      the branch to the task's `branchName`, and in-place URL correction.
- [x] 4.3 Include the list in the unsaved-changes comparison and in the save payload.

## 5. Tests
- [x] 5.1 `internal/db/adjustment_test.go`: a merged recorded PR does not veto a newer PR on the
      same branch; a PR on an unrelated branch is still refused; the first PR of a task is
      accepted.
- [x] 5.2 `internal/db/stage_test.go`: the set grows in order and `pr_url` tracks its last entry
      across two transitions.
- [x] 5.3 A migration test: a row written with `pr_url` only is backfilled to one link, and a
      second open of the database does not duplicate it.
- [x] 5.4 `internal/trackerapi`: branch evidence with two merged PRs and none open returns the
      most recently merged; two open PRs still fail.
- [x] 5.5 `UpdateTask`: editing and emptying `prLinks` keeps `pr_url` consistent.
- [x] 5.6 Run `go test ./...` and the web suite (`npm test`, `tsc --noEmit`, `oxlint`).

## 6. Documentation
- [x] 6.1 State the rule in `docs/contracts/server-agent-v1.md`: `prUrl` records the current PR of
      an ordered set, a follow-up on the same branch is accepted, an unrelated branch is refused.
- [x] 6.2 Update the `tasks` fields in `docs/API_AND_DATA_SPEC.md`.
- [x] 6.3 Update the transition prompts in `internal/db/skilltemplates.go` so an agent knows a
      task can carry several PRs and that a same-branch follow-up is legitimate.
- [x] 6.4 `openspec validate 197-link-several-pull-requests --strict`.

## 7. Recorded during implementation
- [x] 7.1 `docs/adrs/0010-a-task-links-several-pull-requests.md`: the set, the branch-lineage gate
      and the recovery path it depends on, superseding the single-PR identity rule of ADR 0009.
- [x] 7.2 `design.md` amended: `recordTaskPullRequest` replaced by pure helpers written inside the
      existing transactions, and `validateStagePR` loses its already-dead `expected` parameter.
- [x] 7.3 The set rules moved to `internal/models/pullrequests.go`: the agent's own adjustment
      pre-check (`internal/agent/agent.go`) enforced the same single-PR rule and would have refused
      the follow-up before the server saw it. Server and agent now share one implementation.
