# Interactive or headless skill runs

## Why
Every skill launch opens a terminal window and puts the CLI in the foreground of a TTY
(`cmd/agent/agent_run_unix.go:16`), so a run always waits for a human even when nothing needs
to be answered. Two booleans already exist to express the opposite — `StageStep.Interactive`
(`internal/db/board.go:388`) and `SkillEditorEntry.Interactive`
(`internal/models/models.go:422`) — but neither is read at launch, so neither means anything.

The autonomous run has the symmetrical problem: it always stops at `reviewed`
(`AutonomousStopStage`, `internal/db/board.go:410`), which forces a PR on a chain someone may
want to stop right after implementation.

## What Changes
- A skill run has a mode: **interactive** (terminal window, the user answers, the user confirms
  the transition) or **non interactive** (headless CLI, no window, output streamed into the
  activity, the worker posts the transition).
- The mode is resolved per launch: one-off override from the card's `...` menu, then the skill's
  own setting, then a new project default, then interactive.
- The project gains `defaultSkillMode` (`interactive` | `non_interactive`, default
  `interactive`) and `autonomousStopStage` (`implemented` | `reviewed`, default `reviewed`).
- The `>>` autonomous run always forces the non-interactive mode and stops at the configured
  stage.
- A provider with no attested headless mode (`agy`, `gemini`, `cursor`, and a custom
  `AICommandTemplate` without a mode placeholder) refuses a non-interactive launch with an
  explicit error instead of silently falling back to interactive.
- The two dead `Interactive` booleans are reconciled: the skill-level one becomes the real
  per-skill setting, editable in the skill editor; `StageStep.Interactive` stops being a second
  source of truth.

## Impact
`internal/models` (project + skill editor entry), `internal/db` (project update, skill
templates, `board.go`, `EnqueueAutonomousRun`, the job worker), `internal/agentconfig`
(`Dispatch` gains the mode), `internal/handlers` (the advance endpoint), `cmd/agent`
(`agentCommandLine`, `dispatchCommand`, `wrapRun`), and the web UI (project modal, card `...`
menu, skill editor). Additive schema change, both new fields defaulting to today's behaviour.

## Out of scope
The web PTY terminal (`HandleTerminalWs`), the content of the skills themselves, adding an AI
provider, and changing what `/api/tasks/{id}/advance/confirm` does for an interactive run.
