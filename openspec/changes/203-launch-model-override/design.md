# Design

## Context
The model a run uses is resolved in two places today, and they do not see the
same levels:

- The **server** merges global and project levels into the execution contract
  (`internal/db/agentconfig.go:72-77`, `internal/db/db.go:5707-5711`) with
  `agentconfig.MergeModels`, and resolves a skill with
  `agentconfig.ResolveSkillModel` (`internal/runner/runner.go:466`).
- The **agent** folds the workstation override from `.taskflow/agent.json` on
  top of that contract (`internal/agentconfig/local.go:72-73`) and resolves the
  final value in `dispatchCommand` (`internal/agent/agent_config.go:526`,
  `model := agentconfig.ResolveModel(config, skillID)`), then hands it to
  `launchCommandLine`, `ExpandModel` and `ModelArgs`.

A launch travels through one of two paths, both ending in the same agent code:

- **Direct dispatch**: `POST /api/tasks/{id}/run-skill` with a connected agent
  (`internal/handlers/handlers.go:1770-1845`) records the run with
  `StartAgentRun(task.ID, skill, RunLaunch{Mode})` and sends an
  `agentconfig.Dispatch` over the websocket.
- **Queued launch**: `POST /api/tasks/{id}/advance` (`handlers.go:2160-2190`)
  and the no-agent fallback file a `SkillJob` (`internal/db/db.go:4234-4311`);
  the worker (`db.go:3470-3490`) records the run and calls the agent with an
  `agentprotocol.Operation`, which the handler side turns into a `Dispatch`
  (`internal/handlers/agent_operations.go:39`).

The mode override added by #123 already travels every hop of both paths:
`RunSkillRequest.Mode` → `SkillJob.Mode` → `RunLaunch.Mode` (persisted as
`run_mode`) → `Operation.Mode` → `Dispatch.Mode`. The model follows the same
route, field for field.

## Decisions

### An untouched control sends no override
The launch field is empty by default and its placeholder shows the model the
server resolves for the task's project and skill. Only a non-empty value is
sent, as `model` on the launch request.

Rejected: pre-filling the input with the resolved value and always sending it.
The server cannot see the workstation override, so a pre-filled value would
outrank it on every launch and change the command line of a user who touched
nothing. The mode override sets the precedent: "an untouched control sends no
override" (`skill-execution-mode`, #123).

The placeholder is computed client-side from data the web already holds:
`taskProject.aiSkillModels[skillId]`, then `taskProject.aiModel`, then
`settings.aiSkillModels[skillId]`, then `settings.aiModel`, then the CLI
default wording used by `ProjectModal.tsx:780`. The helper lives in
`web/src/lib/aiModels.ts` next to `isValidModel`. The label states that a
workstation override may still apply, since the web has no way to know it.

### The card menu offers a list to pick from, not a form
The card's `...` menu is made of one-click entries (`TaskCard.tsx:318-400`); a
text input there would have no precedent and would fight the menu's
click-to-close behaviour. The reviewer asked for a submenu instead: an entry
`Advance with model…` that opens a nested list, each row launching the next
step under that model in the configured execution mode.

- **What the list holds**: the suggestions for the task project's provider
  (`AI_MODEL_SUGGESTIONS` in `web/src/lib/aiModels.ts`, provider from
  `taskProject.aiProvider`, else `settings.aiProvider`), with the configured
  resolution from `resolveConfiguredModel` listed first and marked as current
  when it is not already in the list. Picking the current one sends no
  override, which keeps the "untouched sends nothing" rule on the card too.
- **When the entry is absent**: a provider with no suggestion list (`agy`,
  `vibe`, `custom`) has nothing to list, and `agy` / `vibe` ignore the model
  anyway. The entry is not rendered for them; the detail view's free-text field
  remains the way to type an identifier for a `custom` template with `{model}`.
- **Mode**: the configured one. The submenu does not multiply the mode entries
  by the model entries; a user who wants both picks the mode in the detail
  view, which carries both fields.
