# Plan #510 - Several AI engines, switched per task from the desktop task list

Behaviour: `spec.md`. This file says how.

## Stack

Go agent (`internal/agent`, `internal/agentconfig`), Electron desktop
(`desktop/electron`, `desktop/src`). No server, database, web or MCP change:
the server keeps receiving the same capability report and the same run engine
report.

## Current state (what the change starts from)

- `agentconfig.Settings` (`internal/agentconfig/local.go`) is the local file,
  layout 2 (`SettingsLayout`, `workstation.go`). The engine lives in
  `Execution` fields (`AIProvider`, `AICommandTemplate`,
  `AICommandTemplateAutonomous`, `AIModel`, `AISkillModels`), embedded both in
  `Defaults` and in each `ProjectSettings`.
- `agentconfig.Resolve(c, s)` (`workstation.go`) composes project section over
  defaults over provider defaults, with the template-drop rule on a provider
  change and `MergeModels` for models. It is called from about fifteen places
  (`agent.go`, `agent_run.go`, `agent_config.go`, `agent_console.go`,
  `agent_desktop.go`, `agent_desktop_settings.go`, `agent_macro_dispatch.go`,
  `agent_operations.go`, `capabilities.go`); none of them passes a task.
- `WriteSettings` (`settings.go`) replaces the owned keys (`layout`,
  `defaults`, `projectSettings`, `repositories`, `disconnectedProjects`,
  `mcpConnections`, `skills`, `seeded`) and keeps every other top-level key.
  An agent that predates a new key therefore keeps it when it saves, but drops
  any field it does not know inside an owned key.
- `ReadSettings` folds the legacy pre-#305 keys in memory; the next write
  persists the result. `UpdateSettings` / `LockSettings` serialise writers.
- `SetupProviders(config)` (`locations.go`) installs skills and MCP for the
  selected provider plus `config.SetupProviders`. `Scaffold` runs at every
  dispatch (`prepareDispatchLocked`, `agent_config.go:387`), and
  `bootstrapLocalMCP` right after it.
- `LaunchModel` / `launchEngine` / `dispatchCommand` (`agent_config.go`) build
  the command line; `launchEngine` feeds `postRunEngine`, which corrects the
  run record on the server.
- `capabilityOf` (`capabilities.go`) reports one engine per project from
  `Resolve(stub, settings)`.
- Seed (`seed.go`): `ApplyWorkstationSeed` and `ApplyProjectSeed` write
  server values into the engine fields of `Defaults` and project sections;
  `Seeded.DefaultValues` records what the defaults seed wrote.
- Desktop: `executionDefaultsPanel` (`desktop/src/main.js:995`) edits the
  workstation level including provider, model, per-skill models and templates;
  the project dialog (`main.js:~1740-1935`) edits the same engine fields per
  project through `projectSettingsInput` (`agent_desktop_settings.go`). The
  ticket table is `renderTicketsTable` / `ticketRow` (`main.js:2110-2256`,
  `TICKET_COLUMNS`). The agent's capabilities are read from `/desktop/status`
  (`main.cjs:325-371`).

## Architecture

### 1. Local file layout (`internal/agentconfig`)

A new top-level key `engines`, **outside every owned key of layout 2**, so an
agent that predates this change keeps it when it saves:

```json
{
  "layout": 3,
  "defaults": { "terminal": "...", "useWorktrees": true, "aiProviderModels": {} },
  "projectSettings": { "<projectId>": { "path": "...", "parallelism": 2 } },
  "engines": {
    "catalogue": [
      { "id": "e-1f3a9c20b7d4", "name": "Claude Opus", "provider": "claude",
        "model": "claude-opus-5", "skillModels": { "implement": "claude-sonnet-5" },
        "command": "", "commandAutonomous": "" },
      { "id": "e-7c01d2e9aa51", "name": "Codex", "provider": "codex" }
    ],
    "default": "e-1f3a9c20b7d4",
    "projects": { "<projectId>": "e-7c01d2e9aa51" },
    "tasks": { "<taskId>": "e-7c01d2e9aa51" }
  }
}
```

Types (new file `internal/agentconfig/engines.go`):

