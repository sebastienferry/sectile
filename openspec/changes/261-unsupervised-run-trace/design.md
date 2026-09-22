# Design

## Context
The clarification settled the two reversible choices this design would otherwise have to make: the
stream is parsed **in the agent**, and the trace is shown **in the desktop application**. Both are
recorded on the ticket and are the starting point here, not a decision to revisit.

What follows is the consequence of the second one. Taskativ carried the reasoning from the runner to
the browser, so it needed a `task_reasoning` table, a `/api/tasks/:id/reasoning?after=` cursor and a
SQLite migration. Sectile's desktop is the local agent's own UI: it already attaches to a run over
`ws://<agent>/desktop/terminal?id=<runId>` (`desktop/electron/main.cjs:243`) and pipes the bytes it
receives into xterm. The trace never has to leave the machine, and none of that server-side storage
is built.

## Decisions

### D1 — The streaming flags go on the default `claude` headless command line only
`headlessCommandLine` (`internal/agent/agent_config.go:406`) is the one place that builds a command
line Sectile owns for an autonomous run: a configured template returns earlier in `modeCommandLine`,
and a dedicated autonomous template earlier still in `launchCommandLine`. Appending
`--output-format stream-json --verbose` in the `claude` branch therefore reaches exactly the launches
whose output this change knows how to read, and nothing else.

The gate is an engine check, `engineStreamsReasoning(provider)`, in the spirit of Taskativ's
`EngineStreamsReasoning`: it is the function a second engine is added to the day its stream format is
attested, and until then the answer for `agy`, `codex`, `vibe`, `gemini`, `cursor` and `custom` is
no. It is deliberately not a user setting — an engine that does not stream must never be handed the
flags, and one that does costs nothing when nobody is watching the pane.

Rejected: a `StreamReasoning` setting like Taskativ's. It doubles the states a run can be in for no
decision the user actually wants to make, and the failure it protects against — the flags on an
engine that cannot read them — is already prevented by the engine check.

### D1bis — A run is traced when its command line asks for the stream
The supervisor has to know whether the output it is about to read is a stream of
JSON objects or an answer. It reads that off the command line that is about to
run (`commandReadsReasoning`), not from the provider: the decision was made while
building that line, through branches a configured template and a dedicated
autonomous command leave early, so asking the provider again at the far end would
answer for a branch that was not taken. It also makes the right answer for a
template that asks for the stream itself — its output *is* that stream.

Rejected: threading a boolean back through `headlessCommandLine`,
`modeCommandLine`, `launchCommandLine` and `dispatchCommand`. Four signatures and
their call sites changed to carry a fact the artefact already states.

### D2 — The parser is a port, in `internal/runner`
`internal/runner/reasoning.go` is Taskativ's file, kept as it is: the same three event kinds, the
same ordered `toolDetailKeys`, the same clamping, the same rule that a line it cannot read yields
nothing and no error. It is pure and depends only on the standard library, and `internal/agent`
already imports `tasks/internal/runner` (`agent_headless.go`), so it costs no new coupling. The
server never imports it, which keeps the runtime boundary of `.agents/MEMORY.md` §1 intact.

Porting rather than rewriting is the point: its comments record what was measured rather than
assumed — that Claude's thinking blocks come back empty with only a signature, which is why the
`thinking` kind exists but does not fire today. That knowledge is worth more than the code.

### D3 — The trace is rendered in the agent, into terminal lines
The pane on the other side is xterm. Sending it structured events would mean a new IPC channel, a new
renderer module and a second place where "one line per tool call" is decided; sending it rendered
lines means the desktop keeps doing the only thing it does today with an attached socket — write the
bytes into the terminal.

So the agent renders each event once:

- `text` — the prose as it arrives, wrapped in nothing, ending `\r\n`.
- `tool` — one line, `▸ <tool> · <detail>`, dimmed, with the detail omitted when there is none.
- `thinking` — one dim line, for the day the API surfaces it.

Rejected: JSON events to the renderer with the rendering in `main.js`. It buys a styled pane the
design does not need yet, and costs a new preload channel, a new module and a divergence risk
between what the pane shows and what a future second consumer would.

### D4 — The trace is served on the existing attach route
`/desktop/terminal?id=<runId>` currently answers 409 "Console is not ready" when the run has no PTY
session (`internal/agent/agent_desktop.go:261-275`). For a headless run holding a trace, it upgrades
the connection itself instead, replays the buffered lines, then streams the new ones. The desktop's
`attach` IPC handler, its socket, its `terminal-output` channel and its `onOutput` subscription are
untouched.

Read-only is enforced on the agent side rather than by not wiring the input: `terminal.onData` in
`desktop/src/main.js:76` is global, and a renderer-side condition would be one `if` away from sending
keystrokes into a run that has no ear. The trace socket reads frames only to notice the client
leaving, and discards their content.

Rejected: a sibling `/desktop/trace?id=` route. It is the same handler behind a second URL, and it
would force the renderer to decide which of the two to open — which is exactly the decision the run's
`trace` flag already carries.

### D5 — The activity keeps receiving what it receives today
With the streaming flags on, the engine's stdout is JSON lines instead of the final answer, and
`superviseHeadlessRun` must not post those to `postRunOutput`: the activity, the specification report
in the task detail and `skill-result.mjs` all read that text, and filling it with protocol frames
would break three surfaces to enrich one.

A traced run therefore reads its output line by line and routes each line:

- a line the parser recognises as reasoning → the trace, never the activity;
- the `result` line → its answer text goes to the activity, which is what `claude -p` prints today;
- any other line → the activity unchanged. This is the important one: stderr shares the pipe
  (`agent_headless.go:44`), a crash, a missing binary or a shell error arrives as plain text, and
  that is where a failed run explains itself.

An untraced run keeps the current byte-level path exactly: no parsing, no line buffering.

### D6 — The buffer is bounded, per run, and dies with the run
The trace lives on `controlledRun` beside `desktop`, so it is forgotten when the agent forgets the
run — on restart, or when the user clears the history. It is capped (2000 lines, oldest dropped), for
the same reason the terminal session history is: a run printing a large file must not grow the
daemon's memory without limit.

### D7 — Nothing about the trace may fail a run
The rule Taskativ's parser already states, applied to the whole path: an unparseable line is ignored,
a rendering that produces nothing is not sent, a websocket write error drops that subscriber alone,
and a subscriber that stops reading is disconnected rather than allowed to block the supervisor — the
send is non-blocking on a buffered channel. The supervisor's job is to finish the run.

### D8 — An agent without the trace degrades to today's notice
The desktop reads a new `trace` field on the run. An older agent does not send it, so
`needsConsoleNotice` keeps returning `true` for its headless runs and the pane keeps showing the
sentence it shows today. The desktop is versioned separately from the agent and the pair is routinely
mismatched; the degradation is the contract, not an accident.

## Risks
- **The flags change what stdout is.** A Claude version that drops or renames `stream-json` would
  turn every line into "any other line" (D5) and post the raw frames to the activity. Bounded: the
  run still completes, the answer is still in the text, and the fix is one branch in the parser.
- **A very chatty run.** The cap in D6 bounds the memory; the pane is a terminal and scrolls.
