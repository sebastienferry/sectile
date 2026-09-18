# Tasks

## 1. The per-provider model list
- [x] 1.1 Add `AIProviderModels map[string][]string json:"aiProviderModels,omitempty"` to `models.Settings` (`internal/models/models.go:150`), with a comment stating that an empty list for a provider falls back to the built-in seed.
- [x] 1.2 Add `agentconfig.NormalizeProviderModels` and `ValidProviderModels(map[string][]string) error`, the latter naming the offending provider and value, beside `ValidModelConfig` (`internal/agentconfig/model.go:38`). The shipped fallback list stays in the web alone, where the fallback is actually applied: a second copy in Go would be read by nothing and could drift from it.
- [x] 1.3 Add the `ai_provider_models TEXT NOT NULL DEFAULT '{}'` column to `settings`: the `CREATE TABLE` (`internal/db/db.go:196` area), the `ALTER TABLE` migration (`db.go:417` area), both `SELECT` lists (`db.go:2463`, `db.go:2909`) and the `INSERT ... ON CONFLICT` with its placeholder and `excluded` assignment (`db.go:3226-3267`). Normalise on write: drop empty provider keys, empty identifiers and duplicates.
- [x] 1.4 Validate the list in the settings handler (`internal/handlers/handlers.go:2470`) with `ValidProviderModels`, answering 400 as the model config check does.
- [x] 1.5 Tests: normalisation, a rejected identifier naming its provider, and what actually reaches a command line for a flagless provider or a template with no `{model}` slot. The seed fallback is covered on the web side, where it lives.

## 2. Run contracts and storage
- [x] 2.1 Add `Model string json:"model,omitempty"` to `models.RunSkillRequest` (`models.go:917`) and `Provider` / `Model` (`omitempty`) to `models.TaskActivity` (`models.go:46`).
- [x] 2.2 Add `Model` to `agentconfig.Dispatch` (`internal/agentconfig/config.go:47`) and to `agentprotocol.Operation` (`internal/agentprotocol/operations.go:6`), both `omitempty`, with a comment stating that empty means "no override".
- [x] 2.3 Add `Model` to `db.SkillJob` (`internal/db/db.go:25`) and `Provider` / `Model` to `db.RunLaunch` (`internal/db/remoterun.go:30`).
- [x] 2.4 Add the `run_provider` and `run_model` columns (`TEXT NOT NULL DEFAULT ''`) beside `run_mode` in the migration block (`db.go:395`), and write them in `startRemoteRun` next to `run_mode` (`remoterun.go:91`).
- [x] 2.5 Read both columns in every explicit `task_activities` column list: `db.go:2815` (INSERT, with matching `?`), `db.go:2829`, `db.go:4372`, `db.go:4471`, `postback.go:255`, and their `Scan` calls.
- [x] 2.6 Add `SetRemoteRunEngine(runID, provider, model string) error` mirroring `SetRemoteRunWaiting` (`remoterun.go:220`): update only a running `remote_run`, then `notifyPostBackListeners`.
- [x] 2.7 Tests in `internal/db`: a run recorded with a launch model reads it back; an engine report updates a running run and is refused on a finished one; the INSERT placeholder count matches the column list (extend the existing activity round-trip test).

## 3. Server handlers
- [x] 3.1 In the run-skill handler (`handlers.go:1729`), validate `req.Model` with `agentconfig.ValidModel`, answer 400 on failure with the same wording as the configuration field, and pass the value to `StartAgentRun` (`RunLaunch{Mode, Provider, Model}`) and to the `Dispatch` (`handlers.go:1812`).
- [x] 3.2 Compute the server-side resolution for the run record: project provider, `ResolveSkillModel` over the merged global and project levels, the launch override on top when set. Reuse the merge already done in `internal/db/agentconfig.go:72` rather than re-implementing it.
- [x] 3.3 In the advance handler (`handlers.go:2162`), decode `model`, validate it the same way, and pass it through `EnqueueSkillOnTaskWithOverrides`.
- [x] 3.4 Rename `EnqueueSkillOnTaskWithMode` to `EnqueueSkillOnTaskWithOverrides(taskID, skillID, prompt, mode, model)`; thread `Model` into `SkillJob` (`db.go:4304`), the worker's `RunLaunch` and `Operation` (`db.go:3483-3485`), and the full chain path with an empty model.
- [x] 3.5 Add `POST /api/activities/{id}/engine` beside the waiting route (`handlers.go:2606`): body `{"provider": string, "model": string}`, `ValidModel` on the model, `SetRemoteRunEngine`, 404 when the run is not running.
- [x] 3.6 Tests in `internal/handlers`: a run-skill request with a malformed model is refused with 400 and records no run; a valid model reaches the `Dispatch` (extend `agent_launch_test.go`); an advance request carries the model into the job; the engine route updates the run; a model absent from the configured list is still accepted, since validation is on shape.

## 4. Local agent
- [x] 4.1 In `dispatchCommand` (`internal/agent/agent_config.go:515`), accept the dispatched model, refuse a value `ValidModel` rejects, and resolve `model := override` when set, `ResolveModel(config, skillID)` otherwise. `live()` (discussion, bare terminal) keeps ignoring the override.
- [x] 4.2 Thread `payload.Model` from the dispatch site (`internal/agent/agent.go:950`) and from the operation path.
- [x] 4.3 After the command line is built, post `{"provider", "model"}` to `/api/activities/{runId}/engine`, modelled on `postRunWaiting` (`agent_run.go:254`), on both the PTY path (`agent.go:1008`) and the headless path (`agent_headless.go:78`). Set `desktopRun.Provider` and the new `desktopRun.Model` at the same time.
- [x] 4.4 Tests in `internal/agent` (extend `agent_model_test.go` / `agent_config_test.go`): with no override the line equals today's byte for byte, for a provider taking the flag, a flagless provider and a template with `{model}`; with an override the override reaches `--model` / `{model}` and the workstation override is outranked; a per-skill configured entry is outranked by the override; a malformed override is refused before any command is built; the engine post carries the final value.

