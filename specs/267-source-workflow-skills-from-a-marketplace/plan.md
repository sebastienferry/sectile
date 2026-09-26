# #267: Implementation plan

Behaviour is in `spec.md`. Every file:line below was read on `origin/main` at `cb3c387c`.

## Starting point on the branch

`feat/267` carries the round-3 implementation (`6a79d266` to `a9406ffa`: `internal/marketplace`,
the `marketplace_*` agent actions, the `skill_marketplaces` / `project_skill_packs` tables,
the pack UI, ADR 0016) and is 182 commits behind `main`. None of it serves this design.

1. The owner closes PR #350.
2. On `feat/267`, revert the round-3 commits in one `revert(267): ...` commit, so the code
   stays reachable in history as the material the clarification wants kept, then merge
   `origin/main`. No rebase and no force push: the branch is already pushed.
3. The new pull request is opened from `feat/267` at the implemented stage.

The migration numbers the round-3 code used are gone with the revert; this design adds no
table and no column.

## Stack and facts that shape the design

- **The agent already holds every skill's content at dispatch.** `GET /api/v1/agent/config`
  (`internal/db/agentconfig.go:13-88`) returns `agentconfig.Skill{ID, Directory, Command,
  Content, CommandContent, RequiresReconciliation}` (`internal/agentconfig/config.go:6-13`)
  for every effective skill, and `prepareDispatchLocked` fetches it
  (`internal/agent/agent_config.go:345`). What is missing is whether a skill is *custom*.
- **The prompt is built by `dispatchCommand`** (`internal/agent/agent_config.go:743-792`),
  not by `runner.skillSlashPrompt` / `installedSkillPath`, which are only reached from
  `runner.RunAI` and tests. This plan changes `dispatchCommand` and leaves the runner alone.
- **There is already an inline path**: `skillID == "custom"` sends
  `"Sectile task: <key>\n\n<prompt>"` (`agent_config.go:762-767`).
- **Command-line limits.** On Windows the wrapped line goes through `--command-base64`
  (`internal/agent/agent_run.go:110-146`), 4/3 inflation against the 32 767-character
  CreateProcess limit; interactive lines are typed into a PTY, whose line editor truncates
  long input (comment on `runner.SessionCommandLine`, `runner.go:1091-1143`). A composed
  `pickup` skill runs to tens of kilobytes.
- **Plugin format, checked on installed plugins of this workstation** (not from memory):
  `.claude-plugin/plugin.json` with `name`, `version`, `description`, `"skills": "./skills/"`
  and a `userConfig` map (`type`, `title`, `description`, `required`, `sensitive`);
  `.mcp.json` at the plugin root with `mcpServers`, values expanded from
  `${user_config.<key>}`, `${VAR}` / `${VAR:-default}` and `${CLAUDE_PLUGIN_ROOT}`; a
  `sensitive` value is kept in Claude's secure storage. Installed state:
  `~/.claude/plugins/installed_plugins.json`, `version: 2`, `plugins["<name>@<marketplace>"]`
  = list of `{scope: "user" | "project", projectPath?, installPath, version, gitCommitSha}`;
  enablement in `~/.claude/settings.json` `enabledPlugins["<name>@<marketplace>"]`.
- **Workstation settings** are `agentconfig.Settings` (`internal/agentconfig/local.go:24-39`)
  in `~/.config/sectile/settings.json`, `Defaults` in `internal/agentconfig/workstation.go:49-55`,
  edited through `GET|PUT /desktop/workstation` (`internal/agent/agent_desktop_settings.go:247-285`),
  "Execution defaults" panel in `desktop/src/main.js:1223-1370`.

## D1. The plugin generator

`internal/skills/plugin.go`, pure (no I/O):

```go
// RenderPlugin returns the Claude plugin distributing Sectile's workflow skills,
// keyed by path relative to the plugin root.
func RenderPlugin(version string) (map[string][]byte, error)
```

Output:

