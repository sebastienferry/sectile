# Specification #764 - Claude settings: support sandbox excludedCommands

- Ticket: https://github.com/sebastienferry/sectile/issues/764
- Branch: `feat/764`
- Clarification: `docs/clarifications/764.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-06)
- Extends: `specs/700-desktop-sandbox-configuration/`,
  `specs/730-global-sandbox-settings/`, `specs/745-claude-settings-presets/`
- Framework: Spec Kit

## Summary

The Claude settings gain a list of commands Claude Code runs outside its
sandbox, Claude Code's `sandbox.excludedCommands`, at workstation and project
level. A CLI that cannot work inside the macOS sandbox (`git` over SSH, `gh`,
`glab`) then runs outside it while every other command stays sandboxed. An
opt-in preset fills the list with the network commands of git and the GitHub
and GitLab CLIs. The "Dangerous actions" preset stops denying
`git push --force-with-lease`.

## Scope

In scope: the stored list, its merge, its place in the generated `--settings`
file, its editor in both desktop categories, the new preset, the four exact
force-push deny rules, `docs/CAPABILITIES.md` and `CHANGELOG.md`.

Out of scope:

- `allowUnsandboxedCommands` and the "Autonomy in sandbox" action.
- The web app, the server and the database. No migration.
- Windows, where Claude Code's sandbox does not run: the list is not written
  there.
- A "Move to global" action for this list (it exists for allow rules only).
- Rewriting the force-push rules an owner already holds.

## Vocabulary

- **Excluded command**: an entry of `excludedCommands`, written as the inside
  of a `Bash(...)` rule. `glab *` matches `glab` with or without arguments;
  `glab` alone matches only the bare command. Claude Code runs a matching call
  outside the sandbox, with no filesystem or network restriction, and still
  applies its allow and deny rules and its prompts.

## User stories

### US1 (P1) - Commands outside the sandbox reach the launch

1. Given a project whose Claude settings hold `excludedCommands`
   `["git *", "glab *"]`, when a Claude Code launch of it writes its settings
   file on macOS or Linux, then the file holds
   `"sandbox": {"excludedCommands": ["git *", "glab *"], ...}`.
2. Given the same values on Windows, then the file holds no `sandbox` object;
   when nothing else is set, no file is written.
3. Given a list holding only excluded commands, on macOS or Linux, then a
   file is written.
4. Given workstation entries `["git fetch *", "gh *"]` covering the project
   and project entries `["gh *", "glab *"]`, then the launch receives
   `["git fetch *", "gh *", "glab *"]`: workstation entries first, each once.
5. Given an entry that is empty, only spaces, or spans several lines, then the
   save is refused with the list named, as for the other lists. Entries are
   trimmed and stored as typed: Sectile adds no ` *`.
6. Given a workstation save while another writer added an entry to the stored
   list, then that entry is kept, as for the other lists.

### US2 (P1) - The desktop edits the list

1. Given the workstation or a project Claude settings, then a row "Commands
   outside the sandbox" follows "Allowed network domains", with the
   placeholder `glab *` and a hint saying that a pattern ending in ` *` matches
   the command with arguments, a bare name matches it alone, and that the
   command runs with full access and still follows the allow and deny rules.
2. Given a project covered by workstation entries, then those entries show as
   "Global", not removable, as in the other lists.
3. Given Windows, then the row is disabled, as "Allowed network domains" is.
4. Given a project whose only value is an excluded command, on macOS or
   Linux, then the command preview shows the `--settings` argument; on
   Windows it does not.
5. Saving sends the list; emptying it clears it.

### US3 (P2) - The "Outside the sandbox" preset

1. Given the workstation presets, then "Outside the sandbox (git, gh, glab)"
   is listed, with exactly the excluded commands `git fetch *`, `git pull *`,
   `git push *`, `git clone *`, `git ls-remote *`, `gh *`, `glab *` and no
   other entry.
2. Its description says the commands run outside the sandbox with full access
   and still follow the allow and deny rules, so `git push` keeps prompting
   unless an allow rule covers it.
3. Applying it adds the seven entries; removing it takes them out, except
   those another applied preset holds (none today).
4. "Apply recommended" does not apply it (`RECOMMENDED` stays
   `common`, `dangerous`).
5. On Windows it has nothing to apply: its button is hidden and it never reads
   as applied, as for a preset with only domains and paths.
6. Its line shows the count of its excluded commands next to the other
   counts, and its expanded entries list them under "Commands outside the
   sandbox".

### US4 (P2) - Force-with-lease is no longer denied

1. In "Dangerous actions", `Bash(git push --force*)` and
   `Bash(git push * --force*)` are replaced by `Bash(git push --force)`,
   `Bash(git push --force *)`, `Bash(git push * --force)` and
   `Bash(git push * --force *)`. The `-f` rules are unchanged.
2. Given these rules, `git push --force-with-lease`,
   `git push origin --force-with-lease` and `git push --force-if-includes`
   match no deny rule; `git push --force`, `git push --force origin main`,
   `git push origin --force`, `git push origin main --force`, `git push -f`
   and `git push origin -f` each match one.
3. Existing settings are not rewritten: an owner who applied the preset before
   keeps the two old rules, and the preset reads as not applied until applied
   again.

## Functional requirements

- FR1: `claudeSandbox` gains an optional `excludedCommands` list of strings at
  both levels, absent meaning empty.
- FR2: The list is normalized, merged on save, resolved across levels and
  returned to the desktop exactly as `allowedDomains` is.
- FR3: The generated file writes it as `sandbox.excludedCommands` only when
  the list is non-empty and the platform is not Windows.
- FR4: Every preset carries an `excludedCommands` list, applied only where the
  sandbox runs.
- FR5: The documentation explains when to prefer an excluded command over
  `allowUnsandboxedCommands: true`, and that the match is on the command text.

## Acceptance criteria

- [ ] US1.1 to US1.6 covered by Go tests.
- [ ] US2 covered by desktop unit tests and the sandbox settings UI test.
- [ ] US3 and US4 covered by `desktop/tests/claude-presets.test.mjs`.
- [ ] Manual check on macOS: in a launched session with the preset applied,
  `git fetch origin` over SSH and `glab mr view` succeed with the sandbox
  enabled, and other commands stay sandboxed.
- [ ] `docs/CAPABILITIES.md` and `CHANGELOG.md` updated.

## Open points

None. The clarification settled every product question.