- **Shared fragment**: the entry joins `modeActions`, the fragment both card
  shapes render (#177), so neither shape can lose it on its own.
- **Mechanics**: the nested list renders inside the same portal as the menu,
  to the side of the entry, with `aria-haspopup="menu"` on the entry and the
  rows as `role="menuitem"`. It opens on click and on `ArrowRight`, closes on
  `ArrowLeft` and `Escape`, and picking a row closes the whole menu. The
  `MENU_WIDTH` / `MENU_MAX_HEIGHT` placement (`TaskCard.tsx:77`) flips the
  submenu to the other side when it would overflow the viewport.
- **Request path**: the card's advance goes through `advanceTask` →
  `runSkill` (`AppContext.tsx:2418-2428`), not through `/advance`. `advanceTask`
  gains a `model` argument forwarded as `{ mode, model }`, so the card and the
  detail view share one request shape. `/advance` still gains the field for
  the other clients that use it.

Rejected: a free-text input in the menu. No precedent, invalid interaction with
the menu's outside-click close, and the ticket's reviewer explicitly asked for a
list.

Rejected: listing every model of every provider. A row the provider cannot
run would launch, be ignored, and show a misleading label on the run.

### A fourth, most specific level, folded where the levels already merge
On the agent, the dispatched model is applied as one more `ModelConfig` level
on top of the local configuration before `ResolveModel` runs:

```go
cfg := config
if override := strings.TrimSpace(payload.Model); override != "" {
    merged := agentconfig.MergeModels(agentconfig.ModelConfig{Model: override}, cfg.Models())
    cfg.AIModel, cfg.AISkillModels = merged.Model, merged.SkillModels
}
```

A bare model on the highest level would normally lose to a per-skill entry
below it, by the #132 rule. A launch override names one run, which is more
specific than any per-skill entry, so the merge above is not enough on its own:
the skill map is also cleared for the launched skill. Concretely
`dispatchCommand` resolves `model := override` when the override is set and
falls back to `ResolveModel(config, skillID)` otherwise. `MergeModels` is not
modified; the override is a precedence rule of the launch, not of the
configuration.

Rejected: passing the override as a fully resolved model that bypasses
`launchCommandLine`. The template rule (`{model}` placeholder, `ExpandModel`,
`UsesCommandTemplate`) and the flagless-provider rule (`ModelArgs`) must keep
governing the line. The override only changes the value that enters them.

Rejected: writing the override into the project or the agent configuration
for the duration of the run. It would race with concurrent launches on the same
project and is exactly what the ticket wants to stop doing by hand.

### The same shape rule, enforced three times
- **Web**: `isValidModel` on the launch field; the skill row's launch buttons
  are disabled while the value is invalid, with the same inline hint as
  `AIModelField`.
- **Server**: `agentconfig.ValidModel(req.Model)` in the run-skill and advance
  handlers, answering 400 like the existing `mode invalide` check.
- **Agent**: `agentconfig.ValidModel(payload.Model)` in `dispatchCommand`,
  refusing the launch with an error naming the value. The agent is the process
  that runs `sh -c`; it must not trust the server for a word it places on the
  command line, for the same reason `escapeForDoubleQuotes` exists.

### Every hop carries the model
| Hop | Field | Location |
| --- | --- | --- |
| Web → server | `RunSkillRequest.Model`, advance body `model` | `internal/models/models.go:917`, `handlers.go:2162` |
| Server queue | `SkillJob.Model` | `internal/db/db.go:25`, `enqueueSkillOnTask` gains a `modelOverride` argument beside `modeOverride` |
| Run record | `RunLaunch.Model`, `RunLaunch.Provider` | `internal/db/remoterun.go:30` |
| Server → agent (queued) | `agentprotocol.Operation.Model` | `internal/agentprotocol/operations.go:6` |
| Server → agent (direct) | `agentconfig.Dispatch.Model` | `internal/agentconfig/config.go:47` |
| Agent | `dispatchCommand(..., model string, ...)` | `internal/agent/agent_config.go:515` |

`EnqueueSkillOnTaskWithMode` becomes `EnqueueSkillOnTaskWithOverrides(taskID,
skillID, prompt, mode, model)`; the full chain path keeps passing an empty
model.

### The run record stores what the server knows, then what the agent knows
Two new columns on `task_activities`, `run_provider` and `run_model`, both
`TEXT NOT NULL DEFAULT ''`, added beside `run_mode` (`internal/db/db.go:395`).
Empty reads as "unknown" on every run recorded before this change.

- **At launch**, `startRemoteRun` writes the server's own resolution: the
  project provider and `ResolveSkillModel` over the merged global and project
  levels, with the launch override on top when one was sent.
- **From the agent**, once `dispatchCommand` has built the line, the agent posts
  `{"provider": "<p>", "model": "<m>"}` to `POST /api/activities/{id}/engine`,
  modelled on `postRunWaiting` (`internal/agent/agent_run.go:254`) and its
  handler (`handlers.go:2606`). `SetRemoteRunEngine` mirrors
  `SetRemoteRunWaiting`: update on `skill_id='remote_run' AND status='running'`,
  then `notifyPostBackListeners` so the web refreshes. The agent posts for both
  the PTY path (`agent.go:1008`) and the headless path (`agent_headless.go:78`).
- **Serialisation**: `models.TaskActivity` gains `Provider string
  json:"provider,omitempty"` and `Model string json:"model,omitempty"`. The five
  explicit column lists of `task_activities` (`db.go:2815`, `db.go:2829`,
  `db.go:4372`, `db.go:4471`, `postback.go:255`) gain both columns in the same
  edit, per the trap recorded in `.agents/MEMORY.md` §6.
- **Desktop**: `desktopRun.Provider` is already serialised but only set for
  free consoles; task runs now set it and gain `Model`. `runLabel` in
  `desktop/src/main.js:45` appends the pair to the skill name for task runs.

Rejected: a new websocket message type from agent to server. The waiting relay
proved an HTTP post from the agent to `/api/activities/{id}/...` is enough and
reaches the same listeners.

Rejected: the server-resolved value alone. It cannot see the workstation
override and would sometimes name a model the CLI never ran. The reviewer
confirmed the agent-reported value at the specification review.

### Template and flagless providers: notice, no refusal
The launch field reuses `AIModelField` with the task project's provider and
command template, as `TaskDetailModal.tsx:137` already computes the provider.
The component's two notices ("the template governs the command line", "this
provider takes no model") appear exactly as in the settings. A launch is never
refused for that reason: the configured model is a silent no-op in the same
cases, and refusing here would make the launch stricter than the settings.

### Discussion and free console stay out
`discuss` and `open_terminal` resolve with an empty skill ID and never read the
override (`agent_config.go:519-523`, `live()`). The launch panel does not offer
a model for them.

## Risks
- **Column lists**: a column added to `task_activities` without its `?` in the
  `INSERT` yields `SQL logic error: N values for M columns` at runtime, caught
  only by the DB tests. The tasks list the five sites explicitly.
- **Mixed versions**: an older agent ignores `Dispatch.Model` and runs the
  configured model while the server recorded the override at launch. The run
  record then lies until the agent is updated. Acceptable because the field is
  additive and the version skew is transient; no schema version bump.
- **Web tests are source assertions** (`web/tests/skillLaunchMode.test.mjs`,
  #162): the new wiring is pinned the same way, by regex on the handler calls,
  which is what stops an unrelated refactor from dropping the field.
- **Desktop UI tests load `dist/index.html`**: `npx vite build` runs before
  them.
