# #364 — Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Runner (FR2, FR3, FR6, Q1)

- [x] T1.1 Add `Forge` to `PullRequestEvidence`; switch positional literals to keyed ones.
- [x] T1.2 Add `ErrNoMatchingPullRequest` and `ErrAmbiguousPullRequest`; wrap the absence errors.
- [x] T1.3 GitLab parsing: refuse several open MRs as ambiguous; latest `merged_at` wins; closed ignored.
- [x] T1.4 `BranchPullRequest`: `glab mr list --source-branch <b> --all --output json`; set `Forge` on the answer.
- [x] T1.5 Tests in `internal/runner/adjustment_test.go`.

## 2. Agent operation (FR2, FR6)

- [x] T2.1 Add `pr_evidence` to the accepted actions and handle it on the resolved checkout.
- [x] T2.2 Return refusals as `refusal`, failures as errors.

## 3. Server (FR1, FR4, FR5, FR7)

- [x] T3.1 `PullRequest.Forge` in `trackerapi` (`gitlab`, or empty for GitHub).
- [x] T3.2 `stagePRForge(project)`; `lookupStagePR(task, actor, branch)` routing; `agentBranchPullRequest`.
- [x] T3.3 Forge-aware messages in `validatePullRequestEvidence` and `validateStagePR`.
- [x] T3.4 Update the existing callers and tests to the new `lookupStagePR` signature.

## 4. Tests (D7)

- [x] T4.1 Forge selection: Jira + GitLab remote, GitHub repo + GitLab remote, GitHub remote, no remote.
- [x] T4.2 GitLab evidence through `prEvidenceLookup`: open, draft, merged, stale head, lookup failure, GitLab wording.
- [x] T4.3 Agent route through `SetAgentOperations`: success records the MR URL; refusal; unknown operation is a lookup failure.

## 5. Verify

- [x] T5.1 `go build ./...`, `go vet ./...`, `go test ./...`.
- [x] T5.2 Changelog entry under `[Unreleased]`.
