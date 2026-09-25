# #456: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Shared model (FR1, FR2, FR3)

- [ ] T1.1 `models.ProjectRepository`, `NormalizeProjectRepositories(codeRemote, urls)`: code remote first, dedupe by `RepositoryIdentity`, empty entries dropped.
- [ ] T1.2 `Project.Repositories`, `Project.RepositoriesMigration`, `Task.Repository`, `Task.ChangedRepositories` and the request types; `FolderMapEntry`.
- [ ] T1.3 Table tests: SSH and HTTPS of one remote dedupe; code remote kept first; primary resolution order of the plan (pinned, single, mono-repo, one mapped, none mapped, ambiguous).

## 2. Storage (FR2, FR3, FR5, FR7)

- [ ] T2.1 Migrations 15 to 19 in `internal/db/migrations.go` only, one statement each.
- [ ] T2.2 Read/write of the new columns in the project and task queries; pin validated against the project's repositories (400 otherwise); `registerProjectRepoPathUnsafe` no longer fed by new writes.
- [ ] T2.3 `ApplyRepositoryConversion`: one transaction, compare-and-set on `repositories_migration`, pins tasks, clears legacy paths, stores the report; 409 when already migrated.
- [ ] T2.4 `MarkRunAwaitingRepository` / `ClearRunAwaitingRepository`: set and clear `waiting_since` and `waiting_reason` for any mode; owner only.
- [ ] T2.5 Tests on SQLite and PostgreSQL (`SECTILE_TEST_POSTGRES_DSN` on a throwaway database): migration on a pre-#456 database, conversion race (second call gets 409), pin refused outside the list, waiting on an autonomous run.

## 3. Server API and MCP (FR5, FR7, FR10, FR11)

- [ ] T3.1 `GET /api/projects/{id}/legacy-repo-paths`, `POST /api/projects/{id}/repositories/convert`.
- [ ] T3.2 `POST /agent/runs/{id}/awaiting-repository` with the agent token.
- [ ] T3.3 `prepare_repository_worktree` MCP tool and `DB.PrepareRepositoryWorktree`: refusals of the plan, relay `repository_worktree` through `callAgentContext`, echo check, `AddChangedRepository`.
- [ ] T3.4 `transition_stage` and post-back accept `prUrls`; `validateStagePRs` per the plan; `adjustmentPrerequisite` iterates over changed repositories.
- [ ] T3.5 `RemoveTaskWorktree` sends the task's repositories.
- [ ] T3.6 Tests: two changed repositories with both PRs pass `implemented`; one missing PR refused naming the repository; PR outside the required set refused; head mismatch refused; the #392 test shapes (no declared repositories) unchanged; `prepare_repository_worktree` refusals and idempotence with a fake agent.

## 4. Local agent (FR4, FR6, FR7, FR8, FR9, FR10, FR12)

- [ ] T4.1 `Overrides.Repositories`; `WriteSettings` deletes an emptied `repositories` and `specRepos`; error text names `~/.config/sectile/settings.json`.
- [ ] T4.2 `resolveRepositoryRoot` and `resolvePrimaryRepository`; `prepareDispatchLocked` creates the worktree in the primary root; unmapped pin fails naming the repository.
- [ ] T4.3 `convertLegacyRepoPaths` on first dispatch or desktop listing of an unconverted mapped project; log of the report.
- [ ] T4.4 `awaitRepository`: mark waiting, release the run slot, poll `GET /api/tasks/{key}` every 5 s over REST, resume the same dispatch on pin, finish `canceled` on cancel; never `finishDesktopRun(..., "failed")` for a wait.
- [ ] T4.5 `buildFolderMap`, env `SECTILE_REPOSITORIES`, prompt block; `--add-dir` for claude interactive and headless; `{addDirs}` placeholder; no flag for other providers.
- [ ] T4.6 `repository_worktree` operation through `ensureLocalWorktree`; `remove_workspace` over `op.Repositories` with `removed` / `failed`.
- [ ] T4.7 `checkoutCandidates` puts the mapping of `op.Repository` first.
- [ ] T4.8 Tests with temp repositories: mapping refused on origin mismatch; worktree in the pinned repository; ambiguous autonomous launch waits then resumes on pin (fake server); cancel while waiting; folder map content; command lines for claude with and without context folders, codex unchanged; secondary worktree reuses an existing branch; removal across two repositories with one failure.

## 5. Desktop and web (US1, US2, US4)

- [ ] T5.1 Agent desktop endpoints: `repositories` in `GET/POST /desktop/projects`; `GET /desktop/tasks/{key}/repositories`; `POST /desktop/tasks/{key}/repository`.
- [ ] T5.2 Desktop project dialog: a folder per project repository, origin check message; repository picker on a run waiting with reason `repository`, with map-from-picker.
- [ ] T5.3 Web `ProjectModal`: repositories editor when `monoRepo` is false, duplicate refusal, conversion report notice.
- [ ] T5.4 Web `TaskDetailModal`: repository select replacing the `repoPath` field.
- [ ] T5.5 French UI strings in `translations.ts` and the desktop; no other UI text translated.
- [ ] T5.6 UI tests: desktop (`npx vite build` first; restore `web/.../webui/.gitkeep` after) and web browser tests for the editor, the select and the picker.

## 6. Skills, docs and verification

- [ ] T6.1 Skill fragments (implement, adjust, handoff): read `SECTILE_REPOSITORIES`, call `prepare_repository_worktree` before changing a context folder, one PR per changed repository passed as `prUrls`.
- [ ] T6.2 ADR `docs/adrs/0027-repositories-are-keyed-by-remote.md`: remote-keyed repositories, workstation mappings, agent-side conversion, on-demand secondary worktrees, the waiting exception for autonomous runs.
- [ ] T6.3 `README.md` / `docs/` where local settings and project configuration are described.
- [ ] T6.4 `CHANGELOG.md`: `Added` line under `[Unreleased]`.
- [ ] T6.5 `go test ./...` (rerun the handlers keepalive test alone if it flakes), `go vet`, web `tsc` and `oxlint`, desktop and web browser tests.
- [ ] T6.6 Manual check on a copy of the dev database, server started without tracker tokens: map two repositories, run a task in the second, change both, pass `implemented` with two PRs.
