# #392: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Shared helpers (FR1, FR3)

- [x] T1.1 `models.RepositoryIdentity(remote)`, moved from `internal/agent/agent_config.go`; the agent calls it.
- [x] T1.2 `models.PullRequestLink` and `models.ParsePullRequestLink(raw)` per the plan's contract.
- [x] T1.3 Table tests: GitHub link, gitlab.com nested groups, self-hosted GitLab host, `.git` and case differences in remotes, scp-like remotes, query/fragment/user info/`..`/non-numeric refused.

## 2. Runner (FR4, FR7)

- [x] T2.1 `ErrNoOriginRemote` with the FR7 message; `BranchPullRequest` checks `git remote` before any CLI.
- [x] T2.2 `RepositoryPullRequest(repository, branch)`: `glab mr list --source-branch <b> --all --output json -R https://<repository>` (all pages) or `gh pr view <b> --json ... -R <owner/repo>`, reusing `parsePullRequestEvidence`.
- [x] T2.3 Tests: temp repository without remote → `errors.Is(err, ErrNoOriginRemote)` and message names `origin`; `-R` argument built from the repository identity through an injected `run`.

## 3. Agent (FR6, FR8, FR9)

- [x] T3.1 `agentprotocol.Operation.Repository`.
- [x] T3.2 `pr_evidence` with `Repository` → `RepositoryPullRequest`; echo `repository`.
- [x] T3.3 `git_evidence` with `Repository` → verified checkout search (task `repoPath`, project `repoPath` and `repoPaths` from `GET /api/projects/<id>`, origin match, worktree on branch); echo `repository`; `found`.
- [x] T3.4 Adjust launch pre-check in `agent.go` uses `RepositoryPullRequest` when the task's current PR is foreign to the local checkout.
- [x] T3.5 Tests with temp repositories: candidate with another origin ignored; branch in a linked worktree found; branch not checked out → `found: false`; dirty checkout → `clean: false`.

## 4. Server (FR1 to FR9)

- [x] T4.1 `resolveStagePRTarget(project, prUrl)`: same / foreign / refused (mono-repo with remote); an unrecognized link keeps the project path.
- [x] T4.2 `lookupStagePR(..., prURL)` routing: foreign GitHub → server client on the foreign repo; foreign GitLab → agent with `Repository`; echo check.
- [x] T4.3 `validateStagePR` returns `(url, notice, err)`; foreign head check through `git_evidence` with `Repository`; notice when `found: false`; echo check.
- [x] T4.4 `stage.go` appends the notice to the note; `postback.go` adds it as an activity step.
- [x] T4.5 `adjustmentPrerequisite` passes `models.CurrentPullRequest(task.PrLinks)` as `prURL`.
- [x] T4.6 Widen `prEvidenceLookup` to `(repo, branch, prURL)` and update existing tests mechanically.

## 5. Server tests (acceptance)

Through `prEvidenceLookup` for the forge answer and `SetAgentOperations` for `git_evidence` / `pr_evidence`:

- [x] T5.1 SFE shape: no project repository, GitLab MR in `gitlab.com/smartadserver/private/arch/argocd-arch`, branch `feature/SFE-360-remove-arch-api`, `found: false` → `implemented` succeeds, PR recorded, `validateStagePR` returns the notice (US2, US3).
- [x] T5.2 Same with `found: true` and equal SHA → succeeds without notice; different SHA → refused; dirty on `reviewed` → refused (US3).
- [x] T5.3 `monoRepo=false` GitHub project with a foreign GitHub PR → succeeds (US2).
- [x] T5.4 Mono-repo project with remote and foreign `prUrl` → refused with the FR2 message, no lookup performed (US4).
- [x] T5.5 Foreign draft: `implemented` succeeds, `reviewed` refused; foreign merged: `reviewed` succeeds (US2).
- [x] T5.6 Foreign PR on another branch → refused (US4). An unrecognized link keeps the project path and is refused by the evidence check, as the existing `https://forge/pull/<n>` tests show (see plan).
- [x] T5.7 Agent without echo on `pr_evidence` or `git_evidence` → lookup failure naming the agent update; agent error on no-remote → FR7 message surfaces (US1, US5).
- [x] T5.8 Existing same-repository GitHub and GitLab tests unchanged and passing (US6).
- [x] T5.9 Post-back path records the notice step; adjust prerequisite uses the current foreign PR.

## 6. Verify and document

- [x] T6.1 `go build ./...`, `go vet ./...`, `go test ./...`.
- [x] T6.2 `CHANGELOG.md` `Fixed` entry under `[Unreleased]`: stage transitions accept a pull request from another repository for coordination projects, and name a missing `origin` remote.
- [ ] T6.3 Hand the owner the manual SFE-360 check (success criterion 3) in the implementation report.
