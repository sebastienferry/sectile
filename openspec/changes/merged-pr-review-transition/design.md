# Design

## Where the rejection came from
Three independent gates required an open PR:

1. `trackerapi.(*Client).BranchPullRequest` queried GitHub with `state=open`, so a
   merged PR was not even fetched, and `len(pages) != 1` surfaced as
   "expected one matching open pull request, got 0".
2. `db.validatePullRequestEvidence` required `pr.Open`.
3. `runner.parsePullRequestEvidence`, the agent-side pre-check, required
   `state == "OPEN"` (`"opened"` on GitLab).

All three had to accept a merged PR; relaxing one alone only moves the failure.

## Selection rule
The lookup now queries `state=all` for the branch head and partitions the result:

- exactly one open PR → that one, unchanged behaviour;
- no open PR and exactly one merged PR → the merged one;
- anything else → an error naming both counts.

An open PR keeps priority over a merged one, so a branch reopened for follow-up
work is adjusted against the live PR rather than the merged history. Closed and
unmerged PRs are ignored entirely: a branch whose PR was closed without merging
is abandoned work, and counting it would resurrect a PR nobody accepted.

## What readiness means once merged
The draft gate exists so adjustment cannot declare success on a PR no reviewer
can see. A merged PR is past that question, so readiness is not evaluated for it
— the forge cannot report a merged PR as a draft. The remaining evidence still
applies in full, and is what keeps this from becoming a bypass: the PR head SHA
must equal the agent checkout commit and the branch must match, so a merged PR
belonging to other work cannot complete a task.

## Rejected alternatives
- **Let the reviewed transition skip PR evidence when the branch is contained in
  the default branch.** It removes the forge from the loop entirely and would
  accept a branch merged by any means, including a local merge never reviewed.
- **Record reviewed automatically on merge.** Merging is the human's decision and
  says nothing about whether the adjustment review ran; the stage would start
  recording work that was never done.
- **Relax only the server-side check.** The agent's own pre-check would still
  refuse to start, so the Adjust action would stay unusable on a merged PR.
