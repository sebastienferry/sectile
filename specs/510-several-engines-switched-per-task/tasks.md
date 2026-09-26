# Tasks #510 - Several AI engines, switched per task from the desktop task list

Ordered checklist. Each group is one commit (Conventional Commits) and leaves
the tree buildable and the tests green. Merge `origin/main` first (the spec
branch is already pushed: merge, do not rebase), and check the next free ADR
number. Go tests run outside the sandbox (`httptest`), with `GOCACHE` under
`$TMPDIR`. Never start a branch server or agent on the dev settings file or
database: test the conversion on copies under `$TMPDIR` with `HOME` pointed
there.

## 1. Catalogue types and accessors (FR-1, FR-2, FR-3, FR-4, FR-8, FR-9)

- [ ] T1.1 `internal/agentconfig/engines.go`: `Engine`, `Engines`,
  `MaxEngines = 20`, `MaxEngineName = 64`; `Settings.Engines` with the
  `engines` JSON key; `engines` added to `ownedKeys`; `SettingsLayout = 3`.
- [ ] T1.2 Accessors `Engine`, `DefaultEngine` (with the implicit engine),
  `ProjectEngine`, `TaskEngine`, `SetTaskEngine`, `SetProjectEngine`,
  `ReplaceCatalogue` (ID assignment, pruning of removed IDs, missing default
  refused), and `prune()` called from `WriteSettings`.
- [ ] T1.3 `ValidateEngines`, moving the custom-provider `{prompt}` rule from
  the desktop handlers into it; messages name the engine.
- [ ] T1.4 `engines_test.go`: every accessor fallback, pruning, validation
  cases, `WriteSettings` round trip, emptied `tasks` map leaves the file, an
  older-agent-style write (owned keys replaced) keeps `engines`.

## 2. Conversion of the #305 engine settings (FR-11, US6)

