# #267 — Source the workflow skills from a marketplace

## Context

The ten workflow skills are a catalogue compiled into the binary: `StageSkills`
(`internal/db/skilltemplates.go:52`) holds both their metadata and their prose, and
`RenderSkillContent` assembles a `SKILL.md` out of it. A project may already edit any of
them — `project_skills` stores the edited body, `EffectiveProjectSkills`
(`internal/db/projectskills.go:139`) resolves built-in template → project edit, and the
result is installed into the agent conventions.

Nothing today lets a team share a set of skill bodies. Improving the prompts means either
patching Sectile or re-typing the same edit in every project.

This ticket adds a third source underneath the project's own edits: a **Claude plugin
marketplace**, read directly by Sectile. The clarification (`docs/clarifications/267.md`)
settled the format, what a pack may contain, where it is configured and when it becomes
effective; this file describes the resulting behaviour, and `plan.md` the technical
choices.

## Decision being specified

A deployment registers marketplaces. A project picks **one plugin** from one of them, sees
the diff, and applies it. From then on the pack bodies are the project's baseline, the
project's own edits still win over them, and Sectile keeps generating the contracts that
attach an agent to the workflow protocol. Nothing ever re-resolves on its own.

Vocabulary, shared with #106: a **marketplace** is the repository, a **plugin** is the unit
a project selects, a **pack** is that plugin's skill bodies as resolved at one commit, and
**applying** is the explicit action that makes a pack the project's baseline.

## User stories

### US1 — Register a marketplace once for the deployment (P1)

As the person who administers a Sectile deployment, I want to register the marketplaces my
team uses once, so that every project can pick from them without re-typing a URL.

- **Given** I administer the deployment
- **When** I register a marketplace by GitHub repository (`owner/repo`), by git URL, or by
  local directory path
- **Then** it appears in the registry with its name, its owner and the date it was last
  fetched, and every project of the deployment can select from it.
- **Given** a locator that does not resolve, or a repository without
  `.claude-plugin/marketplace.json` at its root
- **When** I try to register it
- **Then** registration is refused with the reason, and nothing is stored.
- **Given** I am a member and not an administrator
- **When** I try to add, edit or remove a marketplace
- **Then** the call is refused the way a deployment setting is refused today, and the
  registry stays readable.
- **Given** a registered marketplace
- **When** I remove it while a project still pins one of its plugins
- **Then** I am told which projects pin it; removal keeps those projects working on the
  bodies they already applied, and their pin is reported as orphaned rather than silently
  reverted.

### US2 — Pick a plugin for a project (P1)

As the owner of a project, I want to browse a registered marketplace and choose the plugin
whose skills my project should run, so that my workflow uses my team's prompts.

- **Given** a registered marketplace
- **When** I open the skills screen of a project and choose that marketplace
- **Then** I see its plugins with their name, description and version, and for each the
  workflow skills it supplies.
- **Given** a plugin I selected
- **When** Sectile resolves it
- **Then** it reads `skills/<name>/SKILL.md` under the plugin directory — the path named by
  `"skills"` in `plugin.json` when present, `./skills/` otherwise — and matches each skill
  directory name against the ten workflow directories: `clarify-issue`, `specify-issue`,
  `code-issue`, `adjust-issue`, `handoff-issue`, `create-pr`, `pickup-issue`,
  `pickup-issues`, `rewrite-story`, `refine-macro`.
- **Given** the plugin also ships skills, commands, agents or hooks that are not among the
  ten
- **When** the pack is resolved
- **Then** each one is **listed as ignored with its directory name**, and none of them is
  installed anywhere.
- **Given** a plugin that supplies none of the ten
- **When** I try to resolve it
- **Then** it is an error naming what was found, not an empty success, and nothing is
  applied.

### US3 — See the diff before anything becomes effective (P1)

As the owner of a project, I want to read what a pack would change before it changes
anything, so that I never run a prompt I have not seen.

