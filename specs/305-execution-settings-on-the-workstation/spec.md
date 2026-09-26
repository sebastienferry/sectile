# Specification #305 - Execution settings live on the workstation

- Ticket: https://github.com/sebastienferry/sectile/issues/305
- Branch: `feat/305`
- Clarification: `docs/clarifications/305.md` (rounds 1 and 2, owner
  confirmed, no open product question)
- Follow-ups already filed: #492 (drop the read-only columns), #493 (team
  sharing of project execution defaults), #494 (committed
  `.taskflow/agent.json`)
- Framework: Spec Kit

## Summary

The server owns the method; the workstation owns the invocation. Every
setting that only decides *how* a workstation runs an AI CLI is read by the
local agent from `~/.config/sectile/settings.json`, edited from the desktop
app, and is no longer written, edited or used for execution by the server or
the web interface. The server keeps the stored values read-only for one
release, only to seed each workstation once, so an existing setup keeps
working without being configured again. The agent reports to the server what
it will actually run, per project, and the web shows that report instead of
guessing.

## Scope

In scope, the **execution settings**:

| Setting | Level on the workstation |
| --- | --- |
| AI provider | global default, per project |
| model, per-skill models | global default, per project |
| interactive and headless command templates | global default, per project |
| model list per provider (the models a launch may pick) | global |
| terminal emulator | global default, per project |
| editor command | global |
| use worktrees | global default, per project |
| parallelism | global default, per project |
| extra setup providers (agents that get the skills and MCP) | global default, per project |
| skill command names (the slash command a stage runs) | per project |
| checkout folder, specifications folder | per project (already local) |

Also in scope: the one-time seed of each workstation, the capability report,
the web and desktop editors, `get_project_context`, the open-editor and
open-terminal actions, the full-chain refusal message, and the removal of
`ttyMode`.

Out of scope:

- the method, identity, secrets, tracker, sync, board mirror and web
  presentation settings (ticket table C), which do not change;