```go
type Engine struct {
    ID                string            `json:"id"`
    Name              string            `json:"name"`
    Provider          string            `json:"provider"`
    Model             string            `json:"model,omitempty"`
    SkillModels       map[string]string `json:"skillModels,omitempty"`
    Command           string            `json:"command,omitempty"`
    CommandAutonomous string            `json:"commandAutonomous,omitempty"`
}

type Engines struct {
    Catalogue []Engine          `json:"catalogue"`
    Default   string            `json:"default,omitempty"`
    Projects  map[string]string `json:"projects,omitempty"`
    Tasks     map[string]string `json:"tasks,omitempty"`
}
```

`Settings` gains `Engines Engines `json:"engines"``, and `engines` joins
`ownedKeys`. `SettingsLayout` becomes 3.

Deviation from the clarification's wording: round 2 sketched
`Defaults.Engines` and a project default engine ID in the project section.
Both live under the top-level `engines` key instead, because `defaults` and
`projectSettings` are owned keys of layout 2: an agent that predates this
change would drop those fields on its next save and lose the catalogue and the
project picks. Keeping them in a key that agent does not own makes spec US6.9
hold. The meaning is the one the clarification settled.

`Execution` keeps its engine fields, **read only**, for the conversion
(section 2) and for the pre-#305 fold. `json` tags stay so they can be read;
`WriteSettings` never emits them (the conversion empties them before any
write, and `ValidateExecution` refuses them from the desktop, section 6).

Accessors on `Settings`:

- `Engine(id string) (Engine, bool)`;
- `DefaultEngine() Engine`: `Engines.Default` if it names an entry, else the
  first entry, else the implicit engine `{ID: "", Name: "Antigravity",
  Provider: DefaultProvider}` (a file never saved by this agent);
- `ProjectEngine(projectID) Engine`: `Engines.Projects[projectID]` if it names
  an entry, else `DefaultEngine()`;
- `TaskEngine(projectID, taskID) Engine`: `Engines.Tasks[taskID]` if it names
  an entry, else `ProjectEngine(projectID)`;
- `SetTaskEngine(taskID, engineID)`, `SetProjectEngine(projectID, engineID)`
  (empty engineID deletes the entry), `ReplaceCatalogue(list, defaultID)`
  which assigns IDs to new entries (`"e-" + 12 hex` from `uuid`), drops
  project and task entries naming a removed ID (FR-9) and refuses a missing
  default.
- `prune()` called by `WriteSettings`: drops `Projects` / `Tasks` entries that
  name no catalogue entry (FR-8), and nulls an emptied map (see memory
  "New Overrides key needs a merge": an emptied map must leave the file).

`ValidateEngines(Engines)`: 1 to 20 entries (`MaxEngines`), unique non-empty
IDs, name required, at most 64 characters (`MaxEngineName`), unique by
`strings.ToLower(strings.TrimSpace(name))`, default names an entry, and each
entry passes `ValidateExecution` on the equivalent `Execution` plus the
custom-provider rule "interactive template required and containing
`{prompt}`" (today enforced in the desktop handlers, moved here). Errors name
the engine: `engine "Codex": ...`.

`overlay` (legacy repository file) leaves `Engines` untouched: the legacy
`.taskflow/agent.json` has no catalogue; its engine fields reach the catalogue
through the conversion like any other.

### 2. Conversion of the #305 engine settings (`internal/agentconfig/engines_migration.go`)

`convertEngines(s *Settings) bool` is a pure function, applied by
`ReadSettings` **in memory** after the legacy fold, whatever the layout: it
reports whether it changed anything. It runs whenever a level states any
engine field (FR-11), which covers a layout-2 file, a legacy file, and a file
an older agent wrote after the conversion.

Algorithm:

1. `workstation := profileOf(s.Defaults.Execution)` when the defaults state
   any engine field: provider (`DefaultProvider` when empty), templates as
   stated, model and per-skill models as stated.
2. For each project section stating any engine field, compute what it
   resolves today with the layout-2 rules (the current body of `Resolve`,
   kept as `resolveLegacyEngine(defaults, project Execution) Engine`): the
   provider change drops the inherited templates, `MergeModels` merges the
   models. Templates are stored as stated, **not** through
   `EffectiveCommandTemplate`, so a provider-default command stays implicit.
3. `findOrCreate(profile)`: an entry whose provider, model, per-skill models
   and both templates are equal (after trim, nil and empty maps equal) is
   reused; otherwise a new entry is appended. Identical profiles collapse
   (spec US6.5).