## 5. Web: the list and the settings
- [x] 5.1 Replace the hardcoded `AI_MODEL_SUGGESTIONS` (`web/src/lib/aiModels.ts:8`) with `providerModels(settings, provider)`, returning the configured list for that provider and falling back to the same values as the built-in seed. Add `resolveConfiguredModel(project, settings, skillId)` (per-skill map first, then bare model, project before global), with unit tests in `web/tests`.
- [x] 5.2 Add `aiProviderModels?: Record<string, string[]>` to the settings type (`web/src/types/index.ts:552`) and to the profile save payload (`ProfileModal.tsx:150`).
- [x] 5.3 In the AI engine section of `ProfileModal.tsx` (`:440-500`), let the user edit the model list of the selected provider: add and remove identifiers, with `isValidModel` marking a bad entry and blocking the save, as the model field already does. Order is the insertion order, which is the order the launch surfaces offer; no reordering control, since nothing in the requirements depends on rearranging an existing list.
- [x] 5.4 Feed the `AIModelField` datalist from `providerModels` instead of the removed constant, leaving its free-text input untouched (`AIModelField.tsx:29`, `ProjectModal.tsx:1497`).

## 6. Web: the launch surfaces
- [x] 6.1 Extend `advanceTask` (`AppContext.tsx:2418`) with a `model?: string` argument forwarded to `runSkill` as `{ mode, model }`; the full chain keeps passing none. Send `model` only when non-empty (`AppContext.tsx:2757`, same pattern as `mode`).
- [x] 6.2 In `TaskCard.tsx`, add to the shared `modeActions` fragment (`:322`) an `Advance with model…` entry with `aria-haspopup="menu"` opening a nested list inside the menu portal: the configured model first, marked as current and sending no override, then the other models of `providerModels` for the task project's provider; a row calls `handleAdvance(false, undefined, model)`. The entry is not rendered when that provider has no models.
- [x] 6.3 Submenu mechanics: opens on click and `ArrowRight` with focus on its first row, closes on `ArrowLeft` and `Escape` returning focus to its entry, and picking a row closes the whole menu. The list renders in flow inside the menu, indented under its entry, rather than floating beside it: the menu is already a fixed-position portal with its own `maxHeight` and scrolling (`:77`), so an in-flow list inherits that placement and cannot leave the viewport on its own. No side placement, therefore no flipping.
- [x] 6.4 In `TaskDetailModal.tsx`, add a `launchModel` state beside `launchMode` (`:170`) and a `<select>` beside the mode selector (`:1463`): the configured model as the default option, then the other models for the task project's provider. Show the existing template and flagless-provider notices; hide the selector when the provider has no models.
- [x] 6.5 Pass `launchModel` through `handleTriggerSkill` (`:650`) to `runSkill(..., { mode, model })` for the interactive, autonomous and configured-mode buttons alike.
- [x] 6.6 Add the `compactCard.advanceWithModel`, `compactCard.currentModel` and the detail view's selector label to both locales (`web/src/locales/translations.ts:332`, `:719`, `:1104`).
- [x] 6.7 Source-assertion tests in `web/tests/skillLaunchModel.test.mjs`, in the style of `skillLaunchMode.test.mjs`: the request carries `model`, the current model sends none, `advanceTask` forwards the model, the card submenu lives in the shared `modeActions` fragment and is referenced from both card shapes, the detail selector sits next to the mode selector and feeds the three launch buttons, neither surface renders a free-text input, and the activity views render the provider and model pair.

## 7. Run record display
- [x] 7.1 Add `provider?` and `model?` to `TaskActivity` (`web/src/types/index.ts:22`) and render `provider · model` beside the skill name in `ActivitiesView.tsx:485` and `:608`, `TaskDetailModal.tsx:1546`, and in the `RemoteRunBadge.tsx:55` title. Show nothing when both are empty.
- [x] 7.2 In `desktop/src/main.js:45`, make `runLabel` append `provider · model` to the skill name when the run carries them; leave free consoles unchanged.
- [x] 7.3 Add a desktop UI test asserting the label for a run with and without an engine (run `npx vite build` first).

## 8. Documentation
- [x] 8.1 In `docs/CAPABILITIES.md`, extend the model precedence paragraph with the launch override as the most specific level, state that the launch surfaces pick from the per-provider list, and that picking the configured model sends no override.
- [x] 8.2 In `README.md:32-33`, mention the per-provider model list and the per-launch choice.
- [x] 8.3 Update `.agents/MEMORY.md` §6 to list the two new `task_activities` columns and the new `settings` column among the ones every explicit column list must carry.

## 9. Verification
- [x] 9.1 `go build ./... && go vet ./... && gofmt -l .` clean; `go test ./internal/...` green.
- [x] 9.2 `cd web && npm test` green; `npx vite build` then the desktop UI tests green.
- [x] 9.3 Manual check against a project with a configured model: launch untouched from both surfaces and compare the command line in the desktop console with the previous one; launch with another model from the card submenu on a condensed and on an expanded card, and from the detail view, and confirm `--model <choice>` and the run label; confirm the project and global settings are unchanged afterwards.
- [x] 9.4 Manual check on the list: add a model in the profile, confirm it appears in both launch surfaces and in the settings suggestions; remove it and confirm it disappears from the launch surfaces.
- [x] 9.5 `openspec validate 203-launch-model-override --strict` passes.
