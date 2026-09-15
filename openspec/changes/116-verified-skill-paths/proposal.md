# Verified user-level skill paths

## Why
Change 113 installed managed skills in each agent's user configuration, but the three
destinations were carried over from the prefixes this repository already wrote rather than
checked against each CLI. Two of them are wrong, and the failure is silent: Codex and
Antigravity receive `SKILL.md` files in directories they never read, so the skills simply do
not exist for those agents. Claude, meanwhile, receives the same command twice: once as a
skill and once as a legacy command file, with different bodies and no defined winner.

## What Changes
- Codex reads `$HOME/.agents/skills`, not `~/.codex/skills`. Its documented search set is
  `.agents/skills` from the working directory up to the repository root, `$HOME/.agents/skills`,
  `/etc/codex/skills`, and skills bundled by OpenAI.
- Antigravity reads `~/.gemini/config/skills`, not `~/.agy/skills`: the same `~/.gemini/config`
  root as the MCP registration already in production use.
- Claude installs one file per skill. `~/.claude/commands` is dropped: custom commands have been
  merged into skills, and both files produced the same `/name`.
- Claude's `SKILL.md` carries the command body, so a skill invoked as `/skill <ticket>` still
  receives its argument through `$ARGUMENTS`.

## Impact
`internal/agentconfig` location table and skill destinations, and their tests. The wrong
directories are retired from users' machines by the manifest logic that already ships, with no
new migration. No change to MCP targets, the set of supported providers, the skill catalogue,
the `setupProviders` setting, or the checkout retirement.