4. The workstation profile, when present, becomes `Engines.Default` **only if
   the catalogue had no default**; otherwise it is added (or reused) and the
   existing default is kept (the older-agent round trip of US6.9: the owner's
   catalogue choice wins over a stale field).
5. A project whose section stated engine fields gets `Engines.Projects[id]`
   set to its entry, unless it already has a pick.
6. Every engine field of `Defaults.Execution` and of each project section is
   cleared.
7. When the catalogue is still empty (no engine field anywhere, fresh
   layout-2 file), append the implicit engine for `DefaultProvider` and mark it
   default (US6.6). A file that does not exist yet stays empty in memory and
   resolves the implicit engine (`DefaultEngine()`); it gets its entry on its
   first write.

IDs of converted entries must be identical from one in-memory read to the
next, or a task choice stored between two reads would dangle: they are
**derived** as `"e-" + first 12 hex of sha256(canonical JSON of the profile)`,
with `-2`, `-3`... appended if that ID is already taken by a different
profile. Entries created from the desktop use random IDs.

Names of converted entries (default rule, see open requirements): the
provider's display name (`Claude`, `Codex`, `Antigravity`, `Gemini`, `Cursor`,
`Vibe`, `Custom`), followed by ` - <model>` when the profile sets a model,
followed by ` (<n>)` to make it unique. The owner can rename them.

Persisting: `MigrateSettings(legacyRoot string) (bool, error)` runs at agent
start (`agent.go`, before the first project sync and the first capability
report), under `settingsMu`: it reads the raw file, and when
`convertEngines` changed anything, it copies the previous file to
`settings.json.bak-layout<N>` (a timestamp suffix when that name exists), then
writes the converted settings with `WriteSettings`. Any later
`UpdateSettings` would persist the conversion anyway, since every read
applies it.

### 3. Seed (`seed.go`)

- `ApplyWorkstationSeed`: the provider, templates and models the pre-#305
  agent ran (`legacyApply` with an empty global statement, since the engine
  fields are gone) become a profile. When the catalogue is empty (fresh
  workstation), it becomes its first entry and default. When it is not
  empty, the workstation already states its engine and nothing engine-related
  is written, as #305 only wrote keys the workstation did not set. The
  terminal, editor and provider model lists keep their current treatment.
  `Seeded.DefaultEngine` (new, `omitempty`) records the ID the seed created.
- `ApplyProjectSeed`: the profile the server composed for the project
  (`legacyApply` with the workstation's own statement: the default engine's
  profile unless it is `Seeded.DefaultEngine`, then an empty statement) is
  compared with `ProjectEngine(id)`; when they differ and the project has no
  pick, `findOrCreate` and set `Engines.Projects[id]`. Worktrees, setup
  providers, terminal and skill command names keep their current treatment.
- `globalBeforeSeed` loses its engine part.

### 4. Resolution (`workstation.go`)

`Resolve(c, s)` keeps its signature and resolves the **project default
engine** (FR-6). New `ResolveTask(c, s, taskID string) Config` resolves the
**task engine** (FR-5). Both call `resolve(c, s, engine Engine)`:

```go
c.AIProvider = engine.Provider (DefaultProvider when empty)
c.AICommandTemplate = EffectiveCommandTemplate(provider, engine.Command)
c.AICommandTemplateAutonomous = EffectiveCommandTemplate(provider, engine.CommandAutonomous)
c.AIModel, c.AISkillModels = engine.Model, copy(engine.SkillModels)
c.EngineID, c.OnProjectDefaultEngine = engine.ID, engine.ID == s.ProjectEngine(c.ProjectID).ID
```

The non-engine part (worktrees, setup providers, terminal, spec artefacts,
skill commands and contents) is unchanged. `Config` gains two fields that are
never serialised (`json:"-"`): `EngineID` and `OnProjectDefaultEngine`.

Setup providers (FR-10): after the configured list is computed, the providers
of every catalogue engine that installs skills (`ResolveLocations(p)` succeeds
and `InstallsSkills()`, so `custom` is skipped) are appended to
`c.SetupProviders`, deduplicated, keeping the configured order first.
`SetupProviders(config)` needs no change. Note that the configured "none"
(empty list) is still extended by the catalogue providers: the clarification
says "on top of".