- **Given** a plugin I selected
- **When** Sectile has resolved it
- **Then** I see, per workflow skill, the current resolved content against what the pack
  would make it, plus the list of ignored directories and of skills the pack does not
  supply — and **nothing is written yet**: the project's skills, its files on disk and its
  runs are untouched.
- **Given** that preview
- **When** I close it without applying
- **Then** the project is exactly as it was, including the fetch having left no pin behind.
- **Given** that preview
- **When** I apply it
- **Then** the pack bodies become the project's baseline, the pin records the marketplace,
  the plugin, the plugin `version` and the **resolved commit SHA**, and the skills are
  re-installed into the agent conventions exactly as an edit does today.

### US4 — My own edits still win (P1)

As someone who has hand-edited a skill for my project, I want applying a pack never to
destroy that edit, so that adopting a team pack is not a choice between two customisations.

- **Given** a skill I edited for this project
- **When** a pack that also supplies that skill is applied
- **Then** my edited body stays effective, and the pack body becomes the baseline behind it.
- **Given** that same skill
- **When** I reset it
- **Then** it falls back to the **pack** body, not to the built-in one, and only falls back
  to the built-in one when no pack supplies it or no pack is pinned.
- **Given** a skill the pack supplies and I never edited
- **When** the pack is applied
- **Then** the skill is no longer reported as custom: it matches what this project would
  otherwise get, and its origin is shown as the marketplace with the plugin name and
  version.

### US5 — The protocol survives any pack (P1)

As the person responsible for a board, I want a third-party body never to be able to detach
an agent from the workflow protocol, so that a pack cannot leave tickets stuck.

- **Given** any pack body, including one that mentions no tool at all
- **When** the skill is rendered
- **Then** the rendered file still carries the frontmatter, the stage line, the task-access
  contract, the session-title contract, the ticket-transition contract and the project's
  pull request policy, all generated by Sectile.
- **Given** a pack body that contains its own frontmatter block
- **When** the skill is rendered
- **Then** that block is stripped and Sectile's frontmatter is the only one in the file.
- **Given** a pack that supplies `pickup-issue` or `pickup-issues`
- **When** those skills are rendered
- **Then** their embedded stage sections are still composed from the project's **resolved**
  `clarify`, `specify`, `implement` and `adjust` bodies, so a batch run cannot lag behind a
  stage the pack updated.
- **Given** any pack
- **When** a run is dispatched
- **Then** the slash command is unchanged — `/clarify-issue`, `/specify-issue`, … — because
  ids, directory names and stages stay Sectile's and a pack carries none of them.

### US6 — A pinned pack is reproducible and works offline (P1)

As someone re-running a ticket a month later, I want the same pack bodies as the first run,
so that a remote edit cannot change my workflow without me knowing.

- **Given** a project with a pinned pack
- **When** the marketplace repository receives new commits
- **Then** nothing changes for the project: the bodies stay as applied and the pinned commit
  stays the pinned commit.
- **Given** a project with a pinned pack and no network
- **When** skills are installed or a run starts
- **Then** everything works from what was already applied; no fetch is attempted.
- **Given** a pinned pack
- **When** I ask for an update
- **Then** Sectile re-resolves the plugin at the marketplace head, shows the diff as in US3,
  and changes nothing until I apply it — the marketplace's own `autoUpdate` flag is read as
  information and never acted upon.

### US7 — Stop using a pack (P2)

As the owner of a project, I want to go back to Sectile's own skills, so that adopting a
pack is reversible.

- **Given** a project with a pinned pack
- **When** I unpin it
- **Then** the baseline is the built-in catalogue again, my own edited skills are untouched,
  and the files are re-installed from the new resolution.
- **Given** a project whose newer pack no longer supplies a skill the previous one did
- **When** that newer pack is applied
- **Then** the skill returns to the built-in body rather than keeping an orphaned pack body.

### US8 — Framework variants, and a fetch that fails (P2)

As the owner of an OpenSpec project, I want a pack written for Spec Kit not to silently
break my specification step, so that the framework choice stays mine.

