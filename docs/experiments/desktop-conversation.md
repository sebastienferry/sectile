# Claude Code Desktop conversation prototype

The `feat/desktop-conversation-test` branch adds a conversation view for Claude,
off by default. **Settings → Appearance → Claude consoles** switches between
**Terminal** (the PTY, unchanged) and **Conversation**, a workstation setting
kept in the desktop's `settings.json` as `consoleView`.

In Conversation mode, a **Project prompt** with Claude opens in this view in the
project's local repository, and **Claude chat (test)** appears in the execution
toolbar. Select an execution with a local directory first: the action opens an
independent local free console in that directory. It does not resume the
original PTY session or advance a ticket's workflow. Codex, other engines, and
catalogue engines with a custom launch command keep the terminal, as does an
agent older than this view, which ignores the request. Skill executions always
run in the terminal. A conversation does not wait for a busy shared checkout.

## Run and try

Install and authenticate Claude Code before starting Sectile. Run `make run`,
turn on Conversation in Appearance, then open a Claude **Project prompt**, or
select an execution and click **Claude chat (test)**, and send a message. Messages
and tool calls appear as structured events. Tool details can be expanded.
Headings, fenced code blocks, inline code and bold text have basic formatting;
HTML from engine output is always displayed as text. The view polls the local
agent while selected; no transcript is sent to the Sectile server.

The prototype uses Claude's `acceptEdits` permission mode. Edits and actions
already allowed by Claude's local configuration can execute. Calls that would
require an interactive prompt are denied in print mode; the result's permission
denials are shown. There is no approval dialog or `AskUserQuestion` bridge yet.
Avoid running simultaneous changes in the source execution and its chat.

Each message launches `claude -p --output-format stream-json --verbose` with
`--permission-mode acceptEdits`. The prompt travels on stdin, never through a
shell. Subsequent messages pass `--resume` with the session ID Claude reported.
Output appears at assistant-message boundaries, rather than token by token.
The effort picked in the composer is sent with each message and passed as
`--effort`; **Default effort** omits the flag and leaves the CLI to decide. The
ring beside the send button shows how much of the model's context window the
latest main-thread request used: its input, cache and output tokens from the
`assistant` frame's `usage`, against the `contextWindow` the `result` frame
reports for the model the `init` frame named. Subagent requests are ignored.
The ring stays hidden until a first turn has reported a window, and neither
value survives an agent restart.
Only one message can be in flight in a conversation. A failed turn leaves the
conversation available for retry, and a missing CLI is displayed as an error.

**Stop execution** interrupts the active process tree and closes the chat.
Stop idle chats before restarting the agent. The daemon also stops its chat
children during shutdown. History uses the existing bounded run trace and run
store, with at most 2,000 retained events. After restarting the agent, the
transcript is read-only; a new chat starts a new Claude session.

## Remaining work before a general replacement for PTY

- Interactive tool approvals, questions and per-session permission selection.
- Token deltas and tool-result correlation.
- Restore a live conversation after an agent restart.
- Launch workflow skills directly in this view with the existing run context.
- Honour custom Claude launch templates, attached directories and engine profiles.
- Support providers other than Claude through a shared event contract.

See [ADR 0042](../adrs/0042-experimental-claude-conversations-use-process-pipes.md)
for the prototype's scope and process ownership.
