# Choose the model when launching a skill on a task

## Why
Model selection landed in #132 as configuration only: a global model, a project
model, a per-skill map at each level and the workstation override in
`.taskflow/agent.json`. Per-run switching was an explicit non-goal of that
ticket. The result is that trying a stronger model on one stubborn task means
editing the project settings, launching, then remembering to put them back.

Nothing on a task's launch surfaces mentions the model today: `TaskCard.tsx`,
`TaskDetailModal.tsx` and `CopyTaskSkillMenu.tsx` carry a one-off execution
mode override since #123 and #177, but no model. The run record is equally
silent: `TaskActivity` exposes neither the engine nor the model a run used, and
the only trace is the `🤖 Moteur IA : X (model)` step line of the server-side
headless path, which the agent-launched runs never produce.

## What Changes
- The task detail view's skill launcher gains a model field beside its existing
  mode selector. Its placeholder shows the model the configured levels resolve
  for that project and skill. An untouched field sends no override, so the
  launch reproduces today's command line byte for byte.
- A model typed at launch travels with the run as a per-run override that
  outranks every configured level, including the workstation override, for
  that run only. Nothing is written back to the project or the global settings.
- The override is accepted as the same free-text identifier as the configured
  field and validated by the same shape rule (`ValidModel`) in the web
  interface, in the server handlers and in the local agent, since the value
  reaches a line run through `sh -c`.
- The override applies to both execution modes, interactive and autonomous, and
  to both delivery paths, the direct dispatch to a connected agent and the
  queued launch.
- A run record carries the provider and the model it actually ran against. The
  server stores its own resolution at launch; the agent, which is where the
  workstation override is applied, reports the final value once it has built
  the command line. The web activity views and the desktop run list show the
  pair beside the skill name.

## Impact
- Schema: `run_provider` and `run_model` columns on `task_activities`, read by
  every explicit column list of that table.
- Contracts, all additive: `RunSkillRequest.model`, `SkillJob.Model`,
  `RunLaunch.Model`, `agentprotocol.Operation.Model`,
  `agentconfig.Dispatch.Model`, `TaskActivity.provider` / `TaskActivity.model`,
  and a run-update body on `/api/activities/{id}/engine`.
- Code: `internal/models/models.go`, `internal/db/{db,remoterun}.go`,
  `internal/handlers/handlers.go`, `internal/agentprotocol/operations.go`,
  `internal/agentconfig/config.go`, `internal/agent/{agent,agent_config,
  agent_headless,agent_run,agent_desktop}.go`, `web/src/types/index.ts`,
  `web/src/context/AppContext.tsx`, `web/src/components/{TaskDetailModal,
  ActivitiesView,RemoteRunBadge}.tsx`, `desktop/src/main.js`,
  `docs/CAPABILITIES.md`, `README.md`.
- Backward compatible: an agent that predates the field ignores it and runs the
  configured model; the run record then shows the server-resolved value, which
  is the best the server can know.

## Non-goals
- Changing the configured precedence settled in #132 (most specific statement
  wins, per-skill entries outrank bare models).
- Cost or quota tracking.
- Choosing the provider at launch: this change is about the model only.
- A model choice on the full chain run, which takes no override of any kind
  today and re-enqueues each step from the previous run record.
- A model entry in the task card's `...` menu, in the desktop next-step control
  or in the "Copy command" menu. The copy menu copies a prompt, not a command
  line, so it cannot carry a model at all.

## Open points carried from clarification
The clarification ran unattended and settled the three product questions on
their recommended option. This specification applies those options; each stays
open for the reviewer to overturn before implementation starts, and the
requirement it would change is named:

1. **Surface**: the detail view's skill launcher only, not the card menu.
   Changes "The task detail launcher offers a per-run model" if overturned.
2. **Run record value**: agent-reported final model, server resolution as the
   initial value. Changes "A run record names the engine and model it used" if
   the reviewer prefers the server-resolved value alone.
3. **Full chain**: excluded. Adding it would extend "A launch override outranks
   every configured level for that run" to every step of a chain and would
   require the override to be persisted on the chain like `chain_stop_stage`.
