# Specification #510 - Several AI engines, switched per task from the desktop task list

- Ticket: https://github.com/sebastienferry/sectile/issues/510
- Branch: `feat/510`
- Clarification: `docs/clarifications/510.md` (rounds 1 and 2, owner
  confirmed on 2026-09-26, no open product question)
- Amends: #305 (`specs/305-execution-settings-on-the-workstation/`, ADR 0031)
- Framework: Spec Kit

This file describes behaviour only. How it is built is in `plan.md`, the
ordered work in `tasks.md`.

## Summary

A workstation keeps a **catalogue of named engines**. An engine is a full
profile: a provider, a model, per-skill models, an interactive command
template and a headless command template. One engine of the catalogue is the
**workstation default engine**. A project no longer describes an engine: it
may pick its **project default engine** from the catalogue, and otherwise uses
the workstation default engine.

In the desktop ticket table, every task shows the icon of the engine its next
run uses. Clicking the icon moves the task to the next engine of the
catalogue. The choice stays with the task until the next click, and every
launch of that task that runs on this workstation uses it, whoever or
whatever started the launch.

The engine settings shipped by #305 are converted into catalogue entries once,
automatically, so nobody configures anything again after the upgrade.

## Scope

In scope:

- the engine catalogue and the workstation default engine, edited in the
  desktop workstation settings;
- the project default engine, picked in the desktop project settings, which
  lose their provider, model, per-skill model and template fields;
- the per-task engine, shown and switched in the desktop ticket table;
- applying the task's engine to every launch of that task on this
  workstation: desktop, web, relaunch, each stage of a full chain, queued and
  parallel runs;
