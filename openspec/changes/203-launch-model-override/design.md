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

The list of models a provider can run is, today, the hardcoded
`AI_MODEL_SUGGESTIONS` map of `web/src/lib/aiModels.ts:8-13`. It is presentation
data for a free-text field, it lives only in the web bundle, and nothing on the
server or the agent knows it.

## Decisions

### The model list is global configuration, per provider
A new setting `aiProviderModels`, a map from provider id to an ordered list of
model identifiers, stored as JSON in a new `settings.ai_provider_models` column
and edited in the AI engine section of the profile
(`web/src/components/ProfileModal.tsx:440-500`). Each entry is validated with
the existing `agentconfig.ValidModel`, and a `ValidProviderModels` helper
reports the offending provider and value, as `ValidModelConfig` already does for
a level.

An empty or absent list for a provider falls back to a built-in seed, today's
`AI_MODEL_SUGGESTIONS` values, which keeps a fresh install usable and makes the
setting an override rather than a prerequisite. That seed lives in the web
alone, beside the code that applies the fallback: the server and the agent never
offer models, so a second copy in Go would be read by nothing and could drift
from the one users actually see.

This one list feeds three consumers: the datalist of the existing configuration
fields (`AIModelField`), the card submenu and the detail view selector. Adding a
model in one place therefore makes it available everywhere it can be chosen.

Rejected: keeping the list hardcoded and only restricting the launch surfaces to
it. The reviewer asked for a list "configured globally per LLM provider", and a
hardcoded list would make a newly released model unreachable at launch while it
is reachable in the settings, which is the inconsistency this change is meant to
remove.

Rejected: a per-project list. The provider is already a per-project setting, and
a project that departs from the global model does so through `aiModel` and
`aiSkillModels`. A second per-project dimension would multiply the levels
without answering a need the ticket states.

### Both launch surfaces pick from that list, never free text
The configured model for the task and the skill is always the first entry and
is marked as current; picking it sends no override. The remaining entries are
the models configured for the task project's provider, minus the current one
when it is already among them.

- **Card menu** (`TaskCard.tsx`, the shared `modeActions` fragment): an entry
  opening a nested list that *selects* rather than launches. One row is ticked,
  picking another only changes what the card retains, and the card announces it
  in four characters at most right before its action buttons. Every control of
  the card then uses it. Separating the choice from the launch is what makes a
  model usable with the controls that already exist, instead of duplicating each
  of them per model; it also lets the full chain carry the model, since from a
  card that chain is a single `pickup` run.

  The selection is remembered per task in `localStorage`
  (`web/src/lib/launchModel.ts`), like the board display mode: it describes how
  the user wants to work on that ticket, not a single click. A selection the
  project's provider no longer offers is ignored, so changing a project's engine
  or trimming its list cannot launch a model that is no longer on the list.
- **Detail view** (`TaskDetailModal.tsx:1463`): a `<select>` beside the existing
  mode selector, defaulting to the current model, applying to the interactive,
  autonomous and configured-mode buttons of every skill row alike.

`AIModelField` keeps its free-text input: it edits configuration, which #132
requires to accept any identifier. The restriction is a property of the launch
surfaces, where a typo would be discovered only when the CLI fails.

The entry and the selector are not rendered when the task project's provider has
no models at all, which is the case for the providers that take none (`agy`,
`vibe`) unless the user configured some. A `custom` provider whose template
carries `{model}` is served by configuring its list like any other.

Rejected: a free-text input in the card menu. No precedent in that menu, it
fights the menu's outside-click close, and the reviewer ruled against it twice.

Rejected: listing every model of every provider. A row the provider cannot run
would launch, be ignored, and show a misleading label on the run.

### The card's request path joins the detail view's
The card advances through `advanceTask` → `runSkill`
(`AppContext.tsx:2418-2428`), not through `/advance`. `advanceTask` gains a
`model` argument forwarded as `{ mode, model }`, so both surfaces send the same
request shape. `/advance` gains the field too, for the other clients that use
it.

### A fourth, most specific level, folded where the levels already merge
On the agent, the dispatched model is applied on top of the local configuration
before `ResolveModel` runs. A bare model on the highest level would normally
lose to a per-skill entry below it, by the #132 rule, and a launch override
names one run, which is more specific than any per-skill entry. So
`dispatchCommand` resolves `model := override` when the override is set and
falls back to `ResolveModel(config, skillID)` otherwise. `MergeModels` is not
modified: the override is a precedence rule of the launch, not of the
configuration.

Rejected: passing the override as a fully resolved model that bypasses
`launchCommandLine`. The template rule (`{model}` placeholder, `ExpandModel`,
`UsesCommandTemplate`) and the flagless-provider rule (`ModelArgs`) must keep
governing the line. The override only changes the value that enters them.

Rejected: writing the override into the project or the agent configuration for
the duration of the run. It would race with concurrent launches on the same
project and is exactly what the ticket wants to stop doing by hand.

### Shape validation stays the security boundary, membership stays a client rule
- **Server**: `agentconfig.ValidModel(req.Model)` in the run-skill and advance
  handlers, answering 400 like the existing `mode invalide` check.
- **Agent**: `agentconfig.ValidModel(payload.Model)` in `dispatchCommand`,
  refusing the launch with an error naming the value. The agent is the process
  that runs `sh -c`; it must not trust the server for a word it places on the
  command line, for the same reason `escapeForDoubleQuotes` exists.

Neither checks membership in the configured list. #132 requires validation on
shape and not on membership, so that a model released after a build still works;
a client holding a list edited a minute ago would otherwise have its launch
refused. The list governs what can be picked, not what is accepted.

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

- **At launch**, `startRemoteRun` writes the server's own resolution: the project
  provider and `ResolveSkillModel` over the merged global and project levels,
  with the launch override on top when one was sent.
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
- **Desktop**: `desktopRun.Provider` is already serialised but only set for free
  consoles; task runs now set it and gain `Model`. `runLabel` in
  `desktop/src/main.js:45` appends the pair to the skill name for task runs.

Rejected: a new websocket message type from agent to server. The waiting relay
proved an HTTP post from the agent to `/api/activities/{id}/...` is enough and
reaches the same listeners.

Rejected: the server-resolved value alone. It cannot see the workstation
override and would sometimes name a model the CLI never ran. The reviewer
confirmed the agent-reported value at the specification review.

### Template and flagless providers: notice, no refusal
The detail view's selector reuses the two notices `AIModelField` already shows,
fed with the task project's provider and command template as
`TaskDetailModal.tsx:137` already computes the provider. A launch is never
refused for that reason: the configured model is a silent no-op in the same
cases, and refusing here would make the launch stricter than the settings.

### Discussion and free console stay out
`discuss` and `open_terminal` resolve with an empty skill ID and never read the
override (`agent_config.go:519-523`, `live()`). The launch surfaces do not offer
a model for them.

## Risks
- **Settings column**: `settings` is written by one `INSERT ... ON CONFLICT`
  whose column list, placeholder count and `excluded` assignments must change
  together (`db.go:3226-3267`), plus the two `SELECT` lists (`db.go:2463`,
  `db.go:2909`). Same trap as `task_activities` below.
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
- **Desktop UI tests load `dist/index.html`**: `npx vite build` runs before them.
