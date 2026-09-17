# ADR 0010: A task links several pull requests

Status: Accepted

## Context

ADR 0004 records one pull request per task, in the `pr_url` column, and ADR 0009
adds that a merged PR is valid adjustment evidence while keeping the gate that the
forge PR "must match the PR already recorded on the task".

One ticket routinely produces several pull requests. A first one is merged; a later
clarification pass produces real work, pushed on the same branch as a second PR.
`validateStagePR` then promotes the recorded `pr_url` to the expected identity and
`validatePullRequestEvidence` rejects the branch PR with `adjustment replaced the
original PR`. Every PR-bearing stage is affected — `create_pr`, `adjust`, `pickup`,
`implement`, and `specify` when it owns creation.

Observed on task #132: `pr_url` held the merged PR #181, the follow-up work sat in
PR #195 on the same branch `feat/132`, and the task was stranded at specified with
the work pushed and reviewable. No recovery existed: the web UI could not edit or
clear the link, so the only ways out were forging evidence or writing straight to
the database.

A second wall sat behind the first. The forge lookup accepted a single merged PR per
branch, so a branch whose only PRs are two merged ones failed outright — the exact
shape `feat/132` ended in.

## Decision

A task holds an **ordered set** of pull request links, each keeping its URL and the
branch it was opened from, stored as JSON in a new `pr_links` column. `pr_url`
survives as the task's *current* pull request and is always the last entry of the
set; the two are written by the same statement and never diverge. Existing rows are
backfilled to a single link, idempotently.

The identity gate is restated over that set. A forge PR is accepted when the set is
empty, when it already holds that URL, or when it shares a branch with a recorded
link — that is the follow-up case, and a merged recorded link no longer vetoes it.
It is refused when every recorded link is on another branch: that is a PR
substituted for an unrelated one, which is the case the gate exists for. The
refusal names the branches the task recorded.

The forge lookup selects the most recently merged PR when a branch has several
merged ones and none open. Several *open* PRs on one branch stays an error: which
one is current cannot be guessed without letting the forge's listing order decide
the identity gate.

The web UI gains an editor for the set — add, correct in place, detach — in the task
detail view.

## Consequences

The branch-lineage gate is only as good as the branches recorded on the links, so a
task legitimately re-branched must have its stale links detached by a human. That is
why the editor ships in the same change as the gate: a guard with no recovery path
is the defect this ADR removes, not one to reintroduce.

`pr_url` remains the single link carried by the tracker comment, the
`transition_stage` contract and the cards; making all of those set-aware is not
required to unblock the workflow and is deliberately not done. No state of a PR
(open, draft, merged) is cached locally — the forge stays the authority, which is
the class of staleness this ADR is about.

Sectile still never merges, approves or closes a task, and no new workflow state is
introduced.
