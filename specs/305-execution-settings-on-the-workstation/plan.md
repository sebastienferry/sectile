# Plan #305 - Execution settings live on the workstation

Behaviour: `spec.md`. This file says how.

## Stack

Go server (`internal/db`, `internal/handlers`, `internal/taskmcp`,
`cmd/server`), Go agent (`internal/agent`, `internal/agentconfig`), SQLite and
PostgreSQL through the numbered migrations (ADR 0021), React web client
(`web/src`), Electron desktop (`desktop/electron`, `desktop/src`).

## Current state (what the change starts from)

- `db.AgentConfig` (`internal/db/agentconfig.go`) composes provider, templates,
  model, per-skill models, terminal, `useWorktrees` and `setupProviders` from
  the project row over the deployment `settings` row, and substitutes
  `Project.SkillOverrides[id]` into `Skill.Command`.
- The agent overlays `agentconfig.Overrides` with `ApplyOverrides`
  (`internal/agentconfig/local.go`) on top of that server composition. The
  local file mixes global scalars (`aiProvider`, `terminal`, ...) and
  per-project maps (`projects`, `worktrees`, `parallelism`, `commands`,
  `commandsAutonomous`, `aiProviders`, `aiModels`, `terminals`, `specRepos`).
- Two writers of the local file: `agentconfig.WriteSettings` (agent) and the
  Electron `save-settings` IPC (`desktop/electron/main.cjs:76`), a shallow
  merge used by the desktop "AI Engine CLI" panel (`desktop/src/main.js:998`).
- `ResolveTaskEngine` (`internal/db/db.go:5279`) guesses the run's engine from
  server columns; the agent corrects it later through
  `POST /api/activities/{id}/engine` (`postRunEngine`).
- `HandleOpenEditor` (`handlers.go:3700`) resolves the editor from the
  caller's `user_settings`; `launchTaskExternalTerminal` (`handlers.go:3770`)
  fills `Dispatch.TerminalOverride`, which the agent never reads.
- `sessionContext` (`internal/taskmcp/server.go:142`) serves `useWorktrees`,
  `aiProvider`, `aiModel`.
- The headless refusal is on the agent (`admitProjectRun`,
  `agent_run.go:228`; `agent.go:998`). A queued chain step whose
  `execute_skill` call fails ends through `FinishRemoteRun(failed, err)`
  (`db.go:4322`) and `handBackRun` (`chain.go:83`) notes the stop.
- Web editors: `ProfileModal` tab `aiEngine`, `ProjectModal` categories
  `agent` and `skills` (command names), `ProjectModal`/`SyncView` `repoPath`,
  `TaskCard` picker and badge (`lib/aiModels.ts`), `TaskDetailModal`
  provider, `CommandPalette` spec install.

## Architecture

### 1. Local file layout (`internal/agentconfig`)

`Overrides` is renamed `Settings` and becomes the configuration. New layout,
written with `"layout": 2`:

```json
{
  "server": "...", "deviceId": "...", "apiKey": "...",
  "layout": 2,
  "defaults": {
    "aiProvider": "claude",
    "aiCommandTemplate": "", "aiCommandTemplateAutonomous": "",
    "aiModel": "claude-opus-5", "aiSkillModels": {"implement": "claude-sonnet-5"},
    "aiProviderModels": {"claude": ["claude-opus-5", "claude-sonnet-5"]},
    "terminal": "ghostty", "editorCommand": "cursor",
    "useWorktrees": true, "parallelism": 2, "setupProviders": ["codex"]
  },
  "projectSettings": {
    "<projectId>": {
      "path": "/abs/checkout", "specPath": "/abs/specs",
      "aiProvider": "", "aiCommandTemplate": "", "aiCommandTemplateAutonomous": "",
      "aiModel": "", "aiSkillModels": {},
      "terminal": "", "useWorktrees": null, "parallelism": 0,
      "setupProviders": null, "skillCommands": {"implement": "code-issue"}
    }
  },
  "repositories": {"github.com/owner/other": "/abs/other"},
  "disconnectedProjects": {}, "mcpConnections": {}, "skills": {},
  "seeded": {"defaults": "https://sectile.example", "projects": {"<projectId>": "2026-09-25T00:00:00Z"}}
}
```

Types (new file `internal/agentconfig/workstation.go`):

