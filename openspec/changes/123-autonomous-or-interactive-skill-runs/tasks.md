# Tasks

## 1. Model and persistence
- [ ] 1.1 Add `DefaultSkillMode` (`interactive` | `autonomous`) to the project model
  (`internal/models/models.go`), following the `UseWorktrees` pattern, defaulting to
  `interactive` on read.
- [ ] 1.2 Add `FullChainStopStage` (`implemented` | `reviewed`) to the project model, defaulting
  to `reviewed` on read; reject unknown values by reading them as `reviewed`.
- [ ] 1.3 Carry both through the project update path (`ProjectUpdate` and its handler) and
  through `get_project_context`.
- [ ] 1.4 Turn `StageSkill.Interactive` / `SkillEditorEntry.Interactive` into the ternary mode
  value (`""` | `interactive` | `autonomous`), reading the current `true` on `refine_macro`
  as `interactive`; make it writable from the skill editor payload.
- [ ] 1.5 Remove `StageStep.Interactive` (`internal/db/board.go`) and every reference to it.
- [ ] 1.6 Add the optional `Mode` field to `models.RunSkillRequest`, an absent value meaning
  "no override" rather than `interactive`.

## 2. Mode resolution
- [ ] 2.1 Add `ResolveSkillMode(override, skill, project)` in `internal/db` implementing the
  precedence override > skill > project > interactive.
- [ ] 2.2 Unit-test the four precedence branches and the unset/unknown-value readings.
- [ ] 2.3 Add `Mode` to `agentconfig.Dispatch` and set it from the resolved value when enqueuing.
- [ ] 2.4 Read the override from `RunSkillRequest` in the run-skill handler and pass it to
  `ResolveSkillMode`; reject an unrecognised value with an explicit 400.

## 3. Launch path
- [ ] 3.1 Add `headlessCommandLine(provider, prompt)` in `cmd/agent/agent_config.go` covering
  `claude -p`, `codex exec`, `vibe -p`; error for every other provider.
- [ ] 3.2 Detect a mode placeholder in a custom `AICommandTemplate`; without one, refuse an
  autonomous launch with an explicit error.
- [ ] 3.3 Branch `dispatchCommand` on the dispatched mode, selecting the headless or interactive
  command line.
- [ ] 3.4 In `wrapRun`, skip `startControlledCommand` for an autonomous run; capture stdout
  and stderr and stream them onto the run activity, with a size bound.
- [ ] 3.5 Test both branches: headless emits the print/exec form and opens no TTY; interactive is
  unchanged.
- [ ] 3.6 Test the refusal for an unsupported provider and for a placeholder-less template.

## 4. Transitions
- [ ] 4.1 Ensure an autonomous run is transitioned by the worker when it ends, and that a
  failed run leaves the stage untouched.
- [ ] 4.2 Keep `/api/tasks/{id}/advance/confirm` and `CompleteInteractiveStep` on the interactive
  path only; make a confirm on an already-transitioned step a no-op rather than an error.
- [ ] 4.3 Test: an autonomous success transitions once; an autonomous failure does not transition.

## 5. Full chain run
- [ ] 5.1 Parameterise `EnqueueAutonomousRun` (`internal/db/db.go:4138`) on the project's stop
  stage instead of the constant; keep the constant, renamed, as the fallback.
- [ ] 5.2 Force the autonomous mode on every step a full chain run enqueues.
- [ ] 5.3 Refuse a full chain run when the provider has no headless mode, naming the provider.
- [ ] 5.4 Test: stops at `implemented` when configured, at `reviewed` by default, refuses on a
  task already at or past the stop stage, and refuses on an unsupported provider.
- [ ] 5.5 Rename the `>>` action to "full chain" in the UI strings and translations
  (`web/src/locales/translations.ts`, `compactCard.advanceAuto`), and rename
  `db.AutonomousStopStage` to a full-chain name.

## 6. Web UI
- [ ] 6.1 Add the default skill mode and the full chain stop stage to `ProjectModal`.
- [ ] 6.2 Make the skill mode editable in the skill editor, replacing the read-only badge in
  `SkillsView.tsx`, including a "no opinion" value that falls through to the project.
- [ ] 6.3 Add the one-off autonomous / interactive launch entries to the card `...` menu and
  send the override with the advance request (`advanceTask`, `runSkill`).
- [ ] 6.4 Add the same one-off choice to the task detail modal's skill launcher.
- [ ] 6.5 Surface a refused autonomous launch as a readable error on the card and in the modal.

## 7. Desktop app
- [ ] 7.1 Add the mode control to the "Launch" dialog (`desktop/src/main.js`, `browseTasks`) and
  to the "Relaunch" dialog, defaulting to "use the configured mode".
- [ ] 7.2 Carry the value through `launchServerTask` (`desktop/electron/preload.cjs`), the
  `launch-server-task` IPC handler (`desktop/electron/main.cjs`), the `/desktop/tasks` POST input
  and the body `desktopTasks` forwards (`cmd/agent/agent_desktop.go`).
- [ ] 7.3 Leave the footer "Next: <label>" button on one click with no override, and surface a
  refusal in the footer status with the provider named.
- [ ] 7.4 Keep the run row and the output pane for an autonomous desktop run, streaming captured
  output read-only; keep stop and log export working.
- [ ] 7.5 Stop treating an autonomous run as consoleless in the run selection path
  (`desktop/src/main.js:116`), so it is not presented as a launch error.
- [ ] 7.6 Surface a refused autonomous launch as the dialog's notice text, keeping the dialog
  open for a retry in interactive mode.
- [ ] 7.7 Test the three desktop entry points: the two dialogs carry the override, the footer
  button does not.

## 8. Documentation
- [ ] 8.1 Update `README.md` and `docs/CAPABILITIES.md` with the two project settings, the
  precedence rule, the renamed full chain action, and the list of providers supporting a
  headless run.
- [ ] 8.2 Update `docs/API_AND_DATA_SPEC.md` for the new project fields, the `run-skill` mode
  override and the `/desktop/tasks` payload.
