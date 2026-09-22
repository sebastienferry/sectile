# Design

## Context
An autonomous launch runs `bash -lc <line>` with no PTY, no stdin and its own session
(`startHeadlessRun`). Stderr is merged into stdout, the pipe is drained into a buffer, and
`superviseHeadlessRun` posts whatever accumulated every 3 seconds to
`/api/v1/agent/run-output`, which appends it to the run activity (256 KiB cap). That transport
works and is not touched here.

Two things break around it:

- The AGY preset ends in `jq -rs 'map(select(.event == "result"))[0].result.response'`. `-s`
  reads the entire stream into an array before the program runs, so nothing is written until
  the CLI exits, and the filter keeps only the terminal event. The Claude preset has no
  `--output-format`, so it prints one answer at the end and nothing before.
- The desktop registers the run with `Headless: true` and no `sessionId`, so
  `needsConsoleNotice` short-circuits `select()` to a static line and `api.detach()`. There is
  no code path that puts a headless run's bytes on screen.

## Goals
- A user watching an autonomous run sees readable progress within a flush interval.
- A crashed or cancelled run shows everything it printed before it died.
- Adding a fourth provider is a preset, not a Go change.
- A missing `jq` reads as a stated failure, not as an empty panel.

## Decisions

### D1. The event stream is rendered by `jq` in the preset, not by Go in the agent
Settled by the owner in clarification round 3, reversing round 2's recommendation. What stays
agnostic is the agent: `superviseHeadlessRun` keeps moving bytes and knows no provider's
schema. A Go renderer would add one provider-specific branch per CLI inside the agent, which is
the coupling being avoided, and a new CLI would need a Sectile release.

Stated plainly: the filters are *not* shared. `claude`, `agy` and `codex` emit different event
schemas, so each preset carries its own `jq` program. The agnosticism is in the mechanism.

*Rejected:* parsing NDJSON in `superviseHeadlessRun`. It would remove the `jq` dependency and
be unit-testable per provider, but it puts provider knowledge in the agent.

### D2. Every filter obeys the same four rules
1. `--unbuffered` and no `-s`. `-rs` is the defect; it must not reappear in any preset.
2. Render every event, not only the terminal one. A `select(.event == "result")` filter prints
   nothing until the end and nothing at all on a crash.
3. Read with `-R` and recover with `fromjson?`, so a non-JSON line (a warning on the merged
   stream) is printed verbatim instead of aborting the pipeline and blanking the panel.
4. Wrap the per-event rendering in `try … catch .`, so an unrecognised event shape degrades to
   its raw line rather than killing `jq` mid-run.

Starting shape, one line per event with the final answer set apart:

```
jq --unbuffered -Rr 'try (fromjson
  | if .type=="result" or .event=="result"
    then "\n─── result ───\n" + ((.result.response // .result // .) | if type=="string" then . else tostring end)
    else "[" + ((.type // .event // "event")|tostring) + "] " + ((.message.content[0].text? // .text? // .delta? // "") | if type=="string" then . else tostring end)
    end) catch .'
```

This is a starting point, not a verified expression: the event schemas belong to external CLIs.
Implementation validates each filter against a real recorded stream per provider (task 1.1) and
adjusts the field paths. What the spec fixes is the behaviour, not the field names.

### D3. The Claude and Codex presets ask for an event stream
`claude -p` gains `--output-format stream-json`. Note for implementation: that flag is refused
without `--verbose` on current Claude CLI versions, so the preset carries both if the check in
task 1.1 confirms it. `codex exec` gains its JSONL flag, confirmed the same way. If a provider's
CLI turns out to have no event stream, its preset keeps its current shape and the reason is
recorded in the change — a preset that cannot stream is not made worse.

### D4. `jq` is checked before the launch, not diagnosed after it
`jq` becomes a runtime prerequisite of an autonomous run. The agent, in `startHeadlessRun`,
looks for a `jq` word in the resolved command line and, when one is there, resolves `jq` on the
PATH; a failure finishes the run as `failed` with a message naming `jq`, before `bash` is
spawned.

The detection is deliberately shallow — a word-boundary match on the command line. A false
positive only demands a tool the user was about to need anyway; a false negative falls back to
the shell's own `jq: command not found`, which D5 makes visible and fatal.

*Rejected:* documenting `jq` as a prerequisite and relying on the shell error. It reaches the
user as a one-line error inside an otherwise empty run, after the run is spent.
*Rejected:* a generic "every binary in the pipeline" preflight. Parsing an arbitrary shell line
correctly is a much larger promise than this defect needs.