- `.claude-plugin/plugin.json`: `name: "sectile"`, `version` (refused when empty or not
  SemVer without a leading `v`), `description`, `"skills": "./skills/"`, and
  `userConfig`:
  - `server_url`: string, required, "Sectile server URL, for example https://sectile.example.com";
  - `api_key`: string, required, `sensitive: true`, "Workstation API key, from the desktop
    Connection settings".
- `.mcp.json`:
  ```json
  {"mcpServers": {"sectile": {"type": "http",
    "url": "${user_config.server_url}/mcp",
    "headers": {"Authorization": "Bearer ${user_config.api_key}"}}}}
  ```
  Same shape as the direct setup's Claude entry (`internal/agentconfig/mcp.go:30-46`).
- `skills/<DirName>/SKILL.md` for every entry of the catalogue (`internal/skills/catalog.go`),
  rendered by `RenderGenericSkillContent` (D2).
- `.claude-plugin/marketplace.json` is **not** emitted in the plugin; `RenderMarketplace(version)`
  emits a one-plugin marketplace (`name: "sectile"`, `plugins: [{name: "sectile", source:
  "./plugins/sectile", version}]`) used by the test and by a local try-out. How the real
  repository is laid out is Q11, out of scope.

Command: `cmd/sectile-plugin` (`go run ./cmd/sectile-plugin -version X.Y.Z -out DIR`),
writes the files, refuses a non-empty `DIR` unless `-force`, and with `-marketplace` writes
the marketplace wrapper around it. `make plugin VERSION=... OUT=...` wraps it. No pipeline
step publishes it (Q11).

## D2. Generic skill content

`RenderGenericSkillContent(s StageSkill) string` in `internal/skills/catalog.go`, beside
`RenderSkillContent` (`catalog.go:437`), sharing its assembly:

- Frontmatter `name: <DirName>` and a description; no framework-specific display name
  (`specifyFrameworkName` and its siblings, `catalog.go:342-370`) is used.
- For each fragment with framework variants (`read-first` and `steps` of `specify`,
  `refine_macro`, `realign_macro`, and the ones `renderPickupSteps` composes), emit one
  subsection per framework found on disk:
  `### When get_project_context reports specFramework "openspec"` followed by the
  `.openspec.md` fragment, then the same for `speckit`, then the unsuffixed fragment under
  "Otherwise" when it exists. The list of frameworks is read from the embedded fragment
  names, never re-typed.
- The pull-request policy that `EffectiveProjectSkills` appends per project
  (`internal/db/projectskills.go:174`) becomes a generic section on the same five skills:
  "Read prCreationStage from get_project_context before executing", followed by the
  existing wording for each stage value.
- The contracts (task access, session title, transition) are appended as in
  `RenderSkillContent`.
- No `$ARGUMENTS` command variant: plugin skills are invoked as `/sectile:<dir> <args>` and
  Claude passes the arguments.

A test asserts that no generic skill contains `speckit`-only or `openspec`-only text outside
its framework subsection, and that every fragment `RenderSkillContent` uses for either
framework appears in the generic output.

## D3. Stop writing at dispatch, start and save

Remove, keeping the functions for the explicit paths:

| Site | Change |
| --- | --- |
| `prepareDispatchLocked`, `agent_config.go:387-394` | drop `agentconfig.Scaffold` and `bootstrapLocalMCP` |
| `prepareMacroSkills`, `agent_macro_dispatch.go:180-203` | drop `Scaffold` and `bootstrapLocalMCP`; keep `Resolve` / `Validate` |
| `syncLocalProject`, `agent_config.go:807-825`, called from `connect` (`agent.go:514`) | delete the function and its call; `connect` no longer fails on a write |
| `refreshMCPConnections` at start, `agent.go:344` | delete the start-up call; the desktop MCP save (`agent_mcp_settings.go:27`) keeps writing when the user saves |
| `SaveProjectSkillContent`, `ResetProjectSkillContent`, `internal/db/projectskills.go:297,332` | drop the `WriteProjectSkillToRepo` call |
| `WriteProjectSkillToRepo` / `WriteAllProjectSkillsToRepo`, `projectskills.go:346-352` | delete when no caller remains |

