# Provider-scoped agent setup

## Why
Every dispatch scaffolds the full managed skill set into five directories at once
(`.agents/skills`, `.agy/skills`, `.claude/skills`, `.gemini/skills`, `.skills`) plus
`.claude/commands`, whatever the selected AI provider is. Four of the five are dead weight
for any given run, and all of them land inside the user's repository and worktrees, polluting
`git status` and review diffs with files that are agent configuration rather than project source.
MCP registration is already provider-targeted but is still written into the checkout.

## What Changes
- Install managed skills into the **user-level configuration folder of the selected AI provider only**;
  providers with no skill convention (`codex`, `cursor`, `vibe`, `custom`) receive no skill files.
- Register the Sectile MCP server in the selected provider's **user-level** configuration for every
  provider, as Antigravity already does. Failure to register remains fatal for the dispatch.
- Stop writing Sectile-managed skill, command, MCP and project-context files into repositories and
  worktrees, and retire the previously managed copies that the user has not edited.
- Make managed skill content **project-generic**: project identity and workflow context reach the
  agent through the dispatch prompt and the `get_project_context` MCP tool rather than through
  files baked into a checkout.
- **BREAKING**: `AGENTS.md` no longer receives the managed Sectile project-context block and
  `.taskflow/config.json` is no longer generated. Agents obtain that information over MCP.

## Impact
`internal/agentconfig` (skill destinations, scaffolding root, project context, MCP location,
manifest), `cmd/agent` dispatch preparation and skill read-back, their tests, and the
documentation describing where Sectile writes agent configuration. No change to the skill
catalogue, the prompt templates, the dispatch command line, the set of supported providers,
or the MCP tool contract.