- the one-time conversion of the #305 engine settings into the catalogue;
- the server seed (#305 US6) writing into the catalogue;
- skills and Sectile MCP registration for the provider of every catalogue
  engine;
- the capability report describing the project default engine.

Out of scope:

- the web task card and any web editor: the web shows no per-task engine and
  offers no switch;
- a server-side store of engines or of per-task choices, and any new field on
  the server dispatch;
- sharing engines across a team (#493);
- choosing an engine from the Launch, Relaunch or Custom instructions dialogs;
- the free console's own provider choice (Codex or Claude), which stays as it
  is;
- macro-scoped skills (`refine-macro`, `realign-macro`), which have no task and
  run on the project default engine.

## Definitions

- **Engine**: a named, full invocation profile stored on the workstation:
  identifier, name, provider, model, per-skill models, interactive template,
  headless template. An empty model or template means "the provider's own
  default", as today.
- **Catalogue**: the ordered list of the workstation's engines. The order is
  the one the owner gives it in the workstation settings.
- **Workstation default engine**: the catalogue entry marked as default.
  There is always exactly one.
- **Project default engine**: the engine a project picks from the catalogue,
  else the workstation default engine.
- **Task engine**: the engine a task was switched to in the ticket table,
  else the project default engine.
- **Engine settings of #305**: `aiProvider`, `aiCommandTemplate`,
  `aiCommandTemplateAutonomous`, `aiModel` and `aiSkillModels`, at the
  workstation level and in project sections.

Settings that do not describe an engine keep their #305 levels unchanged:
worktrees, parallelism, terminal and extra setup providers (workstation
default, project override), skill command names (per project), the model
list per provider and the editor (workstation).

## User stories (prioritised)

### US1 - The workstation holds a catalogue of engines (P1)

As a developer, I describe each AI CLI I use once, on my workstation, and give
it a name, so that any project and any task can use it.

**Acceptance**

1. **Given** the desktop workstation settings, **when** I open them, **then**
   an Engines section lists the catalogue in its order, each entry showing its
   name, provider icon, provider and model (or "provider default"), and the
   workstation default engine is marked.
2. **Given** the Engines section, **when** I add an engine, **then** I choose
   its name, provider, model, per-skill models, interactive template and
   headless template, and it is appended to the catalogue after saving.
3. **Given** an engine, **when** I edit it, **then** every field of its
   profile can change, including its provider, and its identity is kept:
   projects and tasks pointing at it keep pointing at it.
4. **Given** two engines with the same provider (for example Claude with Opus
   and Claude with Sonnet), **when** I save, **then** both are accepted: names
   tell them apart.
5. **Given** two engines, **when** I give them the same name (ignoring case
   and surrounding spaces), **then** the save is refused with a message naming
   the duplicate.
6. **Given** the catalogue, **when** I move an engine up or down, **then** the
   new order is saved and is the cycling order of the ticket table.
7. **Given** a non-default engine, **when** I mark it as default, **then** it
   becomes the workstation default engine and the previous one stays in the
   catalogue.
8. **Given** the workstation default engine, **when** I try to remove it,
   **then** the removal is not offered until another engine is marked as
   default; the last remaining engine can never be removed.
9. **Given** an engine used as a project default engine or as a task engine,
   **when** I remove it, **then** the confirmation says so, and after removal
   those projects use the workstation default engine and those tasks use their
   project default engine.
10. **Given** an engine with the custom provider, **when** its interactive
    template is empty or lacks `{prompt}`, **then** the save is refused, as a
    custom provider is refused today.
11. **Given** an invalid model, per-skill model or template (the rules of
    #305), **when** I save, **then** the save is refused with a message naming
    the engine and the field.

### US2 - A project picks its default engine (P1)

As a developer, I choose which engine a project uses by default without
retyping a profile.

**Acceptance**

1. **Given** the desktop project settings, **when** I open them, **then** the
   provider, model, per-skill model and command template fields are gone,
   replaced by a single "Default engine" selector.
2. **Given** that selector, **then** it offers "Inherit the workstation
   default (<name>)" followed by every catalogue engine by name, in catalogue
   order.
3. **Given** a project on "inherit", **when** the workstation default engine
   changes, **then** the project follows it.
4. **Given** a project that picked an engine, **when** that engine is edited,
   **then** the project runs the edited profile.
5. **Given** the other project settings (worktrees, parallelism, terminal,
   setup providers, skill command names, checkout and specifications folders),
   **then** they are unchanged by this feature.

### US3 - The ticket table shows and switches a task's engine (P1)

As a developer, I see which engine each task will run with, and hand a task
to another engine with one click.

**Acceptance**

1. **Given** the desktop ticket table of a project, **then** each row shows
   the icon of its task engine, in a dedicated column.
2. **Given** a task never switched, **then** its icon is the project default
   engine's.
3. **Given** the icon, **when** I hover or focus it, **then** a tooltip gives
   the engine's name, provider and model ("provider default" when the engine
   sets none), and says when it is the project default engine.
4. **Given** a task whose engine is not the project default engine, **then**
   its icon is visibly highlighted.
5. **Given** a catalogue of several engines, **when** I click the icon,
   **then** the task moves to the engine that follows its current engine in
   catalogue order, the last one wrapping to the first; the icon and tooltip
   update.
6. **Given** a catalogue of a single engine, **then** the icon is shown and a
   click changes nothing.
7. **Given** a switched task, **when** I restart the desktop app, the agent or
   the workstation, **then** the task still shows the engine it was switched
   to.
8. **Given** a switched task, **when** one of its runs ends, **then** the task
   keeps its engine: nothing resets it.
9. **Given** a task with a run in progress, **when** I click the icon, **then**
   the running execution is not affected; the new engine applies from the next
   launch.
10. **Given** the icon, **then** it is a button reachable with the keyboard,
    activated with Enter or Space, with an accessible name giving the task key
    and the engine name.
11. **Given** a click the agent refuses (for example the engine was removed
    meanwhile), **then** the row shows the error and its icon reverts to what
    the agent stores.
12. **Given** a desktop connected to an agent that does not announce the
    per-task engine capability, **then** the column is not shown.

### US4 - Every launch of the task runs its engine (P1)

As a developer, I trust that the engine the icon shows is the one that runs,
whatever started the launch.

**Acceptance**

1. **Given** a task switched to engine E, **when** a run of that task is
   dispatched to this workstation from the desktop, from the web, by a
   relaunch, or as any stage of a full chain, **then** it runs with E's
   provider, model (per-skill model for the stage when E sets one) and the
   template matching its mode (interactive or headless).
2. **Given** such a run, **then** the provider and model recorded on the run
   are E's, as the run history already shows the engine a run really used.
3. **Given** several runs of the same project queued or running in parallel,
   **then** each uses the engine of its own task.
4. **Given** a task on its project default engine, **when** a one-off model is
   chosen for a launch (web card), **then** that model outranks the engine's
   model for that run, as today.
5. **Given** a task on an engine other than its project default engine,
   **when** a one-off model is chosen for a launch, **then** the one-off model
   is ignored and the run uses E's model.
6. **Given** a headless launch on an engine that has no headless mode, **then**
   the launch is refused with the existing message, naming the engine.
7. **Given** a task whose engine was removed from the catalogue, **then** its
   runs use the project default engine.
8. **Given** a run on another workstation, **then** it follows that
   workstation's own settings: per-task engines are not shared.
9. **Given** a discussion or a bare terminal opened on a task, **then** it
   opens the task engine's provider.

### US5 - Every catalogue provider is ready (P1)

As a developer, I never switch a task to a CLI that lacks Sectile's skills or
MCP registration.

**Acceptance**

1. **Given** a catalogue whose engines use providers A and B, **when** a
   project is set up or refreshed on this workstation, **then** the skills and
   the Sectile MCP registration are installed for A and for B, on top of the
   configured extra setup providers.
2. **Given** an engine added with a new provider, **when** a task switched to
   it is launched for the first time, **then** that provider's skills and MCP
   registration are in place before its CLI starts.
3. **Given** a custom-provider engine, **then** nothing is installed for it,
   as today.

### US6 - An existing setup keeps working after the upgrade (P1)

As a developer upgrading from #305, I do not configure anything again.

**Acceptance**

1. **Given** a settings file with engine settings only at the workstation
   level, **when** the new agent starts, **then** the catalogue holds one
   engine with exactly that profile, marked as workstation default, and every
   project runs what it ran before.
2. **Given** a project section with its own provider, **then** the project
   gets a catalogue engine equal to what it resolved before, and that engine
   is its project default engine.
3. **Given** a project section that changed the provider without templates of
   its own (so the workstation templates were dropped for it), **then** its
   engine carries no template, and it runs the provider defaults as before.
4. **Given** a project section stating only a model or per-skill models,
   **then** its engine carries the workstation provider and templates with the
   project's models, as it resolved before.
5. **Given** several projects that resolved identical profiles, or a project
   identical to the workstation level, **then** they share one catalogue
   entry.
6. **Given** a settings file with no engine setting at all, **then** the
   catalogue holds one engine for the default provider, marked default.
7. **Given** a conversion, **then** a backup of the previous file is written
   next to it before the file is changed, and the engine settings of #305
   disappear from the file.
8. **Given** a converted file, **when** the agent starts again, **then**
   nothing changes: the conversion runs once.
9. **Given** a converted file later saved by an agent that predates this
   change, **then** the catalogue, the project default engines and the task
   engines survive that save; engine settings that older agent wrote are
   converted again on the next start, reusing identical entries.
10. **Given** a new workstation seeded from a server after this change,
    **then** the server's stored values become catalogue entries (a
    workstation default engine, and project default engines where a project
    resolved differently), never engine settings of #305.
11. **Given** a converted workstation, **then** the capability report, the run
    history and the web launch model list describe the same engine per
    project as before the upgrade.

### US7 - The server learns the project default engine (P2)

As a web user, I keep seeing what a project runs on this workstation.

**Acceptance**

1. **Given** a project, **then** the capability report sent to the server
   describes its project default engine: provider, model, per-skill models,
   model list, model slot and headless ability, as it described the single
   project engine before.
2. **Given** the catalogue, a project default engine or the workstation
   default engine changes, **then** a new capability report is sent.
3. **Given** a task switched to another engine, **then** the report is not
   changed by it; the run record is corrected by the engine the agent reports
   when the run starts.

## Functional requirements

- **FR-1** The workstation file holds the catalogue, the workstation default
  engine, the project default engine per project and the task engine per task.
  Nothing of it is sent to the server beyond the capability report.
- **FR-2** Engine identifiers are generated by the agent, stable across edits
  and renames, and never reused for another engine.
- **FR-3** An engine name is required, at most 64 characters, and unique in
  the catalogue ignoring case and surrounding spaces. The catalogue holds at
  least one engine and at most 20.
- **FR-4** An engine is validated with the rules #305 applies to a level:
  supported provider, template length, `{prompt}` for the custom provider,
  model and per-skill model shape.
- **FR-5** Resolution for a task: the task engine if it exists in the
  catalogue, else the project default engine if it exists, else the
  workstation default engine. The resolved engine's profile gives the
  provider, the templates (empty meaning the provider's own command) and the
  models (a per-skill model over the engine model). Nothing is inherited from
  another engine.
- **FR-6** Resolution without a task (project context, capability report,
  macro skills, free console defaults, project refresh): the project default
  engine.
- **FR-7** A one-off launch model applies only when the resolved task engine
  is the project default engine; otherwise it is ignored.
- **FR-8** The ticket table cycles through the whole catalogue in catalogue
  order from the task's current engine, wrapping around. A choice is stored by
  engine identifier; a stored choice naming a removed engine reads as no
  choice and is dropped on the next write.
- **FR-9** Removing an engine drops every project default engine and task
  engine pointing at it. Removing the workstation default engine is refused.
- **FR-10** Skills and MCP registration are installed for the union of the
  selected provider, the configured extra setup providers and the providers of
  every catalogue engine.
- **FR-11** The conversion of the #305 engine settings runs whenever the file
  holds any of them, is idempotent, collapses identical profiles, keeps each
  project's resolution unchanged, writes a backup before the first change, and
  is performed under the settings lock.
- **FR-12** The agent announces a `task-engines` capability on
  `/desktop/status`; the desktop shows the engine column and the engine
  editors only when it is announced.
- **FR-13** The engine settings of #305 are no longer accepted from the
  desktop saves and no longer written to the file.

## Documentation acceptance

- `CHANGELOG.md` gains one `Added` line under `[Unreleased]` for the engine
  catalogue and the per-task switch, and one `Changed` line saying the project
  settings pick a default engine instead of describing one.
- `docs/contracts/server-agent-v1.md` describes the new layout of
  `~/.config/sectile/settings.json` and the new local desktop endpoints.
- `docs/ARCHITECTURE.md` mentions the catalogue where it describes the local
  settings file.
- An ADR records the catalogue model and what it amends in ADR 0031.

## Open requirements

None blocks the implementation. The clarification closed every product
question. Two points are left to the implementation, with a default in
`plan.md`, and do not change any acceptance criterion above:

- the exact letters or glyph drawn for each provider icon (the plan gives a
  default set of monograms; no third-party logo is used);
- the name given to an engine created by the conversion (the plan gives a
  default rule; the owner can rename it afterwards).
