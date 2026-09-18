# Tasks

## 1. Contracts and storage
- [ ] 1.1 Add `Model string json:"model,omitempty"` to `models.RunSkillRequest` (`internal/models/models.go:917`) and `Provider` / `Model` (`omitempty`) to `models.TaskActivity` (`models.go:46`).
- [ ] 1.2 Add `Model` to `agentconfig.Dispatch` (`internal/agentconfig/config.go:47`) and to `agentprotocol.Operation` (`internal/agentprotocol/operations.go:6`), both `omitempty`, with a comment stating that empty means "no override".
- [ ] 1.3 Add `Model` to `db.SkillJob` (`internal/db/db.go:25`) and `Provider` / `Model` to `db.RunLaunch` (`internal/db/remoterun.go:30`).
- [ ] 1.4 Add the `run_provider` and `run_model` columns (`TEXT NOT NULL DEFAULT ''`) beside `run_mode` in the migration block (`db.go:395`), and write them in `startRemoteRun` next to `run_mode` (`remoterun.go:91`).
- [ ] 1.5 Read both columns in every explicit `task_activities` column list: `db.go:2815` (INSERT, with matching `?`), `db.go:2829`, `db.go:4372`, `db.go:4471`, `postback.go:255`, and their `Scan` calls.
- [ ] 1.6 Add `SetRemoteRunEngine(runID, provider, model string) error` mirroring `SetRemoteRunWaiting` (`remoterun.go:220`): update only a running `remote_run`, then `notifyPostBackListeners`.
- [ ] 1.7 Tests in `internal/db`: a run recorded with a launch model reads it back; an engine report updates a running run and is refused on a finished one; the INSERT placeholder count matches the column list (extend the existing activity round-trip test).

## 2. Server handlers
- [ ] 2.1 In the run-skill handler (`internal/handlers/handlers.go:1729`), validate `req.Model` with `agentconfig.ValidModel`, answer 400 on failure with the same wording as the configuration field, and pass the value to `StartAgentRun` (`RunLaunch{Mode, Provider, Model}`) and to the `Dispatch` (`handlers.go:1812`).
- [ ] 2.2 Compute the server-side resolution for the run record: project provider, `ResolveSkillModel` over the merged global and project levels, the launch override on top when set. Reuse the merge already done in `internal/db/agentconfig.go:72` rather than re-implementing it.
- [ ] 2.3 In the advance handler (`handlers.go:2162`), decode `model`, validate it the same way, and pass it through `EnqueueSkillOnTaskWithOverrides`.
- [ ] 2.4 Rename `EnqueueSkillOnTaskWithMode` to `EnqueueSkillOnTaskWithOverrides(taskID, skillID, prompt, mode, model)`; thread `Model` into `SkillJob` (`db.go:4304`), the worker's `RunLaunch` and `Operation` (`db.go:3483-3485`), and the full chain path with an empty model.
- [ ] 2.5 Add `POST /api/activities/{id}/engine` beside the waiting route (`handlers.go:2606`): body `{"provider": string, "model": string}`, `ValidModel` on the model, `SetRemoteRunEngine`, 404 when the run is not running.
- [ ] 2.6 Tests in `internal/handlers`: a run-skill request with a malformed model is refused with 400 and records no run; a valid model reaches the `Dispatch` (extend `agent_launch_test.go`); an advance request carries the model into the job; the engine route updates the run.

## 3. Local agent
- [ ] 3.1 In `dispatchCommand` (`internal/agent/agent_config.go:515`), accept the dispatched model, refuse a value `ValidModel` rejects, and resolve `model := override` when set, `ResolveModel(config, skillID)` otherwise. `live()` (discussion, bare terminal) keeps ignoring the override.
- [ ] 3.2 Thread `payload.Model` from the dispatch site (`internal/agent/agent.go:950`) and from the operation path.
- [ ] 3.3 After the command line is built, post `{"provider", "model"}` to `/api/activities/{runId}/engine`, modelled on `postRunWaiting` (`agent_run.go:254`), on both the PTY path (`agent.go:1008`) and the headless path (`agent_headless.go:78`). Set `desktopRun.Provider` and the new `desktopRun.Model` at the same time.
- [ ] 3.4 Tests in `internal/agent` (extend `agent_model_test.go` / `agent_config_test.go`): with no override the line equals today's byte for byte, for a provider taking the flag, a flagless provider and a template with `{model}`; with an override the override reaches `--model` / `{model}` and the workstation override is outranked; a per-skill configured entry is outranked by the override; a malformed override is refused before any command is built; the engine post carries the final value.

## 4. Web
- [ ] 4.1 Add `resolveConfiguredModel(project, settings, skillId)` to `web/src/lib/aiModels.ts` (per-skill map first, then bare model, project before global) with a unit test in `web/tests`.
- [ ] 4.2 In `TaskDetailModal.tsx`, add a `launchModel` state beside `launchMode` (`:170`), render an `AIModelField` beside the mode selector (`:1463`) fed with the task project's provider and template (`:137`), the placeholder from 4.1 and a label stating that a workstation override may still apply. Disable the skill rows' launch buttons while `isValidModel(launchModel)` is false.
- [ ] 4.3 Pass `launchModel` through `handleTriggerSkill` (`:650`) to `runSkill(..., { mode, model })` for the interactive, autonomous and configured-mode buttons alike; send `model` only when non-empty (`AppContext.tsx:2757`, same pattern as `mode`).
- [ ] 4.4 Add `provider?` and `model?` to `TaskActivity` (`web/src/types/index.ts:22`) and render `provider · model` beside the skill name in `ActivitiesView.tsx:485` and `:608`, `TaskDetailModal.tsx:1546`, and in the `RemoteRunBadge.tsx:55` title. Show nothing when both are empty.
- [ ] 4.5 Source-assertion tests in `web/tests/skillLaunchModel.test.mjs`, in the style of `skillLaunchMode.test.mjs`: the request carries `model`, an empty field sends none, the field is rendered next to the mode selector, the three launch buttons pass the model, the activity views render the pair.

## 5. Desktop
- [ ] 5.1 In `desktop/src/main.js:45`, make `runLabel` append `provider · model` to the skill name when the run carries them; leave free consoles unchanged.
- [ ] 5.2 Add a desktop UI test asserting the label for a run with and without an engine (run `npx vite build` first).

## 6. Documentation
- [ ] 6.1 In `docs/CAPABILITIES.md`, extend the model precedence paragraph with the launch override as the most specific level, and state that an untouched launch control sends none.
- [ ] 6.2 In `README.md:32-33`, mention that the model can be chosen per launch from the task detail view.
- [ ] 6.3 Update `.agents/MEMORY.md` §6 to list the two new `task_activities` columns among the ones every explicit column list must carry.

## 7. Verification
- [ ] 7.1 `go build ./... && go vet ./... && gofmt -l .` clean; `go test ./internal/...` green.
- [ ] 7.2 `cd web && npm test` green; `npx vite build` then the desktop UI tests green.
- [ ] 7.3 Manual check against a project with a configured model: launch untouched and compare the command line in the desktop console with the previous one; launch with an override and confirm `--model <override>` and the run label; confirm the project and global settings are unchanged afterwards.
- [ ] 7.4 `openspec validate 203-launch-model-override --strict` passes.