- dropping the read-only server columns (#492);
- sharing execution defaults across a team (#493); `.taskflow/agent.json`
  keeps its current fallback role and nothing more;
- the committed `.taskflow/agent.json` (#494);
- merging `/pickup-issue` with the server-orchestrated full chain;
- a server-side up-front refusal of a full chain (decision 5 of the
  clarification).

## Definitions

- **Execution setting**: a setting listed in the scope table.
- **Local file**: `~/.config/sectile/settings.json` on the workstation (the
  desktop's data directory when `SECTILE_DESKTOP_DATA_DIR` is set, as today).
- **Workstation defaults**: the global section of the local file.
- **Project section**: the part of the local file that holds one project's
  execution settings, keyed by the project's primary key.
- **Provider defaults**: what applies when neither the project section nor the
  workstation defaults set a value: provider `agy`, the provider's own
  command, no model flag, the model list Sectile ships for the provider, the
  terminal detected on the platform, editor `code`, worktrees on, parallelism
  1, no extra setup provider, the stage's standard skill command.
- **Server values**: the execution settings still stored in the server
  database (deployment settings row, project rows, each account's user
  settings) by releases before this one.
- **Seed**: the one-time copy of the server values into the local file.
- **Capability report**: what an agent tells the server, per project, about
  the engine it will run: provider, model, per-skill models, model list,
  whether the command line carries a model, and whether a headless run is
  possible.

## User stories (prioritised)

### US1 - A workstation runs from its local file only (P1)

As a developer, I want every launch on my workstation to use the execution
settings of my local file, so that what runs is what is installed and chosen
on this machine, whatever the server stores.

**Acceptance**

- **Given** a project section setting provider `claude` and model `opus`, and
  server values naming `codex`, **when** a skill is launched on that project
  from any surface, **then** the command line runs Claude Code with `opus`.
- **Given** a project with no project section and workstation defaults naming
  a provider, model and templates, **when** a skill is launched, **then** the
  workstation defaults apply.
- **Given** neither level sets a value, **then** the provider defaults apply,
  whatever the server values say.
- **Given** a project section that sets a provider different from the
  workstation default and no command of its own, **then** the workstation
  default command is not used for it (a command written for one CLI never
  serves another), exactly as the local override behaves today.
- **Given** a per-skill model at either level, **then** the most specific
  statement wins: a per-skill model outranks a bare model whatever level the
  bare model sits on, and the project level outranks the workstation level for
  the same kind of statement.
- **Given** a one-off model chosen for a launch, **then** it outranks every
  configured level, for that run only, as today.
- **Given** a project section that sets a skill command name, **when** that
  skill is launched, **then** the prompt starts with that command; without
  one, it starts with the stage's standard command.
- **Given** worktrees and parallelism set in the workstation defaults and not
  in the project section, **then** the project uses them; the parallelism
  bound (1 to 10) and the rule "no worktrees means one execution at a time"
  are unchanged.
- **Given** extra setup providers set at either level, **then** skills and the
  Sectile MCP registration are installed for the effective provider and for
  those extra providers; the project level replaces the workstation default
  list, it does not add to it.
- **Given** a terminal set at either level, **when** a terminal is opened for
  a task from the web, the desktop or a launch, **then** the project level
  wins over the workstation default, then the agent's `--terminal` flag, then
  platform detection. An explicit `--terminal` given to the agent still
  outranks everything, as today.
- **Given** an editor set in the workstation defaults, **when** "open in
  editor" is asked for a task or a project, **then** that editor opens;
  without one, `code`.

### US2 - Nothing writes an execution setting to the server (P1)

As an operator, I want no surface to store an execution setting on the
server, so that the database stops holding values nothing uses.

**Acceptance**

- **Given** any request that creates or updates a project, the deployment
  settings or a person's user settings, **when** it carries an execution
  setting (`aiProvider`, `aiModel`, `aiSkillModels`, `aiCommandTemplate`,
  `aiCommandTemplateAutonomous`, `aiProviderModels`, `externalTerminalCommand`,
  `editorCommand`, `useWorktrees`, `setupProviders`, `skillOverrides`,
  `repoPath`, `repoPaths`, `ttyMode`), **then** the value is ignored, the
  stored value is unchanged, and the rest of the request is applied.
- **Given** a project created after the upgrade, **then** its stored execution
  columns hold their column defaults, whatever the request said.
- **Given** a project saved from the web with other changes, **then** its
  stored `setupProviders` are no longer reset to an empty list.
- **Given** the server's answer for a project, the deployment settings or a
  person's user settings, **then** it no longer carries execution settings,
  except where US6 requires a value for the seed.

### US3 - The web offers no execution setting editor (P1)

As a user of the web interface, I want it to stop offering settings that the
workstation decides, so that I am not misled into changing something that has
no effect.

**Acceptance**

- **Given** the profile dialog, **then** its "Paramètres de l'agent" tab
  (`aiEngine`) no longer shows the provider, the model, the per-skill models or
  the model lists; the MCP engine configuration it also hosts stays in the
  profile, and no tab is left empty.
- **Given** the project dialog, **then** the "Agent settings" category is
  gone; the PR creation stage it held is edited in "Agentic workflow";
  the categories are *Général*, *Tracker*, *Agentic workflow* and
  *Compétences IA & SDD*.
- **Given** the project dialog, **then** it offers no local folder
  (`repoPath`) field and no per-skill command name field.
- **Given** the sync view, **then** it offers no local folder field and saves
  none.
- **Given** the web installs a specification framework from the command
  palette, **then** it names the project only; the agent picks its own
  checkout and provider. The palette no longer requires a server folder to
  offer the action.
- **Given** a task opened in the web, **then** the provider it names is the
  one of the capability report (US5), not a server value.

### US4 - The desktop edits every execution setting (P1)

As a developer, I want to set every execution setting of my workstation from
the desktop app, so that I never need to edit the local file by hand.

**Acceptance**

- **Given** the desktop settings, **then** a workstation screen edits the
  workstation defaults: provider, model, per-skill models, both command
  templates, the model list of each provider, terminal, editor, worktrees,
  parallelism and extra setup providers. The MCP connection choice it shows
  today stays.
- **Given** the desktop project settings, **then** they edit the project
  section: checkout and specifications folders (as today), provider, model,
  per-skill models, both command templates, terminal, worktrees, parallelism,
  extra setup providers and skill command names. Each field says whether it is
  set for the project or inherited, and shows the inherited value; each can be
  reset to inherit.
- **Given** a project field left to inherit, **then** its displayed default is
  the workstation default, else the provider default; never a server value.
- **Given** a save from either screen, **then** it goes through the local
  agent and writes the local file with the same rules; nothing is sent to the
  server except the capability report (US5).
- **Given** a save that empties a map or a list, **then** the emptied value is
  removed from the local file rather than kept from its previous content.
- **Given** an invalid value (a model outside the allowed shape, a custom
  provider whose command lacks `{prompt}`, a parallelism outside 1 to 10, an
  unknown setup provider, a command longer than 4096 characters, a skill
  command name that is not a single word), **then** the save is refused with
  the reason and the file is unchanged.
- **Given** the agent is not running, **then** the desktop shows the settings
  as unavailable instead of writing the file itself.

### US5 - The web shows what the workstation will run (P1)

As a user launching from the web, I want the model picker and the engine
badge to reflect my workstation, so that what the web announces is what runs.

**Acceptance**

- **Given** my agent is connected for a project, **when** it connects, and
  again after each local save, **then** it reports its effective provider,
  model, per-skill models, model list, whether its command line carries a
  model, and whether a headless run is possible, for every project it serves.
- **Given** that report, **then** the task card's pre-run badge shows the
  model the next step would run with, and the model picker offers the reported
  model list, minus the configured model.
- **Given** a reported command line that carries no model, **then** the
  picker is hidden and the badge shows no model.
- **Given** no agent of mine connected for the project, **then** the picker is
  hidden and the badge says the engine is unknown.
- **Given** a launch from the web or a queued chain step, **when** the run is
  recorded, **then** its provider and model are taken from the report of the
  workstation the launch goes to; with no report, they are recorded as unknown
  until the agent reports the engine it actually launched, as it already does.
- **Given** a model chosen in the picker that the workstation no longer
  reports, **then** the launch runs the configured model, as today when a list
  changes.
- **Given** two people on the same project, **then** each sees the report of
  their own workstation.
- **Given** an agent that predates this release, **then** it reports nothing
  and the web behaves as with no agent connected for the picker and badge; its
  launches still run.

### US6 - An existing setup keeps working after the upgrade (P1)

As a developer on an existing deployment, I want my workstation to keep
running what it ran before the upgrade, without reconfiguring it.

**Acceptance**

- **Given** a workstation upgraded with a local file that does not yet hold
  seeded workstation defaults, **when** its agent first connects to the
  server, **then** it writes into the workstation defaults the deployment's
  provider, command templates, model, per-skill models and model lists, and
  the paired account's terminal and editor (which already fall back to the
  deployment's), for every key the local file does not already set.
- **Given** a project the workstation has not seeded yet, **when** the agent
  first resolves that project, **then** it writes into the project section
  every execution setting whose server value, as the server composed it before
  this release, differs from what the local resolution would otherwise give:
  provider, templates, model, per-skill models, terminal, worktrees, extra
  setup providers and skill command names. A key the local file already sets is
  never overwritten.
- **Given** the seed ran, **then** a launch on that project resolves the same
  provider, command line, model, terminal, worktrees and setup providers as it
  did before the upgrade.
- **Given** the seed ran once for the workstation defaults or for a project,
  **then** it never runs again for them, even if the server values change
  later or the local value is removed afterwards.
- **Given** a local file written in the layout that predates this release
  (flat keys and per-project maps), **then** it is read as before, and the
  next save rewrites it in the new layout with the same meaning. No manual step
  is needed.
- **Given** the server is unreachable at the first connection, **then** the
  agent does not mark anything seeded and retries at the next connection.
- **Given** the server has no stored value for a setting, **then** nothing is
  written for it and the provider default applies.
- **Given** a workstation paired to a different server later, **then** the
  workstation defaults are not seeded again; projects of the new server are
  seeded the first time each is resolved.

### US7 - `get_project_context` stops claiming what it cannot know (P2)

**Acceptance**

- **Given** `get_project_context`, **then** it no longer returns
  `useWorktrees`, `aiProvider` or `aiModel`; every other field is unchanged.
- **Given** the skill references it returns, **then** each command is the
  stage's standard command; a workstation's own command name is not the
  server's to know.

### US8 - A refused headless chain says why (P2)

As a user starting a full chain, I want a workstation that cannot run
headless to stop the chain with a reason I can act on.

**Acceptance**

- **Given** a full chain started on a project whose workstation provider has
  no headless mode (and no command template carrying a mode placeholder),
  **when** its first step reaches the agent, **then** the agent refuses it
  against its local provider, the step's run ends as failed, its summary holds
  the agent's reason verbatim (which names the provider and what to change),
  followed by the note that the full chain stops there, and no further step is
  queued.
- **Given** the same refusal, **then** the launch activity is failed with the
  same reason, and the board shows the task as not running.
- **Given** a chain whose workstation can run headless, **then** nothing
  changes.
- **Given** a chain, **then** the server never refuses it up front on a
  provider value.

### US9 - `ttyMode` is gone (P3)

**Acceptance**

- **Given** the upgraded database, **then** the projects table has no
  `tty_mode` column, on SQLite and on PostgreSQL, and every project row
  survived the migration.
- **Given** any API answer or request, **then** `ttyMode` no longer appears
  and is ignored when sent.

## Functional requirements

- **FR-1** The agent resolves every execution setting from the local file
  only, project section over workstation defaults over provider defaults. It
  never uses a server value for execution, except through the seed.
- **FR-2** The resolution rules that exist today are kept: a provider change
  without its own command drops the inherited command, the most specific model
  statement wins, a one-off launch model outranks all, parallelism is 1 to 10
  and 1 without worktrees, an explicit `--terminal` outranks all terminals.
- **FR-3** No server endpoint, MCP tool or background job writes an execution
  setting. Requests carrying one are applied without it.
- **FR-4** The web offers no editor for an execution setting.
- **FR-5** The desktop edits every execution setting at both levels, through
  the agent, which is the only writer of the local file's execution sections.
- **FR-6** The agent reports its capability per project on connection and
  after each local save; the server keeps the latest report per account,
  workstation and project, and serves the one of the workstation connected for
  that account and project.
- **FR-7** The run's recorded engine before the agent's own engine report comes
  from the capability report, else it is unknown.
- **FR-8** Each workstation is seeded once for its defaults and once per
  project, from the server values, only into keys it does not set, and only
  where the value changes the outcome of the local resolution.
- **FR-9** The legacy local layout is read with the same meaning and rewritten
  in the new layout on the next save.
- **FR-10** `get_project_context` serves no `useWorktrees`, `aiProvider` or
  `aiModel`, and standard skill commands.
- **FR-11** A headless refusal on the agent ends a chain with the agent's
  reason and the chain-stop note, and queues nothing more.
- **FR-12** The server columns holding execution settings stay in the schema,
  read-only, and are read only to answer the seed; `projects.tty_mode` is
  dropped.
- **FR-13** An agent that predates this release keeps connecting and running
  launches on the new server; it receives no execution settings from it and
  reports no capability (ADR 0006 still asks to upgrade both together).

## Documentation acceptance

- A new ADR records "the workstation owns the invocation", the seed, the
  capability report, and supersedes the sentence of ADR 0015 saying the AI
  configuration keeps reading the deployment row.
- `docs/contracts/server-agent-v1.md` describes the configuration without the
  execution fields, the seed endpoint, the capability report, the new local
  file layout and the new precedence.
- `desktop/README.md` and `README.md` describe where execution settings are
  set, if they mention it today.
- `CHANGELOG.md` `[Unreleased]` gets:
  - under **Changed**: execution settings (provider, models, commands,
    terminal, editor, worktrees, parallelism, setup providers, skill command
    names) are set per workstation in the desktop app and no longer in the web
    interface; an existing workstation takes over the values the server held,
    once, on its first connection; the web model picker and engine badge show
    what the connected workstation will run;
  - under **Removed**: the web editors for those settings, and the unused
    project "TTY mode".

## Open requirements

None. Every product question was settled in the clarification. The technical
choices made on the way (file layout, endpoint shapes, seed trigger) are in
`plan.md` and are reversible.
