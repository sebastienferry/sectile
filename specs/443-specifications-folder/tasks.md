# Tasks #443 - Specifications folder

Ordered checklist. Each group is one commit (Conventional Commits). Check the
next free migration number against `origin/main` before group 5.

## 1. Shared specification lookup (FR7, US6)

- [ ] T1.1 Create `internal/sddfiles` with `SearchRoots`, `FindMacroDir`,
  `Read(ctx, folder, framework, key, file)`, moved from
  `internal/db/sddslicing.go`; branch fallback only when the folder is a Git
  checkout; refusal texts name the desktop "Specifications folder" setting and
  omit the branch hypothesis on a plain folder.
- [ ] T1.2 Move the lookup tests from `internal/db/sddslicing_test.go` /
  `sddentries_test.go` to `internal/sddfiles`; add: plain folder found, plain
  folder missing (no "branche" in the message), Git folder read from the macro
  branch.

## 2. Layout and resolver on the agent (FR3, US2, US3)

- [ ] T2.1 `agentconfig.Config.MonoRepo *bool`, filled by `DB.AgentConfig`;
  test in `internal/db/agentconfig_test.go` and the projection test in
  `internal/handlers/mcp_test.go`.
- [ ] T2.2 `localSpecRepo(overrides, projectID, root, monoRepo)`; callers in
  `agent_macro_dispatch.go` pass `config.MonoRepo` (absent = mono-repo).
  Tests: override wins; mono-repo empty → root; multi-repo empty → error naming
  "Specifications folder"; missing override → error naming the folder.

## 3. Plain folders for macro operations (FR6, US5)

- [ ] T3.1 `ensureMacroWorktree`: plain folder returns the folder, empty
  branch, `worktree: false`, the warning; nothing created. Test next to the
  existing ones in `agent_macro_worktree_test.go` (Git cases unchanged).
- [ ] T3.2 Launch environment with an empty branch: `SECTILE_SPEC_BRANCH=""`,
  `SECTILE_SPEC_WORKTREE=false`; check `dispatchCommand` with an empty
  branch.
- [ ] T3.3 `realign_macro` fragments: empty-branch rule (write in `path`, no
  git, no commit, no push, report it); regenerate golden files; run
  `internal/skills` tests.

## 4. Slicing import through the local agent (FR7, US6)

- [ ] T4.1 `agentprotocol.Operation.SpecFile`; agent `macro_spec_file` routed in
  `executeOperation` before the task switch; `macroSpecFileFor` resolves the
  folder with `localSpecRepo` and calls `sddfiles.Read`. Agent test with a
  temporary folder.
- [ ] T4.2 `"macro_spec_file": 15s` in `localInspections`; extend
  `agentoperations_test.go`.
- [ ] T4.3 `DB.TodosFromSDD(ctx, userID, projectID, key, source)` reads through
  `callAgentContext`, keeps parsing and merge; delete the server-side readers.
  Tests with a fake `agentOperations` returning content, an unknown-operation
  error and a no-agent error.
- [ ] T4.4 Slicing handler passes `r.Context()` and `h.webSessionUser(r)` and
  maps the two errors to the French messages of `plan.md` decision 2; make the
  operation path wrap `ErrNoAgentConnected` if needed. Handler test for each
  message; "stories" source unaffected.

## 5. Server path removed (FR1, US1)

- [ ] T5.1 New numbered migration dropping `projects.spec_repo_path`; never the
  baseline. Adjust `migrations_test.go` and `activerun_test.go`, which drop the
  column to simulate old schemas.
- [ ] T5.2 Remove `SpecRepoPath` from `models.Project`,
  `CreateProjectRequest`, `UpdateProjectRequest` and from every project
  SELECT / scan / INSERT / UPDATE in `internal/db/db.go`; update
  `postgres_macro_test.go` and `sddslicing_test.go`. Test: an update request
  carrying `specRepoPath` still succeeds.
- [ ] T5.3 Web: remove the "Dépôt des spécifications" field, its state, payload
  key, `Project.specRepoPath` type and strings from `ProjectModal.tsx`,
  `types/index.ts`, `locales/translations.ts`. `tsc` and `oxlint` clean (see
  project memory for `node_modules` in worktrees).

## 6. Desktop settings (FR2, FR4, FR5, US1-US4)

- [ ] T6.1 Map-project handler: absolute + existing directory required; Git
  top level when inside a checkout, cleaned path otherwise; English errors.
  Tests: plain folder accepted, Git subfolder normalised, relative refused,
  missing refused, file refused.
- [ ] T6.2 Project info `GET`: `specDefault`, `specKind`
  (`git`/`folder`/`missing`/`unset`). Tests for each kind.
- [ ] T6.3 `desktop/src/main.js`: label "Specifications folder"; placeholder
  from `specDefault` or "Required for a multi-repo project"; hint per layout;
  kind shown next to the field; required flag on multi-repo when empty;
  refreshed after save. UI test in `desktop/tests/project-settings*`
  (build first: `npx vite build`, then restore `webui/.gitkeep`).

## 7. Documentation

- [ ] T7.1 `docs/contracts/server-agent-v1.md`: `macro_spec_file`, `monoRepo`
  in the config, empty `branch` in `macro_worktree`, empty
  `SECTILE_SPEC_BRANCH`.
- [ ] T7.2 ADR `0027-specifications-folder-is-a-workstation-setting.md`;
  "superseded in part" note in ADR 0026.
- [ ] T7.3 `prepare_macro_worktree` tool description in
  `internal/taskmcp/server.go`.
- [ ] T7.4 `CHANGELOG.md` `[Unreleased]`: `Changed` and `Removed` lines (FR10).

## Test plan

- `go test ./...` (PostgreSQL suite with a throwaway DSN, never dev; see
  project memory), including the migration tests on SQLite and PostgreSQL.
- `node --test` for desktop and web unit tests; desktop UI tests after a build.
- Manual, on a copy of the dev database and without tracker token:
  1. mono-repo project, no override: desktop shows the checkout as inherited,
     kind "Git repository"; web import of tasks.md works;
  2. multi-repo project, empty: desktop flags the field; web import and a
     `realign-macro` launch fail naming the setting;
  3. plain folder with `specs/M-1-x/tasks.md`: kind "Folder"; web import reads
     it; `prepare_macro_worktree` answers an empty branch with the warning;
  4. desktop app stopped: web import shows the "connect the desktop app"
     message;
  5. web project options: no "Dépôt des spécifications" field.
