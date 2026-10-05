# Plan #727 - Desktop Conversation mode error

## Stack

Go only: the agent's conversation turn in `internal/agent`. No migration, no
API change, no web or desktop change. The trace format is unchanged.

## Cause

`conversationTurn` (`internal/agent/agent_conversation.go`) decodes every stdout
line into one anonymous struct whose `Event` field is a `conversationStream`, an
object. In the `ui_invalidate` frame, `event` is the string `"ui.render"`, so
`json.Unmarshal` returns a `*json.UnmarshalTypeError`. The guard
`json.Unmarshal(...) == nil && frame.Type != ""` fails and the line reaches the
last branch, which writes any non-empty line as an `error` event.

`encoding/json` already decodes every other field when it meets a type
mismatch: it skips the offending value and reports the first mismatch. The
frame is therefore fully read except for that field; only the guard throws it
away.

## Design

Target file: `internal/agent/agent_conversation.go`, the `line` callback of
`conversationTurn` (the probe callback in `probeConversationCommands` only looks
for the initialize response and is not touched).

1. **Read `event` lazily (FR4).** Declare the field as
   `Event json.RawMessage \`json:"event"\``. In the `stream_event` branch,
   decode it into a `conversationStream`; on failure leave the zero value,
   which `streamPartial` treats as a no-op and which is not `message_start`, so
   neither the draft nor `requestAt` moves (US2-3).
2. **Tolerate a field of the wrong type (FR3).** Replace the guard with:

   ```go
   err := json.Unmarshal([]byte(line), &frame)
   var mismatch *json.UnmarshalTypeError
   if (err == nil || errors.As(err, &mismatch)) && frame.Type != "" {
   ```

   A type mismatch keeps every other decoded field, so the frame goes through
   the existing dispatch. `ui_invalidate` is a `system` frame with no `model`
   and no `mcp_servers`; it only records its `session_id`, which is the
   conversation's own, and `runner.ParseReasoningLine` / `ParseToolResults`
   return nothing for it (FR5, FR6).
3. **Only non-JSON lines become error blocks (FR1, FR2).** Change the final
   branch to:

   ```go
   if strings.TrimSpace(line) != "" && !json.Valid([]byte(line)) {
       conversationWrite(run.trace, "error", line, "")
   }
   ```

   A JSON value without `type`, or that is not an object, is dropped silently,
   as the reasoning parser does. The "frame too large to display" notice is not
   JSON and keeps its rendering.
4. Update the `conversationOutput` doc comment only if it no longer matches;
   it already states "Unknown JSON stays out of the rendered chat".

`errors` is likely already imported; add it otherwise.

## Rejected alternatives

- **Special-casing `subtype == "ui_invalidate"`**: fixes this frame and leaves
  the next unexpected shape broken; the clarification ruled it out (choice 3).
- **Decoding into `map[string]json.RawMessage` then field by field**: same
  result as tolerating `UnmarshalTypeError`, with much more code.
- **Dropping every line that fails to decode, JSON or not**: would hide a real
  non-JSON error Claude Code prints on stdout, which is out of scope.

## Tests

`internal/agent/agent_conversation_test.go`, following
`TestConversationStreamsTheReplyInProgress` (POSIX fake `claude` script,
skipped on Windows, `testhome.Temp`, `conversationFixture`):

- `TestConversationKeepsUnexpectedFramesOutOfTheChat`: the fake CLI prints, in
  one turn, the ticket's `ui_invalidate` frame, a `stream_event` whose `event`
  is a string, a typeless JSON object `{"foo":1}`, a `system` frame carrying a
  `model` and a string `event`, an assistant message and a successful result.
  Assert after the turn ends: no `"kind":"error"` in the trace, exactly one
  `"kind":"assistant"`, and the conversation's session is the result's.
- Extend or add a case where the fake CLI prints a plain non-JSON line, and
  assert it still yields one `"kind":"error"` entry holding that line (FR2).

Run `go test ./internal/agent/ -run Conversation` with `GOCACHE` under
`$TMPDIR`, outside the sandbox if the HTTP fixtures need local binding.

## Changelog

`CHANGELOG.md`, `[Unreleased]` / `### Fixed`, one line, for example:

> **A conversation no longer shows Claude Code's internal messages as errors.**
> In Sectile Desktop's Conversation mode, some interface messages Claude Code
> sends for itself appeared in the chat as a red error block holding raw JSON;
> they are now ignored, like the other messages a conversation does not
> display. Upgrade the agent. (#727)
