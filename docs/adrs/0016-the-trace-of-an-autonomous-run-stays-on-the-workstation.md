# ADR 0016: The trace of an autonomous run stays on the workstation

Status: Accepted

## Context

An autonomous run has no terminal on purpose: `startHeadlessRun` launches the
engine with no PTY, and the desktop answered its selection with one sentence —
"Autonomous execution: no terminal to answer." That sentence is accurate and
useless. For the ten or forty minutes the run lasts, the pane says nothing, the
task activity receives the answer only at the end, and a run stuck on a tool that
will never return looks exactly like a run that is working.

Claude can be asked to say what it is doing: with `--output-format stream-json
--verbose` it prints one JSON object per line — the prose it writes, the tools it
calls, then a result message carrying the answer.

Taskativ solved this before Sectile and carried the reasoning from its runner to
a browser: a `task_reasoning` table, a `/api/tasks/:id/reasoning?after=` cursor
endpoint, a SQLite migration and a read-only panel on the ticket. The question
here was whether to port that too.

## Decision

**The trace is shown in the desktop application, and never leaves the
workstation.** Sectile's desktop is the local agent's own UI: it already attaches
to a run over `ws://<agent>/desktop/terminal?id=<runId>` and writes what it
receives into xterm. Serving the trace there costs one handler on a route that
already exists, and none of the server-side storage: no table, no migration, no
cursor endpoint, no polling. The trace lives in the agent's memory, bounded at
2000 rendered lines, and is forgotten with the run.

**The stream is parsed in the agent**, with `internal/runner/reasoning.go` — a
port of Taskativ's reader, kept as it is because its comments record what was
measured rather than assumed. The agent renders each event into a terminal line,
so the desktop keeps doing the only thing it does with an attached socket.

**The engine is asked for the stream only where its output is read.** The options
go on the default `claude` headless command line, gated by an engine check.
Interactive launches, the other engines and every configured command template
keep exactly the command line they had.

**The task activity keeps recording what it recorded before**: the result
message's answer, plus any line that is not part of the protocol — standard error
shares the pipe, and that is where a failed run explains itself. No protocol
frame reaches it, so the activity view, the specification report and the skill
result are untouched.

## Consequences

- The trace is visible in the desktop application of the machine that ran the
  skill, and nowhere else. A person watching the web board sees what they saw
  before: the activity, with the answer at the end.
- Surfacing it on the board would need the server-side storage this decision
  drops. That is a separate change, and the cost is the honest reason it was not
  made here rather than a limitation of the reader.
- The trace is a courtesy and the run is the work: an unreadable line is
  dropped, a watcher that stops reading is disconnected, and nothing about the
  trace can slow down or fail the run it describes.
- A desktop connected to an older agent receives no trace flag on its runs and
  keeps showing the notice, which is the contract between two components that are
  versioned apart.
- A future Claude version that renames or drops `stream-json` turns every line
  into an unrecognised one: the run still completes and its answer still reaches
  the activity, with the protocol frames as noise beside it until the reader is
  updated.
