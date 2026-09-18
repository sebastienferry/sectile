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
- The task card's `...` menu gains a submenu listing the models suggested for
  the task project's provider, the configured resolution first. Picking one
  launches the next step under that model, in the configured execution mode.
  It is a list to pick from, not a form: free text stays in the detail view.
  Both card shapes, condensed and expanded, share the entry, as they share the
  mode entries since #177.
- A model chosen at launch travels with the run as a per-run override that
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
  `web/src/context/AppContext.tsx`, `web/src/lib/aiModels.ts`,
  `web/src/components/{TaskDetailModal,TaskCard,ActivitiesView,
  RemoteRunBadge}.tsx`, `web/src/locales/translations.ts`, `desktop/src/main.js`,
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
- Free-text model entry on the task card: the card lists models to pick from,
  and an identifier outside the list is typed in the detail view.
- A model entry in the desktop next-step control or in the "Copy command"
  menu. The copy menu copies a prompt, not a command line, so it cannot carry
  a model at all.

## Points settled at specification review
The clarification ran unattended and settled three product questions on their
recommended option. The reviewer ruled on them at the specification review:

1. **Surface**: overturned. The card's `...` menu offers the model too, as a
   submenu of models to pick from rather than a form. The detail view keeps the
   free-text field. Both are specified in `skill-execution-mode`.
2. **Run record value**: confirmed. Agent-reported final model, with the
   server resolution as the initial value.
3. **Full chain**: not ruled on, so the recommended option stands: excluded.
   Adding it would extend "A launch override outranks every configured level
   for that run" to every step of a chain and would require the override to be
   persisted on the chain like `chain_stop_stage`. Still open for the reviewer.
