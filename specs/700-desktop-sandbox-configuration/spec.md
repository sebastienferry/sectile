# Specification #700 - Desktop: sandbox configuration

- Ticket: https://github.com/sebastienferry/sectile/issues/700
- Branch: `feat/700`
- Clarification: `docs/clarifications/700.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-02)
- Framework: Spec Kit

## Summary

From the settings of a project in Sectile Desktop, the owner sets what Claude
Code's sandbox allows (on or off, allowed network domains, extra writable
paths) and the permission rules Claude Code applies (allowed and denied
tools). The values are kept on the workstation, per project, and apply to
every Claude Code launch of that project: Desktop conversations, interactive
terminal sessions and headless runs. "Always allow" in a Desktop conversation
adds its rule to these values, so it still applies to the next task, after the
task's worktree is gone.

## Scope

In scope: a "Sandbox" category in the Desktop settings of a project, the
per-project values on the workstation, their application to every built-in
Claude Code command line, the "Always allow" decision of a Desktop
conversation, the report of what a sandbox refused in a headless run, the
Desktop command preview, and the changelog.

Out of scope:

- Electron's own renderer sandbox (`webPreferences.sandbox`).
- The web client, which edits no workstation setting.
- Engines other than Claude Code (Codex, AGY...): they receive nothing.
- A custom AI command template: it is the owner's own line and is not
  rewritten.
- Workstation-wide values shared by every project (only per project).
- Editing Claude Code's own settings files (`~/.claude/settings.json`, the
  checkout's or the worktree's `.claude/settings*.json`).

## Vocabulary

- **Sandbox values**: the values a project holds for Claude Code: the sandbox
  state (inherited, on, off), the allowed network domains, the extra writable
  paths, the allow rules and the deny rules.
- **Rule**: a Claude Code permission rule as Claude Code writes it, for
  example `Bash(npm test:*)`, `WebFetch(domain:example.com)` or `Read`.
- **Built-in Claude line**: a Claude Code command Sectile builds itself: a
  Desktop conversation turn, an interactive terminal session, a headless run.
- **Inherited**: a value the project does not set, so Claude Code's own
  settings decide.

## User stories

### US1 (P1) - Set a project's sandbox from Desktop

As an owner, I open the settings of a project, choose "Sandbox", and set the
sandbox state, the allowed domains and the writable paths, without editing a
Claude Code file.

1. Given a project with no sandbox values, when I open its "Sandbox"
   category, then the sandbox state reads "Inherited", the lists are empty,
   and a hint says Claude Code's own settings apply.
2. Given the "Sandbox" category, when I set the state to "On", add the domain
   `registry.npmjs.org` and the path `~/.cache/go-build`, and save, then
   reopening the category shows the same values.
3. Given a domain or a path already in its list, when I add it again, then it
   is not duplicated.
4. Given an entry in a list, when I remove it and save, then it is gone from
   the category and from the next launch.
5. Given values saved on a project, when I quit and restart Desktop, then they
   are unchanged.
6. Given two projects, when I set values on one, then the other keeps its own.

### US2 (P1) - Edit the permission rules of a project

1. Given the "Sandbox" category, when I add `Bash(make test:*)` to the allow
   rules and `Bash(git push:*)` to the deny rules and save, then both lists
   show them on reopening.
2. Given a rule that is empty or only spaces, when I try to add it, then it is
   refused with a message, and nothing is saved.
3. Given a rule that is in both lists, then the category says that the deny
   rule wins, as it does in Claude Code.

### US3 (P1) - The values apply to every Claude Code launch of the project

1. Given a project with sandbox values, when a Desktop conversation, an
   interactive terminal session or a headless run of Claude Code starts on one
   of its tasks, then that Claude session applies the values.
2. Given a project with no sandbox values, then its Claude Code command lines
   are exactly what they are today.
3. Given values saved while a session is running, then the running session
   keeps what it started with, and the next launch applies the new values.
4. Given a headless run, then the deny rules and the sandbox values apply,
   while the allow rules change nothing, since the run already approves every
   tool.
5. Given a project whose engine is not Claude Code, or a launch through a
   custom AI command template, then the launch is unchanged, and the "Sandbox"
   category says that such launches do not receive the values.
6. Given the Desktop command preview of a project with values, then it shows
   the line Sectile actually runs, settings included.

### US4 (P1) - "Always allow" survives the task

As an owner approving a tool call in a Desktop conversation, I choose "Always
allow" and the rule still applies to the next task, after this task's
worktree has been removed.

1. Given a conversation that asks to run `npm test`, when I choose "Always
   allow", then the call runs, the rule Claude proposed is added to the
   project's allow rules, and the rest of the conversation no longer asks for
   it.
2. Given that rule, when a later task of the same project runs `npm test` in a
   new worktree, then Claude does not ask for it.
3. Given that rule, when I open the "Sandbox" category, then it is listed
   among the allow rules, and I can remove it there.
4. Given a rule that is already in the project's allow rules, when "Always
   allow" proposes it again, then it is not duplicated.
5. Given "Always allow" on a call whose proposal holds no rule (a mode change,
   for example), then the call is allowed for the rest of the conversation
   only, as today, and nothing is added to the project.
6. Given "Always allow", then no file in the task worktree or the checkout is
   written for it.

### US5 (P2) - A headless run says what the sandbox refused

1. Given a headless run whose command or network access is refused by the
   sandbox or a deny rule, then the run's activity log names the refused
   command, or the refused host when Claude Code reports one, in French as the
   rest of that log.
2. Given such a refusal, then the log also says that the project's "Sandbox"
   settings are where it can be allowed.
3. Given a headless run with no refusal, then its log is unchanged.

### US6 (P2) - Windows

1. Given Desktop on Windows, when I open the "Sandbox" category, then the
   sandbox part (state, domains, writable paths) is disabled with a hint that
   Claude Code's sandbox does not run on Windows, while the rules part stays
   editable and applies.

## Functional requirements

- **FR1** The settings of a project in Desktop have a "Sandbox" category,
  saved with the project's other local settings.
- **FR2** It edits five values: the sandbox state (Inherited, On, Off), the
  allowed network domains, the extra writable paths, the allow rules and the
  deny rules. Each list keeps its order of entry, trims its entries and
  ignores duplicates; an empty entry is refused.
- **FR3** The values are stored per project in the workstation settings of
  Sectile, never sent to the server, and kept across restarts. Removing a
  project from Desktop forgets them.
- **FR4** Every built-in Claude line of a project with values applies them to
  the Claude session it starts; a project without values gets the command line
  of today, byte for byte.
- **FR5** The values reach Claude Code without writing into the repository,
  the checkout or the task worktree, so they are neither committed nor removed
  with a worktree.
- **FR6** A value set by Sectile adds to Claude Code's own settings and cannot
  remove an entry they already contain; the category says so. The sandbox
  state, when set, overrides the inherited one.
- **FR7** "Always allow" in a Desktop conversation adds the allow rules Claude
  proposes to the project's allow rules, applies them to the running
  conversation, and writes no Claude Code settings file.
- **FR8** A headless run surfaces each refusal that Claude Code reports in its
  final result, in the run's activity log, in French.
- **FR9** Launches through a custom AI command template, and engines other
  than Claude Code, are unchanged.
- **FR10** On Windows, the sandbox part of the category is disabled with an
  explanation; the rules apply.
- **FR11** The Desktop command preview shows the settings argument a project
  with values adds.
- **FR12** `CHANGELOG.md` gets one `Added` line under `[Unreleased]`, and one
  `Fixed` line for "Always allow" rules lost with the task worktree.

## Acceptance criteria

- US1 to US6 scenarios pass.
- The command line tests of today pass unchanged for a project without values.
- A conversation test asserts that an "Always allow" answer carries no
  persistent destination and that the project's allow rules gain the rule.
- No new child process opens a console window on Windows.

## Edge cases

- Values set on a project whose engine is later switched away from Claude
  Code are kept, and apply again once it switches back.
- A path with `~` is kept as typed; Claude Code expands it.
- Saving values while the settings file of Sectile is being written by
  another Sectile process: the existing key-by-key merge keeps both.
- A rule Claude proposes in a shape Sectile does not recognise is applied to
  the running conversation only, and logged, never dropped silently.
- Claude Code rejects the generated settings at launch (a version without a
  key): the launch fails with Claude's own message in the activity, as any
  failed launch does.
- A task run in a shared batch worktree applies the values of its project.

## Open points

None. Every product question was settled in the clarification. Two facts
about Claude Code are verified at implementation time (see `plan.md`, "To
verify"); neither changes the behaviour specified here.