Kept as the explicit writers: `initializeProvider` (`internal/agent/init.go:166-214`, used by
`init` and the desktop `action=initialize`), the `sync_config` operation and
`POST /api/projects/{id}/install-skills` (explicit install request), the desktop MCP
connection save. Before deleting the start-up `refreshMCPConnections`, check whether a
stored `MCPConnection` depends on a value that changes between starts (loopback port,
rotated token); if one does, that entry is rewritten when the value changes, not at every
start, and the implementation says so in the pull request.

The skills editor keeps its read-only divergence badges (`ListProjectSkillEditor`,
`projectskills.go:186-267`); they keep describing the direct copy only, and their label
says "direct copy" rather than "installed".

## D4. Knowing a skill is custom

- `agentconfig.Skill` gains `Custom bool \`json:"custom,omitempty"\``.
- `db.AgentConfig` (`internal/db/agentconfig.go:74-83`) sets it when `project_skills` holds
  non-empty `content` for that skill (`projectSkillOverrides`, `projectskills.go:44-67`).
  The pull-request policy suffix does not make a skill custom.
- `Resolve` (`internal/agentconfig/workstation.go:217-223`) sets it too when the workstation
  `Settings.Skills[id]` overrides the content: that override is a custom skill of this
  workstation and follows the same rule.
- `docs/contracts/server-agent-v1.md` documents the field. An older server sends no field:
  every skill reads as not custom, so the agent runs installed skills, which is today's
  outcome for an unedited project.

## D5. The two workstation settings

In `agentconfig.Defaults` (`workstation.go:49-55`):

```go
// CustomSkillsWin runs a project's custom skill instead of the installed one.
// Nil means true.
CustomSkillsWin *bool `json:"customSkillsWin,omitempty"`
// InstalledSkillSource is the source tried first: "direct" or "plugin".
// Empty means "direct".
InstalledSkillSource string `json:"installedSkillSource,omitempty"`
```

Workstation-wide only (Q12, Q13 name a workstation setting; no per-project override).
Validated in `ValidateDefaults` (unknown source refused). `GET|PUT /desktop/workstation`
carries both; `desktop/src/main.js` "Execution defaults" panel gains a checkbox "Custom
project skills win" and a select "Installed skills source: Direct copy / Claude plugin",
with one line of help each. `SettingsLayout` does not change: both fields are optional.

## D6. Resolving the skill at dispatch

New `internal/agent/skillsource.go`:

```go
type skillChoice struct {
    Kind    string // "custom", "direct", "plugin", "command"
    Command string // slash command without its leading "/", empty for "custom"
    File    string // run-private SKILL.md, "custom" only
}

func (d *Dispatcher) chooseSkill(config agentconfig.Config, overrides agentconfig.Settings,
    project agentconfig.ProjectSettings, skill agentconfig.Skill, provider, workDir, runID string) (skillChoice, error)
```

Order (FR5):

1. `skill.Custom && customSkillsWin` → write the content to the run file (D7), kind
   `custom`.
2. `project.SkillCommands[skill.ID]` set explicitly → kind `command`, used verbatim (US5,
   last scenario). `Resolve` today copies it into `Skill.Command` (`workstation.go:207`);
   `Resolve` records that it did (`Skill.CommandOverridden bool`, not serialised) so the
   choice can tell an explicit command from the catalogue default.
3. Provider without `SkillDir` in `ResolveLocations` (gemini, cursor, vibe) → kind
   `direct` with `skill.Command`, no check (US6, second scenario).
