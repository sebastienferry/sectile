# Plan #730 - Desktop: global Sandbox settings with a project whitelist

Behaviour: `spec.md`. This file holds the implementation choices. It builds on
`specs/700-desktop-sandbox-configuration/plan.md` and ADR 0048.

## Stack

- Agent (Go): `internal/agentconfig` (workstation settings, resolution, fold),
  `internal/agent` (launch, Desktop API, "Always allow").
- Desktop (vanilla JS, Electron): `desktop/src/sandbox-settings.mjs`,
  `desktop/src/main.js`.
- No server, database, tracker or web change.

## Data model

Two new fields of `agentconfig.Defaults` (`internal/agentconfig/workstation.go`),
stored under `defaults` of `~/.config/sectile/settings.json`:

```go
// ClaudeSandbox is the Sandbox values every covered project applies (#730),
// under its own values. Nil states nothing.
ClaudeSandbox *ClaudeSandbox `json:"claudeSandbox,omitempty"`
// ClaudeSandboxProjects is the whitelist of the projects ClaudeSandbox
// applies to, by project ID. Empty covers every project.
ClaudeSandboxProjects []string `json:"claudeSandboxProjects,omitempty"`
```

One new field of `agentconfig.Seeded`:

```go
// SandboxFolded is when the project Sandbox values were folded into the
// workstation defaults (#730), so the fold never repeats.
SandboxFolded string `json:"sandboxFolded,omitempty"`
```

- `defaults` and `seeded` are already owned keys of `WriteSettings`
  (`internal/agentconfig/settings.go`, `ownedKeys`), so no new key.
- `overlay` (`internal/agentconfig/local.go`) builds `out.Defaults` field by
  field: carry both fields from `top`, falling back to `base` for the
  sandbox with `firstSandbox`, and `firstList` for the whitelist (memory "New
  Overrides key needs a merge").
- `ProjectSettings.ClaudeSandbox` is unchanged: it now holds the project
  values only.

## Resolution

New function in `internal/agentconfig/claude_sandbox.go`:

```go
// ResolvedClaudeSandbox is what a launch of the project applies (#730): the
// workstation values under the project's own when the whitelist covers the
// project, the project's alone otherwise. Nil when nothing is stated.
func (s Settings) ResolvedClaudeSandbox(projectID string) *ClaudeSandbox

// CoversProject reports whether the workstation Sandbox values apply to the
// project: an empty whitelist covers every project.
func (d Defaults) CoversProject(projectID string) bool
```

- Lists: global entries, then the project's, deduplicated on the trimmed
  entry, order kept (reuse `normalizeEntries`' dedupe; entries are already
  normalised on save).
- State: the project's `Enabled` when non-nil, else the global one.
- `agentDaemon.projectClaudeSettings` (`internal/agent/agent_claude_sandbox.go`)
  passes `settings.ResolvedClaudeSandbox(projectID)` to `ClaudeSettingsFile`
  instead of the project section. Everything downstream of #700 (the file per
  project, `--settings=`, the conversation, the headless line, Windows) is
  unchanged.

## Fold

New `foldProjectSandboxes(*Settings) (bool, []string)` in
`internal/agentconfig/claude_sandbox.go`, called from `readConverted`
(`internal/agentconfig/settings.go`) beside `convertEngines` and
`dropRetiredProviders`, so `MigrateSettingsReport` persists it at agent start
with the existing `.bak-layout<n>` backup, and every read before that sees the
folded values.

1. Return at once when `Seeded.SandboxFolded` is set.
2. Walk `ProjectSettings` in sorted project ID order (Go maps have no order;
   the settings file is written with sorted keys, so this is "the order of the
   settings file"). For each project with a non-nil `ClaudeSandbox`, append its
   four lists to the global lists through `AddAllow`-style dedupe. An entry
   `normalizeEntries` would refuse is left on the project and returned as a
   warning, logged by the agent at start.
3. States: collect the non-nil `Enabled` values. When there is at least one
   and they are all equal, set the global `Enabled` and clear the projects'.
   Otherwise leave every project's state and the global one as they are.
4. Clear the moved lists from each project; drop a section that `IsZero`
   reports empty (`SetProject` does it).
5. Leave `ClaudeSandboxProjects` empty. Set `Seeded.SandboxFolded` to the
   current UTC time in RFC 3339, also when there was nothing to fold, so a
   value added later is never folded.
6. Report `changed` when anything moved or the marker was set, so
   `MigrateSettingsReport` writes the file once.

The legacy layouts that `readFolded` overlays predate #700 and hold no
sandbox values: nothing to do for them beyond `overlay` carrying the new
fields.

## Desktop API

- `GET /desktop/workstation` (`workstationView`,
  `internal/agent/agent_desktop_settings.go`): add `claudeSandbox`
  (`claudeSandboxPayload(settings.Defaults.ClaudeSandbox)`),
  `claudeSandboxProjects` (the whitelist, `[]` when empty) and
  `platformSandbox`.
- `PUT /desktop/workstation` replaces `settings.Defaults` as a whole today.
  Keep the stored `ClaudeSandbox` and `ClaudeSandboxProjects` whatever the
  body says (FR11): the Execution panel's payload does not carry them.
- New `PUT /desktop/workstation/sandbox`, routed beside `/desktop/workstation`
  in `internal/agent/agent_desktop.go`, body
  `{claudeSandbox, claudeSandboxBase, projects}`:
  - `claudeSandbox` merged with `MergeClaudeSandbox(sent, base, stored)` when
    `claudeSandboxBase` is present, then `NormalizeClaudeSandbox`, as the
    project save does; a zero value stores nil.
  - `projects`: trimmed, deduplicated, restricted to IDs that have a
    `ProjectSettings` section or are known to the agent's project list; the
    owner's list replaces the stored one (no three-way merge: the whitelist
    has no concurrent writer).
  - Under `d.prepareMu`, through `agentconfig.UpdateSettings`.