```go
type Execution struct {
    AIProvider                  string            `json:"aiProvider,omitempty"`
    AICommandTemplate           string            `json:"aiCommandTemplate,omitempty"`
    AICommandTemplateAutonomous string            `json:"aiCommandTemplateAutonomous,omitempty"`
    AIModel                     string            `json:"aiModel,omitempty"`
    AISkillModels               map[string]string `json:"aiSkillModels,omitempty"`
    Terminal                    string            `json:"terminal,omitempty"`
    UseWorktrees                *bool             `json:"useWorktrees,omitempty"`   // nil inherits
    Parallelism                 int               `json:"parallelism,omitempty"`    // 0 inherits
    SetupProviders              []string          `json:"setupProviders,omitempty"` // nil inherits; [] is "none"
}
type Defaults struct {
    Execution
    AIProviderModels map[string][]string `json:"aiProviderModels,omitempty"` // key present = choice, even empty
    EditorCommand    string              `json:"editorCommand,omitempty"`
}
type ProjectSettings struct {
    Path          string            `json:"path,omitempty"`
    SpecPath      string            `json:"specPath,omitempty"`
    Execution
    SkillCommands map[string]string `json:"skillCommands,omitempty"`
}
type Seeded struct {
    Defaults string            `json:"defaults,omitempty"` // server URL the defaults were seeded from
    Projects map[string]string `json:"projects,omitempty"` // projectId -> RFC 3339 time
}
```

`Settings` holds `Layout int`, `Defaults`, `ProjectSettings
map[string]ProjectSettings`, `Repositories`, `DisconnectedProjects`,
`MCPConnections`, `Skills` (content overrides, unchanged) and `Seeded`.

Keys stay distinct from the legacy ones (`projects` was a
`map[string]string`), so an older binary reading a new file hits no decode
error; it simply sees no mapping, which ADR 0006 already accepts.

**Legacy read** (`ReadSettings`): the file is decoded twice, into `Settings`
and into a private `legacySettings` struct holding the old keys. When
`layout` is absent, `foldLegacy` maps them:

| Legacy key | New place |
| --- | --- |
| `aiProvider`, `aiCommandTemplate`, `aiCommandTemplateAutonomous`, `aiModel`, `aiSkillModels`, `terminal` | `defaults.*` |
| `projects[id]` | `projectSettings[id].path` |
| `specRepos[id]` | `projectSettings[id].specPath` |
| `worktrees[id]`, `parallelism[id]`, `terminals[id]` | `projectSettings[id].useWorktrees`, `.parallelism`, `.terminal` |
| `aiProviders[id]`, `aiModels[id]` | `projectSettings[id].aiProvider`, `.aiModel` |
| `commands[id]`, `commandsAutonomous[id]` | `projectSettings[id].aiCommandTemplate`, `.aiCommandTemplateAutonomous` |

The `.taskflow/agent.json` fallback (`ReadOverrides`, renamed
`readLegacyRepositoryFile`) keeps its current rules (whole file when
`settings.json` is missing, then `projects`/`worktrees`/`parallelism` when the
local file lacks them) and goes through the same `foldLegacy`. Nothing is
written to it.

**Write** (`WriteSettings`): keeps the connection fields it does not own,
deletes every legacy key listed above, writes `layout: 2`, and keeps the
null-for-an-emptied-map rule (memory: *New Overrides key needs a merge*) for
`projectSettings`, `repositories` and every map inside `defaults` and each
project section. Atomic replace, mode 0600, as today.

### 2. Resolution moves to the agent

`ApplyOverrides(c Config, o Overrides) Config` becomes
`Resolve(c Config, s Settings) Config`:

1. It first **clears** every execution field of `c` (`AIProvider`,
   templates, `AIModel`, `AISkillModels`, `ExternalTerminalCommand`,
   `UseWorktrees`, `SetupProviders`), so a value an older server still sends
   is never used.
2. Provider and templates: workstation defaults, then the project section,
   with today's rules (provider change without its own command drops the
   inherited command, for both templates independently as today), then
   `EffectiveCommandTemplate(provider, template)` (moved from the server).
3. Models: `MergeModels(project, defaults)`; the one-off launch model is still
   applied later by `launchEngine`/`dispatchCommand`.
4. `UseWorktrees`: project, else defaults, else `true`.
5. `SetupProviders`: project list when non-nil, else defaults, else none;
   normalised with `models.NormalizeSetupProviders`.
6. Terminal: project, else defaults (the `--terminal` flag and detection stay
   in `resolveTerminalForProject`, which stops calling `fetchConfig`).
