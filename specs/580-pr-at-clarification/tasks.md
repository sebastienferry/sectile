# Tasks #580 - Open the draft pull request at clarification

Ordered checklist. Each group is one commit (Conventional Commits) and leaves
the tree buildable. Merge `origin/main` first (it carries #561, which edited
`docs/contracts/server-agent-v1.md`): merge, do not rebase. No migration.

## 1. The rule in one place (FR1, FR11)

- [x] T1.1 `models.PRCreationStages`, `ValidPRCreationStage`,
  `PRCreationOwner` in `internal/models`. Table test: the three values map to
  `clarify`, `specify`, `implement`; empty and unknown values map to
  `implement`; `ValidPRCreationStage` refuses `""`, `new`, `reviewed`.

## 2. Server setting (FR1, US1)

- [x] T2.1 `CreateProject` and `UpdateProject` validate with
  `ValidPRCreationStage`; error lists the three values.
- [x] T2.2 Extend `internal/db/pr_policy_test.go`: `clarified` round trip
  through `UpdateProject`, `GetProjectByID` and `AgentConfig`; `new` still
  refused and leaves the stored value unchanged; create with `clarified`.

## 3. Ownership and transition check (FR5, FR6, FR9, US2, US3)

- [x] T3.1 `prCreationOwner` via `models.PRCreationOwner`;
  `stagePRRequired` covers `clarify` (owner `clarify`) and `specify` (owner
  `clarify` or `specify`).
- [x] T3.2 `"clarified": "clarify"` in the stage-to-skill maps of
  `stage.go` and `postback.go`.
- [x] T3.3 Tests (next to `adjustment_test.go` and the stage-PR tests):
  - owner `clarify`: a `clarified` transition without PR evidence is refused,
    the task stays `new`; with an open PR on the branch it is accepted and the
    PR recorded; the managed post-back path behaves the same;
  - owner `clarify`: a `specified` transition requires the PR;
  - owners `specify` and `implement`: a `clarified` transition without a PR
    is accepted (no forge lookup is made).

## 4. Dropped artefacts (FR7, US4)

- [x] T4.1 `prDeferredBySpecArtifacts` accepts `clarify` and `specify` when
  the owner is `clarify`.
- [x] T4.2 Extend `internal/db/specartifacts_test.go`: owner `clarify`,
  agent answers `drop` -> `clarified` and `specified` accepted without PR
  with `prDeferredNotice` in the note; agent answers `keep` or errors -> the
  requirement holds; `implemented` still requires the PR.

## 5. Skill instructions (FR3, FR4, FR9, OP1)

- [x] T5.1 `EffectiveProjectSkills`: validated timing; policy paragraph on
  `clarify` only for `clarified`; `clarified` wording from `plan.md`.
- [x] T5.2 Tests: with `clarified`, `clarify`, `specify`, `implement`,
  `adjust`, `pickup`, `pickup_issues` contain "final clarification round",
  "clarified transition", "never force", "Intermediate rounds open no PR" and
  the dropped-artefacts sentence; with `specified` and `implemented`, the
  `clarify` content has no "Project pull request policy" section and the other
  skills' wording is unchanged (existing assertions still pass).
- [x] T5.3 Runner and agent prompt lines (`runner.go`, `agent.go`) include
  `clarify` and the three-owner wording; update their tests if they assert on
  the text.

## 6. Agent tolerance (FR11)

- [x] T6.1 `agentconfig.Config.Validate` no longer rejects the creation
  stage. Test: a config with `clarified` and one with an unknown value both
  validate; the unknown value resolves to owner `implement`.

## 7. Web and desktop (FR2, FR8, US1, US5)

- [x] T7.1 `PRCreationStage` type alias; `ProjectModal` third option first;
  `draftAfterClarification` in both locales.
- [x] T7.2 `prRecoverySkill` test: `clarified` -> `implement`, `specified`
  -> `specify`, absent -> `implement`.
- [x] T7.3 `desktop/tests/workflow.test.mjs` (and `workflow.ui.cjs` if it
  enumerates stages): `implemented` task without `prUrl` under `clarified` ->
  `implement` / "Create PR".
- [x] T7.4 Web type check and lint (`tsc`, `oxlint`) with the main checkout's
  `node_modules` if the worktree has none; desktop tests need
  `npx vite build` first.

## 8. Documentation (FR12)

- [x] T8.1 `docs/contracts/server-agent-v1.md`: three values, `clarify`
  owner, final-round rule, deferral, tolerant validation.
- [x] T8.2 ADR 0004 amendment section for #580.
- [x] T8.3 `CHANGELOG.md` `[Unreleased]` / `Added` line (see `plan.md`).

## Test plan

- `go test ./internal/models/... ./internal/db/... ./internal/agentconfig/...
  ./internal/agent/... ./internal/runner/... ./internal/skills/...` (outside
  the sandbox for `httptest`; `GOCACHE` under `$TMPDIR`). Run the PostgreSQL
  variant of the db tests when `SECTILE_TEST_POSTGRES_DSN` names a throwaway
  database, never the dev one.
- Web: `node --test` on `web/src/lib/workflow` tests, `tsc --noEmit`,
  `oxlint`.
- Desktop: `npx vite build`, then `node --test desktop/tests/workflow.test.mjs`
  and the `workflow.ui.cjs` suite.
- Manual, on a copy of the database and without the tracker token (a branch
  server writes to the real tracker): set a test project to "Draft after
  clarification", check the option reads back in the web options and in
  `get_project_context`, and that a `clarified` transition without a PR is
  refused.