- `GET /desktop/project` (`agent_desktop.go`, the payload with
  `claudeSandbox`): add `claudeSandboxGlobal`, the global values the project
  inherits (`claudeSandboxPayload` of the global values when
  `CoversProject`, `null` otherwise), and `claudeSandboxCovered`. The project
  save is unchanged: it sends and stores the project values only.
- New `POST /desktop/project/sandbox/promote`, body `{projectId, rule}`: in
  one `UpdateSettings`, remove the trimmed rule from the project's allow rules
  and add it to the global ones with `AddAllow`. 404 when the project has no
  such rule. Answers with the refreshed project payload fields
  (`claudeSandbox`, `claudeSandboxGlobal`).
- Project removal (`agent_desktop.go`, the handler that deletes
  `settings.ProjectSettings[id]`): also remove `id` from
  `Defaults.ClaudeSandboxProjects`.

## "Always allow"

Unchanged: `addProjectAllowRules` keeps writing to the project. The trace
notice "Rule added to the project's Sandbox settings" stays true. The headless
refusal line in `internal/agent/agent_headless.go` keeps pointing at the
project's Sandbox settings and is not rewritten (user-visible French string,
AGENTS.md).

## Desktop

- `SETTINGS_CATEGORIES` (`desktop/src/main.js`): add
  `{id:'Sandbox',label:'Sandbox',icon:<the project category's shield icon>}`
  after `Engines`.
- `desktop/src/sandbox-settings.mjs`: make `sandboxSettings` serve both
  levels:
  - an option `inherited` (the global values, or `null`), rendered in each
    list as entries marked "Global" (a `sandbox-entry-inherited` class, no
    Remove button), above the own entries, and in the state hint ("Inherited
    from the workstation: On");
  - an option `onPromote(rule)`, which adds a "Move to global" button to each
    own allow entry; absent on the workstation level;
  - the `scope` paragraph names the level it edits;
  - `bothLists` warns on the union of inherited and own entries.
- New `whitelistEditor` in the same module: one checkbox per project added to
  the workstation (the list the settings navigation already holds, by
  `id`/`name`), the hint "Applies to every project" while none is checked and
  "Applies only to the checked projects" otherwise, and the warning that a
  project added later is not covered once any is checked.
- The workstation panel saves through `api.saveWorkstationSandbox({
  claudeSandbox, claudeSandboxBase, projects })`, its own save button as the
  other workstation panels have. It reloads its values after the save.
- The project panel passes `inherited: info.claudeSandboxGlobal` and
  `onPromote`, which calls the promote endpoint and refreshes the panel with
  its answer. The command preview of the project panel is built from the
  resolved values: `launchesGetSettings` receives the union.
- `desktop/electron/main.cjs` and `desktop/electron/preload.cjs`: expose the two new API calls next to
  the existing project and workstation ones.

## ADR

New `docs/adrs/0050-claude-sandbox-values-have-a-workstation-level.md`,
amending ADR 0048: the two levels, the whitelist and its empty-means-all
rule, the resolution order, the one-time fold and why it widens rules on
purpose, and "Always allow" staying per project. Rejected alternatives: a
global level that replaces the project one, a filesystem-folder whitelist,
headless whitelisting in place of `bypassPermissions`, keeping project values
in place at the upgrade. ADR 0048's Status line gains "Amended by ADR 0050".

## Changelog

One `Added` line under `[Unreleased]` in `CHANGELOG.md`, beside the #700
line:

> **Set Claude Code's Sandbox once for every project, from Desktop.** A new
> **Sandbox** category in the workstation settings holds the sandbox state,
> domains, writable paths and allow and deny rules every project applies, or
> only the projects you check. A project's own Sandbox values add to them, its
> sandbox state overriding the workstation's, and its category shows what it
> inherits; **Move to global** moves one of its allow rules up. The Sandbox
> values set per project so far become workstation values at the first start
> of the upgraded agent and apply to every project, so remove there any rule
> that should stay with one project. Upgrade the agent along with the
> desktop. (#730)

## Rejected alternatives

- Resolving the two levels as two `--settings` files: Claude Code takes one
  `--settings` argument, and a second file would need a different
  mechanism per launch kind.
- Reusing `PUT /desktop/workstation` for the global values: it replaces
  `Defaults` as a whole from the Execution panel, which knows nothing of the
  Sandbox values; one save would erase the other's.
- Folding in `UpdateSettings` or at the first Desktop read: the agent start
  is where the earlier settings migrations run, with their backup.
- A three-way merge for the whitelist: nothing but the owner writes it.

## Files

- `internal/agentconfig/workstation.go`, `local.go`, `settings.go`,
  `claude_sandbox.go` (+ tests)
- `internal/agent/agent_claude_sandbox.go`, `agent_desktop.go`,
  `agent_desktop_settings.go` (+ tests)
- `desktop/src/sandbox-settings.mjs`, `desktop/src/main.js`,
  `desktop/electron/main.cjs`, `desktop/electron/preload.cjs` (+ `desktop/tests/sandbox-settings.ui.cjs`)
- `docs/adrs/0050-claude-sandbox-values-have-a-workstation-level.md`,
  `docs/adrs/0048-…` (status line)
- `CHANGELOG.md`