7. Skill commands: for each `c.Skills[i]`, `projectSettings[id].skillCommands[skill.ID]`
   replaces `Skill.Command` when non-empty.
8. Skill content overrides (`skills`) and the `adjust` reconciliation: unchanged.

`ExecutionLimit(projectID, useWorktrees, s)` reads project parallelism, else
defaults, else 1, clamped to `MaxParallelism`. `localProjectRoot` reads
`projectSettings[id].path` instead of `Projects[id]`; `SpecRepos` readers move
to `specPath`.

`Config` keeps its execution fields as the agent's *resolved* view (every
agent function keeps reading `config.AIProvider`), with a comment saying the
server no longer fills them. JSON tags stay for `.taskflow/remote-config.json`.

A new `agentconfig.DefaultProviderModels` holds the list Sectile ships (moved
from `web/src/lib/aiModels.ts:DEFAULT_PROVIDER_MODELS`), with
`ProviderModels(d Defaults, provider)`: configured key, even empty, else
shipped list.

### 3. Server stops composing and writing

- `db.AgentConfig` serves identity, tracker metadata, `specFramework`,
  `prCreationStage`, `defaultSkillMode`, `fullChainStopStage`, `monoRepo`,
  `repositories` and skills with the **stage's standard command**. It fills no
  execution field. The `adjust` reconciliation keeps reading
  `p.SkillOverrides["review"]` (read-only column, removed by #492).
- Dead code removed: `applySkillCommandOverride`, `ProjectSkillCommand` and
  their tests.
- Writes stopped, the fields being ignored (not refused, so an older web
  client keeps saving):
  - `CreateProject` / `UpdateProject` (`db.go:6461`, `6702`): `RepoPath`,
    `RepoPaths`, `UseWorktrees`, `SkillOverrides`, `SetupProviders`,
    `AIProvider`, templates, `AIModel`, `AISkillModels`,
    `ExternalTerminalCommand`, `TtyMode`. The `INSERT` writes the column
    defaults; the `UPDATE` stops setting those columns.
  - `UpdateSettings` (`db.go:3962-4073`): `ai_*`, `ai_provider_models`,
    `repo_path`, `editor_command`, `external_terminal_command` keep their
    stored value (the upsert copies `current` for them).
  - `UpdateUserSettings` (`usersettings.go:109`): `editor_command`,
    `external_terminal_command` keep their stored value.
  - Setup wizard and any `/api/setup/*` path that writes `ai_provider` or
    `repo_path` (grep `ai_provider =`, `repo_path =` when implementing).
- Answers: `models.Project`, `models.Settings`, `models.UserSettings` stop
  serialising the execution fields (`json:"-"`) while the Go fields stay for
  the seed reader. `ProjectCreate`/`ProjectUpdate` drop them from their
  structs (unknown JSON fields are ignored by the decoder).
- Readers of `repoPath` in `internal/db/agentoperations.go:48` (project alias)
  and `LegacyRepoPaths` keep working on the read-only column.

### 4. Seed endpoint

`GET /api/v1/agent/execution-seed[?projectId=<id>]`, agent bearer
(`AgentAPIAuth`), new handler in `internal/handlers/agent_api.go`:

```json
{
  "schemaVersion": 1,
  "defaults": {
    "aiProvider": "claude", "aiCommandTemplate": "...", "aiCommandTemplateAutonomous": "...",
    "aiModel": "...", "aiSkillModels": {}, "aiProviderModels": {},
    "terminal": "...", "editorCommand": "..."
  },
  "project": {
    "projectId": "...",
    "aiProvider": "...", "aiCommandTemplate": "...", "aiCommandTemplateAutonomous": "...",
    "aiModel": "...", "aiSkillModels": {}, "terminal": "...",
    "useWorktrees": true, "setupProviders": [], "skillCommands": {}
  }
}
```

- `defaults`: deployment `settings` row, with `terminal` and `editorCommand`
  from `UserSettings(credential.UserID)` (which already falls back to the
  deployment). `editorCommand` `code` (the column default) is sent as empty.
- `project`: `db.LegacyProjectExecution(projectID)`, the body `AgentConfig`
  runs today moved verbatim into a function (project over deployment,
  `EffectiveCommandTemplate`, `MergeModels`, `SkillOverrides` as command
  names), so the seed reproduces exactly what the agent used to receive.
- Empty values are omitted. Server-local columns only; nothing is written.

### 5. Seed on the agent (`internal/agent/seed.go`, new)

- **Defaults**: after the first successful identity check of a connection
  (`agent.go` connect path), when `seeded.defaults` is empty: fetch the seed,
  write each `defaults` key the local file does not set, set
  `seeded.defaults` to the server URL, one `WriteSettings`. On a fetch error:
  log, write nothing, mark nothing.
- **Project**: in one resolving function called by every `fetchConfig`
  consumer that is followed by `Resolve` (`prepareDispatchLocked`,
  `admitProjectRun`, `syncLocalProject`, `desktopProject`, operations,
  macro dispatch, consoles) - call it `resolvedConfig(ctx, projectID,
  taskKey)` - when `seeded.projects[id]` is absent: fetch the seed for the
  project, compute `Resolve(config, settings)` without and with each seed
  value, and write into the project section only the keys whose seed value
  differs from the resolution without it and that the section does not set.
  Then set `seeded.projects[id]`. Held under `prepareMu`, as the other writes.
- `useWorktrees`: the seed writes it only when it differs from the resolved
  default (`true` unless the workstation defaults say otherwise).
- A disconnected project is not seeded.

### 6. Capability report

Agent → server, `PUT /api/v1/agent/capabilities`, agent bearer:

```json
{
  "schemaVersion": 1,
  "projects": [{
    "projectId": "...", "provider": "claude",
    "model": "claude-opus-5", "skillModels": {"implement": "claude-sonnet-5"},
    "models": ["claude-opus-5", "claude-sonnet-5"],
    "modelSlot": true, "headless": true
  }]
}
```

- Computed by `capabilityOf(config)` after `Resolve`: `model` and every
  `skillModels` entry through `EffectiveModel(provider, template, ...)` for
  the stage skills (`skills.StageSkills`), `models` from `ProviderModels`,
  `modelSlot` = `EffectiveModel(provider, template, "x") != ""`, `headless` =
  `models.SupportsAutonomousRun(...)`.
- Sent for every project the agent serves (mapped, not disconnected): after
  the WebSocket registration of a connection and after each successful
  `WriteSettings` from a desktop save (`mapProject`, workstation settings,
  disconnect, repositories). Fire-and-forget with a log on failure, like
  `postRunEngine`. Includes the one-off seed write.
- Server: `db.SaveCapabilities(userID, deviceID, reports)` upserts one row per
  project; `deviceID` is `credential.Device.ID`, empty for the shared token.

Storage, migration **23** (`internal/db/migrations.go`, renumber if main
landed one; see tasks), one statement per entry:

```sql
CREATE TABLE agent_capabilities (
    user_id      TEXT NOT NULL,
    device_id    TEXT NOT NULL DEFAULT '',
    project_id   TEXT NOT NULL,
    provider     TEXT NOT NULL DEFAULT '',
    model        TEXT NOT NULL DEFAULT '',
    skill_models TEXT NOT NULL DEFAULT '{}',
    models       TEXT NOT NULL DEFAULT '[]',
    model_slot   INTEGER NOT NULL DEFAULT 0,
    headless     INTEGER NOT NULL DEFAULT 0,
    reported_at  DATETIME NOT NULL,
    PRIMARY KEY (user_id, device_id, project_id)
);
```

Persisted, not in memory: the web request and the agent's WebSocket may sit
on different replicas, which is why presence is in the database too
(`internal/db/presence.go`).

Reading: `db.EngineReport(userID, projectID, deviceID)`; with an empty
`deviceID`, the device comes from `agent_presence` for `(userID, projectID)`
then the fallback slots (`agentProjectFallbacks`), live instances only (same
rule as `agentHolder`). No live holder: no report.

Web read, `GET /api/projects/{id}/engine` (session auth, the caller's own):

```json
{"state": "reported", "provider": "claude", "model": "...", "skillModels": {},
 "models": [], "modelSlot": true, "headless": true, "reportedAt": "..."}
```

or `{"state": "unknown"}`.

`ResolveTaskEngine(projectID, userID, deviceID, skillID, modelOverride)`:
reads the report; the skill's model is `skillModels[skill]` else `model`; a
one-off override wins when `modelSlot`; no report → `("", "")`. Callers:
`handlers.go:2236` passes the session user and `ac.DeviceID`;
`macroruns.go:72` likewise; `db.go:4319` passes `job.ActingUser` and an empty
device. The `"agy"` fallback in the recorded engine goes: unknown is empty and
the agent's `postRunEngine` fills it.

### 7. Open editor and terminal

- `HandleOpenEditor` sends `Operation{Action: "open_editor"}` with no
  `Editor`, and drops the `editorCommand` request field.
- Agent `open_editor` (`agent_operations.go:390`): `settings.Defaults.EditorCommand`,
  else `op.Editor` (an older server), else `code`.
- `launchTaskExternalTerminal` stops reading `user_settings` and the project
  row; `Dispatch.TerminalOverride` is removed from `agentconfig.Dispatch`
  together with `dispatchTerminal` (no non-test caller).
  `resolveTerminalForProject` keeps: explicit call value, project section,
  defaults, `--terminal`/daemon flag, detection; the explicit `--terminal`
  (`d.terminal.explicit`) outranks all, as `dispatchTerminal` did.

### 8. `get_project_context`

`sessionContext` drops `useWorktrees`, `aiProvider`, `aiModel`. The skill
references carry the standard command, which is what `AgentConfig` now
serves. `internal/taskmcp` tests updated.

### 9. Full-chain refusal

No new mechanism. The existing path is locked by tests and its message
checked:

- `db.go:4319-4323`: the `execute_skill` error of the agent is the refusal
  text of `admitProjectRun`; `FinishRemoteRun(failed, err.Error())` puts it in
  the run summary; `handBackRun` appends "the full chain stops here: the run
  ended as failed"; nothing is enqueued.
- Verify that the agent operation error reaches `err.Error()` without being
  replaced by a transport message (`agentOperations` → dispatcher →
  WebSocket reply). If it is wrapped, unwrap to the agent's text there.
- The launch activity (`db.go:4330`) ends failed with the same text.

### 10. `ttyMode`

Migration **24**: `ALTER TABLE projects DROP COLUMN tty_mode;`. Remove
`TtyMode` from `models.Project`, `ProjectCreate`, `ProjectUpdate`, the project
`SELECT`/`INSERT`/`UPDATE` in `db.go`, `web/src/types/index.ts:266`.
Per memory *Migration dropping a column breaks replay fixtures* and *Rewind
tests drop new columns*: update the forget-and-replay fixtures and the rewind
helpers for both migrations 23 and 24, and run the migration tests on
PostgreSQL (memory *PostgreSQL tests need a DSN*, never the dev DSN).

### 11. Desktop

- Electron `save-settings` (`main.cjs:76`) accepts only connection keys
  (`server`, `deviceId`, the key fields, `binary`/`repo` as today) and drops
  any other key; the `settings` IPC keeps returning connection data only.
- New agent routes (`agent_desktop.go`):
  - `GET /desktop/workstation` → `{defaults, effective, providerModels (shipped lists), seeded}`;
  - `PUT /desktop/workstation` → validates and writes `defaults`, then reports
    capabilities. Validation: `ValidProvider`, `ValidModel`,
    `ValidProviderModels`, custom provider needs `{prompt}`, 4096 characters
    per template, parallelism 1..`MaxParallelism`, setup providers in
    `models.SetupProviders`, editor and terminal trimmed, at most 256
    characters.
- `POST /desktop/projects` (`mapProject`) gains `aiSkillModels`,
  `setupProviders`, `skillCommands` (each with an `inherit…` flag, like the
  existing fields); a skill command must match `^[A-Za-z0-9][A-Za-z0-9._:-]*$`
  after an optional leading `/`.
- `GET /desktop/project` returns, per field, `{value, inherited, source}`
  where `source` is `project`, `workstation` or `default`, and no `server`
  config execution values.
- `desktop/src/main.js`: the "AI Engine CLI" panel becomes "Execution
  defaults" with every workstation field of US4 (model lists edited per
  provider, as the web `ProviderModelsField` did); the project dialog gains
  per-skill models, setup providers and skill command names; inherited hints
  read `source`. IPC in `main.cjs`/`preload.cjs`: `workstation-settings`,
  `save-workstation-settings`.

### 12. Web

- `ProfileModal`: remove provider, models, per-skill models, provider model
  lists from `aiEngine`; keep `MCPEngineConfig` (rename the tab label if it
  now only holds MCP, keeping the translation keys in `locales`).
- `ProjectModal`: remove the `agent` category, move `prCreationStage` into
  `workflow`, remove `repoPath`, `useWorktrees`, `setupProviders: []`,
  `skillOverrides` editors and payload fields.
- `SyncView`: remove the folder field and `repoPath` from both saves.
- `CommandPalette`: spec install posts `{framework, projectId}`; the action is
  offered when a project is selected.
- `Sidebar`: project search and subtitle stop using `repoPath`.
- `BoardColumnsEditor`: stop sending `repoPath` (check its server reader).
- New hook `useProjectEngine(projectId)` (fetches `/api/projects/{id}/engine`,
  refreshed when `useAgentStatus` changes and on window focus).
- `TaskCard`, `TaskDetailModal`: provider, badge and picker from the hook;
  `state: "unknown"` → no picker, badge text from a new translation key
  (fr: "Moteur inconnu", en: "Engine unknown").
- `lib/aiModels.ts`: drop `DEFAULT_PROVIDER_MODELS`, `providerModels`,
  `resolveConfiguredModel`, `taskProvider`; add `reportedModel(report,
  skillId)`. `AIModelField`, `ProviderModelsField` and
  `lib/projectAgentSettings.ts` are deleted if nothing uses them any more.
- `types/index.ts`: remove the execution fields from `Project`,
  `UserSettings`, `Settings`; add `EngineReport`.

## Contract and compatibility

- Contract stays version 1 (additive: a new endpoint, a new route, fields no
  longer sent). `docs/contracts/server-agent-v1.md` rewrites *Configuration
  download* (execution fields listed as "no longer sent since #305"),
  *Precedence*, *Execution defaults and local overrides*, *User configuration*
  and adds *Execution seed* and *Capability report*.
- Old agent + new server: runs with its local overrides over empty server
  values, i.e. provider defaults where it had no local value. Covered by
  ADR 0006 (upgrade both); stated in the changelog line.
- New agent + old server: the server still sends execution fields; `Resolve`
  clears them; `execution-seed` answers 404, so the agent logs, seeds nothing
  and retries at the next connection. `capabilities` 404 is logged once.

## Rejected alternatives

- **Keep serving the execution fields in the configuration for the seed**:
  would let an old agent keep working, but leaves the "server value" path
  alive in the new agent and makes "not used for execution" unverifiable. A
  dedicated read-only endpoint keeps the seed separate and removable in #492.
- **Seed every project at first connection**: writes sections for projects
  the workstation never runs. Seeding on first resolution only touches what
  is used.
- **Keep the capability report in memory on the replica holding the socket**:
  the web request may land on another replica; presence already lives in the
  database for this reason.
- **Reuse the key `projects` for the per-project sections**: an older binary
  would fail to decode the whole file (`map[string]string`). New keys fail
  soft.
- **Refuse execution fields in API requests (400)**: breaks an open web tab
  from before the upgrade; ignoring them is enough to meet US2.

## Test plan

- `internal/agentconfig`: `Resolve` table tests ported from `ApplyOverrides`
  (provider switch drops command, per-template independence, model
  precedence, skill commands, setup providers nil vs empty, worktrees and
  parallelism inheritance); server values in `Config` are cleared; legacy
  layout fold (every row of the table in §1); write rewrites legacy keys, sets
  `layout`, nulls emptied maps; `.taskflow/agent.json` fallback unchanged;
  `ProviderModels` empty-key-is-a-choice.
- `internal/agent`: seed writes only unset keys, only differing keys, marks
  once, not on fetch error, not for a disconnected project; a pre-upgrade
  project (provider override without command on the server, global command
  inherited) resolves the same command line after the seed; capability report
  content and triggers; open editor order; terminal order; desktop routes
  validation and 204/400; skill command substitution reaches the prompt.
- `internal/db`: `AgentConfig` has no execution field and standard commands;
  writes ignored for project, settings, user settings (value unchanged after
  a payload that sets it); `LegacyProjectExecution` equals the pre-change
  composition (golden cases); capabilities upsert/read with presence and
  fallback slots, dead instance ignored; `ResolveTaskEngine` unknown without
  report; migrations 23 and 24 on SQLite and PostgreSQL, fixtures and rewind
  helpers; chain refusal: fake agent returning the headless refusal → run
  failed, summary = reason + chain note, no step enqueued, launch activity
  failed with the reason.
- `internal/handlers`: `execution-seed` (auth, user's terminal/editor,
  omitted empties), `capabilities` (auth, shared token device), project
  engine endpoint (own user only, unknown), `HandleOpenEditor` sends no
  editor.
- `internal/taskmcp`: context without the three fields.
- Web (`node --test` and component tests): no execution field in the
  project/profile payloads, `agent` category gone, workflow holds PR stage,
  card picker hidden on unknown and on `modelSlot: false`, badge from report.
- Desktop UI tests (build first: memory *Desktop UI tests need a build*):
  workstation screen save/reset/validation, project dialog inherited sources,
  `save-settings` drops execution keys.
- Keepalive handler test may flake under full load (memory); rerun before
  blaming a change.

## Target files

Server: `internal/db/agentconfig.go`, `db.go` (project/settings CRUD,
`ResolveTaskEngine`, job dispatch), `usersettings.go`, `migrations.go`, new
`capabilities.go`, `presence.go` (reader), `chain_test.go`,
`internal/handlers/agent_api.go`, `handlers.go`, `macroruns.go`,
`cmd/server/main.go` (routes), `internal/taskmcp/server.go`,
`internal/models/models.go`.

Agent: `internal/agentconfig/{local.go,settings.go,config.go,workstation.go,model.go}`,
`internal/agent/{agent.go,agent_run.go,agent_config.go,agent_desktop.go,
agent_desktop_repositories.go,agent_operations.go,agent_macro_dispatch.go,
agent_console.go,agent_mcp_settings.go,repositories.go,init.go,seed.go,capabilities.go}`.

Clients: `desktop/electron/{main.cjs,preload.cjs}`, `desktop/src/main.js`,
`web/src/components/{ProfileModal,ProjectModal,SyncView,CommandPalette,
Sidebar,BoardColumnsEditor,TaskCard,TaskDetailModal,AIModelField,
ProviderModelsField}.tsx`, `web/src/lib/{aiModels,projectAgentSettings}.ts`,
`web/src/hooks/useProjectEngine.ts`, `web/src/types/index.ts`,
`web/src/locales/translations.ts`.

Docs: `docs/adrs/0029-workstation-owns-the-invocation.md` (number checked
against `origin/main`), `docs/adrs/0015-*.md` (superseded note),
`docs/contracts/server-agent-v1.md`, `desktop/README.md`, `CHANGELOG.md`.

## Implementation notes (deviations recorded during implementation)

- **Numbers.** `origin/main` had landed migrations 23 and 24 and ADR 0029:
  the capability table is migration **25**, the `tty_mode` drop migration
  **26**, and the ADR is **0030**. The rewind helpers undo both
  (`undoWorkstationMigrations`).
- **Seed algorithm.** Writing "every server value that differs from the local
  resolution" would break a workstation whose global override hid a server
  value (a global `aiProvider` over a project row's). The agent instead
  computes the pre-#305 outcome with the former `ApplyOverrides` precedence
  (`legacyApply`) and writes only what makes the new resolution equal to it.
  To tell a seeded default from one the workstation set itself, the defaults
  seed records what it wrote in `seeded.defaultValues`. The project seed's
  terminal is the project row's own; the deployment's travels with the
  defaults (the caller's terminal, which falls back to it).
- **Capability report device.** The report carries the `deviceId` the agent
  presents on its WebSocket, which is the device `AgentDispatcher.Route`
  returns; the credential's device ID is a different value. Rows are keyed by
  the credential's user and that device ID. `GET /api/projects/{id}/engine`
  answers only while an agent of the caller is routed for the project.
- **Seed trigger.** The project seed runs inside `localProjectRoot`, the
  choke point every resolution goes through, once the project is mapped. A
  process-wide lock (`agentconfig.UpdateSettings` / `LockSettings`) serialises
  every read-modify-write of the file, since a listing may seed while the
  desktop saves.
- **Validation.** Templates, terminal and editor are bounded at 4096
  characters; a skill command name uses the skill-command component rule
  (`^/?[A-Za-z0-9][A-Za-z0-9_-]*$`), the one `validateSkills` enforces.
- **Settings upsert.** `getSettingsUnsafe` does not read
  `external_terminal_command`, so copying "the current value" back would have
  erased it; the upsert simply leaves the execution columns out of its
  `ON CONFLICT DO UPDATE` list.
- **Dead code removed.** `applyProjectSettings`, `applySkillCommandOverride`,
  `ProjectSkillCommand`, `TaskWorktreesEnabled`, `dispatchTerminal`,
  `Dispatch.TerminalOverride`.
