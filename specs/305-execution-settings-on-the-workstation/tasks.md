# Tasks #305 - Execution settings live on the workstation

Ordered checklist. Each group is one commit (Conventional Commits). Before
group 5, fetch `origin/main` and check the next free migration number (written
as 23 and 24) and the next ADR number (written as 0029); renumber if main
landed one. Merge `origin/main` rather than rebasing the pushed spec branch
(memory *Force push refused after rebase*).

Migrations landed as 25 and 26 and the ADR as 0030 (see plan, *Implementation
notes*). T13.2 (manual check on a copy of the dev database) is left to the
reviewer.

## 1. Local file layout (FR-9, US6)

- [x] T1.1 `internal/agentconfig/workstation.go`: `Execution`, `Defaults`,
  `ProjectSettings`, `Seeded`; rename `Overrides` → `Settings` with `Layout`,
  `Defaults`, `ProjectSettings`, `Repositories`, `DisconnectedProjects`,
  `MCPConnections`, `Skills`, `Seeded` (plan §1).
- [x] T1.2 `ReadSettings`: decode the legacy keys into `legacySettings` and
  `foldLegacy` them when `layout` is absent; `.taskflow/agent.json` fallback
  through the same fold, rules unchanged.
- [x] T1.3 `WriteSettings`: delete every legacy key, write `layout: 2`, null an
  emptied `projectSettings` / `repositories` / inner map, keep connection
  fields, atomic 0600.
- [x] T1.4 Tests: every row of the fold table; legacy file read then written
  equals the new layout with the same meaning; an emptied map is removed; an
  unknown connection key survives; agent.json fallback cases of
  `settings_test.go` still pass.

## 2. Resolution on the agent (FR-1, FR-2, US1)

- [x] T2.1 `Resolve(c, s)` replacing `ApplyOverrides` (plan §2 steps 1-8),
  `ExecutionLimit` reading project then defaults; `EffectiveCommandTemplate`
  applied on the agent.
- [x] T2.2 `DefaultProviderModels` and `ProviderModels(defaults, provider)`
  (list moved from `web/src/lib/aiModels.ts`).
- [x] T2.3 Replace every `ApplyOverrides` call site and every
  `Projects[id]` / `SpecRepos[id]` / `Worktrees` / `Parallelism` /
  `Commands*` / `AIProviders` / `AIModels` / `Terminals` reader in
  `internal/agent` (list in plan *Target files*); `disconnectProject` deletes
  the project section (keeps `repositories`).
- [x] T2.4 Tests ported from `local_test.go` / `model_test.go`: provider
  switch drops the command (each template independently), model precedence
  (per-skill beats bare model at any level), one-off model, skill command
  substitution reaches the prompt (`dispatchCommand`), setup providers nil vs
  `[]`, worktrees and parallelism inheritance and clamp, **server execution
  values in `Config` are ignored**.

## 3. Open editor and terminal (US1)

- [x] T3.1 Agent `open_editor`: local editor, else `op.Editor`, else `code`.
- [x] T3.2 Remove `Dispatch.TerminalOverride` and `dispatchTerminal`;
  `resolveTerminalForProject` without `fetchConfig`; explicit `--terminal`
  first.
- [x] T3.3 Server: `HandleOpenEditor` sends no editor and ignores
  `editorCommand`; `launchTaskExternalTerminal` reads neither `user_settings`
  nor the project row.
- [x] T3.4 Tests for both orders and for the server operations sent.

## 4. Server stops composing and writing (FR-3, FR-12, US2)

- [x] T4.1 `db.AgentConfig` without execution fields, standard skill
  commands; `LegacyProjectExecution(projectID)` holding the former
  composition verbatim. Remove `applySkillCommandOverride`,
  `ProjectSkillCommand`.
- [x] T4.2 Ignore the execution fields in `CreateProject`, `UpdateProject`,
  `UpdateSettings`, `UpdateUserSettings` and any setup path writing
  `ai_provider` / `repo_path` (grep); `INSERT` writes column defaults.
- [x] T4.3 `json:"-"` on the execution fields of `models.Project`,
  `models.Settings`, `models.UserSettings`; drop them from `ProjectCreate` /
  `ProjectUpdate`.
- [x] T4.4 Tests: each write path leaves the stored value unchanged while
  applying the rest; a web save no longer resets `setup_providers`;
  `AgentConfig` golden (no execution field); `LegacyProjectExecution` equals
  the pre-change `AgentConfig` on golden projects (project override,
  inherited global command, legacy bare CLI name, per-skill models).

## 5. Schema (FR-6, FR-12, US9)

- [x] T5.1 Migration 23 `agent_capabilities` (plan §6), migration 24
  `ALTER TABLE projects DROP COLUMN tty_mode`. One statement per entry; never
  touch the baseline `CREATE TABLE` (memory *New columns only in
  migrations.go*).
- [x] T5.2 Remove `TtyMode` from models, project `SELECT`/`INSERT`/`UPDATE`,
  web types.
- [x] T5.3 Update the forget-and-replay fixtures and the rewind helpers for
  both migrations; migration tests on SQLite and PostgreSQL (throwaway DSN
  only).

## 6. Seed (FR-8, US6)

