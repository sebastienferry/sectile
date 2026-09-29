# ADR 0039: Sectile is installed as a Claude plugin, and a dispatch installs nothing

Status: Accepted (#267)

## Context

Until #267 the agent installed Sectile into the user's configuration whenever it
needed it. Every task dispatch, every macro dispatch, every agent connection and
every save in the skills editor wrote the workflow skills into the CLI's
user-level skill folder (`~/.claude/skills`, `~/.agents/skills`,
`~/.gemini/config/skills`) and registered the Sectile MCP server in the CLI's
configuration (`~/.claude.json`, `~/.codex/config.toml`, ...). The agent start
rewrote the MCP registrations saved from the desktop as well.

This had three costs:

- **The user's configuration was rewritten unasked**, including files the user
  edits by hand.
- **Two projects collided.** The skill folder is user-level, so a project with a
  custom `clarify-issue` overwrote the one another project had installed a
  minute earlier.
- **Claude users already have a distribution channel** for skills and MCP
  servers, the plugin, which Sectile did not use.

## Decision

**Sectile's workflow skills and its MCP declaration are distributed as a Claude
plugin that the user installs. The agent no longer writes skills or MCP
registrations on its own; a dispatch uses what it finds.**

- `internal/skills/plugin.go` renders the plugin: one `SKILL.md` per workflow
  skill, generic across projects (framework-specific steps are subsections the
  skill picks from `get_project_context`), a `.mcp.json` whose URL and API key
  come from the plugin's `userConfig` (`server_url`, and `api_key` marked
  sensitive). `cmd/sectile-plugin` (`make plugin`) writes it. Publishing it is
  the job of a separate marketplace repository.
- The direct setup stays, but only on request: `sectile-agent init`, the
  desktop's **Initialize**, the `sync_config` operation and the MCP connection
  saved from the desktop. It is the only route for codex, agy, gemini, cursor
  and vibe. It installs the same generic skills as the plugin, with the local
  HTTP fallback (`directContent` in `GET /api/v1/agent/config`): the folder it
  writes is shared by every project of the workstation, and nothing rewrites
  it per project any more (see *Amendment* below).
- At dispatch the agent resolves each workflow skill, in this order:
  1. the project's custom skill, when the workstation setting **Custom project
     skills win** is on (the default) and the project edited the skill; it is
     written to `~/.config/sectile/runs/<runId>/<dir>/SKILL.md`, readable by
     the user only, named in the prompt, and removed when the run ends (a sweep
     at agent start removes what a crash left behind after a week);
  2. an explicit command from the project's or the workstation's settings,
     used verbatim;
  3. the installed source the workstation prefers (**Installed skills source**:
     direct copy by default, or the Claude plugin, run as `/sectile:<dir>`),
     then the other one;
  4. nothing found: the launch fails before any CLI starts, with a message
     pointing at the plugin, `init` and **Initialize**. Nothing is written to
     repair it.
- The server says which skills are custom (`custom` on each skill of
  `GET /api/v1/agent/config`). An older server sends no flag, so every skill
  reads as not custom and the installed one runs, which is what an unedited
  project ran before.
- The agent reads Claude's plugin state
  (`~/.claude/plugins/installed_plugins.json`, and `enabledPlugins` in
  `~/.claude/settings.json`, then the run folder's `.claude/settings.json` and
  `.claude/settings.local.json`, the later stated key winning, as Claude
  resolves it) and never writes it. A plugin installed for
  another project, or disabled, is not a source.
- A saved desktop MCP connection is rewritten at agent start only when the
  address, the key or the executable it was written with changed. A
  fingerprint of the three, never the key, records what was written. The
  loopback port can change between starts when 8091 is taken, which is the
  case the start-up rewrite existed for.
- Using a custom skill is a passive signal: a step on the run's activity, and,
  in the desktop, a dot on the settings button with the list of the custom
  skills that ran since the agent started.

## Consequences

- **Nothing is reconciled.** The agent compares no version, checksum or date
  between the installed skills and the server's. A plugin update never
  interrupts a customisation, and an outdated direct copy keeps running until
  the user runs the setup again.
- **A workstation without a setup cannot run a workflow skill** it did not
  have before. The failure message says how to install one. An edited skill
  still runs, since it travels with the dispatch.
- **Custom skills no longer collide**: each run has its own file.
- **The skills editor's badges describe the direct copy only**, and a save no
  longer reaches any workstation until the next dispatch.
- **An earlier `sectile` entry in `~/.claude.json`** written by the direct
  setup is not removed by installing the plugin. Both then declare the same
  server; `init` says so.

## Amendment: the direct copy is generic

The first version of this decision kept the direct setup's content per
project: `RenderSkillContent` picks the project's specification framework
(`steps.speckit.md` or `steps.openspec.md`) and appends its pull-request
policy, and a project's edit replaced the built-in skill. Before #267 every
dispatch rewrote the copy for the project it launched; with dispatches no
longer writing, the copy of whichever project was set up last ran for all of
them. An OpenSpec project then followed the Spec Kit steps and another
project's pull-request policy, silently.

The server therefore sends, beside each skill's `content`, a `directContent`
and `directCommandContent`: the built-in skill rendered as the plugin renders
it (`RenderDirectSkillContent`, each framework variant under its own heading,
the pull-request policy for every creation stage, both read from
`get_project_context` at run time), with the local agent's HTTP fallback. The
direct setup installs them; `content` remains the project's own, handed to a
run when it is custom. The skills editor marks a direct copy as diverged when
it is neither form of the generic skill, which is also how a copy an earlier
release rendered for one project shows.

A copy written before this change is not detected at dispatch: comparing
installed skills with the server's is the reconciliation the owner rejected.
It is replaced by the next `sectile-agent init`, **Initialize** or
`sync_config`, which the changelog asks users of the direct setup to run.

## Alternatives rejected

- **Keep writing at dispatch, and skip it when the plugin is present.** Still
  rewrites the configuration of every non-plugin user unasked.
- **Hand the custom skill inline in the prompt.** A composed `pickup` skill
  runs to tens of kilobytes: past the Windows command-line limit once base64
  encoded, and truncated by a PTY's line editor in interactive mode.
- **Write the custom skill into the worktree.** It dirties the checkout that
  `transition_stage` refuses.
- **Install the plugin from the agent** (`claude plugin install`). It is the
  user's choice, and it would tie the agent to the Claude CLI.
- **Version or checksum reconciliation** between installed and server skills,
  rejected by the owner during clarification. This also rules out passing over
  a direct copy that is not the generic skill at dispatch.
- **Keep the direct copy per project and rewrite it at dispatch**, as before
  #267. That is the unasked rewrite this decision removes.
