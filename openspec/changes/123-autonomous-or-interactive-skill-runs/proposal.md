# Autonomous or interactive skill runs

## Why
Every skill launch opens a terminal window and puts the CLI in the foreground of a TTY
(`cmd/agent/agent_run_unix.go:16`), so a run always waits for a human even when nothing needs
to be answered. Two booleans already exist to express the opposite: `StageStep.Interactive`
(`internal/db/board.go:388`) and `SkillEditorEntry.Interactive`
(`internal/models/models.go:422`), but neither is read at launch, so neither means anything.

The chained run has the symmetrical problem: it always stops at `reviewed`
(`AutonomousStopStage`, `internal/db/board.go:410`), which forces a PR on a chain someone may
want to stop right after implementation.

## What Changes
- A skill run has a mode: **autonomous** (headless CLI, no window, output streamed into the
  activity, the worker posts the transition) or **interactive** (terminal window, the user
  answers, the user confirms the transition).
- The mode is resolved per launch: one-off override chosen for that launch, then the skill's
  own setting, then a new project default, then interactive.
- The one-off override is offered on every surface where a user explicitly triggers a skill:
  the card `...` menu and the task detail modal's skill launcher on the web, the "Launch" and
  "Relaunch" dialogs in the desktop app. The desktop footer "Next step" button stays a single
  click and uses the resolved mode.
- The project gains `defaultSkillMode` (`interactive` | `autonomous`, default `interactive`)
  and `fullChainStopStage` (`implemented` | `reviewed`, default `reviewed`).
- The `>>` action is renamed **full chain**, since "autonomous" now names a run mode. A full
  chain run always forces the autonomous mode and stops at the configured stage.
- A provider with no attested headless mode (`agy`, `gemini`, `cursor`, and a custom
  `AICommandTemplate` without a mode placeholder) refuses an autonomous launch with an
  explicit error instead of silently falling back to interactive.
- The two dead `Interactive` booleans are reconciled: the skill-level one becomes the real
  per-skill setting, editable in the skill editor; `StageStep.Interactive` stops being a second
  source of truth.

## Impact
`internal/models` (project, `RunSkillRequest`, skill editor entry), `internal/db` (project
update, skill templates, `board.go`, `EnqueueAutonomousRun`, the job worker),
`internal/agentconfig` (`Dispatch` gains the mode), `internal/handlers` (the run-skill and
advance endpoints), `cmd/agent` (`agentCommandLine`, `dispatchCommand`, `wrapRun`,
`desktopTasks`), the desktop app (`desktop/src/main.js`, `desktop/electron/preload.cjs`,
`desktop/electron/main.cjs`), and the web UI (project modal, card `...` menu, task detail
modal, skill editor). Additive schema change, both new fields defaulting to today's behaviour.

## Out of scope
The web PTY terminal (`HandleTerminalWs`), the content of the skills themselves, adding an AI
provider, and changing what `/api/tasks/{id}/advance/confirm` does for an interactive run.
