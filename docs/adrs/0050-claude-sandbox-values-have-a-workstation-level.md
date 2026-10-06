# ADR 0050: Claude sandbox values have a workstation level

- Status: Proposed
- Date: 2026-10-05
- Issue: [#730](https://github.com/sebastienferry/sectile/issues/730)
- Amends: [ADR 0048](0048-claude-sandbox-values-reach-claude-through-a-generated-settings-file.md)

## Context

ADR 0048 keeps the Claude Code Sandbox values per project, so an owner with
several projects repeats the same domains, paths and rules in each one, and a
rule approved on one project is unknown to the next. #730 asks for one set of
values for the workstation, applied to every project or to a chosen few, with
each project still able to add its own.

## Decision

- **Two levels.** The workstation settings hold Sandbox values under
  `defaults.claudeSandbox`, beside the project values of ADR 0048. A launch of a
  project applies the workstation lists followed by the project's, each entry
  once, and the project's sandbox state when it states one, else the
  workstation's. This mirrors Claude Code's own merge, where a list only adds.
- **A whitelist of projects.** `defaults.claudeSandboxProjects` names the
  projects the workstation values apply to. Empty covers every project, a
  project added later included; a project removed from Desktop leaves it. A
  project that is not covered applies its own values alone.
- **One file per launch, as before.** The file of ADR 0048 is written from the
  resolved values, so command lines keep one `--settings` argument.
- **"Always allow" stays per project.** A rule approved on one repository does
  not silently apply to all; Desktop offers to move it up.
- **The project values move up once.** Settings layout 4 marks a file written
  with the workstation level. Reading an older file moves the four lists of
  every project into the workstation lists, and a sandbox state too when every
  project stating one agrees; diverging states stay on their projects. The
  agent persists it at start with the usual `.bak-layout<n>` backup, and the
  layout stamp keeps it from running again.
  Amended by #744: the fold runs in the start-up migration only, never in an
  ordinary read, and never on a file whose `maxLayout` shows a layout 4 agent
  already wrote it. An older agent that rewrote the file at layout 3 had
  dropped the workstation values; folding again silently moved the rules
  projects gained since into the emptied level. Such a rewrite is now backed
  up and logged, and an agent refuses to save over a newer file.
- **Their own endpoint.** `/desktop/workstation/sandbox` reads and saves the
  workstation values with the base-and-merge save of ADR 0048; the workstation
  defaults form keeps them untouched, since it replaces the defaults whole.

## Consequences

- After the upgrade, a rule first approved on one project applies to every
  project. This is the owner's choice (#730), and the changelog says so; the
  workstation category lists every entry so it can be removed.
- Every settings file of layout 3 is rewritten once, with a backup, even when
  no project held Sandbox values.
- A whitelist naming every project differs from an empty one: a project added
  later is covered only by the empty one.

## Rejected alternatives

- **A workstation level replacing the project one.** It loses the values a
  single project needs, such as a writable path of its own toolchain.
- **A whitelist of filesystem folders.** Sectile reasons in projects, and a
  task worktree lives outside the checkout folder.
- **A whitelist of what headless runs may do, in place of
  `bypassPermissions`.** It changes every unattended stage and was not asked.
- **Leaving the project values in place at the upgrade.** The owner chose to
  find them in one place.
- **A marker in `seeded` instead of a layout.** It needs a time stamp in every
  read, and the layout is how this file already records its migrations.
