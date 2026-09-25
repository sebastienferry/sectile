# #456: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Shared model (FR1, FR2, FR3)

- [x] T1.1 `models.ProjectRepository`, `NormalizeProjectRepositories(codeRemote, urls)`: code remote first, dedupe by `RepositoryIdentity`, empty entries dropped.
- [x] T1.2 `Project.Repositories`, `Project.RepositoriesMigration`, `Task.Repository`, `Task.ChangedRepositories` and the request types; `FolderMapEntry`.
- [x] T1.3 Table tests: SSH and HTTPS of one remote dedupe; code remote kept first; primary resolution order of the plan (pinned, single, mono-repo, one mapped, none mapped, ambiguous).

## 2. Storage (FR2, FR3, FR5, FR7)

- [x] T2.1 Migrations 17 to 21 in `internal/db/migrations.go` only, one statement each.
- [x] T2.2 Read/write of the new columns in the project and task queries; pin validated against the project's repositories (400 otherwise); `registerProjectRepoPathUnsafe` no longer fed by new writes.
- [x] T2.3 `ApplyRepositoryConversion`: one transaction, compare-and-set on `repositories_migration`, pins tasks, clears legacy paths, stores the report; 409 when already migrated.
- [x] T2.4 `MarkRunAwaitingRepository` / `ClearRunAwaitingRepository`: set and clear `waiting_since` and `waiting_reason` for any mode; owner only.
- [x] T2.5 Tests on SQLite and PostgreSQL (`SECTILE_TEST_POSTGRES_DSN` on a throwaway database): migration on a pre-#456 database, conversion race (second call gets 409), pin refused outside the list, waiting on an autonomous run.

## 3. Server API and MCP (FR5, FR7, FR10, FR11)

- [x] T3.1 `GET /api/projects/{id}/legacy-repo-paths`, `POST /api/projects/{id}/repositories/convert`.
- [x] T3.2 `POST /api/activities/{id}/awaiting-repository`, owner or admin.
- [x] T3.3 `prepare_repository_worktree` MCP tool and `DB.PrepareRepositoryWorktree`: refusals of the plan, relay `repository_worktree` through `callAgentContext`, echo check, `AddChangedRepository`.
- [x] T3.4 `transition_stage` and post-back accept `prUrls`; `validateStagePRs` per the plan; `adjustmentPrerequisite` iterates over changed repositories.
- [x] T3.5 `RemoveTaskWorktree` sends the task's repositories.
- [x] T3.6 Tests: two changed repositories with both PRs pass `implemented`; one missing PR refused naming the repository; PR outside the required set refused; head mismatch refused; the #392 test shapes (no declared repositories) unchanged; `prepare_repository_worktree` refusals and idempotence with a fake agent.

## 4. Local agent (FR4, FR6, FR7, FR8, FR9, FR10, FR12)

- [x] T4.1 `Overrides.Repositories`; `WriteSettings` deletes an emptied `repositories` and `specRepos`; error text names `~/.config/sectile/settings.json`.
- [x] T4.2 `resolveRepositoryRoot` and `resolvePrimaryRepository`; `prepareDispatchLocked` creates the worktree in the primary root; unmapped pin fails naming the repository.
- [x] T4.3 `convertLegacyRepoPaths` on first dispatch or desktop listing of an unconverted mapped project; log of the report.
- [x] T4.4 `awaitRepository`: mark waiting, release the run slot, poll `GET /api/tasks/{key}` every 5 s over REST, resume the same dispatch on pin, finish `canceled` on cancel; never `finishDesktopRun(..., "failed")` for a wait.
- [x] T4.5 `buildFolderMap`, env `SECTILE_REPOSITORIES`, prompt block; `--add-dir` for claude interactive and headless; `{addDirs}` placeholder; no flag for other providers.
- [x] T4.6 `repository_worktree` operation through `ensureLocalWorktree`; `remove_workspace` over `op.Repositories` with `removed` / `failed`.
- [x] T4.7 `checkoutCandidates` puts the mapping of `op.Repository` first.
- [x] T4.8 Tests with temp repositories: mapping refused on origin mismatch; worktree in the pinned repository; ambiguous autonomous launch waits then resumes on pin (fake server); cancel while waiting; folder map content; command lines for claude with and without context folders, codex unchanged; secondary worktree reuses an existing branch; removal across two repositories with one failure.

## 5. Desktop and web (US1, US2, US4)

- [x] T5.1 Agent desktop endpoints: `GET /desktop/repositories?projectId=` and `POST /desktop/repositories` (map, pin).
- [x] T5.2 Desktop project dialog: a folder per project repository, origin check message; repository picker on a run waiting with reason `repository`, with map-from-picker.
- [x] T5.3 Web `ProjectModal`: repositories editor when `monoRepo` is false, duplicate refusal, conversion report notice.
- [x] T5.4 Web `TaskDetailModal`: repository select replacing the `repoPath` field.
- [x] T5.5 UI strings in the language each surface already speaks: French in the web task detail and badges, English in the web project settings' Git section and in the desktop; no other UI text translated.
- [x] T5.6 UI tests: desktop (`npx vite build` first) for the waiting run's repository choice; web unit tests for the identity helper and the waiting reason. No web browser test was added for the editor and the select.

## 6. Skills, docs and verification

- [x] T6.1 Skill fragments (implement, adjust, handoff): read `SECTILE_REPOSITORIES`, call `prepare_repository_worktree` before changing a context folder, one PR per changed repository passed as `prUrls`.
- [x] T6.2 ADR `docs/adrs/0027-repositories-are-keyed-by-remote.md`: remote-keyed repositories, workstation mappings, agent-side conversion, on-demand secondary worktrees, the waiting exception for autonomous runs.
- [x] T6.3 `README.md` / `docs/` where local settings and project configuration are described.
- [x] T6.4 `CHANGELOG.md`: `Added` line under `[Unreleased]`.
- [x] T6.5 `go test ./...` (rerun the handlers keepalive test alone if it flakes), `go vet`, web `tsc` and `oxlint`, desktop and web browser tests.
- [ ] T6.6 Manual check on a copy of the dev database, server started without tracker tokens: map two repositories, run a task in the second, change both, pass `implemented` with two PRs.
