# Tasks

## 1. Model and persistence
- [ ] 1.1 Add `DefaultSkillMode` (`interactive` | `non_interactive`) to the project model
  (`internal/models/models.go`), following the `UseWorktrees` pattern, defaulting to
  `interactive` on read.
- [ ] 1.2 Add `AutonomousStopStage` (`implemented` | `reviewed`) to the project model, defaulting
  to `reviewed` on read; reject unknown values by reading them as `reviewed`.
- [ ] 1.3 Carry both through the project update path (`ProjectUpdate` and its handler) and
  through `get_project_context`.
- [ ] 1.4 Turn `StageSkill.Interactive` / `SkillEditorEntry.Interactive` into the ternary mode
  value (`""` | `interactive` | `non_interactive`), reading the current `true` on `refine_macro`
  as `interactive`; make it writable from the skill editor payload.
- [ ] 1.5 Remove `StageStep.Interactive` (`internal/db/board.go`) and every reference to it.

## 2. Mode resolution
- [ ] 2.1 Add `ResolveSkillMode(override, skill, project)` in `internal/db` implementing the
  precedence override > skill > project > interactive.
- [ ] 2.2 Unit-test the four precedence branches and the unset/unknown-value readings.
- [ ] 2.3 Add `Mode` to `agentconfig.Dispatch` and set it from the resolved value when enqueuing.

## 3. Launch path
- [ ] 3.1 Add `headlessCommandLine(provider, prompt)` in `cmd/agent/agent_config.go` covering
  `claude -p`, `codex exec`, `vibe -p`; error for every other provider.
- [ ] 3.2 Detect a mode placeholder in a custom `AICommandTemplate`; without one, refuse a
  non-interactive launch with an explicit error.
- [ ] 3.3 Branch `dispatchCommand` on the dispatched mode, selecting the headless or interactive
  command line.
- [ ] 3.4 In `wrapRun`, skip `startControlledCommand` for a non-interactive run; capture stdout
  and stderr and stream them onto the run activity, with a size bound.
- [ ] 3.5 Test both branches: headless emits the print/exec form and opens no TTY; interactive is
  unchanged.
- [ ] 3.6 Test the refusal for an unsupported provider and for a placeholder-less template.

## 4. Transitions
- [ ] 4.1 Ensure a non-interactive run is transitioned by the worker when it ends, and that a
  failed run leaves the stage untouched.
- [ ] 4.2 Keep `/api/tasks/{id}/advance/confirm` and `CompleteInteractiveStep` on the interactive
  path only; make a confirm on an already-transitioned step a no-op rather than an error.
- [ ] 4.3 Test: headless success transitions once; headless failure does not transition.

## 5. Autonomous run
- [ ] 5.1 Parameterise `EnqueueAutonomousRun` (`internal/db/db.go:4138`) on the project's stop
  stage instead of the constant; keep the constant as the fallback.
- [ ] 5.2 Force the non-interactive mode on every step an autonomous run enqueues.
- [ ] 5.3 Refuse an autonomous run when the provider has no headless mode, naming the provider.
- [ ] 5.4 Test: stops at `implemented` when configured, at `reviewed` by default, refuses on a
  task already at or past the stop stage, and refuses on an unsupported provider.

## 6. Web UI
- [ ] 6.1 Add the default skill mode and the autonomous stop stage to `ProjectModal`.
- [ ] 6.2 Make the skill mode editable in the skill editor, replacing the read-only badge in
  `SkillsView.tsx`.
- [ ] 6.3 Add the one-off interactive / non-interactive launch entries to the card `...` menu and
  send the override with the advance request.
- [ ] 6.4 Surface a refused non-interactive launch as a readable error on the card.

## 7. Documentation
- [ ] 7.1 Update `README.md` and `docs/CAPABILITIES.md` with the two project settings, the
  precedence rule, and the list of providers supporting a headless run.
- [ ] 7.2 Update `docs/API_AND_DATA_SPEC.md` for the new project fields and the advance payload's
  mode override.
