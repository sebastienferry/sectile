# Choose the model an AI engine runs against

## Why
A project picks an AI **provider** (`aiProvider`) but never the **model** that provider runs
against. `execAgentCommand` in `internal/runner/runner.go` calls `claude -p "<prompt>"`,
`gemini -p "..."` and `cursor agent -p "..."` with no `--model`, and `agentCommandLine` in
`cmd/agent/agent_config.go` builds the interactive line the same way. Every run therefore lands on
whatever default the CLI happens to ship, and a user who wants a cheaper model for a clarification
and a stronger one for an implementation has no way to say so short of writing a full custom
`aiCommandTemplate`, which then bypasses every other launch behaviour.

## What Changes
- Add an optional model to the execution settings, resolved global → project → workstation override,
  with a per-skill map at each level so a single skill can run against a different model.
- Carry the resolved model through the `agentconfig.Config` contract so the local agent, the runner
  and the `get_project_context` MCP payload all see the same value.
- Inject the model as `--model <value>` only for the providers that accept the flag (claude, codex,
  gemini, cursor). agy and vibe ignore a configured model rather than failing.
- Leave a command template in charge: when `UsesCommandTemplate` is true, no flag is injected and a
  `{model}` placeholder carries the resolved value instead. An unresolved slot is removed together
  with the option that introduces it, so the template never runs with a flag missing its value.
- Resolve the levels by the most specific statement: a per-skill entry outranks a bare model, even
  a bare model set on a more specific level.
- Offer suggested models per provider in the UI while accepting any free-text identifier, so a newly
  released model needs no Sectile release. Validation checks shape, not membership.
- Report the resolved model on the launch step line next to the engine, and reuse it for the
  desktop free console.

## Impact
- Schema: new `ai_model` and `ai_skill_models` columns on `settings` and `projects`.
- Contract: additive `aiModel` / `aiSkillModels` fields on `agentconfig.Config` and `Overrides`.
- Code: `internal/models/models.go`, `internal/db/agentconfig.go`, `internal/db/db.go`,
  `internal/agentconfig/{config,local,validation}.go`, `internal/runner/runner.go`,
  `cmd/agent/{agent_config,agent_console,agent_operations}.go`, `internal/taskmcp/server.go`,
  `web/src/components/{AIModelField,ProfileModal,ProjectModal}.tsx`, `web/src/lib/aiModels.ts`,
  `internal/agentconfig/{model,template}.go`.
- Backward compatible: every field empty reproduces today's exact command lines.

## Non-goals
- Changing the provider list, per-run model switching from the task card, cost or quota tracking,
  and model routing decided inside prompts.