- [x] T6.1 Handler `GET /api/v1/agent/execution-seed` (plan §4), route in
  `cmd/server/main.go`, caller's `UserSettings` for terminal and editor.
- [x] T6.2 `internal/agent/seed.go`: defaults seed on first connection,
  project seed in `resolvedConfig` under `prepareMu`, only unset keys, only
  differing keys, markers, nothing on fetch error, skip disconnected projects.
- [x] T6.3 Tests: handler auth and content; agent seed cases of plan *Test
  plan*; end-to-end: a pre-upgrade project resolves the same command line,
  model, terminal, worktrees and setup providers after the seed as
  `ApplyOverrides(oldAgentConfig, legacyFile)` did before.

## 7. Capability report (FR-6, FR-7, US5)

- [x] T7.1 `db.SaveCapabilities`, `db.EngineReport` (presence and fallback
  slots, live instances only), in new `internal/db/capabilities.go`.
- [x] T7.2 Handlers `PUT /api/v1/agent/capabilities`,
  `GET /api/projects/{id}/engine`.
- [x] T7.3 Agent `capabilityOf(config)` and `reportCapabilities` after
  registration and after each desktop save.
- [x] T7.4 `ResolveTaskEngine(projectID, userID, deviceID, skillID, model)`
  from the report, unknown without; update the three callers.
- [x] T7.5 Tests: upsert/read, other user's report never served, dead
  instance ignored, shared-token device, unknown path, recorded engine at
  launch from the report, agent report content (`modelSlot` false for a
  template without `{model}`, `headless` false for a provider without mode).

## 8. `get_project_context` (FR-10, US7)

- [x] T8.1 `sessionContext` without `useWorktrees`, `aiProvider`, `aiModel`;
  tests updated.

## 9. Full-chain refusal (FR-11, US8)

- [x] T9.1 `chain_test.go`: fake agent answering `execute_skill` with the
  headless refusal → run failed, summary = agent reason + "the full chain
  stops here…", no queued step, launch activity failed with the reason.
- [x] T9.2 If the reason is lost or replaced on the way (dispatcher,
  WebSocket reply, forwarded instance), carry the agent's text through; test
  the forwarded case too (memory *Tracker writes need an actor or the
  unattended marker* for queue tests).

## 10. Desktop (FR-5, US4)

- [x] T10.1 Agent routes `GET/PUT /desktop/workstation`; `mapProject` new
  fields and validation; `desktopProject` answer with `{value, inherited,
  source}`; capability report after each save.
- [x] T10.2 Electron: `save-settings` keeps connection keys only; IPC
  `workstation-settings` / `save-workstation-settings` in `main.cjs` and
  `preload.cjs`.
- [x] T10.3 `desktop/src/main.js`: "Execution defaults" panel (every US4
  workstation field, model lists per provider, MCP choice kept); project
  dialog per-skill models, setup providers, skill command names, inherited
  source hints; "agent not running" state.
- [x] T10.4 Tests: agent route validation (Go); desktop UI tests after
  `npx vite build` (restore `webui/.gitkeep` afterwards, memory).

## 11. Web (FR-4, US3, US5)

- [x] T11.1 `ProfileModal`, `ProjectModal` (`agent` category removed,
  `prCreationStage` in `workflow`, no `repoPath` / `skillOverrides` /
  `setupProviders` / `useWorktrees`), `SyncView`, `CommandPalette`,
  `Sidebar`, `BoardColumnsEditor`.
- [x] T11.2 `useProjectEngine`, `TaskCard` and `TaskDetailModal` on the
  report; unknown badge translations (fr/en).
- [x] T11.3 Clean `lib/aiModels.ts`, delete unused `AIModelField`,
  `ProviderModelsField`, `lib/projectAgentSettings.ts`; `types/index.ts`.
- [x] T11.4 Tests (`node --test`, component tests; worktree has no
  `node_modules`, memory *Worktrees have no node_modules*): payloads carry no
  execution field, picker hidden on unknown and `modelSlot: false`, badge
  from report; `tsc` and `oxlint` clean.

## 12. Documentation

- [x] T12.1 ADR 0029 "The workstation owns the invocation" (seed, capability
  report, two-step removal, supersedes the AI sentence of ADR 0015); note in
  ADR 0015.
- [x] T12.2 `docs/contracts/server-agent-v1.md` sections listed in plan
  *Contract and compatibility*; `desktop/README.md`, `README.md` if they name
  the web as the place for these settings.
- [x] T12.3 `CHANGELOG.md` `[Unreleased]` lines from spec *Documentation
  acceptance* (with `(#305)`), including the "upgrade agent and server
  together" consequence.

## 13. Gates

- [x] T13.1 `go test ./...` (outside the sandbox for httptest, GOCACHE in
  `$TMPDIR`), PostgreSQL migration tests, web and desktop suites, `go vet`.
- [ ] T13.2 Manual check on a **copy** of the dev database and without
  tracker tokens (memories *Never run a branch server on the dev DB*,
  *Branch server writes reach the real tracker*): upgrade seeds a workstation,
  a launch runs the same command line, the web card shows the reported model,
  stopping the agent shows "Moteur inconnu".