### 5. Applying the task engine at dispatch (`internal/agent`)

Every call site that resolves a configuration **for a task** switches to
`ResolveTask` with the task's full ID:

| Call site | Task ID available as |
| --- | --- |
| `prepareDispatchLocked` (`agent_config.go:359`) | `task.ID` (read just before) |
| `admitProjectRun` (`agent_run.go:243`) | `taskID` argument / `payload.TaskID` |
| `agent.go` dispatch path using `launchEngine` (`agent.go:1141`) | uses the config from `prepareDispatchLocked` |
| `desktopTasks` custom-provider check (`agent_desktop.go:539`) | the posted task ID |
| any other `Resolve` in a path that holds a task (audit with `grep -n "Resolve(" internal/agent`) | - |

Call sites without a task keep `Resolve`: capability report, project
dialog, workstation view, project sync, macro dispatch, operations without a
task, free console.

One-off model (FR-7): `LaunchModel(config, skillID, override)` ignores
`override` when `!config.OnProjectDefaultEngine`, and logs once
`[Agent] One-off model %q ignored: task %s runs on engine %q`. Because
`launchEngine` and `dispatchCommand` both go through `LaunchModel`, the command
line and the run record agree. `Resolve` sets `OnProjectDefaultEngine = true`,
so every path without a task keeps today's behaviour.

The headless refusal messages (`agent.go`, `admitProjectRun`) prefix the engine
name when the configuration came from `ResolveTask`: `engine "Codex": provider
"codex" has no headless mode: ...`.

### 6. Local desktop endpoints (`agent_desktop_settings.go`, `agent_desktop.go`)

- `GET /desktop/engines` returns `{catalogue: [Engine...], default: id,
  providers: [...supported providers], providerModels: {...}}`.
- `PUT /desktop/engines` takes `{catalogue, default}`, validates
  (`ValidateEngines`), applies `ReplaceCatalogue` under `prepareMu` and
  `UpdateSettings`, then `reportCapabilitiesLater()`. 400 with the validation
  message; 409 when the body tries to drop the current default without naming
  another.
- `GET /desktop/task-engines?projectId=<id>` returns `{projectDefault: id,
  catalogue: [{id, name, provider, model}], tasks: {taskId: engineId}}`, the
  tasks map already restricted to entries naming a catalogue engine. The
  desktop calls it when it opens or refreshes a ticket table.
- `PUT /desktop/task-engines` takes `{projectId, taskId, engineId}`; 404 when
  the engine is not in the catalogue (the desktop then reloads); stores with
  `SetTaskEngine`. The desktop computes the next engine itself (section 8), so
  the agent stays stateless about cycling. No capability report is sent: the
  report does not depend on task engines (US7.3).
- `/desktop/workstation` PUT: `Defaults` without engine fields. A body that
  still carries `aiProvider`, templates, `aiModel` or `aiSkillModels` gets 400
  "Engine settings moved to the engine catalogue; update the desktop app".
  `workstationView` drops the engine fields of `workstationEffective` and
  gains `defaultEngine: {id, name, provider, model}`.
