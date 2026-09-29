# Specification #614 - Remove the Gemini, Cursor and Vibe AI engines

- Ticket: https://github.com/sebastienferry/sectile/issues/614
- Branch: `feat/614`
- Clarification: `docs/clarifications/614.md` (rounds 1 to 3, confirmed by
  the owner on 2026-09-29)
- Framework: Spec Kit

## Summary

Sectile Desktop still offers Gemini CLI, Cursor CLI and Mistral Vibe CLI as AI
engines in places, although they are no longer engine choices. With this
change, Sectile supports four AI providers: Antigravity (`agy`), Claude Code
(`claude`), Codex (`codex`) and a custom command (`custom`). Gemini (`gemini`),
Cursor (`cursor`) and Vibe (`vibe`) disappear from the desktop, the local
agent, the web app and the documentation. A workstation that stored settings
for them keeps working: the agent drops those settings, and whatever used them
falls back as if the owner had removed them.

## Scope

In scope:

- every place where a person picks, sees or configures an AI provider: the
  desktop settings (engine editor, model lists, initialization provider,
  command suggestions, command preview, MCP provider configuration), the
  web app's command template and provider type, and the agent's command line;
- what the local agent accepts, runs and sets up;
- workstation settings that already name one of the removed providers, and a
  server seed that does;
- the contract document, the README files, `docs/CAPABILITIES.md` and the
  changelog.

Out of scope:

