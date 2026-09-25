# ADR 0030: The workstation owns the invocation

Status: Accepted

Supersedes: the sentence of [ADR 0015](0015-sign-in-is-mandatory-and-settings-are-personal.md)
saying "The AI configuration is untouched and keeps reading the deployment
row", and its paragraph on the workstation commands following the execution's
owner.

## Context

Since ADR 0006 the server executes nothing: only the local agent runs an AI
CLI. Yet the provider, the models, the command templates, the terminal, the
editor, the worktree choice, the setup providers and the skill command names
were still stored on the server and edited from the web, then overlaid by a
workstation override (#274). What a launch ran was the result of two stores
and two precedence rules, one of which the web could not see, and the server
guessed the engine of a run it did not launch. ADR 0015 had split the settings
along "shared vs personal", not along "web vs workstation", which left
`editorCommand` and `externalTerminalCommand` on the server although they name
binaries installed on one machine.

#305 settles the line. Its clarification is in `docs/clarifications/305.md`
and its specification in `specs/305-execution-settings-on-the-workstation/`.

## Decision

**The server owns the method; the workstation owns the invocation.** The
method is what the skills say, the specification framework, the PR creation
stage, the default skill mode and how far a chain runs unattended. The
invocation is which CLI, which model, which command line, which terminal and
editor, which checkout, whether to use worktrees, how many runs at once, which
extra agents get the skills, and which slash command a stage runs.

- **One store for the invocation.** `~/.config/sectile/settings.json`, layout 2:
  workstation defaults and one section per project. The agent resolves every
  execution setting from it, project section over defaults over provider
  defaults, with the rules that existed: a provider change without its own
  command drops the inherited one, the most specific model statement wins, a
  one-off launch model outranks everything. The agent is the only writer of
  those sections; the desktop edits them through it.
- **The server stops composing and writing.** The agent configuration carries
  no execution field and the standard skill commands. No request writes an
  execution setting; an older client that sends one is saved without it.
  `get_project_context` drops `useWorktrees`, `aiProvider` and `aiModel`.
- **A one-time seed, from read-only columns.** The columns stay for one
  release, read only by `GET /api/v1/agent/execution-seed`. Each workstation
  copies them once into its defaults and, the first time it resolves a
  project, into that project's section, only into keys it does not set and
  only where they change what it would resolve. The copy reproduces the
  resolution the agent had before the upgrade, which is why it is computed
  with the former precedence and why the defaults seed records what it wrote.
  A follow-up (#492) drops the columns.
- **The workstation reports its engine.** The agent sends, per project, the
  provider, model, per-skill models, model list, whether its command line
  carries a model and whether it can run headless, on connection and after
  each local save. The server stores it per account, device and project, in
  the database since the web request and the agent's socket may be on two
  instances. The web picker and badge, and the engine a run records before the
  agent's own report, read it; without it the engine is unknown.
- **The headless refusal stays on the agent**, against its local provider. A
  refused first step ends the chain with the agent's reason.

## Consequences

- A workstation runs what is installed and chosen on it, whatever the server
  stores. Two people on one project each see their own workstation's engine.
- An existing deployment upgrades without reconfiguring anybody, provided the
  agent and the server are upgraded together (ADR 0006): an older agent on the
  new server receives no execution value and runs its provider defaults.
- Team-wide execution defaults are gone for now; sharing them through a
  versioned repository file is #493. The unused project `ttyMode` is dropped.

## Rejected alternatives

- **Keep serving the execution fields for the seed.** An older agent would
  keep working, but the "server value" path would stay alive in the new agent,
  and "not used for execution" would be unverifiable.
- **Seed every project at the first connection.** It writes sections for
  projects the workstation never runs.
- **Keep the engine report in the memory of the instance holding the socket.**
  The web request may land on another instance.
- **Refuse execution fields in requests.** It breaks a web tab opened before
  the upgrade; ignoring them is enough.