- `/desktop/project`: `projectSettingsInput` loses the engine fields and their
  inherit flags and gains `DefaultEngine *string` / `InheritDefaultEngine
  bool` (unknown ID: 400). `executionFields` loses the five engine entries and
  gains `defaultEngine` (`source`: `project` or `workstation`). The GET keeps
  `aiProvider`, `aiModel`, `aiCommandTemplate`, `aiCommandTemplateAutonomous`
  as the resolved project default engine, because other desktop code reads
  them (the free console's initial provider, `main.js:2660`); the `*Override`
  flags for engine fields are removed.
- `/desktop/status` capabilities gain `task-engines` (FR-12).

### 7. Capability report (`capabilities.go`)

No code change beyond `Resolve` now resolving the project default engine:
`capabilityOf(Resolve(stub, settings), settings.Defaults)` describes it
(US7.1). `reportCapabilitiesLater` is already called after a workstation or
project save; add it after `PUT /desktop/engines`.

### 8. Desktop

Electron (`desktop/electron/main.cjs`, `preload.cjs`): four IPC handlers
proxying the endpoints of section 6: `engines`, `save-engines`,
`task-engines`, `set-task-engine`. `set-task-engine` and `save-engines` check
`status.capabilities.includes('task-engines')` like `create-task` does, and
throw the same "update and restart the local agent" message otherwise.

New pure module `desktop/src/engines.mjs` (unit-tested with `node --test`):

- `nextEngine(catalogue, currentId, projectDefaultId)`: the entry after the
  current one in catalogue order, wrapping; `currentId` unknown reads as
  `projectDefaultId`; a single entry returns itself.
- `taskEngine(view, taskId)`: the stored choice if it names an entry, else
  the project default.
- `engineMark(provider)`: the monogram drawn in the icon. Default set (open
  requirement, reversible): `claude` "Cl", `codex` "Cx", `agy` "Ag", `gemini`
  "Ge", `cursor` "Cu", `vibe` "Vi", `custom` "{}". No third-party logo.
- `engineTooltip(engine, isProjectDefault)`: `"<name> - <provider> ·
  <model or 'provider default'>"`, plus `" (project default)"`.

Ticket table (`main.js`): `TICKET_COLUMNS` gains `['engine','Engine']` between
`priority` and `pr`, present only when the agent announces `task-engines`.
`openTickets` loads `api.taskEngines(projectID)` beside the tasks and stores
it on `view.engines`. `ticketRow` renders a `button.engine-toggle` with the
monogram, `title` and `aria-label` = `"Engine for <key>: <tooltip>"`, and
`data-off-default` when not on the project default (CSS highlight in
`style.css`). Click: compute `nextEngine`, update optimistically, call
`api.setTaskEngine`; on failure revert to the stored value, write the error in
`view.status`, and reload `view.engines`. With a single engine the click is a
no-op (the button stays enabled so it keeps its tooltip and focus order).
The polled refresh (`updateTicketRow`) does not touch the icon.

Workstation settings (`executionDefaultsPanel`): the provider, model,
per-skill models, templates and provider presets move into an **Engines**
section: a list (name, monogram, provider, model, "Default" badge) with
Add, Edit, Move up, Move down, Make default and Remove buttons, and an engine
editor dialog reusing the existing provider select, preset buttons, model
input, per-skill model list (`keyValueList`) and template inputs. Remove is
disabled on the default engine and on the last engine; its confirmation
lists the projects and the count of tasks pointing at the engine (from
`GET /desktop/engines` plus the project list the desktop already holds and
`task-engines`). Saves go through `api.saveEngines`.

Project dialog: the provider, model, per-skill model and template rows are
replaced by one "Default engine" `select`: first option "Inherit the
workstation default (<name>)", then the catalogue by name. It saves
`defaultEngine` / `inheritDefaultEngine`. The `execution-fields.mjs` helpers
lose the engine fields.

When the agent does not announce `task-engines` (an older agent), the
desktop keeps the column hidden, shows the Engines section as "Update and
restart the local agent to manage engines", and shows no default engine
selector.

## Contract and compatibility

- **Server contract**: unchanged. Dispatch, capability report and
  `/api/activities/{id}/engine` keep their shape.
- **Older agent reading a layout-3 file**: it ignores `engines`, finds no
  engine field in `defaults` and runs the default provider with provider
  defaults. That is a downgrade regression we accept: desktop and agent ship
  together, and the upgrade is the supported direction. Its saves keep
  `engines` (unowned key). If its desktop writes engine fields again, the new
  agent converts them on its next read (US6.9).
- **Older desktop with a newer agent**: its workstation and project saves
  carrying engine fields are refused with an explicit 400 (section 6) rather
  than silently dropped.
- **Web**: the web card keeps offering one-off models from the capability
  report (the project default engine). On a switched task the one-off model is
  ignored at dispatch (FR-7); the run record shows the engine that ran. The
  web shows no per-task engine (out of scope).

## Rejected alternatives

- **Per-project extra engines** (round 1): replaced by the workstation
  catalogue at the owner's request (round 2, Q4).
- **Engines inside `defaults` / `projectSettings`**: lost on the first save of
  an agent that predates this change (owned keys are replaced whole).
- **Per-task choice in desktop browser storage**: the agent could not apply it
  to web launches, relaunches and chain stages (Q1).
- **A provider field on the server dispatch**: needs a server change and a
  store; the agent already knows the task at dispatch time (Q1).
- **Random IDs for converted entries**: the conversion runs in memory on every
  read until persisted; random IDs would change between reads.
- **Agent-side cycling (`POST .../next`)**: the desktop already holds the
  catalogue order; an explicit target makes a double click idempotent and
  keeps the endpoint a plain setter.

## Test plan

Go (`go test ./internal/agentconfig/... ./internal/agent/...`, outside the
sandbox for `httptest`, `GOCACHE` under `$TMPDIR`):

- `engines_test.go`: accessors and fallbacks (task -> project -> default ->
  implicit), `ReplaceCatalogue` ID assignment and pruning, `ValidateEngines`
  (duplicate names ignoring case, 0 and 21 entries, missing default, custom
  without `{prompt}`, invalid model with the engine named in the message),
  `WriteSettings` round trip including an emptied `tasks` map leaving the
  file, and an unowned-key round trip (an old-style write keeps `engines`).
- `engines_migration_test.go`, one test per source shape (spec US6):
  defaults only; project override with its own provider; project provider
  change that dropped the inherited templates; project with models only;
  identical profiles across projects and with the workstation; no engine field
  at all; legacy pre-#305 flat keys; layout-3 file with engine fields written
  by an older agent (default kept, entry reused); idempotence (second
  conversion changes nothing); ID stability between two reads; for every
  shape, `Resolve` after conversion equals the layout-2 `Resolve` before it
  (provider, both templates, model, per-skill models) for every project.
- `MigrateSettings` test: backup written once with the previous bytes, file
  rewritten at layout 3 without engine fields, second start writes nothing.
- `seed_test.go` updated: fresh workstation seeds a catalogue entry and
  default; a workstation with a catalogue gets no engine write; project seed
  creates or reuses an entry and a pick only when resolution differs.
- `workstation_test.go`: `ResolveTask` picks the task engine, falls back when
  the engine is removed, sets `OnProjectDefaultEngine`; setup providers
  include every catalogue provider, skip `custom`, keep the configured order.
- `agent` tests: `LaunchModel` ignores the override off the project default
  engine and applies it on it; `dispatchCommand` / `launchEngine` agree on
  provider and model for a switched task, interactive and headless; a
  dispatch through `prepareDispatchLocked` for a switched task scaffolds the
  engine's provider; the headless refusal names the engine; the new desktop
  endpoints (validation errors, 404 on unknown engine, pruning, capability
  announced); `/desktop/workstation` and `/desktop/project` refuse engine
  fields; the capability report describes the project default engine and is
  unaffected by a task switch.

Desktop:

- `desktop/tests/engines.test.mjs`: `nextEngine` (wrap, unknown current,
  single entry), `taskEngine`, `engineTooltip`, `engineMark` for every
  provider.
- `desktop/tests/project-settings.test.mjs` and
  `execution-fields.test.mjs` updated for the default engine selector.
- New UI test `desktop/tests/task-engine.ui.cjs` (build with `npx vite build`
  first): the column appears with the capability and not without it; click
  cycles and the tooltip follows; failure reverts; keyboard activation; the
  single-engine no-op. Predicates follow the `waitForFunction` null-safety
  rule.
- Workstation Engines section UI test: add, edit, reorder, make default,
  remove refused on the default.

## Target files

- `internal/agentconfig/engines.go` (new), `engines_migration.go` (new),
  `engines_test.go`, `engines_migration_test.go` (new)
- `internal/agentconfig/workstation.go`, `local.go`, `settings.go`,
  `seed.go`, `config.go`, and their tests
- `internal/agent/agent.go`, `agent_run.go`, `agent_config.go`,
  `agent_desktop.go`, `agent_desktop_settings.go`, `capabilities.go`, and
  their tests
- `desktop/electron/main.cjs`, `desktop/electron/preload.cjs`
- `desktop/src/engines.mjs` (new), `desktop/src/main.js`,
  `desktop/src/execution-fields.mjs`, `desktop/src/style.css`
- `desktop/tests/engines.test.mjs`, `desktop/tests/task-engine.ui.cjs` (new)
  and the updated settings tests
- `docs/adrs/0032-engines-are-a-workstation-catalogue.md` (proposed at this
  stage; renumber if `main` gained an ADR meanwhile)
- `docs/contracts/server-agent-v1.md`, `docs/ARCHITECTURE.md`, `CHANGELOG.md`
