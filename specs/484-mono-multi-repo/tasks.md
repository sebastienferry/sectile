# Tasks #484 - Attached folders, and one kind of project

Specification: `spec.md`. Plan: `plan.md`. Tasks are ordered; each phase ends
green (`go test ./...`, `go vet ./...`) before the next starts. `[P]` marks a
task that can run in parallel with the previous one.

## Phase 1 - Remove the mono-repo/multi-repo flag (US4, FR-014 to FR-016, FR-018)

- [ ] T001 `internal/models/repository.go`: drop `monoRepo` from
  `ResolvePrimaryRepository`, remove `PrimaryAmbiguous` and the `pin` result;
  update its doc comment. Tests: pinned and mapped, pinned and unmapped, pin
  outside the list, no pin with several mapped repositories (now
  `PrimaryDefault`).
- [ ] T002 `internal/agent/repositories.go`, `agent.go`,
  `agent_operations.go`, `specexclude.go`: follow T001; remove
  `errRepositoryAmbiguous`, `repositoryPollInterval`, `awaitRepository` and the
  retry loop at `agent.go:1045`. Remove or rewrite the tests that exercised the
  wait.
- [ ] T003 `internal/agentconfig/config.go`, `internal/db/agentconfig.go`:
  remove `MonoRepo` / `IsMonoRepo`; every caller follows (`repositoryWorktree`
  refusal, `buildFolderMap`, `localSpecRepo`, `agent_desktop.go`).
- [ ] T004 `internal/agent/agent_macro_dispatch.go`: `localSpecRepo` defaults to
  the code checkout, `errNoSpecFolder` removed (US5, FR-017). Test: no setting
  gives the root; a set folder wins; a missing set folder still fails.
- [ ] T005 `internal/agent/agent_desktop.go`: `specDefault` is the root for
  every configured project; `monoRepo` leaves the answer. Test the GET answer.
- [ ] T006 `internal/models/models.go`, `internal/db/db.go`: remove
  `Project.MonoRepo`, the request fields and every `mono_repo` read and write
  (baseline `CREATE TABLE` untouched).
- [ ] T007 `internal/db/migrations.go`: migration 30 drops `projects.mono_repo`
  and clears `waiting_reason = 'repository'`, for SQLite and PostgreSQL. Update
  the replay fixtures (`forgetSchemaVersion`, the `version >= N` fixtures) to
  re-add `mono_repo`. Tests: the column is gone after migration; a parked
  activity is cleared; a database at version 29 migrates.
- [ ] T008 `internal/db/repositories.go`: `taskPin` without the mono-repo
  guard; remove `MarkRunAwaitingRepository`. `internal/handlers/handlers.go`:
  remove the `awaiting-repository` route and its tests.
