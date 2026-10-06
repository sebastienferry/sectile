# Specification #730 - Desktop: global Sandbox settings with a project whitelist

- Ticket: https://github.com/sebastienferry/sectile/issues/730
- Branch: `claude/clarify-issue-gh-11a4f59c-c1b638`
- Clarification: `docs/clarifications/730.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-05)
- Extends: `specs/700-desktop-sandbox-configuration/`
- Framework: Spec Kit

## Summary

The owner sets Claude Code's Sandbox values once, in the workstation settings
of Sectile Desktop, instead of repeating them in every project. The global
values reach every project, or only the projects the owner checks in a
whitelist. Each project keeps its own Sandbox category, whose values add to
the global ones. The values projects already hold are folded into the global
level once, when the agent first starts with this change.

## Scope

In scope: a "Sandbox" category in the workstation settings of Desktop, its
project whitelist, how the global and project values combine for a launch,
moving an allow rule from a project to the global level, the one-time fold of
the existing project values, the project category's view of what it inherits,
and the changelog.

Out of scope:

- Headless runs keep approving every tool: the allow rules still change
  nothing for them, and a whitelist of what headless runs may do is not part
  of this change.
- A whitelist of filesystem folders: the whitelist names Sectile projects.
- The web client, engines other than Claude Code, custom command templates and
  Claude Code's own settings files, as in #700.
- Where "Always allow" writes: it keeps writing to the project.

## Vocabulary

- **Sandbox values**: as in #700: the sandbox state (Inherited, On, Off), the
  allowed network domains, the extra writable paths, the allow rules and the
  deny rules.
- **Global values**: the Sandbox values of the workstation settings.
- **Project values**: the Sandbox values of one project's settings.
- **Whitelist**: the projects the global values apply to. Empty means every
  project.
- **Covered project**: a project the global values apply to: every project
  when the whitelist is empty, the checked ones otherwise.
- **Resolved values**: what a launch of a project applies: the global values
  combined with the project values for a covered project, the project values
  alone for any other.

## User stories

### US1 (P1) - Set the Sandbox values once for every project

As an owner, I open the workstation settings, choose "Sandbox", and set the
values every project of the workstation applies.

1. Given no global values, when I open the workstation "Sandbox" category,
   then the state reads "Inherited", the four lists are empty, and the
   whitelist reads "All projects".
2. Given the category, when I set the state to "On", add the domain
   `registry.npmjs.org` and the deny rule `Bash(git push:*)`, and save, then
   reopening the category, or restarting Desktop, shows the same values.
3. Given global values and two projects with no values of their own, when a
   Claude Code launch starts on a task of either project, then it applies the
   global values.
4. Given the entry rules of #700 (trimmed, never twice, an empty entry
   refused, a rule in both allow and deny flagged with "the deny rule wins"),
   then the global lists follow them too.

### US2 (P1) - Limit the global values to some projects

1. Given the workstation "Sandbox" category, then it lists the projects added
   to the workstation, each with a checkbox, none checked, and says that the
   global values apply to every project while none is checked.
2. Given projects A and B, when I check only A and save, then launches of A
   apply the global values, and launches of B apply only B's own values.
3. Given a checked whitelist, when a project is added to the workstation
   later, then it is not covered until I check it. With an empty whitelist, a
   project added later is covered at once.
4. Given a checked project, when it is removed from Desktop, then it leaves
   the whitelist. When the whitelist becomes empty that way, every project is
   covered again, and the category says so.

### US3 (P1) - Project values add to the global values

1. Given a covered project, then each of its four lists applies as the global
   list followed by the project's own entries, without duplicates.
2. Given a covered project whose state is "Inherited", then it applies the
   global state. Given a project whose state is "On" or "Off", then it applies
   its own state, whatever the global one.
3. Given a covered project, when I open its "Sandbox" category, then each list
   shows the entries coming from the global level, marked as such and not
   removable there, above the project's own entries; the state hint says when
   the global state applies.
4. Given a project that is not covered, then its category says that the
   global values do not apply to it, and shows only its own values.
5. Given a covered project, then its command preview shows the line a launch
   runs with the resolved values.

### US4 (P2) - Move an allow rule up to the global level

1. Given an allow rule of a project, when I choose "Move to global" on it,
   then the rule is added to the global allow rules and removed from the
   project's, and the category shows it among the inherited entries if the
   project is covered.
2. Given a rule the global allow rules already hold, when I move it, then it
   is removed from the project and not duplicated globally.
3. Given "Always allow" in a Desktop conversation, then the rule is added to
   the project's allow rules, as in #700, never to the global ones.

### US5 (P1) - Existing project values become global once

As an owner who set Sandbox values per project before this change, I find
them in the workstation category after the upgrade, applying to every
project.

1. Given projects with values from before the change, when the agent starts
   with this change for the first time, then the four lists of every project
   are combined into the global lists, projects in the order of the settings
   file and entries in their order, without duplicates, and removed from the
   projects.
2. Given that every project stating a sandbox state states the same one, then
   that state becomes the global state and is removed from the projects.
3. Given projects stating different states, then each keeps its own, and the
   global state stays "Inherited".
4. Given the fold, then the whitelist is empty, so every project applies the
   combined values, including rules first approved on another project.
5. Given the fold has run, when a project gains values later (by "Always
   allow" or in its category), then they stay on the project and are never
   folded again.
6. Given a workstation with no project values, then the fold changes nothing
   and leaves the global values empty.
7. Given the fold changed the settings file, then the previous file is kept
   beside it as a backup, as the earlier settings migrations do.

### US6 (P2) - Windows

1. Given Desktop on Windows, then the workstation category disables its
   sandbox part (state, domains, writable paths) with the same hint as the
   project category, while the rules and the whitelist stay editable and
   apply.

## Functional requirements

- **FR1** The workstation settings of Desktop have a "Sandbox" category,
  saved with the workstation's other local settings and never sent to the
  server.
- **FR2** It edits the five Sandbox values of #700, with the same entry rules,
  and a whitelist of the projects added to the workstation.
- **FR3** An empty whitelist covers every project, a project added later
  included. A non-empty whitelist covers only the projects it names. Removing
  a project from Desktop removes it from the whitelist.
- **FR4** A launch of a covered project applies, for each list, the global
  entries followed by the project's own, without duplicates, and the project
  state when it is not "Inherited", else the global state. A launch of a
  project that is not covered applies its project values alone.
- **FR5** Every built-in Claude line of a project applies its resolved values,
  as #700 applies the project values today, without writing into a
  repository, a checkout or a worktree. A project whose resolved values state
  nothing gets the command line of today, byte for byte.
- **FR6** The project category shows the global entries it inherits, read
  only and marked, and says whether the project is covered.
- **FR7** "Move to global" on a project allow rule moves it to the global
  allow rules in one save.
- **FR8** "Always allow" adds its rule to the project's allow rules, as today.
- **FR9** The values a workstation holds per project when the agent first
  starts with this change are folded once into the global values, as US5
  states, the previous file kept as a backup. The fold is recorded and never
  repeats.
- **FR10** A save of the global values keeps an entry added to them meanwhile
  (a rule moved up while the dialog was open), as the project save of #700
  does.
- **FR11** Saving the other workstation settings (Execution defaults, AI
  engines) never changes the global Sandbox values.
- **FR12** On Windows, the sandbox part of the workstation category is
  disabled with the explanation of #700; the rules and the whitelist apply.
- **FR13** `CHANGELOG.md` gets one `Added` line under `[Unreleased]`, saying
  that existing project values become global and apply to every project.

## Acceptance criteria

- US1 to US6 scenarios pass.
- The command line tests of today pass unchanged for a workstation with no
  global and no project values.
- A test asserts the resolved values for a covered and an uncovered project,
  the list order and the state precedence.
- Fold tests cover: lists combined in order, agreeing states moved up,
  diverging states kept, nothing to fold, and a second start that folds
  nothing.
- No new child process opens a console window on Windows.

## Edge cases

- A whitelist naming a project no longer known (its section removed by hand):
  the entry is ignored at launch and dropped on the next save.
- A whitelist that names every project is not the same as an empty one: a
  project added later is not covered. The category says which applies.
- A project-level "Off" under a global "On" switches the sandbox off for that
  project only, as the state precedence states.
- A rule in the global allow rules and in a project's deny rules: the launch
  holds it in both lists and Claude Code denies it; the project category
  flags it with the warning of #700.
- The fold meets an entry the category would refuse (empty, or with a line
  break, written by hand into the file): that entry stays on its project, the
  agent log names it, and the rest is folded.
- Global values changed while a session runs: the running session keeps what
  it started with, and the next launch applies the new values, as in #700.
- A task in a shared batch worktree applies the resolved values of its own
  project.

## Open points

None. Every product question was settled in the clarification.
