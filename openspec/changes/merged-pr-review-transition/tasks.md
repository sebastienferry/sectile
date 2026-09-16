# Tasks

- [x] Query the branch PR with `state=all` and select open, then merged, in
      `internal/trackerapi/github.go`; carry a `Merged` flag on `PullRequest`.
- [x] Accept `Open || Merged` in `db.validatePullRequestEvidence` while keeping
      branch, URL, replacement and draft gates.
- [x] Accept a merged PR in `runner.parsePullRequestEvidence` for both GitHub and
      GitLab, preferring an open one when both are present.
- [x] State in the adjustment contract and skill template that a merged PR is
      reviewed in place and never pushed onto.
- [x] Cover the selection rule, the merged transition, the closed-unmerged
      rejection and the stale-head rejection with tests.
- [x] Record the decision as an ADR.
