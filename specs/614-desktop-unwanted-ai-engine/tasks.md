# Tasks #614 - Remove the Gemini, Cursor and Vibe AI engines

Order matters: the agent's provider set first, then the drop on load that
depends on it, then the clients and the docs. Each task names its test.

## Phase 1 - Agent provider set (US3, FR-001 to FR-003, FR-008)

- [x] T001 Add `RetiredProviders` in `internal/agentconfig` and restrict
  `ValidProvider` to `"", agy, claude, codex, custom`.
  Test: `validation_test.go` - each retired provider is refused, each
  supported one accepted.
- [x] T002 Remove the retired entries from `providerNames` (`engines.go`),
  `DefaultProviderModels` (`workstation.go`) and the model-flag providers
  (`model.go`).
  Test: `workstation_test.go` - `ProviderModels(d, "cursor")` is empty;
  `model_test.go` - `ModelArgs` of a retired provider is empty.
- [x] T003 `ValidProviderKeys` refuses a retired provider key (see plan).
  Test: `providermodels_test.go` - a `gemini` key fails with a message naming
  it; `ValidateDefaults` refuses it.
- [x] T004 Remove the retired cases from `ResolveLocations` (`locations.go`),
  `httpMCPProviders` and `mcpEntry` and the TOML `vibe` case (`mcp.go`), and
  the provider branches of `mcp_migration.go`.
  Test: `locations_test.go` - retired providers error; `mcp_test.go` and
  `mcp_connections_test.go` fixtures moved to supported providers.
- [x] T005 Remove the retired entries from `engineProviders`
  (`agent_desktop_engines.go`), the console line (`agent_console.go`), the
  autonomous and interactive command lines (`agent_config.go`) and the
  `--provider` help and errors (`init.go`).
  Test: `agent_console_test.go`, `agent_config_test.go`, `init_test.go` - a
  retired provider is refused; help lists `agy`, `claude`, `codex`.
- [x] T006 Remove the retired branches from `internal/runner/runner.go`
  (`CheckCliTools` details, autonomous run, trace launch, interactive line and
  its error message) and `specframework.go`; update `SupportsAutonomousRun`
  and the provider comments in `internal/models/models.go`.
  Test: `agentlaunch_test.go` - the interactive refusal message lists only
  supported providers; spec-framework mapping tests fall to the defaults.
- [x] T007 Desktop API: a catalogue `PUT` with a retired engine answers 400
  naming the engine; `GET|POST /desktop/mcp?provider=gemini` answers 400.
  Test: agent desktop handler tests.

## Phase 2 - Drop on load (US2, US4, FR-004 to FR-007)

- [x] T008 Write `dropRetiredProviders(*Settings) RetiredDrop` as the plan
  describes (engines, choices, default, seeded default, model lists, MCP
  connections, initialization provider).
  Tests (`engines_migration_test.go` or a new `retired_providers_test.go`),
  one per spec scenario US2.1 to US2.7:
  - Claude + Gemini engines, project P and task T on Gemini → only Claude,
    P and T choices gone, resolution gives the workstation default;
  - Cursor default then Codex → Codex is the default;
  - Vibe only → implicit Antigravity engine is the default;
  - model lists and MCP connections keep only `claude`;
  - initialization provider `vibe` → empty;
  - a #305 file with `defaults.aiProvider: gemini` and a project section with
    `aiProvider: cursor` → no retired engine, both resolve to the implicit
    engine.
- [x] T009 Call it from `readConverted` after `convertEngines`, and return the
  report through `MigrateSettings`; log it in `internal/agent/agent.go` in the
  existing log language.
  Tests: `settings_test.go` - a file with a retired engine is rewritten once,
  the backup exists and holds the original bytes; a file without one is
  neither rewritten nor backed up (US2.8, US2.9).
- [x] T010 Seeds: `ApplyWorkstationSeed` and the project seed create no
  engine for a retired provider; `NormalizeProviderModels` skips retired keys.
  Tests: `seed_test.go` (agentconfig and agent) - US4.1 to US4.3.

## Phase 3 - Desktop (US1)

- [x] T011 `desktop/src/main.js`: the initialization provider select offers
  `agy`, `claude`, `codex`; `KNOWN_COMMANDS` loses the `gemini` and `vibe`
  lines. Edit in small hunks (the file is large).
- [x] T012 `desktop/src/command-preview.mjs` and `desktop/src/engines.mjs`:
  remove the retired providers from `MODEL_FLAG_PROVIDERS`, the preview cases
  and `MARKS`.
  Tests: `engines.test.mjs`, `project-settings.test.mjs` and the command
  preview tests updated; `workstation-settings.ui.cjs` asserts the
  initialization select options and that the model lists section has no
  `gemini`, `cursor` or `vibe` row (build with `npx vite build` first, then
  restore `internal/webui/dist/.gitkeep`).

## Phase 4 - Web and shared (US5.1)

- [x] T013 `web/src/types/index.ts` `AIProvider`, `web/src/lib/commandTemplate.ts`,
  `shared/mcpConfig.mjs` (`mcpProviders`, `mcpSnippet`).
  Tests: `commandTemplate` and `mcpConfig` unit tests updated; `tsc` and
  `oxlint` clean (symlink the main checkout's `node_modules` if the worktree
  has none, and remove the link afterwards).

## Phase 5 - Docs (US5.2, US5.3)

- [x] T014 `docs/contracts/server-agent-v1.md`, `README.md`,
  `desktop/README.md`, `docs/CAPABILITIES.md`: provider lists.
- [x] T015 `CHANGELOG.md` `[Unreleased]` → `### Removed`: one line, for
  example "**Gemini CLI, Cursor CLI and Mistral Vibe CLI are no longer AI
  engines.** Sectile runs Antigravity, Claude Code, Codex or a custom command.
  On the first start after the upgrade, the agent removes a workstation's
  engines, model lists and MCP choices for them, keeping a backup of the
  settings file; projects and tasks that used them run their default engine.
  The Cursor editor is unaffected. (#614)"

## Phase 6 - Verification

- [x] T016 Fixtures: move the remaining Go and desktop tests that use a
  retired provider as a convenient second provider to `codex` or `agy`
  (`internal/agent/*_test.go`, `internal/agentconfig/*_test.go`,
  `internal/db/*_test.go`, `internal/handlers/launchmodel_test.go`,
  `internal/runner/agentlaunch_test.go`).
- [x] T017 `grep -rnE "\"(gemini|cursor|vibe)\"|'(gemini|cursor|vibe)'"` over
  `internal cmd web/src desktop/src shared` returns only `RetiredProviders`,
  the kept items of FR-010 (`checkoutMCPFiles`, `.gemini` skill directories,
  Antigravity locations, the Cursor editor) and the tests of the removal.
- [x] T018 Run `go test ./...` (outside the sandbox, `GOCACHE` under
  `$TMPDIR`), the desktop `node --test` suites and UI tests, and the web
  tests. Rerun the handlers keepalive test alone before blaming the change.
