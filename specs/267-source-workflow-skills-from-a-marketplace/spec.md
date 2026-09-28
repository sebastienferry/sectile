# #267: Sectile distributed as a Claude plugin, the direct setup made optional

This file replaces the round-3 specification ("a project pins a third-party pack whose
bodies replace Sectile's built-in ones"). That feature left the ticket in round 4 of the
clarification; its specification and its code stay in the history of `feat/267`
(last at `a9406ffa`) as material for a possible separate ticket. The decisions applied here
are the section "Settled design (round 4)" of `docs/clarifications/267.md`. Behaviour only;
the technical choices are in `plan.md`.

## Context

Today the local agent installs Sectile into the AI CLI by itself, every time: each task
dispatch, each macro dispatch, each agent start on a single project, and each save in the
skills editor writes the workflow skills into the CLI's user-level folder
(`~/.claude/skills`, `~/.agents/skills`, `~/.gemini/config/skills`) and writes the
`sectile` MCP registration (`~/.claude.json`, `~/.codex/config.toml`, ...). A write that
fails aborts the dispatch. Because the folders are user-level, two projects that edit the
same skill overwrite each other's copy: the last project dispatched wins.

Claude Code has its own distribution channel for exactly this: a plugin, installed from a
marketplace, carrying skills and MCP server declarations.

## What changes

1. Sectile's workflow skills and its MCP server declaration are published as a **Claude
   plugin** named `sectile`, generic across projects: a skill reads the project specifics
   (specification framework, pull-request creation stage) from `get_project_context` at run
   time.
2. The agent **never writes** skills or an MCP registration on its own any more. The direct
   setup stays available, **on request only**, from the agent's `init` command and from the
   desktop. It stays the only route for codex, agy, gemini, cursor and vibe.
3. At dispatch the agent **uses what it finds**, without comparing versions, checksums or
   dates:
   1. the project's **custom skill** held by the server, when the workstation setting
      "custom project skills win" is on (default **on**) and the project has one;
   2. otherwise the **installed skill** from the source preferred by the workstation setting
      "installed skills source" (default **direct copy**), falling back to the other one;
   3. otherwise the dispatch fails, says what is missing and how to install it, and writes
      nothing.
4. Signals are **passive**: when a custom skill was used, the desktop shows a warning icon
   next to its settings entry and a "Custom skills used" notice inside the settings. Nothing
   interrupts, prompts or nags.

Vocabulary: a **direct copy** is a skill the agent's direct setup wrote into the CLI's
user-level folder, run as `/<dir>` (for example `/clarify-issue`); a **plugin skill** is the
same skill installed through the Claude plugin, run as `/sectile:<dir>`; a **custom skill**
is a skill the project edited in the skills editor, stored on the server.

## User stories

### US1: Install Sectile in Claude as a plugin (P1)

As a Claude Code user, I want to install Sectile with Claude's own plugin commands, so that
Sectile's skills and its MCP server arrive and update the way my other plugins do.

- **Given** the Sectile marketplace is added to Claude (`claude plugin marketplace add ...`)
- **When** I install the `sectile` plugin
- **Then** Claude asks me for the Sectile server URL and my workstation API key, stores the
  key in its secure storage rather than in a plain file, and afterwards lists the workflow
  skills as `/sectile:clarify-issue`, `/sectile:specify-issue`, ... and the `sectile` MCP
  server as connected to that URL with that key.
- **Given** the plugin is installed and I open a session on any project of that server
- **When** I run `/sectile:specify-issue <task>`
- **Then** the skill reads the project's specification framework and pull-request creation
  stage from `get_project_context` and follows them, with the same steps and contracts as
  the skill the direct setup would have written for that project.
- **Given** two projects using different specification frameworks
- **When** each runs the same plugin skill
- **Then** each follows its own framework; one installation serves both.

### US2: The agent no longer writes behind my back (P1)

As a workstation user, I want the agent to leave my CLI configuration alone unless I ask,
so that what I installed, including the plugin, is what runs.

- **Given** any provider, and the agent started on one project or on all of them
- **When** the agent starts, dispatches a task step, dispatches a macro step, or a project is
  re-synchronised after a reconnection
- **Then** no file in the CLI's skill folders, no MCP registration, no Claude hook setting
  and no Sectile manifest is created, rewritten or removed.
- **Given** a project member saves, resets or imports a skill in the skills editor
- **When** the save completes
- **Then** the custom skill is stored on the server and nothing is written on any
  workstation; the next dispatch of that skill picks it up (US4).
- **Given** a workstation where writing an MCP registration would fail (read-only file,
  ambiguous legacy entry)
- **When** a task is dispatched
- **Then** the dispatch is not aborted by it, since nothing is written.

### US3: Set up a CLI directly, when I ask (P1)

As a user of codex, agy, gemini, cursor or vibe, or a Claude user who prefers not to use the
plugin, I want to install the skills and the MCP registration myself in one action.

- **Given** the agent's `init` command, run with a provider
- **When** it completes
- **Then** it writes that provider's skills and MCP registration exactly as today, and
  reports each step as succeeded, failed, skipped or not run.
- **Given** the desktop project settings, Deployment tab
- **When** I choose a provider and press "Initialize"
- **Then** the same happens for that provider, and the result is shown step by step as
  today.
- **Given** the desktop and the agent's `init`
- **When** I read the text around the initialize action
- **Then** it says the step is optional and that Claude users can install the `sectile`
  plugin instead.

### US4: A project's custom skill runs by default (P1)

As the owner of a project that edited a skill, I want that edit to run on every
workstation, without anybody reinstalling anything.

- **Given** the workstation setting "custom project skills win" is on (the default) and the
  project has a custom version of the skill being dispatched
- **When** the skill is dispatched, as a task step or a macro step, interactive or headless
- **Then** the CLI runs the project's custom version as stored on the server at dispatch
  time, whatever is installed as a direct copy or as a plugin skill, and nothing is written
  into the CLI's user-level folders.
- **Given** two projects with different custom versions of the same skill
- **When** each is dispatched on the same workstation, one after the other or at the same
  time
- **Then** each runs its own version.
- **Given** the setting is off
- **When** a skill the project customised is dispatched
- **Then** the installed skill runs (US5), as if the project had no custom version.
- **Given** a skill the project did not customise
- **When** it is dispatched
- **Then** the installed skill runs (US5), whatever the setting.

### US5: Choose which installed source runs (P2)

As a Claude user who has both old direct copies and the plugin, I want to decide which one
runs.

- **Given** the workstation setting "installed skills source" is "direct copy" (the default)
- **When** a skill without an applicable custom version is dispatched to Claude
- **Then** `/<dir>` runs when the direct copy is installed, otherwise `/sectile:<dir>` runs
  when the plugin is installed and enabled.
- **Given** the setting is "Claude plugin"
- **When** the same dispatch happens
- **Then** `/sectile:<dir>` runs when the plugin is installed and enabled, otherwise `/<dir>`
  runs when the direct copy is installed.
- **Given** the plugin is installed only for another project folder (Claude "project"
  scope), or is disabled
- **When** a dispatch looks for it
- **Then** it is treated as not installed for this dispatch.
- **Given** a provider other than Claude
- **When** a skill is dispatched
- **Then** the setting has no effect: only the direct copy exists for that provider.
- **Given** a project section that sets an explicit slash command for a skill
  ("skill commands" in the project settings)
- **When** that skill is dispatched without an applicable custom version
- **Then** that command runs as written, including a namespaced one such as
  `/sectile:clarify-issue`, without looking for either source.

### US6: Nothing found is said, not repaired (P1)

As a workstation user, I want a dispatch that has no skill to run to fail with a message
that tells me what to do.

- **Given** a provider that has a skill folder (claude, codex, agy), no applicable custom
  version, no direct copy and, for Claude, no installed and enabled plugin
- **When** the skill is dispatched
- **Then** the dispatch fails before any CLI is launched, the run is recorded as failed with
  a message naming the skill and the provider and pointing at the three ways out: install
  the `sectile` plugin (Claude only), run the agent's `init` with that provider, or press
  "Initialize" in the desktop project settings; and nothing is written.
- **Given** a provider without a skill folder (gemini, cursor, vibe)
- **When** a skill without an applicable custom version is dispatched
- **Then** the dispatch behaves as today: the slash command is sent, no check is made.

### US7: Know when custom skills are in use (P2)

As a workstation user, I want to see, without being interrupted, that a project's custom
skill ran instead of the installed one.

- **Given** a dispatch on this workstation ran a project's custom skill since the agent
  started
- **When** I look at the desktop
- **Then** a warning icon sits next to the settings entry, and the settings show a "Custom
  skills used" notice listing the project, the skill and when it last ran, next to the
  "custom project skills win" setting.
- **Given** a plugin update or a new direct copy is installed while a custom skill exists
- **When** the next dispatch happens
- **Then** the custom skill keeps running, with no prompt, dialog or blocking notice.
- **Given** the setting is turned off
- **When** the next dispatches run
- **Then** the notice keeps what was used before, and no new entry is added.
- **Given** a dispatch ran a custom skill
- **When** I read the run's activity steps
- **Then** one step says the project's custom skill was used.

## Functional requirements

- **FR1** A generator in the sectile repository produces the Claude plugin from the built-in
  catalogue: a manifest named `sectile` with a version given at generation time, one
  `skills/<dir>/SKILL.md` per workflow skill, and an MCP declaration for a server named
  `sectile`. The same catalogue revision and version always produce byte-identical output.
- **FR2** Plugin skills carry no project-specific value: where the direct setup embeds the
  framework variant or the pull-request creation stage, the plugin skill carries every
  variant and tells the agent to read the project's value from `get_project_context`.
- **FR3** The plugin's MCP declaration takes the server URL and the workstation API key from
  values the user supplies at install time; the key is declared sensitive. No URL, key,
  host name or internal project name is present in the generated files.
- **FR4** No agent start, task dispatch, macro dispatch, project re-synchronisation or
  skills-editor save writes, rewrites or removes a skill file, an MCP registration, a Claude
  hook setting or a Sectile manifest. The agent's `init`, the desktop "Initialize" action,
  the desktop MCP connection settings and an explicit install request are the only writers.
- **FR5** Each dispatched skill is resolved in the order: custom skill (when the setting is
  on and one exists) → preferred installed source → other installed source → failure.
- **FR6** A custom skill reaches the CLI through the dispatch itself and is private to that
  run: another run, another project or the CLI's user-level folders never see it.
- **FR7** Two workstation settings, both in the desktop "Execution defaults": "custom
  project skills win" (on/off, default on) and "installed skills source" (direct copy /
  Claude plugin, default direct copy). An agent upgraded without either setting behaves as
  if they held their defaults.
