# Specification #727 - Desktop Conversation mode error

- Ticket: https://github.com/sebastienferry/sectile/issues/727
- Branch: `feat/727`
- Clarification: `docs/clarifications/727.md` (round 1, no product question)
- Framework: Spec Kit
- Type: fix

## Summary

In Sectile Desktop's Conversation mode, Claude Code can print a well-formed
JSON frame that Sectile does not expect, such as

```json
{"type":"system","subtype":"ui_invalidate","event":"ui.render","uuid":"b1c51aa9-74b3-4db0-9710-8496455ac164","session_id":"209887cc-9814-45f6-a711-c66f1dde5d22"}
```

The chat shows it today as a red error block holding the raw line. After the
fix, such a frame stays out of the chat, as every other frame Sectile does not
read already does, and the frames Sectile does read keep working when one of
their fields has an unexpected shape.

## Scope

In scope: how the conversation turn reads the lines Claude Code prints on
stdout, a regression test, and the changelog.

Out of scope:

- Rendering `ui_invalidate`, or any other new system frame, as a chat message.
- How a stdout line that is not JSON at all is shown (it stays an error block).
- The headless run parser, the approval parser and the reasoning parser, which
  already ignore a frame they cannot decode.
- The web app and the desktop front end: no change to the trace format.

## Vocabulary

- **Frame**: one line Claude Code prints on stdout in stream-JSON mode.
- **JSON frame**: a frame that is syntactically valid JSON.
- **Non-JSON line**: a non-empty stdout line that is not valid JSON.
- **Error block**: the red chat entry the trace records with kind `error`.
- **Known field**: a frame field the conversation reads (`type`, `session_id`,
  `is_error`, `model`, `permission_denials`, `parent_tool_use_id`,
  `message.usage`, `modelUsage`, `event` on a `stream_event`, `mcp_servers`).

## User stories

### US1 (P1) - An unexpected frame leaves the chat untouched

As someone talking with Claude in Sectile Desktop, I see only Claude's
messages, tool calls and real errors, never a raw JSON line Claude Code emitted
for its own interface.

**Acceptance**

1. **Given** a conversation turn, **when** Claude Code prints the
   `ui_invalidate` frame quoted in the summary, **then** no error block is
   added to the chat and the turn goes on as if the frame had not been printed.
2. **Given** a conversation turn, **when** Claude Code prints a JSON frame of a
   type Sectile does not read, whatever the shape of its fields, **then** no
   error block is added to the chat.
3. **Given** a conversation turn, **when** Claude Code prints a line that is not
   JSON, **then** the chat shows it as an error block, as today.

### US2 (P1) - Known fields survive an unexpected one

As someone talking with Claude, the conversation keeps its session, model,
context and MCP status even when a frame carries a field Sectile reads under
another shape.

**Acceptance**

1. **Given** a `system` frame whose `event` is a string, **when** it also
   carries a valid `session_id`, `model` or `mcp_servers`, **then** those values
   are read as they would be without the `event` field.
2. **Given** a `stream_event` frame whose `event` is an object, **when** it is
   read, **then** the draft of the reply in progress is built as today.
3. **Given** a `stream_event` frame whose `event` is not an object, **when** it
   is read, **then** the draft is left unchanged and no error block is added.

### US3 (P2) - The fix is announced

**Acceptance**

1. **Given** the release notes, **when** the next version ships, **then**
   `CHANGELOG.md` holds one `Fixed` line under `[Unreleased]` saying the
   conversation no longer shows some of Claude Code's internal messages as an
   error.

## Functional requirements

- **FR1**: A JSON frame never produces an error block with its raw text.
- **FR2**: A non-JSON line, empty or whitespace-only lines excepted, produces an
  error block holding the line, as today. The "frame too large to display"
  notice keeps its current rendering.
- **FR3**: A field whose JSON type differs from the one Sectile expects does not
  prevent the other known fields of the same frame from being read.
- **FR4**: The `event` field is interpreted only on `stream_event` frames; on any
  other frame type its value is ignored.
- **FR5**: Frames of the types Sectile reads today (`control_request`,
  `control_response`, `stream_event`, `assistant`, `system`, `result`, and the
  `user` tool results) keep their current behaviour.
- **FR6**: `ui_invalidate` gets no dedicated handling and no chat entry.
- **FR7**: `CHANGELOG.md` carries one `Fixed` line under `[Unreleased]`.

## Edge cases

- A JSON frame with no `type`, or a JSON value that is not an object (`[]`,
  `"text"`, `42`): it is JSON, so it is dropped silently (FR1).
- A known field other than `event` with an unexpected type (for example
  `model` as a number): the frame is not shown raw (FR1) and the other known
  fields are still read (FR3).
- A `ui_invalidate` frame carrying the conversation's `session_id`: the session
  is recorded as from any other `system` frame, which leaves it unchanged.

## Success criteria

- The regression test feeding the ticket's frame through a conversation turn
  finds no `"kind":"error"` entry in the trace, and the turn ends normally.
- The existing conversation tests pass unchanged.

## Open points

None. The clarification settled every choice; see its "Technical choices"
section.
