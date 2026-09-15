# Design

## Context
`ResolveLocations` (`internal/agentconfig/locations.go`) maps a provider to `SkillDir`,
`CommandDir` and `MCPFile` under the user's home. Change 113 shipped `~/.claude/skills` +
`~/.claude/commands`, `~/.codex/skills` and `~/.agy/skills`. Only the first is right.

## Evidence
| Agent | Documented user-level skill path | Source |
| --- | --- | --- |
| Claude | `~/.claude/skills/<name>/SKILL.md` | Claude Code skills documentation |
| Codex | `$HOME/.agents/skills` | Codex "Build skills" documentation |
| Antigravity | `~/.gemini/config/skills` | Antigravity skills documentation |

Claude's documentation also states that custom commands have been merged into skills, that
`.claude/commands/<name>.md` and `.claude/skills/<name>/SKILL.md` both create `/<name>`, and
that skills support `$ARGUMENTS`.

Two third-party claims were rejected in favour of the vendors' own documentation:
`~/.codex/skills` for Codex, and `~/.gemini/antigravity/skills` for Antigravity. An open issue
on the Antigravity CLI reports `~/.agents/skills` is not picked up there, which is why
Antigravity does not share Codex's location.

## Decisions

### One file per skill for Claude
`CommandDir` disappears from the table rather than being left unused, and Claude's `SKILL.md`
receives `Skill.CommandContent`, the same text as `Content` plus the `## Ticket $ARGUMENTS`
section. This keeps `/clarify-issue #116` passing the ticket key, which the skill body alone
would drop, and removes the undefined precedence between two files declaring the same command.
Other agents keep `Content`: `$ARGUMENTS` is a Claude placeholder and would be literal text
elsewhere.

### `.agents/skills` is a convention, not an accident
Change 113 read the repository's five skill prefixes as redundant copies. `.agents/skills` is
in fact the cross-agent standard that Codex and Antigravity both document at workspace level,
and Codex at user level. That does not revive per-checkout installation (checkouts stay clean),
but it makes `$HOME/.agents/skills` the right user-level home for Codex, shared with any other
agent adopting the same convention.

### No new migration
A path that leaves the managed set is already retired by `refresh` when its content still
matches the manifest, and preserved when edited. Correcting the table is therefore enough:
on the next dispatch, `~/.codex/skills`, `~/.agy/skills` and `~/.claude/commands` are backed up
and removed, and the correct directories are populated.

## Risks / Trade-offs
- `$HOME/.agents/skills` is shared between agents by design. A future agent reading the same
  directory would see these skills too, which is the intent of the convention.
- Antigravity's global path is documented but not verified against a running CLI. If it proves
  wrong, the ticket's own rule applies: drop `SkillDir` and let Antigravity run with MCP only.
- Dropping `~/.claude/commands` changes which body Claude runs when both files were present.
  The surviving body is the one that carries the ticket key, so the outcome is the correct one.