4. Probe both sources, preferred first:
   - direct: `~/<SkillDir>/<Directory>/SKILL.md` exists for that provider
     (`internal/agentconfig/locations.go:47-76`);
   - plugin (Claude only): `claudePluginSkill(home, workDir, dir)` reads
     `installed_plugins.json`, keeps keys `sectile@*`, keeps entries with `scope: "user"` or
     `scope: "project"` whose `projectPath` contains `workDir` or the checkout root, skips a
     key whose `enabledPlugins` value is `false` in `~/.claude/settings.json` (missing
     means enabled, missing file means enabled), and requires
     `<installPath>/skills/<dir>/SKILL.md`. Command `sectile:<dir>`. Unreadable or
     malformed JSON means not installed, logged once.
5. Nothing → `errSkillNotInstalled{skill, provider}` (D8).

`dispatchCommand` (`agent_config.go:743-792`) takes the choice:

- `direct`, `plugin`, `command`: `promptArg = "/" + choice.Command + " " + taskKey`, the
  rest unchanged (payload prompt, adjust contract).
- `custom`: `"Sectile task: " + taskKey + "\n\nFollow the skill in " + choice.File +
  " for this task. It replaces any installed skill of the same name.\n\n" + payload.Prompt`,
  the adjust contract appended as today. For Claude the run directory is added with
  `--add-dir=` (already supported by `modeCommandLine`, `agent_config.go:583-614`) so an
  interactive session reads it without a permission prompt.

Called from the task path (`prepareDispatchLocked` → `dispatchCommand`) and the macro path
(`prepareMacroWorkspace`, `agent_macro_dispatch.go:113`). `runner.PrepareAI` is unchanged.

## D7. The run-private skill file

`~/.config/sectile/runs/<runID>/<Directory>/SKILL.md` (beside `settings.json`, created
0700, file 0600), holding `skill.Content` (not `CommandContent`: the ticket is in the
prompt). Removed when the run ends (success, failure, cancel) by the code that already
closes the run, and any directory older than 7 days is swept at agent start, since a crash
can skip the removal. The run ID is validated against the existing run ID shape before it
becomes a path.

Rejected: inline in the prompt (Windows base64 limit and PTY truncation for a composed
`pickup`); a file in the worktree (dirties the checkout that `transition_stage` refuses);
`os.TempDir` (Claude's sandbox and permission rules treat it unpredictably across
platforms, and the sweep needs a directory Sectile owns).

## D8. Failure when nothing is found

`errSkillNotInstalled` becomes the dispatch failure through the existing
`launchFailure` / `sendStatus(... "failed")` path (`agent.go:1059-1062`), before any PTY is
opened. Message (French, a run surface):

> Skill « {dir} » introuvable pour {provider} : aucune copie directe{, aucun plugin Sectile
> installé et activé}. Installez le plugin `sectile` dans Claude, lancez
> `sectile-agent init --provider {provider}`, ou utilisez « Initialize » dans les réglages
> du projet de l'application de bureau.

The Claude-only fragments are dropped for other providers.

## D9. Passive signal

- The dispatcher keeps `customSkillUse map[projectID+skillID]{ProjectName, SkillID, LastRun}`
  in memory, under its mutex, filled when a `custom` choice launches.
- `GET /desktop/workstation` returns it as `customSkillsUsed`, sorted by last run.
- The run records one activity step, in French: « Skill personnalisé du projet utilisé :
  {dir} ». Sent with the existing step reporting of the dispatch.
- `desktop/src/main.js`: when `customSkillsUsed` is non-empty, the sidebar settings button
  (`main.js:42`, icon at :2129) gets a warning badge (`--warn-text`, `style.css:412`) and an
  `aria-label` "Settings, custom skills used"; the "Execution defaults" panel shows a
  "Custom skills used" notice listing project, skill and time, next to the D5 checkbox.
  Refreshed when the settings open and on the existing workstation refresh.

Memory only: the signal says what ran since the agent started, which is what "when a custom
skill is used" asks; persisting it would put run state into the settings file.

## D10. Command validation