- [ ] T009 `internal/db/adjustment.go`: drop the mono-repo refusal in
  `resolveStagePRTarget`; adapt its test (a foreign pull request is now
  accepted under the #456 checks).
- [ ] T010 Handler test: creating and updating a project with a `monoRepo`
  key in the body succeeds and ignores it (FR-014).
- [ ] T011 [P] `web/src`: remove the *Mono-repo* checkbox and state from
  `ProjectModal.tsx`, show the repositories editor on every project;
  `TaskDetailModal.tsx` pins when there are more than one repository;
  remove `monoRepo` from `types/index.ts`; remove the repository wait wording
  in `ActivitiesView.tsx` and `RemoteRunBadge.tsx`. Run the web tests,
  `tsc` and `oxlint` (memory "worktrees have no node_modules").
- [ ] T012 [P] `desktop/src/main.js`: remove the *Repository layout* row and
  `renderServer`'s layout line, the spec folder's required state, the
  `monoRepo` condition on *Other repositories*, and the console's "Waiting for
  a repository" group and picker. Delete `desktop/tests/repository-choice.ui.cjs`;
  update the `monoRepo` fixtures of `console.ui.cjs`, `git-init.ui.cjs`,
  `spec-artifacts.ui.cjs`, `spec-folder.ui.cjs`.

## Phase 2 - Changed repositories beyond the project's list (US3, FR-011 to FR-013)

- [ ] T013 `internal/db/repositories.go`: `taskChangedRepositories` returns every
  recorded identity except the primary one and duplicates, with no filter on
  the project's repositories. Tests: an identity outside the list is returned;
  the primary is left out.
- [ ] T014 `internal/db/repositories.go`: `PrepareRepositoryWorktree` resolves a
  target outside the project's list by `models.RepositoryIdentity`, refuses an
  empty identity, relays, keeps the echo and primary checks, records only after
  the agent answered. Tests with a fake agent: outside identity recorded;
  agent error records nothing; path input refused.
- [ ] T015 `internal/db/stageprs.go` tests: a ticket whose changed repository
  is outside the project's list is refused at `implemented` without that
  repository's pull request, accepted with it, the primary pull request
  staying current; `checkSecondaryPRs` on adjust; `RemoveTaskWorktree` sends
  that identity in `Repositories`.
- [ ] T016 `internal/taskmcp/server.go`: rewrite the `prepare_repository_worktree`
  and `prepare_macro_worktree` descriptions (plan section 8); check the MCP
  contract test and any golden description.

## Phase 3 - Attached folders on the workstation (US1, US2, FR-001 to FR-010)

- [ ] T017 `internal/agentconfig/workstation.go`, `local.go`:
  `ProjectSettings.Folders`; merge it in the overlay; an emptied list is
  written as absent. Tests: round trip, merge, removal of the last folder.
- [ ] T018 `internal/agent/attached.go` (new): `attachedFolders` and
  `repositoryFolder` (mapping first, attached second). Tests with temporary
  Git repositories: Git with origin, Git without origin, plain folder,
  missing folder, subfolder of a checkout described at its top level, mapping
  wins over an attached folder of the same identity.
- [ ] T019 `internal/agent/repositories.go`, `agent_operations.go`:
  `repositoryWorktree`, `removeRepositoryWorktrees` and `git_evidence` use
  `repositoryFolder`. `repositoryWorktree` refuses an attached folder without
  a remote ("change it in place") and an identity neither mapped nor attached
  ("attach it in the desktop project settings"). Tests: worktree created on
  the ticket branch in an attached folder, reused on a second call; removal in
  an attached folder; a missing attached folder is named in `Failed`.
- [ ] T020 `internal/models/repository.go`: `FolderRoleLocal`,
  `FolderMapEntry.Kind`, `FolderMapEntry.Attached`.
- [ ] T021 `internal/agent/repositories.go`: `buildFolderMap`,
  `folderMapDirs`, `folderMapPrompt` per plan section 4. Tests: attached Git
  folder as context, as changed with its worktree; local folder; missing
  folder listed and not in the directories; identity equal to a project
  repository not listed twice; folder equal to the root or spec folder not
  listed twice; prompt wording for each role; no block with a single folder.
- [ ] T022 `internal/agent/agent_config.go`: `addDirArgs` emits
  `--add-dir=<path>` per folder for `codex`. Tests: Claude unchanged, Codex
  repeated flags, vibe nothing, `{addDirs}` placeholder for Codex.
- [ ] T023 `internal/agent/agent_desktop_repositories.go`, `agent_desktop.go`:
  `/desktop/folders` GET, POST, DELETE per plan section 7. Tests: each refusal
  of FR-003 with its message; a project repository's remote stored as its
  mapping (`mappedAs`); code repository refused; duplicate identity refused;
  delete of a missing folder; no server call carries the path (assert on the
  fake server's received requests).
- [ ] T024 `desktop/src/main.js` and the preload/IPC bridge: `api.folders`,
  `api.attachFolder`, `api.detachFolder`; the *Attached folders* row per plan
  section 10, in small hunks. New `desktop/tests/attached-folders.ui.cjs`:
  empty list, add a Git folder, add a plain folder, missing folder shown and
  removed, refused duplicate shows the message, `mappedAs` refreshes *Other
  repositories*. Build with `npx vite build` before the UI tests, restore
  `webui/.gitkeep` after.

## Phase 4 - Documentation (US6)

- [ ] T025 `docs/adrs/0033-attached-folders-and-one-kind-of-project.md`; status
  lines of ADR 0027 and ADR 0028.
- [ ] T026 [P] `README.md`, `docs/ARCHITECTURE.md`, `docs/CAPABILITIES.md`:
  remove the mono/multi-repo wording, document attached folders, the `local`
  role, the `kind`/`attached` keys, Codex `--add-dir`.
- [ ] T027 [P] `CHANGELOG.md` `[Unreleased]`: the four lines of plan section 11.

## Phase 5 - Verification

- [ ] T028 `go vet ./...`, `go test ./...` (rerun a keepalive failure before
  blaming the change), and the PostgreSQL suite with a throwaway
  `SECTILE_TEST_POSTGRES_DSN` (never the dev database).
- [ ] T029 Web: tests, `tsc`, `oxlint`. Desktop: UI tests after a build.
- [ ] T030 `grep -rn 'monoRepo\|MonoRepo\|mono_repo\|awaiting-repository\|errRepositoryAmbiguous'`
  returns only the migration, the replay fixtures and the CHANGELOG.
- [ ] T031 Manual check on a copy of the database, with the tracker token
  unset (memories "never run a branch server on the dev DB", "branch server
  writes reach the real tracker"): attach a Git folder and a plain folder to a
  project, launch a skill, read `SECTILE_REPOSITORIES` and the prompt block;
  call `prepare_repository_worktree` on the attached Git folder and see the
  identity on the ticket; see `implemented` refused without its pull request.

## Acceptance coverage

| Clarification criterion | Tasks |
| --- | --- |
| 1 desktop list | T017, T018, T023, T024 |
| 2 local storage only | T017, T023 |
| 3 launch folder map, CLI dirs, missing folder | T020, T021, T022 |
| 4 `prepare_repository_worktree` on attached folder | T014, T019 |
| 5 transitions require its pull request | T013, T015 |
| 6 in-place rule for folders without remote | T021, T016 |
| 7 flag removed, primary = pin else code | T001 to T012 |
| 8 no repository wait | T002, T007, T008, T011, T012 |
| 9 spec folder default | T004, T005, T012 |
| 10 handoff cleanup | T015, T019 |
| 11 CHANGELOG | T027 |
| 12 ADR | T025 |
