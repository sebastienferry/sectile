# Show what an unsupervised run is doing, while it does it

## Why
An autonomous run has no terminal on purpose: `startHeadlessRun` launches the engine with no PTY,
and the desktop answers the selection with one sentence — "Autonomous execution: no terminal to
answer. Its output is recorded on the task activity." (`desktop/src/run-console.mjs:17`). That
sentence is accurate and useless: for the ten or forty minutes the run lasts, the pane says nothing,
the activity receives the final answer only at the end, and the only way to know whether the run is
working or stuck on a tool that will never return is to wait for it to finish.

Taskativ solved this before Sectile: asked for `--output-format stream-json --verbose`, Claude prints
one JSON object per line as it works — the prose it writes and the tools it calls — and
`internal/runner/reasoning.go` turns those lines into renderable events. The work here is to port
that reader and give the desktop something to show in the pane it currently fills with a notice.

## What Changes
- Ask Claude for its reasoning stream on a headless launch, and only there: the streaming flags are
  appended to the default `claude` headless command line, gated by an engine check, so `agy`,
  `codex`, `vibe` and every configured command template keep their exact current command line.
- Port Taskativ's stream reader into `internal/runner/reasoning.go`: one line in, the events it
  carries out — the assistant's prose, the tool calls with the argument that says what they were
  about — plus the final answer when the line is the result message.
- Render those events in the agent into terminal lines and hold them per run in a bounded buffer, so
  a desktop attaching halfway through a run sees what happened before it attached.
- Serve the trace on the attach channel the desktop already uses: `/desktop/terminal?id=<runId>`
  upgrades to a read-only trace socket for a headless run that has one, instead of refusing with
  "Console is not ready". Input frames from that socket are ignored — nobody is answering this run.
- Show the trace in the desktop console pane: a headless run carrying a trace is attached to rather
  than answered with the notice, and the pane is not focused, because there is nothing to type.
- Keep the activity exactly as it is: the engine's diagnostics and its final answer are posted
  through `postRunOutput` as today; the reasoning events are not, and the server, the web board and
  the skill result are untouched.

## Impact
- New: `internal/runner/reasoning.go` and its tests (port), `internal/agent/agent_trace.go` (the
  per-run buffer, the subscribers and the line rendering), a `trace` field on the desktop run.
- Changed: `internal/agent/agent_config.go` (the claude headless command line),
  `internal/agent/agent_headless.go` (a traced run parses its output instead of forwarding it raw),
  `internal/agent/agent_desktop.go` (the attach route serves the trace), `desktop/src/run-console.mjs`
  and `desktop/src/main.js` (attach instead of the notice), `docs/` and `.agents/MEMORY.md`.
- Unchanged: the run lifecycle, `start_run` / `finish_run`, stage transitions, the tracker, the web
  interface, and every interactive launch.

## Out of Scope
- Surfacing the trace on the web board. It would need the server-side storage this design
  deliberately drops (Taskativ's `task_reasoning` table and its cursor endpoint); the trace is local
  to the workstation that ran the skill. A separate ticket if it is ever wanted.
- The stuck run-state defect noted at the end of the clarification report. Filed separately.
- Engines other than Claude. None of the others is attested to have a reasoning stream, and guessing
  a flag would break the launch rather than enrich it.
- A user setting to turn the stream off. The trace costs nothing when nobody watches it, and the
  engine check is the only gate.