Accept an optional `<namespace>:` prefix on the command only, not on IDs or directories:

- `skillCommandName` (`internal/agentconfig/workstation.go:277`):
  `^/?(?:[A-Za-z0-9][A-Za-z0-9_-]*:)?[A-Za-z0-9][A-Za-z0-9_-]*$`;
- `component` in `validation.go:10,42-58` keeps its pattern for ID and Directory, and the
  command check uses the new pattern;
- `SKILL_COMMAND` in `desktop/src/execution-fields.mjs:18`, same pattern.

## D11. The direct setup, labelled optional

- `init` (`internal/agent/init.go`) usage text and final message: the setup is optional,
  Claude users can install the `sectile` plugin instead, and an older `sectile` entry in
  `~/.claude.json` is not removed by the plugin.
- Desktop Deployment tab (`desktop/src/main.js:2049-2076`): same help line under the
  provider select.

## Data contracts

- Agent config (`GET /api/v1/agent/config`), each skill: `+ "custom": true` when custom.
- Workstation settings, `defaults`: `+ "customSkillsWin": false` (absent = true),
  `+ "installedSkillSource": "plugin"` (absent = "direct").
- `GET /desktop/workstation`: `+ "customSkillsUsed": [{"projectId", "projectName",
  "skillId", "directory", "lastRun"}]`.
- Plugin files: D1.

## Documentation

- ADR `docs/adrs/0034-sectile-is-installed-as-a-claude-plugin.md`: plugin distribution,
  the agent stops writing, resolution order, no reconciliation; rejected alternatives below.
- `docs/contracts/server-agent-v1.md`: the `custom` field.
- `README.md`: installing Sectile in Claude through the plugin; the direct setup as the
  alternative and the only route for other CLIs.
- `CHANGELOG.md` `[Unreleased]`: Added (plugin, the two settings, the custom-skill notice),
  Changed (the agent no longer writes skills or MCP registration at dispatch, start or
  skills-editor save).

## Target files

- New: `internal/skills/plugin.go`, `internal/skills/plugin_test.go`,
  `internal/skills/testdata/plugin/` (golden), `cmd/sectile-plugin/main.go`,
  `internal/agent/skillsource.go`, `internal/agent/skillsource_test.go`,
  `docs/adrs/0034-sectile-is-installed-as-a-claude-plugin.md`.
- Changed: `internal/skills/catalog.go`, `internal/agentconfig/config.go`,
  `internal/agentconfig/workstation.go`, `internal/agentconfig/validation.go`,
  `internal/db/agentconfig.go`, `internal/db/projectskills.go`,
  `internal/agent/agent_config.go`, `internal/agent/agent_macro_dispatch.go`,
  `internal/agent/agent.go`, `internal/agent/agent_desktop_settings.go`,
  `internal/agent/init.go`, `desktop/src/main.js`, `desktop/src/execution-fields.mjs`,
  `desktop/src/style.css`, `web/src/components/SkillsView.tsx` (badge wording),
  `Makefile`, `README.md`, `CHANGELOG.md`, `docs/contracts/server-agent-v1.md`.

## Rejected alternatives

- **Keep writing at dispatch, only skip it when the plugin is present.** Still rewrites the
  user's configuration unasked, which Q6 forbids.
- **Version or checksum reconciliation** between installed and server skills: rejected by
  the owner (Q9).
- **Serving the plugin from the Sectile server** or publishing it from the sectile
  repository: the owner chose a separate marketplace repository (Q8).
- **Hard-coding the server URL in the plugin**, or reading it from an environment variable:
  one plugin serves every deployment, and a `userConfig` value with a sensitive key is what
  the format offers for per-install values.
- **Installing the plugin through `claude plugin install` from the agent**: it is the
  user's choice (Q6); the agent reads Claude's state and never writes it.
- **Per-project values of the two settings**: the owner named workstation settings (Q12,
  Q13); a per-project override can come later without changing the defaults.