- The Cursor **editor** ("Open in editor", #535): it opens a worktree, it is
  not an AI engine, and it stays.
- Antigravity, which keeps its `~/.gemini/config/...` locations, the `.gemini`
  skill directories that serve it, and the `gemini-*` model names offered for
  it.
- Files that earlier releases wrote on the workstation for the removed
  providers (`~/.gemini/settings.json`, `~/.cursor/mcp.json`,
  `~/.vibe/config.toml`): the agent does not touch a provider's configuration
  on its own (#267), so they stay as they are.
- The cleanup of registration files that earlier releases wrote inside a
  checkout, which keeps removing `.gemini/settings.json`, `.cursor/mcp.json`
  and `.vibe/config.toml` from old checkouts.
- The server's stored records, which the server neither validates nor serves
  as an engine choice.

## User stories

### US1 (P1) - The desktop offers only the supported providers

As a desktop user, I only see the providers Sectile supports wherever the
settings let me choose or configure one.

1. Given the Execution defaults panel, when it opens on a workstation that
   stores no model list, then the model lists section shows one row per
   supported provider that has a shipped list (`agy`, `claude`, `codex`), and
   no `gemini`, `cursor` or `vibe` row.
2. Given the Execution defaults panel, when the initialization provider select
   opens, then it offers exactly `agy`, `claude` and `codex`.
3. Given the engine editor, when the provider select opens, then it offers
   AGY, Claude, Codex and Custom, and no Gemini, Cursor or Vibe.
4. Given the engine editor, when the command field suggests known command
   lines, then none of them starts with `gemini`, `cursor` or `vibe`.
5. Given the MCP provider configuration, when its provider select opens, then
   it offers no Gemini, Cursor or Vibe entry.

### US2 (P1) - Existing settings for a removed provider are dropped

As the owner of a workstation that had a Gemini, Cursor or Vibe engine, the
agent starts, my other settings stay, and what used the removed engine runs
something else without my intervention.

1. Given a catalogue holding a Claude engine and a Gemini engine, with the
   Gemini engine chosen by project P and by task T, when the agent starts,
   then the catalogue holds only the Claude engine, project P runs the
   workstation default engine, and task T runs its project default engine.
2. Given a catalogue whose default engine is a Cursor engine and which holds
   a Codex engine after it, when the agent starts, then the Cursor engine is
   gone and the Codex engine is the workstation default engine.
3. Given a catalogue holding only a Vibe engine, when the agent starts, then
   the catalogue holds the engine a workstation stating nothing runs
   (Antigravity), and it is the workstation default engine.
4. Given stored model lists for `gemini`, `cursor`, `vibe` and `claude`, when
   the agent starts, then only the `claude` list remains.
5. Given stored MCP connection choices for `gemini`, `cursor`, `vibe` and
   `claude`, when the agent starts, then only the `claude` choice remains.
6. Given an initialization provider set to `gemini`, `cursor` or `vibe`, when
   the agent starts, then the initialization provider is unset and reads as
   the default one.
7. Given a settings file written before #510 whose defaults or project section
   name `gemini`, `cursor` or `vibe` as the provider, when the agent starts,
   then no engine with that provider is created, and the level resolves as in
   stories 1 to 3.
8. Given any of the situations above, when the agent starts, then the
   settings file is rewritten once with the removed entries gone, the previous
   file is kept beside it as a backup, and the agent log says which engines,
   model lists, MCP choices and initialization provider it dropped.
9. Given a settings file naming none of the removed providers, when the agent
   starts, then the file is neither rewritten nor backed up for this reason.

### US3 (P1) - The agent refuses the removed providers

As anyone driving the agent directly, I get an explicit refusal when I name a
removed provider.

1. Given a desktop save of the engine catalogue with an engine whose provider
   is `gemini`, `cursor` or `vibe`, when the agent receives it, then it
   refuses the save with a message naming the engine and the unsupported
   provider, and the stored catalogue is unchanged.
2. Given a desktop save of the execution defaults with a model list or an
   initialization provider for a removed provider, when the agent receives
   it, then it refuses the save with a message naming the provider.
3. Given `sectile-agent init --provider gemini` (or `cursor`, or `vibe`), when
   it runs, then it fails with a message saying the provider is unsupported,
   and its help and error texts list only `agy`, `claude` and `codex`.
4. Given an MCP configuration request for `gemini`, `cursor` or `vibe`, when
   the agent receives it, then it answers 400.
5. Given a launch whose engine cannot be run interactively, when the agent
   explains the refusal, then the providers its message lists are the
   supported ones only.

### US4 (P2) - A server seed naming a removed provider is ignored

As a person pairing a workstation with a server that stored Gemini, Cursor or
Vibe as its provider before #305, I get a workstation that runs a supported
engine.

1. Given a workstation whose catalogue is still unstated, and an execution
   seed whose provider is `gemini`, `cursor` or `vibe`, when the seed is
   applied, then the workstation default engine stays the one a workstation
   stating nothing runs.
2. Given an execution seed carrying model lists for removed providers and for
   `claude`, when the seed is applied, then only the `claude` list is copied.
3. Given an execution seed naming a removed provider for one project, when
   that project is seeded, then it creates no engine and the project runs the
   workstation default engine.

### US5 (P2) - The web app and the docs follow

As a web user or a reader of the documentation, I no longer meet the removed
providers as AI engines.

1. Given the web app, when it builds a command preview or an MCP snippet for
   a provider, then `gemini`, `cursor` and `vibe` are not among the providers
   it knows.
2. Given `README.md`, `desktop/README.md`, `docs/CAPABILITIES.md` and
   `docs/contracts/server-agent-v1.md`, when they list the supported AI
   providers, then they list `agy`, `claude`, `codex` and `custom` only.
3. Given `CHANGELOG.md`, when read, then `[Unreleased]` carries one line under
   `Removed` saying that Gemini CLI, Cursor CLI and Mistral Vibe CLI are no
   longer AI engines, and what happens to a workstation's existing settings
   for them.

## Functional requirements

- **FR-001** The supported AI providers are `agy`, `claude`, `codex` and
  `custom`. Every list of providers a person picks from, in the desktop and
  the web, offers those only (the web and desktop MCP lists offer `agy`,
  `claude` and `codex`, since `custom` has no configuration file).
- **FR-002** The agent refuses `gemini`, `cursor` and `vibe` wherever it
  validates a provider: an engine, a model list, an initialization provider, a
  setup provider, an MCP configuration request and `init --provider`.
- **FR-003** The agent ships model lists for `agy`, `claude` and `codex` only;
  the desktop's model lists section follows them.
- **FR-004** When it reads its settings, the agent drops every catalogue
  engine whose provider is removed, the project and task choices naming them,
  the model lists and MCP connection choices keyed by a removed provider, and
  an initialization provider set to one. A removed default engine is replaced
  by the first remaining engine, or by the engine of a workstation stating
  nothing when none remains.
- **FR-005** Legacy engine fields (before #510) naming a removed provider
  produce no engine; the level they belonged to resolves as in FR-004.
- **FR-006** FR-004 and FR-005 are persisted once, at agent start, with a
  backup of the previous file, and logged with the list of what was dropped.
  A file with nothing to drop is left alone.
- **FR-007** A server execution seed naming a removed provider creates no
  engine, and its model lists for removed providers are not copied.
- **FR-008** Messages the agent shows that list supported providers list only
  the supported ones. These messages keep the language they are written in.
- **FR-009** Nothing outside Sectile's settings file is created, changed or
  deleted on the workstation because of this change.
- **FR-010** The Cursor editor choice, Antigravity's `~/.gemini/config`
  locations and `.gemini` skill directories, and the checkout cleanup of
  earlier registration files are unchanged.

## Success criteria

- A workstation upgraded with any mix of Gemini, Cursor and Vibe settings
  starts, resolves every project and task to a supported engine, and its
  settings file names none of the removed providers after the first start.
- No desktop or web control offers `gemini`, `cursor` or `vibe` as an AI
  provider.
- The agent's tests, the desktop tests and the web tests pass with fixtures
  that no longer rely on the removed providers, except those that check they
  are refused or dropped.

## Open requirements

None. Every product decision was settled during clarification.
