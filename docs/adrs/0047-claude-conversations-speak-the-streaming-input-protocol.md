# ADR 0047: Claude conversations speak the streaming input protocol

- Status: Proposed
- Date: 2026-10-01
- Pull request: [#696](https://github.com/sebastienferry/sectile/pull/696)
- Extends: [ADR 0042](0042-experimental-claude-conversations-use-process-pipes.md)

## Context

ADR 0042 trialled a Claude conversation in Desktop: one `claude -p` process per
message, the prompt on stdin, the session resumed with `--resume`. It left
open what a conversation needs to replace the terminal for real work: tool
approvals, interruption, token deltas, workflow launches.

#696 opens a task's interactive launches (a skill such as clarify, or a
discussion) in the conversation view. A skill run calls tools its owner's
Claude Code rules do not allow, starting with Sectile's own MCP tools. In print
mode those calls were denied with no way to grant them, so the skill could not
report its run. The goal is the experience of Claude Code itself: a tool call
the rules do not allow waits for the owner, who allows it once, always, or not
at all.

Claude Code answers that through the protocol the Claude Agent SDK uses:
`--input-format stream-json` keeps stdin as a JSON line channel, and
`--permission-prompt-tool stdio` makes Claude send a `control_request` of
subtype `can_use_tool` (tool, input, `tool_use_id`, suggested rules) and wait
for a `control_response` on that stdin. The same channel carries an
`interrupt` request. This was checked against the CLI on 2026-10-01, with the
user's settings ignored so that the request fires.

## Decision

- **A turn is still one process, now spoken to over stdin.** The agent writes
  an `initialize` control request and the user message as JSON lines, keeps
  stdin open while the turn runs, and closes it on the `result` frame, which
  lets Claude exit. The next message starts a new process with `--resume`, as
  before. Effort, model and folders keep being chosen per message without any
  restart logic.
- **Approvals wait in the agent.** A `can_use_tool` request is held on the
  conversation with its id; `GET /desktop/conversation` lists the pending ones
  and `POST /desktop/conversation` with `{"approval":{"id","decision"}}`
  answers `allow` (with the input unchanged), `always` (with Claude's
  suggested rules as `updatedPermissions`) or `deny`. The decision is written
  to the trace, so the history shows it. Requests of other subtypes are
  answered with an error at once, so Claude never waits on them. Pending
  requests are dropped, with a notice, when the turn ends.
- **Sectile's MCP tools need no approval** (`--allowedTools=mcp__sectile`):
  they are the contract of every skill.
- **An interrupt is asked of Claude first.** The `interrupt` control request
  ends the turn and keeps the conversation; a turn still running five seconds
  later is stopped as a stop always is.
- **Desktop draws a request in the card of the tool call it concerns**, matched
  by `tool_use_id`, with Allow, Always allow (when Claude proposes a rule) and
  Deny.

## Consequences

- A skill run in a conversation reaches its tools the way it would in Claude
  Code, under the owner's own rules; nothing is allowed that the owner did not
  allow, apart from Sectile's tools.
- **A message sent while Claude works joins the turn** on its stdin, and
  Claude reads it at its next request, as Claude Code does (checked against
  the CLI: both messages were answered under one result). Stdin then stays
  open three seconds past the result when the message came after Claude's
  last request, so a message not yet read is not cut off. A message arriving
  once stdin has closed is kept and starts the next turn. A joined message
  keeps the turn's effort and folders.
- A turn still pays one CLI start per message that starts one. A persistent
  process per conversation, on the same protocol, remains possible; it needs
  a restart policy for a change of effort or folders and crash recovery, which
  this decision avoids.
- A request whose answer never comes holds the turn: Stop answer or Stop end
  it. No notification tells an owner who looked away that a tool call waits.
- `AskUserQuestion` arrives as an approval too. It is answered with the
  decision `answer`: the call is allowed with its input plus `answers`, keyed
  by question text, which is what Claude reads (checked against the CLI). The
  agent takes answers for that tool only, one non-empty answer per question
  asked.
- Rejected: `--permission-mode bypassPermissions` for conversations (it would
  run whatever the skill asks, with no owner in the loop); an MCP permission
  tool served by the agent (one more MCP surface for what stdin already
  carries).
