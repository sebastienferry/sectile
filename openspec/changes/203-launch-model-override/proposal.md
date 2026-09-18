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

The models a workstation is entitled to run are also not written down anywhere
a user can edit. `AI_MODEL_SUGGESTIONS` in `web/src/lib/aiModels.ts` hardcodes
three or four identifiers per provider, so a newly released model, or one an
organisation forbids, needs a Sectile release to appear or disappear.

## What Changes
- The global settings gain a per-provider model list, editable in the AI engine
  section of the profile. It is seeded with today's hardcoded suggestions and
  becomes the one place naming which models exist for each provider.
- That list feeds the suggestions of the existing configuration fields and, more
  importantly, the two launch surfaces below. A model absent from it cannot be
  picked at launch.
- The task card's `...` menu gains a submenu listing the models configured for
  the task project's provider, the model the configured levels resolve first.
  It is a selection, not a launcher: one row is ticked, picking another changes
  what the card retains and starts nothing. The card shows that model, in four
  characters at most, immediately before its action buttons, and every launch
  it starts uses it, the full chain included. The selection is remembered per
  task and survives a reload. Both card shapes share the entry, as they share
  the mode entries since #177.
- The task detail view's skill launcher gains a model selector beside its
  execution mode selector, fed by the same list, defaulting to the configured
  model. Its value applies to every launch control of that view.
- Neither launch surface accepts free text. Choosing the configured model, which
  is the default on both, sends no override, so a launch nobody touched
  reproduces today's command line byte for byte.
- A model chosen at launch travels with the run as a per-run override that
  outranks every configured level, including the workstation override, for that
  run only. Nothing is written back to the project or the global settings.
- The override is still validated on shape (`ValidModel`) by the server and by
  the local agent, since the value reaches a line run through `sh -c`.
- The override applies to both execution modes, interactive and autonomous, and
  to both delivery paths, the direct dispatch to a connected agent and the
  queued launch.
- A run record carries the provider and the model it actually ran against. The
  server stores its own resolution at launch; the agent, which is where the
  workstation override is applied, reports the final value once it has built the
  command line. The web activity views and the desktop run list show the pair
  beside the skill name.

## Impact
- Schema: `ai_provider_models` on `settings`; `run_provider` and `run_model` on
  `task_activities`, read by every explicit column list of that table.
- Contracts, all additive: `Settings.aiProviderModels`, `RunSkillRequest.model`,
  `SkillJob.Model`, `RunLaunch.Model`, `agentprotocol.Operation.Model`,
  `agentconfig.Dispatch.Model`, `TaskActivity.provider` / `TaskActivity.model`,
  and a run-update body on `/api/activities/{id}/engine`.
- Code: `internal/models/models.go`, `internal/db/{db,remoterun}.go`,
  `internal/handlers/handlers.go`, `internal/agentprotocol/operations.go`,
  `internal/agentconfig/{config,model}.go`, `internal/agent/{agent,agent_config,
  agent_headless,agent_run,agent_desktop}.go`, `web/src/types/index.ts`,
  `web/src/context/AppContext.tsx`, `web/src/lib/aiModels.ts`,
  `web/src/components/{TaskDetailModal,TaskCard,ProfileModal,AIModelField,
  ActivitiesView,RemoteRunBadge}.tsx`, `web/src/locales/translations.ts`,
  `desktop/src/main.js`, `docs/CAPABILITIES.md`, `README.md`.
- Backward compatible: an empty per-provider list falls back to the built-in
  seed, and an agent that predates the model field ignores it and runs the
  configured model; the run record then shows the server-resolved value, which
  is the best the server can know.

## Non-goals
- Changing the configured precedence settled in #132 (most specific statement
  wins, per-skill entries outrank bare models).
- Changing the configuration fields of the project and profile settings, which
  keep accepting a free-text identifier as #132 requires. Only the launch
  surfaces are restricted to the list.
- Cost or quota tracking.
- Choosing the provider at launch: this change is about the model only.
- A model choice on the server's own chain entry, `POST /api/tasks/{id}/advance`
  with `{"auto": true}`, which re-enqueues each step from the previous run
  record and has no surface offering a model. The full chain started from a card
  is a single `pickup` run and carries the retained model like any other launch.
- A model entry in the desktop next-step control or in the "Copy command" menu.
  The copy menu copies a prompt, not a command line, so it cannot carry a model
  at all.

## Points settled at specification review
The clarification ran unattended and settled three product questions on their
recommended option. The reviewer ruled on them at the specification review:

1. **Surface**: overturned. The card's `...` menu offers the model too, and
   neither surface takes free text: both pick from the per-provider list
   configured in the global settings. Specified in `skill-execution-mode` and
   `agent-model-selection`.
2. **Run record value**: confirmed. Agent-reported final model, with the server
   resolution as the initial value.
3. **Full chain**: not ruled on, so the recommended option stands: excluded.
   Adding it would extend "A launch override outranks every configured level
   for that run" to every step of a chain and would require the override to be
   persisted on the chain like `chain_stop_stage`. Still open for the reviewer.
