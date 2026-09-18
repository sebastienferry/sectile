# Tasks

## 1. The stream reader
- [ ] 1.1 Port Taskativ's `internal/runner/reasoning.go` into `internal/runner/reasoning.go`: the three
  event kinds, `ReasoningEvent`, `ParseReasoningLine`, the ordered `toolDetailKeys`, `toolDetail`,
  `scalar`, `clampDetail` and `shortToolName`, comments included.
- [ ] 1.2 Port its tests into `internal/runner/reasoning_test.go`: an assistant line carrying prose, a
  tool call resolving its detail from each shape (`command`, `file_path`, `Skill`, `Agent`), an empty
  thinking block yielding nothing, the result line returning the answer and `done`, a non-JSON line,
  a truncated line, an unknown message type, an MCP tool name shortened, and a detail clamped.

## 2. Asking the engine for its stream
- [ ] 2.1 Add `engineStreamsReasoning(provider string) bool` in `internal/agent/agent_config.go`,
  answering true for `claude` alone, with the comment that says what it takes to add an engine.
- [ ] 2.2 Append `--output-format stream-json --verbose` to the `claude` branch of
  `headlessCommandLine`, after the permission mode and before the model flag and the prompt.
- [ ] 2.3 Go tests in `internal/agent`: the claude headless line carries the flags in that order; the
  codex and vibe lines are unchanged; an interactive claude line carries none; a configured template
  and a dedicated autonomous template are expanded with nothing added.

## 3. Holding and rendering the trace in the agent
- [ ] 3.1 Add `internal/agent/agent_trace.go`: a `runTrace` holding the rendered lines under a mutex,
  capped at 2000 with the oldest dropped, and a set of subscriber channels.
- [ ] 3.2 Render one event into one terminal line: prose as it comes, `▸ <tool> · <detail>` for a tool
  call with the detail omitted when empty, a dim line for a thinking block, every line ending `\r\n`.
- [ ] 3.3 Publish a line to the buffer and to every subscriber with a non-blocking send on a buffered
  channel, dropping a subscriber that is not keeping up (design D7).
- [ ] 3.4 Hang the trace off `controlledRun` and create it in `registerHeadlessRun` when the launch was
  traced, so it is forgotten with the run.
- [ ] 3.5 Go tests: lines are replayed in order to a late subscriber, the buffer drops the oldest past
  the cap, a stalled subscriber is dropped without blocking the publisher, and a run with no trace
  publishes nothing.

## 4. Reading the stream on a traced run
- [ ] 4.1 Carry "this launch is traced" from the command-line decision to `startHeadlessRun`, rather
  than re-deriving it from the provider at the far end.
- [ ] 4.2 In `superviseHeadlessRun`, read a traced run's output by lines: reasoning events go to the
  trace, the result line's answer and every unrecognised line go to the pending activity buffer,
  keeping the existing flush interval and the existing final flush.
- [ ] 4.3 Leave the untraced path byte-for-byte as it is.
- [ ] 4.4 Go tests: a stream of assistant lines posts nothing to the activity and everything to the
  trace; the result line's answer reaches the activity; a plain diagnostic line reaches the activity;
  an untraced run still posts its raw bytes; a line split across two reads is assembled before being
  parsed.

## 5. Serving the trace on the attach route
- [ ] 5.1 In `internal/agent/agent_desktop.go`, have `/desktop/terminal?id=` upgrade to the trace
  socket when the run has no live PTY session but holds a trace, keeping the 409 for a run with
  neither.
- [ ] 5.2 Replay the buffered lines on connect, then stream new ones; read incoming frames only to
  notice the client leaving, and discard their content.
- [ ] 5.3 Close the socket when the run finishes and its trace is closed.
- [ ] 5.4 Add `Trace bool \`json:"trace,omitempty"\`` to `desktopRun`, set on a traced headless run, so
  `/desktop/runs` carries it.
- [ ] 5.5 Go tests: a traced headless run upgrades and receives its replay; an untraced headless run
  still gets 409; input frames change nothing; a second client gets its own replay.

## 6. Showing it in the desktop
- [ ] 6.1 `desktop/src/run-console.mjs`: `needsConsoleNotice` returns false for a headless run carrying
  a trace, and the notice text is unchanged for every other case.
- [ ] 6.2 `desktop/src/main.js`: such a run is attached to like any other, without focusing the
  terminal — there is nothing to type into it.
- [ ] 6.3 Desktop tests in `desktop/tests/run-console.test.cjs` (node --test): a headless run with a
  trace attaches, a headless run without one keeps the notice, and the queued, preparing and
  missing-session cases are unchanged.
- [ ] 6.4 UI test `desktop/tests/autonomous-trace.ui.cjs`: the real application against a stub agent
  serving a trace socket shows the streamed lines in the pane, and shows the notice when the stub
  reports no trace.

## 7. Documentation
- [ ] 7.1 `docs/CAPABILITIES.md`: the autonomous run is no longer blind.
- [ ] 7.2 An ADR recording that the trace is local to the workstation, and what surfacing it on the
  web board would cost.
- [ ] 7.3 `.agents/MEMORY.md`: the claude headless command line now carries the streaming flags, and
  what that means for anything reading a headless run's stdout.

## 8. Validation
- [ ] 8.1 `make test` (gofmt check, `go test ./...`, web tests, `tsc --noEmit`, `oxlint`).
- [ ] 8.2 `cd desktop && npm test`, and the new UI test.
- [ ] 8.3 `openspec validate 261-unsupervised-run-trace --strict`.