- **Given** a pack supplying a single `specify-issue` (or `refine-macro`)
- **When** it is applied to a project on either framework
- **Then** that one body is used for both, and the preview says so.
- **Given** a pack supplying neither
- **When** it is applied
- **Then** the built-in framework-specific body is kept for that skill.
- **Given** a project whose local agent is offline
- **When** I try to resolve, preview or apply a pack
- **Then** the action fails with the agent error the other agent operations already report,
  and the project keeps the bodies it had.

## Functional requirements

- **FR1 — Registry.** The deployment holds a set of marketplaces, each with a unique name,
  a kind (`github`, `git`, `path`), a locator, an owner, a description, the last resolved
  commit and the date last fetched. Writes are administrator-only; reads are not.
- **FR2 — Format.** Sectile parses `.claude-plugin/marketplace.json`
  (`{name, owner, metadata, plugins[]}`, each plugin `{name, source, description, version}`)
  and, per plugin, `plugin.json` at the plugin root or under its `.claude-plugin/`. It never
  invokes the `claude` CLI, so a project on `codex`, `agy`, `gemini`, `cursor` or `vibe`
  uses a marketplace exactly as a `claude` project does.
- **FR3 — Mapping.** A pack entry is accepted only when its skill directory name equals one
  of the ten workflow directory names. Anything else is reported as ignored. A plugin whose
  `source` escapes the marketplace directory is rejected.
- **FR4 — Validation.** An accepted `SKILL.md` has a parsable frontmatter block, a non-empty
  body after it, and a size within the accepted limit. A failed entry is reported with its
  reason and does not take part in the pack; a plugin with zero accepted entries is an error.
- **FR5 — Pin.** A project pins at most one plugin: marketplace name, plugin name, plugin
  version, resolved commit, applied date, and the list of skills the pack supplied.
- **FR6 — Precedence.** Resolution is built-in default → pack body → project edit. The
  editor's default content is the resolved baseline, so "custom" keeps meaning "differs from
  what this project would otherwise get", and each entry carries its origin (`builtin` or
  `marketplace`) with the pack coordinates.
- **FR7 — Generated contracts.** The frontmatter, the title, the stage line, the task-access
  contract, the session-title contract, the ticket-transition contract and the pull request
  policy are always Sectile's and are never taken from a pack.
- **FR8 — Composition.** `pickup` and `pickup_issues` are composed from resolved bodies.
- **FR9 — Explicit application.** Fetching and previewing write nothing. Applying is a
  distinct action, per project, and triggers the same installation as an edit.
- **FR10 — Agent-side fetch.** All repository access happens on the workstation through the
  local agent, beside `sync_config` and `spec_install`, and each fetch or apply is recorded
  as an activity the way `recordSpecFrameworkActivity` records an installation. The server
  never reaches the network itself.
- **FR11 — Cache.** A fetched marketplace is cached on the workstation and re-used; a pinned
  project reads its cache and never refetches without an explicit action. Removing a
  marketplace from the registry removes its cache.

## Out of scope

- Adding a workflow step: a pack replaces bodies, it cannot create a stage (clarification
  round 3, question 2).
- Publishing Sectile's own skills to a marketplace, and distributing them to the agent's
  global folder — that is #106, which shares this vocabulary.
- Changing the stage graph, the ids, the directory names or the slash commands.
- Changing where rendered skills are installed (#106 / `ResolveLocations`).
- Any use of the `claude` CLI, and any adoption of the format's `autoUpdate`.

## Still open

Nothing blocks implementation. Two items are known and deliberately left outside:

- **Where Sectile publishes its own catalogue, and who may publish to it**, remains a
  product question. Any git repository or local directory is a valid source, and a local
  path is enough to test and to ship this ticket.
- **#239 (round-based clarification)** edits the body of `clarify`, a skill a pack may
  replace. Whichever of the two lands second inherits a textual conflict in the built-in
  catalogue; neither blocks the other.