- **FR8** No version, checksum or timestamp comparison is made between installed skills and
  the server's skills, and none is shown.
- **FR9** When nothing is found, the dispatch fails with the message of US6 and writes
  nothing.
- **FR10** The agent keeps, per project and skill, when a custom skill last ran on this
  workstation since it started, and the desktop shows it as described in US7.
- **FR11** A namespaced command (`<plugin>:<dir>`) is accepted wherever a workstation user
  can set a skill command.
- **FR12** The user-facing messages added on the agent's run path (activity step, failure)
  are in French, like the surfaces they appear on; the desktop strings are in English, like
  the rest of the desktop.

## Edge cases

- A Claude user who installs the plugin and keeps an older `sectile` entry in `~/.claude.json`
  gets two MCP servers pointing at Sectile. Nothing removes the older one automatically; the
  "Initialize" help text says so.
- A user who installs the plugin while old direct copies remain keeps running the direct
  copies until they switch "installed skills source" to "Claude plugin". This trade-off was
  accepted in the clarification.
- A custom skill saved while a run of the same skill is in progress does not change that
  run; the next dispatch uses it.
- A custom skill that is empty on the server (only a mode is stored) is not a custom skill.
- A plugin older than the server keeps being used as it is; no compatibility check is made
  (FR8).

## Out of scope

- Third-party packs replacing built-in skill bodies (round 3, PR #350).
- How the separate marketplace repository is created, versioned, fed and published, and who
  may publish to it (Q11). This ticket produces the plugin directory and nothing more.
- Any reconciliation between installed and server-side skills.
- Plugins for other CLIs than Claude.
- Removing existing direct copies or MCP registrations from workstations.
- Adding workflow steps.

## Open points

None blocks the specification. Two need the owner's hand, not a decision:

- The owner closes PR #350 (said in round 4) before a pull request for this design is
  opened on `feat/267`.
- The owner rewrites the ticket description and title (said in round 4); the specification
  folder keeps its current name.
