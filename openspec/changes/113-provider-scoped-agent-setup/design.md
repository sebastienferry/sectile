# Design

## Context
`prepareDispatch` (`cmd/agent/agent_config.go:255`) calls `agentconfig.Scaffold(workDir, config)` and
then `bootstrapLocalMCP(workDir, &config)`. Both are rooted at the worktree. `skillFiles`
(`internal/agentconfig/validation.go:40`) expands each skill into six destinations regardless of
provider; `BootstrapMCP` (`internal/agentconfig/mcp.go:18`) already selects a per-provider file but
resolves it against the checkout for every provider except `agy`, which redirects to `$HOME`.

## Goals / Non-Goals
- Goals: one install location per dispatch, chosen by provider; nothing managed left in checkouts;
  a clean migration for repositories that already carry managed copies.
- Non-Goals: adding providers, changing the skill catalogue or prompts, changing the MCP tool
  contract, changing worktree or branch handling.

## Decisions

### Provider location table
A single table maps a provider to its user-level roots. `agy` already proves the pattern.

| Provider | Skills | Commands | MCP |
| --- | --- | --- | --- |
| `claude` | `~/.claude/skills/<dir>/SKILL.md` | `~/.claude/commands/<cmd>.md` | user-level Claude MCP registry |
| `gemini` | `~/.gemini/skills/<dir>/SKILL.md` | — | `~/.gemini/settings.json` |
| `agy` | `~/.agy/skills/<dir>/SKILL.md` | — | `~/.gemini/config/mcp_config.json` (unchanged) |
| `codex`, `cursor`, `vibe` | none | none | existing per-provider file, moved to user level |
| `custom` | none | none | resolved from the template's first word, then as above |

**Open:** the exact user-level path for each provider must be confirmed against the CLI's own
documentation during implementation, not assumed. Where a path cannot be confirmed, the provider is
treated as having no skill convention — skills are skipped, which the specification already allows.
The MCP location is not optional: an unconfirmed MCP target is a fatal dispatch error by decision.

### Rooting writes outside the checkout
`Scaffold` and `BootstrapMCP` keep their `os.Root`-guarded, temp-file-plus-rename discipline; only
the root changes, from the worktree to the provider's configuration home. Path validation
(`component`, `managedSkillPath`) must be re-expressed against the new roots so that a malformed
provider or skill identifier can never escape the configuration directory.

### Manifest and migration
The retirement mechanism in `Scaffold` is reused as-is: a path that leaves the managed set is backed
up and removed when its content still matches the recorded digest, and left alone when edited.
Two manifests are involved.
- A **global manifest** under the agent's own state directory records what is installed per provider.
  It is what lets a provider switch retire the previous provider's files.
- The **existing per-checkout manifest** (`.taskflow/agent-manifest.json`) is read once per checkout
  to retire the repository copies, then emptied. Reading it is the migration; no separate command.

Backups keep going to `.taskflow/skill-backups/` in the checkout, so a user who wants an edited
skill back finds it where the current release already puts it.

### Project-generic skill content
`projectContextFiles` stops running. Managed skill content must therefore not interpolate project
values; the dispatch prompt and `get_project_context` carry them. This is also what makes a shared
global install safe under parallel dispatches: concurrent projects write byte-identical content, so
the last writer wins harmlessly. Per-skill workstation overrides (`.taskflow/agent.json`) continue
to apply and are, by nature, workstation-wide rather than per project — consistent with a global
install.

### Rejected alternatives
- *Namespacing skills per project* (`~/.claude/skills/<project>-clarify-issue/`): removes collisions
  but breaks the `/clarify-issue` command names and multiplies the CLI's skill list per project.
- *Keeping an agent-neutral copy in the repo as a fallback*: reintroduces exactly the repository
  noise this change removes.
- *Making MCP failure non-fatal*: an agent without MCP cannot transition stages or finish its run,
  so a "successful" launch would silently produce no workflow result.

## Risks / Trade-offs
- **Breaking**: the managed `AGENTS.md` block and `.taskflow/config.json` disappear. A human reading
  the repository loses that summary, and any custom instruction referring to it must be updated.
  Mitigation: state it in the changelog and leave the existing block in place untouched — retirement
  applies to it only when its content still matches what Sectile wrote.
- A global install is shared by every project on the workstation. Two Sectile servers with different
  skill catalogues would fight over the same files; this is accepted and out of scope.
- Users who relied on committed `.claude/skills` to share skills with teammates lose that path.

## Migration
On the first dispatch after upgrade, per checkout: retire the recorded managed copies, install to the
provider location, register MCP at user level. No user action required; edited files are preserved
and reported.