### D5. The headless line runs under `pipefail`
`bash -lc 'cli … | jq …'` exits with `jq`'s status. A provider CLI that dies leaves `jq` exiting
zero, and the run is recorded as completed. `startHeadlessRun` prefixes the line with
`set -o pipefail;`, so the run's status reflects the whole pipeline. This applies to any
headless line, not only the shipped presets.

### D6. The desktop reads the run's output from the agent, not from the server
The agent already holds the bytes; it is on the same machine as the desktop, and the desktop
already speaks to it for status, diffs and results. A new read-only route,
`GET /desktop/run-output?id=<run-id>&offset=N`, returns the bytes after `offset` and the new
offset. It is spelled like its sibling `/desktop/run-result` rather than as a nested path,
which is the shape every other desktop route on the loopback handler already takes.
`superviseHeadlessRun` keeps an appended transcript on the `controlledRun` alongside what it
posts, bounded by the same 256 KiB budget as the activity record and truncated head-first with
a stated marker.

`offset` counts bytes of the run's whole output, not bytes of the transcript still held. The
two part company at the first truncation: the window slides, and a position expressed in it
would point at output the pane has already shown. The agent therefore counts the bytes its head
has lost and reads the position against that count; a reader that has fallen out of the window
is served what is left, headed by the truncation marker. The marker is served with the window
rather than stored in it, so it never shifts a position by its own length.

*Rejected:* having the desktop poll the server's activity output. It adds a network hop, a
second authentication path and the server's own lag for data already present locally.
*Rejected:* pushing output over the existing console channel. That channel is a PTY session;
a headless run has none, and faking one would make the pane look typeable.

### D7. The console pane renders a headless run read-only
`needsConsoleNotice` keeps returning true for a headless run — the pane still must not attach a
PTY — but `select()` gains a branch before the notice: for `run.headless === true` it writes a
one-line banner stating the run is autonomous and takes no input, then starts a poll that
writes each new chunk into the same xterm instance. The poll is cleared when the selection
changes, when the run reaches a terminal status after its last chunk, or when the agent
disconnects. The offset is kept per run so reselecting a run does not reprint it.

Polling, not streaming: the output is already only refreshed every 3 seconds upstream, the
desktop already polls for runs and results, and a socket would add a lifecycle for no gain.

### D8. Truncation is stated on the surface that truncates
Two independent caps now exist: the server's 256 KiB on the activity, and the agent's on the
local transcript. Each prints its own marker when it drops bytes, so a user reading a shortened
transcript knows the run is not what was cut.

## What implementation settled

- **D3 confirmed.** `claude -p --output-format stream-json` is refused without `--verbose`
  ("When using --print, --output-format=stream-json requires --verbose"), so the preset carries
  both. `codex exec` takes `--json`; that one is written from the documented flag, not verified
  against an installed CLI.
- **The fixtures are one recording and two reconstructions.** `claude-stream.jsonl` is a real
  recording; `agy` and `codex` are not installed on the workstation this was written on, so
  their fixtures follow the schemas those CLIs document. Recorded in
  `web/src/lib/__fixtures__/README.md`.
- **`catch .` had to become `catch $raw`.** Inside a jq `catch`, `.` is the error jq raised, not
  the input line, so D2.3 as written printed "Invalid numeric literal ..." where the warning
  should have been. Every filter now binds `. as $raw` first.
- **The transcript route is `GET /desktop/run-output?id=…&offset=N`,** not
  `/desktop/runs/{id}/output`. The agent's desktop surface is flat and keyed by a query
  parameter (`/desktop/run-result?id=…`, `/desktop/stop?id=…`); a nested path would have been
  the only one of its shape.
- **A queued run keeps its own notice even when it is headless.** `showsHeadlessOutput` gates
  the transcript on the run being past `queued`/`preparing`, which is the order `consoleNotice`
  already reads its branches in.

## Risks
- **The filters depend on external event schemas.** A provider changing its schema degrades to
  raw lines by D2.3/D2.4 rather than to an empty panel. This is the accepted cost of D1.
- **The output is far more verbose than before.** A long run reaches the 256 KiB activity cap
  where a final-answer-only run never did. Accepted and out of scope for this change.
- **`pipefail` changes the recorded status of existing headless runs.** A run whose CLI was
  already failing behind a forgiving pipeline now reads as failed. That is the correct reading.
- **Secrets in the event stream.** An event stream carries more of the CLI's internals than a
  final answer did. Nothing here adds a new sink: the same activity record, seen by the same
  users, and the desktop transcript stays on the workstation.