- [ ] T2.1 `engines_migration.go`: `resolveLegacyEngine` (the current body of
  `Resolve`'s engine part, kept verbatim for the conversion),
  `findOrCreate` with profile equality, derived IDs (`sha256` prefix with
  collision suffix), default names, `convertEngines(*Settings) bool`
  following plan section 2, steps 1 to 7.
- [ ] T2.2 `ReadSettings` applies `convertEngines` after the legacy fold,
  whatever the layout.
- [ ] T2.3 `MigrateSettings(legacyRoot)`: under `settingsMu`, backup
  `settings.json.bak-layout<N>` (timestamp suffix when it exists) before
  the first change, then `WriteSettings`. Called once at agent start in
  `agent.go`, before the first project sync and capability report; a failure
  is logged and the agent keeps running on the in-memory conversion.
- [ ] T2.4 `engines_migration_test.go`: one test per source shape listed in
  plan "Test plan", each asserting that `Resolve` per project is identical
  before and after; idempotence; ID stability across two reads; the backup
  holds the previous bytes and is written once.

## 3. Resolution on the catalogue (FR-5, FR-6, FR-10)

- [ ] T3.1 `Resolve` resolves the project default engine through a shared
  `resolve(c, s, Engine)`; new `ResolveTask(c, s, taskID)`. `Config` gains
  `EngineID` and `OnProjectDefaultEngine` (`json:"-"`).
- [ ] T3.2 Setup providers extended with every catalogue provider that
  installs skills, deduplicated, configured order first.
- [ ] T3.3 `ValidateExecution` / `ValidateDefaults` / `ValidateProject` refuse
  engine fields (they are read-only now); `workstation_test.go`,
  `settings_test.go`, `local_test.go` updated: engine fields in fixtures move
  to catalogue entries; new tests for `ResolveTask` and setup providers.

## 4. Seed on the catalogue (US6.10)

- [ ] T4.1 `ApplyWorkstationSeed` writes a catalogue entry and default on an
  empty catalogue only, records `Seeded.DefaultEngine`; `ApplyProjectSeed`
  creates or reuses an entry and sets the project pick only when the
  resolution differs and the project has no pick; `globalBeforeSeed` loses
  its engine part.
- [ ] T4.2 `seed_test.go` updated for both seeds, including the "catalogue
  already stated" case.

## 5. Task engine at dispatch (FR-5, FR-7, US4)

- [ ] T5.1 Switch every task-holding `Resolve` call to `ResolveTask`
  (`prepareDispatchLocked`, `admitProjectRun`, the `desktopTasks` custom
  check, and any other found by `grep -n "Resolve(" internal/agent`); list the
  audited call sites in the commit body.
- [ ] T5.2 `LaunchModel` ignores the one-off model off the project default
  engine, with one log line; headless refusal messages name the engine.
- [ ] T5.3 Tests: override ignored and applied; `dispatchCommand` and
  `launchEngine` agree for a switched task (interactive and headless);
  queued runs of two tasks of one project use their own engines; a switched
  task's dispatch scaffolds its provider; discussion opens the task engine's
  provider.

## 6. Local desktop endpoints (FR-12, FR-13)

- [ ] T6.1 `GET`/`PUT /desktop/engines`, `GET`/`PUT /desktop/task-engines`
  as in plan section 6; `reportCapabilitiesLater` after a catalogue save.
- [ ] T6.2 `/desktop/workstation` and `/desktop/project`: engine fields
  refused with the explicit 400; `projectSettingsInput` gains
  `defaultEngine` / `inheritDefaultEngine`; `executionFields` and
  `workstationView` updated; the resolved `aiProvider`/`aiModel`/templates stay
  in the project GET.
- [ ] T6.3 `task-engines` in the `/desktop/status` capabilities.
- [ ] T6.4 Handler tests for each endpoint (validation, 404 unknown engine,
  pruning after a removal, capability announced) and for the capability
  report describing the project default engine and ignoring task switches.

## 7. Desktop: catalogue and project default (US1, US2)

- [ ] T7.1 IPC handlers and preload entries `engines`, `saveEngines`,
  `taskEngines`, `setTaskEngine`, gated on the `task-engines` capability.
- [ ] T7.2 `desktop/src/engines.mjs` with `nextEngine`, `taskEngine`,
  `engineMark`, `engineTooltip`, and `desktop/tests/engines.test.mjs`.
- [ ] T7.3 Workstation settings: Engines section and engine editor dialog
  (moving the provider, presets, model, per-skill models and templates into
  it); Remove disabled on the default and the last engine, confirmation
  listing the projects and task count affected. Edit `main.js` directly, in
  small hunks.
- [ ] T7.4 Project dialog: "Default engine" selector replacing the engine
  rows; `execution-fields.mjs` and its tests updated;
  `project-settings.test.mjs` updated.

## 8. Desktop: ticket table toggle (US3)

- [ ] T8.1 `engine` column in `TICKET_COLUMNS` when the capability is
  announced; `openTickets` loads the task engines; `ticketRow` renders the
  `engine-toggle` button (monogram, tooltip, accessible name, off-default
  highlight in `style.css`); click cycles with optimistic update and revert
  on failure; single engine is a no-op; `updateTicketRow` leaves it alone.
- [ ] T8.2 `desktop/tests/task-engine.ui.cjs` (after `npx vite build`, and
  restore `internal/agent/webui/.gitkeep` afterwards) plus the workstation
  Engines UI test.

## 9. Documentation

- [ ] T9.1 ADR `docs/adrs/0032-engines-are-a-workstation-catalogue.md`
  moved from Proposed to Accepted, with any deviation found while
  implementing.
- [ ] T9.2 `docs/contracts/server-agent-v1.md`: layout 3 of
  `~/.config/sectile/settings.json` and the four local endpoints;
  `docs/ARCHITECTURE.md` where it describes the local settings file.
- [ ] T9.3 `CHANGELOG.md` under `[Unreleased]`: one `Added` line (engine
  catalogue on the workstation, per-task switch from the desktop ticket
  table, every launch of the task uses it) and one `Changed` line (the
  project settings pick a default engine; existing settings are converted
  automatically). Reference `(#510)`.
- [ ] T9.4 `desktop/README.md` if it describes the engine fields of the
  project or workstation settings.

## 10. Gates

- [ ] T10.1 `go build ./...`, `go vet ./...`, `go test ./internal/agentconfig/...
  ./internal/agent/...` (rerun the keepalive test alone if it flakes under
  load).
- [ ] T10.2 Desktop: `node --test desktop/tests/*.test.*`, the UI tests
  touched, `npx vite build`; the web is untouched, no web gate needed beyond
  `git diff --stat origin/main -- web` being empty.
- [ ] T10.3 Manual check on a copy of a real layout-2 settings file (with
  `HOME` in `$TMPDIR`): start the agent, check the backup, the catalogue and
  that the capability report is unchanged; switch a task in the desktop and
  launch it from the web, check the run's provider and model.
