# Design

## Context
Three places already hold a notion of "interactive", none of them connected:

- `StageStep.Interactive` (`internal/db/board.go:388`) — `false` on all five steps, never read;
  `internal/handlers/handlers.go:2083` enqueues the step without looking at it.
- `SkillEditorEntry.Interactive` / `StageSkill.Interactive` (`internal/models/models.go:422`,
  `internal/db/skilltemplates.go:30`) — only `refine_macro` is `true`; exposed read-only to the
  UI as a badge (`web/src/components/SkillsView.tsx:141`).
- `CompleteInteractiveStep` (`internal/db/interactive.go:22`) — the transition path an
  interactive session uses, reached through `/api/tasks/{id}/advance/confirm`.

Launching is unconditionally interactive: `agentCommandLine`
(`cmd/agent/agent_config.go:293`) emits `claude '<prompt>'`, `codex '<prompt>'`, `agy -i`,
`cursor agent`, and `wrapRun` / `startControlledCommand` (`cmd/agent/agent_run_unix.go:16`) put
the process in the foreground of a TTY.

## Decisions

### Mode resolution lives on the server, not on the agent
`resolveMode(override, skill, project)` is computed where the job is enqueued
(`internal/db`), and the resolved value travels to the agent as one more field on
`agentconfig.Dispatch`. The agent applies what it is told; it never re-reads project settings to
decide. Rationale: the agent already fetches execution settings separately from launch intent
(`internal/agentconfig/config.go:37`), and a mode decided in two places is exactly the failure
the existing dead booleans illustrate.

**Rejected**: resolving on the agent from its local config copy. It would let a stale agent
config silently open a window inside an autonomous chain.

### One source of truth for the per-skill setting
The existing `SkillEditorEntry.Interactive` / `StageSkill.Interactive` becomes the per-skill
setting and gains a write path in the skill editor. `StageStep.Interactive` is removed: a step
is the pair (stage, skill), and the skill already carries the mode.

**Rejected**: introducing a fresh `mode` field and leaving both booleans alone. It would make
three names for one concept and require a migration that only moves the confusion.

### Tri-state per-skill setting
The precedence "skill setting, then project default" needs a skill with *no* opinion to fall
through to the project. A bare `bool` cannot express that: `false` is indistinguishable from
unset. The per-skill value is therefore a nullable/ternary value (`""` | `interactive` |
`non_interactive`) in the model and the persisted JSON, with the existing `Interactive: true`
on `refine_macro` read as `interactive`.

**Rejected**: keeping `bool` and treating `false` as "project decides". `refine_macro` would be
the only skill that could ever pin a mode, and no skill could pin non-interactive against a
project default of interactive.

### Headless invocation per provider
A `headlessCommandLine(provider, prompt)` in `cmd/agent/agent_config.go` mirrors
`agentCommandLine` and covers only what the repository attests
(`consoleCommand`, `cmd/agent/agent_console.go:14`): `claude -p`, `codex exec`, and `vibe -p`
which is already headless today. Every other provider returns an error, which the launch turns
into the explicit refusal. A custom `AICommandTemplate` wins over provider defaults, as it does
today, unless it carries a mode placeholder — the template author owns the mode otherwise.

**Rejected**: guessing a `-p` flag for `agy` / `gemini` / `cursor`. An unsupported flag either
fails opaquely or is swallowed as prompt text; adding a provider to the supported set later is
a one-line change once its headless mode is verified.

### The run path forks before the TTY, not inside it
A non-interactive run does not call `startControlledCommand` at all: no PTY, no `TIOCSPGRP`, no
window. Output is captured from the process pipes and streamed onto the run activity, the same
channel the agent already uses for run output. The interactive path is untouched.

### Transitions
A non-interactive step is transitioned by the worker when the run ends, as headless steps are
already assumed to be (`internal/db/interactive.go` comment). `/api/tasks/{id}/advance/confirm`
and `CompleteInteractiveStep` stay reserved for interactive runs; confirming a step whose run was
non-interactive is a no-op the worker has already handled.

### Autonomous stop stage
`AutonomousStopStage` stops being a constant and becomes a project field read by
`EnqueueAutonomousRun` (`internal/db/db.go:4138`) and by whatever chains the next step. The
constant is kept as the fallback value for a project with nothing stored, so existing projects
do not change behaviour. Only `implemented` and `reviewed` are accepted; anything else reads as
`reviewed` rather than erroring, so a bad stored value cannot wedge a board.

## Risks
- **Output volume**: a headless run streams everything the CLI prints into one activity. If the
  existing activity output has no bound, a long run can produce a large record. Cap or truncate
  at the capture site rather than letting the record grow unbounded.
- **Autonomous chain on an unsupported provider**: refusing at the first step is correct but
  visible only in the run error; make sure the refusal names the provider, since the user did
  not choose the mode explicitly in that case.

## Open questions
None blocking. One deliberate reservation carried from clarification: if `agy`, `gemini` or
`cursor` turn out to have an attested headless mode, adding them to the supported set is a
local change that does not alter any requirement here.
