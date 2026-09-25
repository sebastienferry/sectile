# Tasks #487 - Drop the specification artefacts

Ordered checklist. Each group is one commit (Conventional Commits) and leaves
the tree buildable. Merge `origin/main` first (the spec branch is already
pushed: merge, do not rebase) and check the next free migration number.

## 1. Server setting (FR1, US1)

- [ ] T1.1 `models.NormalizeSpecArtifacts`; `SpecArtifacts` on
  `models.Project`, the create request and (`*string`) the update request.
  Unit test for the normaliser.
- [ ] T1.2 New numbered migration adding `projects.spec_artifacts TEXT NOT
  NULL DEFAULT 'keep'`; column added to the `projects` SELECT, INSERT and
  UPDATE lists of `internal/db/db.go`; baseline untouched. Rewind helpers of
  the db tests drop the column.
- [ ] T1.3 Create/update validation in `db.go` ("specArtifacts must be keep or
  drop"); omitted on update keeps the stored value. Tests: default `keep` on
  create and on an upgraded row, `drop` round trip, invalid value refused,
  omitted field unchanged. Run the PostgreSQL variant when a DSN is available
  (never the dev database).
- [ ] T1.4 `agentconfig.Config.SpecArtifacts` filled by `DB.AgentConfig`;
  extend `internal/db/agentconfig_test.go` and the config projection test in
  `internal/handlers/mcp_test.go` if it lists fields.

## 2. Workstation override (FR2, FR3, US3)

- [ ] T2.1 `Overrides.SpecArtifacts`; `ApplyOverrides` applies it; merge in
  `localProjectRoot`; `WriteSettings` nulls an emptied map. Tests in
  `internal/agentconfig`: override wins, absent follows the server, clearing
  the last entry leaves no key (mirror
  `TestSettingsSpecReposClearTheLastEntry`).
- [ ] T2.2 Desktop save input `specArtifacts` / `inheritSpecArtifacts` in
  `agent_desktop.go`, validated (`keep`/`drop`, else 400); GET returns
  `specArtifacts`, `specArtifactsOverride`. Handler tests next to the
  worktree override ones.

## 3. Exclude block (FR4, FR5, US2, US4)

- [ ] T3.1 `internal/agent/specexclude.go`: `excludeFilePath` (shared with
  `excludeTaskWorktrees`), `artefactPatterns`, `ensureSpecExclusions`,
  `removeSpecExclusions`, atomic write.
- [ ] T3.2 Tests on a temporary repository with a linked worktree: block
  created in the common dir; idempotent (no rewrite when unchanged); second
  task appended and sorted; user lines above and below preserved byte for
  byte on add and remove; two projects' blocks independent; unsafe key
  (`a b`, `x*`, newline) yields no pattern; upper-case key adds the
  lower-case form; `git check-ignore` confirms the paths of the task are
  ignored and `specs/488-x/` is not; a tracked file under an excluded path
  stays tracked.
- [ ] T3.3 Wire into `prepareDispatchLocked` (drop -> ensure, keep -> remove,
  error returned) and into the desktop save (keep -> remove on the code
  checkout and the mapped repositories, error logged). Tests: a dispatch
  with `drop` writes the block; switching to `keep` removes it; a
  multi-repo task writes it in the primary repository only.

## 4. Skills and launch notice (FR6, FR7, FR8)

- [ ] T4.1 Update the clarify, specify (three variants), implement and
  adjust fragments per `plan.md` decision 4; define `<n>` as the key without
  `#`.
- [ ] T4.2 Launch notice in `internal/agent/agent.go` for the six skills
  when `config.SpecArtifacts == "drop"`. Test on the built prompt for
  `drop`, `keep`, and a skill outside the list.
- [ ] T4.3 `UPDATE_GOLDEN=1 go test ./internal/skills/`, review the golden
  diff, then run `go test ./internal/skills/` without the variable. Add a
  catalog test asserting the clarify, specify, implement and pickup bodies
  mention `git check-ignore`.

## 5. Pull request at specified (FR9, US6)

- [ ] T5.1 Agent operation `spec_artifacts` in `executeOperation`, returning
  the effective value; `localInspections` entry. Agent test for both values.
- [ ] T5.2 `validateStagePR`: specify + owner specify + no `prUrl` -> ask the
  agent; `drop` accepts with the notice; `keep`, unknown operation and errors
  fall back to today's lookup. Tests with a fake agent for the four answers,
  plus: a `prUrl` given is still validated; implement still requires its PR.
- [ ] T5.3 The `specified` policy sentence in `projectskills.go`; update the
  policy test in `internal/db/pr_policy_test.go`.

## 6. Tracked warning (FR10, US5)

- [ ] T6.1 `specArtifactsTracked` in the desktop project GET, counted from
  `git ls-files -- specs openspec/changes docs/clarifications`; 0 on error or
  invalid mapping. Test on a temporary repository with two tracked specs.

## 7. Web and desktop UI (US1, US3, US5)

- [ ] T7.1 `web/src/types/index.ts` and `ProjectModal.tsx`: checkbox and help
  text per `plan.md` decision 8, loaded from and saved to `specArtifacts`.
  Extend the ProjectModal test if one covers the execution defaults panel;
  run the web type check and linter (symlink the main checkout's
  `node_modules` when the worktree has none, then remove the link).
- [ ] T7.2 `desktop/src/main.js`: "Specifications" row per decision 9, reset,
  hint, warning when effective `drop` and `specArtifactsTracked > 0`, save
  payload. Build with `npx vite build`, restore `webui/.gitkeep` if the build
  removed it, then add a `.ui.cjs` case: row present, hint "Inherited · Server
  default: Keep", choosing Drop sends `specArtifacts: "drop"`, reset sends
  `inheritSpecArtifacts: true`, warning shown with a tracked count.

## 8. Documentation and closing checks (FR12)

- [ ] T8.1 `CHANGELOG.md`, `[Unreleased]` / `Added`: "Projects can keep
  clarifications and specifications out of their repository: they stay in
  the task worktree and are never committed, with a per-workstation override
  in the desktop settings (#487)."
- [ ] T8.2 Mention the option in the relevant `docs/` page on project
  settings if one describes `useWorktrees`; otherwise none.
- [ ] T8.3 Full run: `go build ./...`, `go vet ./...`, `go test ./...`
  (outside the sandbox for httptest), web and desktop test suites. Quote the
  output in the implemented report.

## Test plan summary

| Requirement | Covered by |
| --- | --- |
| FR1 server setting | T1.3, T1.4 |
| FR2/FR3 override, effective value | T2.1, T2.2 |
| FR4/FR5 block written and removed | T3.2, T3.3 |
| FR6 skills decide from Git | T4.3 |
| FR7 launch notice | T4.2 |
| FR8 durable trace | T4.1 (fragment text), T4.3 (golden) |
| FR9 PR at specified | T5.2, T5.3 |
| FR10 tracked warning | T6.1, T7.2 |
| FR11 lifetime | unchanged behaviour of `git worktree remove`; no test |
| FR12 changelog | T8.1 |
