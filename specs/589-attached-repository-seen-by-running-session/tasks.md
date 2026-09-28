# Tasks #589 - An attached repository is seen by a running session

Ordered. Each task names the requirements it covers and its tests.

## Phase 1 - Reproduce

- [ ] **T1** Merge `origin/main` into `feat/589` (no rebase: memory "Force
  push refused after rebase").
- [ ] **T2** Test `TestAttachingAFolderAppliesToTheNextOperation`
  (`internal/agent/agent_desktop_repositories_test.go`): refused, attach via
  `POST /desktop/folders`, accepted, detach, refused, all on one daemon with
  no restart. (FR1, FR2, US1-1, US1-3)
  - Red: the cause is in the agent, go to T9 with it.
  - Green: caching is ruled out; keep the test as the FR2 guard.
- [ ] **T3** Manual repro on the owner's workstation (needs the owner): agent
  connections for user and project, the MCP bridge's server against the
  desktop's, ticket steps 1 to 5, device that answered. Best done after T6 so
  the log line and the device name are available. (FR1)

## Phase 2 - Folder diagnosis and refusals

- [ ] **T4** `describeFolder` records `Err` for a `git` failure that is
  neither "not a git repository" nor "no such remote"; kinds unchanged.
  (FR4)
  - Test `TestDescribeFolderKeepsTheGitError` (`repositories_test.go`):
    missing path, plain folder, checkout without origin, checkout with origin,
    and a `git` failure forced by an unreadable `.git` (or `GIT_DIR` pointing
    nowhere) that sets `Err` and is not reported as "no origin".
  - `TestAttachedFoldersAreReadFromTheDisk` and the `/desktop/folders` tests
    stay green: the desktop sees the same kinds.
- [ ] **T5** `repositoryWorktree` builds its refusal from the diagnosis
  (US2-1 to US2-9), in French, naming the device. (FR5)
  - Test `TestRepositoryWorktreeRefusalNamesTheReason`: one subtest per case
    US2-1 to US2-7; the "attachez son dossier" advice appears in US2-1 only;
    the device name appears in US2-1.
  - Test: a failing folder beside a folder that answers for the repository
    does not prevent success (US2-8).
  - `TestRepositoryWorktreeInAnAttachedFolder` and
    `TestRepositoryWorktreeReusesTheTaskBranch` stay green. (FR6)
- [ ] **T6** `executeOperation` passes the device name and logs one line per
  operation (action, project, task, repository, outcome). (FR1, FR3)

## Phase 3 - Fix the confirmed cause

- [ ] **T7** Record the cause confirmed by T2/T3 in
  `docs/clarifications/589.md` and in the PR description.
- [ ] **T8** If another agent answered because of a stale or duplicate route:
  fix the route replacement in `AgentDispatcher`, with a test in
  `internal/handlers` where a second connection for the same user and project
  replaces the first and the operation reaches the new one. Propose an ADR if
  a routing rule is introduced. (FR3)
- [ ] **T9** If the cause is in the agent (T2 red) or in the `git`
  environment: fix it there, with T2 turning green. (FR3)
- [ ] **T10** If the cause is by design (another workstation or user
  environment): no fix beyond T5; state it in T7. (FR3)

## Phase 4 - Messages and documentation

- [ ] **T11** `stageprs.go`: the prUrls refusal adds that a repository
  becomes changed through `prepare_repository_worktree`. Update the tests
  matching "is not in a repository". (FR7, US3)
- [ ] **T12** `taskmcp/server.go`: the tool description states US4-1; update
  a test that pins it, if any. (FR8)
- [ ] **T13** `CHANGELOG.md`: one `Fixed` line under `## [Unreleased]`,
  worded after the confirmed cause. (FR9)
- [ ] **T14** Check `docs/contracts/server-agent-v1.md` for the error text of
  `repository_worktree`; update only if it is quoted there.

## Test plan

- `go test ./internal/agent/... ./internal/db/... ./internal/taskmcp/...
  ./internal/handlers/...` with the sandbox off and `GOCACHE` under
  `$TMPDIR`. The handlers keepalive test is a known flake under full-suite
  load: rerun it alone before blaming the change.
- `go vet ./...`.
- Manual: T3 repeated after the fix, the ticket's step 5 succeeds (or the
  refusal names the answering device when the cause is by design); then the
  owner links argocd-pl-tooling !844 to SFE-376 (out of scope for this PR).
