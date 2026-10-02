# Tasks #678 - A context repository prepared but left unchanged blocks the implemented transition

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 0. Base

- [ ] T0.1 Move `feat/678` onto `origin/main`: `git rebase --onto origin/main
  6e148d26` while the branch is unpushed, a merge of `origin/main` otherwise.
  Check `git log origin/main..HEAD` lists only the #678 commits.

## 1. Agent: the `branch_changes` operation (FR1, FR3, FR5)

- [ ] T1.1 Add `branch_changes` to `agentprotocol.Operations`.
- [ ] T1.2 `repositoryCheckout(ctx, repository, candidates)` in
  `internal/agent/branchchanges.go`; rewrite `verifiedCheckout` on top of it
  without changing its answers (existing `evidence` tests stay green).
- [ ] T1.3 `branchChanges(ctx, checkout, branch)`: default branch from
  `origin/HEAD`, refs (local, remote-tracking, `ls-remote`), `ahead` as the
  maximum `rev-list --count`; every failure an error; all through `gitLocal`.
- [ ] T1.4 Dispatch `branch_changes` in `agent_operations.go` with the
  `repositoryFolder` candidate first, then `checkoutCandidates`; echo
  `repository`.
- [ ] T1.5 Tests (`branchchanges_test.go`): the nine cases of the plan's
  agent test plan.
- [ ] T1.6 Do not add it to `localInspections` (it reaches the network).

## 2. Server: skip an unchanged secondary repository (FR2, FR6 to FR10)

- [ ] T2.1 `unchangedRepository` in `internal/db/stageprs.go` and its notice
  (`evidenceTerms` for "merge request" or "pull request").
- [ ] T2.2 `validateStagePRs`: ask it for a non-primary repository with no
  given or recorded link, before `validateStagePRAt`; skip and append the
  notice on true.
- [ ] T2.3 `checkSecondaryPRs`: same skip when no recorded link, silently.
- [ ] T2.4 `fakeRepoAgent` answers `branch_changes` from a per-repository
  map, default `{"found":true,"exists":true,"ahead":1}`, and records what it
  was asked; update the other fakes that set a changed repository
  (`crossrepo_test.go`, `no_repository_change_test.go`, `activerun_test.go`)
  so no existing test changes meaning.
- [ ] T2.5 Tests: the server cases of the plan's test plan (unchanged, gone,
  ahead, each failed-lookup kind, given link, recorded link, adjustment,
  early-owner specified, managed result).

## 3. Documentation

- [ ] T3.1 `docs/contracts/server-agent-v1.md`: the `branch_changes`
  paragraph after the `git_evidence` one.
- [ ] T3.2 `CHANGELOG.md`: the `Fixed` line of the plan under
  `## [Unreleased]`.

## 4. Gates

- [ ] T4.1 `gofmt -l internal` prints nothing; `go vet ./...`; `go build ./...`.
- [ ] T4.2 `go test ./internal/agentprotocol/... ./internal/agent/...
  ./internal/db/... ./internal/handlers/...` outside the sandbox; rerun a
  keepalive flake before blaming the change.
- [ ] T4.3 Check every acceptance criterion of `spec.md` against a test.
