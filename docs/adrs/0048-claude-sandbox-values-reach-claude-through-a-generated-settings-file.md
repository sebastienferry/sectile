# ADR 0048: Claude sandbox values reach Claude through a generated settings file

- Status: Proposed; amended by [ADR 0050](0050-claude-sandbox-values-have-a-workstation-level.md)
- Date: 2026-10-02
- Issue: [#700](https://github.com/sebastienferry/sectile/issues/700)
- Extends: [ADR 0047](0047-claude-conversations-speak-the-streaming-input-protocol.md)

## Context

#700 lets the owner set, per project and from Desktop, what Claude Code's
sandbox allows (on or off, network domains, extra writable paths) and the
permission rules Claude Code applies (allow and deny). The values must apply to
every Claude Code launch Sectile builds for the project: conversation turns,
interactive terminal sessions and headless runs.

They must also survive the task. Until now "Always allow" in a conversation
handed Claude's suggestions back as they came, and Claude wrote the rule where
it proposed, usually `localSettings`: the task worktree's
`.claude/settings.local.json`. The worktree goes at handoff and takes the rule
with it, and a launch in the checkout itself risks committing the file.

Claude Code merges the settings of several sources, and `claude --settings
<file-or-json>` adds one for a single process. Its array settings are merged
across sources, so a value Sectile adds cannot remove one the owner's own
settings hold. Claude Code 2.1.286 reads `sandbox.enabled`,
`sandbox.network.allowedDomains`, `sandbox.filesystem.allowWrite`,
`permissions.allow` and `permissions.deny`, which was checked against the
binary and a live run on 2026-10-02.

## Decision

- **The values live in the workstation settings**, under
  `projectSettings.<projectId>.claudeSandbox`, beside the project's other local
  settings. They are never sent to the server and go with the project when it
  is removed from Desktop.
- **Each launch writes them to a file Sectile owns** and hands it to Claude as
  `--settings=<path>`: `~/.config/sectile/claude/<projectId>.json`, outside
  every repository and worktree, `0600`, written atomically, only the keys that
  are set, without the `sandbox` object on Windows where Claude Code's sandbox
  does not run. Values that state nothing write no file and remove a stale one,
  so a project without values keeps its command lines byte for byte.
- **Only built-in Claude lines receive it.** A custom command template is the
  owner's own line and is left untouched; other engines get nothing.
- **"Always allow" answers for the session and keeps the rule in the
  project.** Every suggested update goes back to Claude with `destination:
  "session"`, which Claude applies to the running process without writing a
  file (checked on 2026-10-02). The allow rules among them are added to the
  project's allow rules. Since every turn of a conversation is a new process
  (ADR 0047) that regenerates the file, the next turns and the next tasks of
  the project apply them. A save of the Desktop settings sends the values it
  read as a base, and the agent keeps the entries the store gained since, so
  a rule approved while the dialog is open is not overwritten.
- **A headless run reports Claude's refusals.** The `permission_denials` of its
  result message become lines of the run's activity, bounded to ten.

## Consequences

- The rules the owner approves are visible and removable in one place, the
  project's Sandbox settings, rather than scattered across worktrees.
- An update Sectile cannot read is still applied to the turn, and logged, but
  is not kept: the next turn asks again.
- The settings Sectile adds can only widen or tighten by adding entries; an
  entry of the owner's own Claude Code settings cannot be removed from Sectile.
- A Claude Code version that rejects a key fails the launch with its own
  message, as any failed launch does.

## Rejected alternatives

- **Writing `~/.claude/settings.json`.** It applies to every Claude session on
  the machine, outside Sectile, and edits a file Sectile does not own.
- **Copying the values into each worktree's `.claude/settings.local.json`.**
  The file is removed at handoff, or committed by mistake when the checkout is
  used directly: the very problem with "Always allow".
- **Inline JSON as the `--settings` value.** It shows the rules in the process
  list and in the terminal command line, and needs shell quoting.
- **Keeping Claude's destination for "Always allow".** It is what loses the
  rule.
