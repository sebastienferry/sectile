# Claude Code Desktop conversation

Desktop has a conversation view for Claude, off by default. **Settings →
Appearance → Claude consoles** switches between **Terminal** (the PTY,
unchanged) and **Conversation**, a workstation setting kept in the desktop's
`settings.json` as `consoleView`.

## What opens in it

In Conversation mode:

- a task's interactive launches from Desktop: a skill such as clarify or
  implement from the launch menu, **Relaunch** or **Next step**, and
  **Discussion (no skill)**. A skill sends its command, the one the terminal
  would have typed with the dispatch's instructions, as the first message, and
  runs with the skill's model; a discussion waits for the first message. Both
  run in the task's worktree with the `SECTILE_TASK_*` environment the
  terminal would have had;
- a **Project prompt**, in the project's local repository;
- **Claude chat (test)** in the execution toolbar, an independent conversation
  in the selected execution's directory.

The engine must be a Claude one. A Claude engine with a launch template
converses with the template's model and leaves the template's other options
aside; the first notice says so. Codex and other engines keep the terminal, as
do autonomous launches, which keep their read-only trace, and launches from the
web interface. An agent older than this view ignores the request and opens the
terminal. The server never sees the view: Desktop passes it to the agent with
the launch, and the agent keeps it two minutes against the task until the
dispatch comes back.

## What it shows

Claude's replies and thinking render as Markdown, through the renderer of the
Changes panel and its rules: raw HTML stays text, images never load, and only
web and mail links open, in the browser. The user's messages show as typed.
Each reply streams as Claude writes it, ending on a caret, and settles into the
history once complete.

Each tool call is a card drawn from its arguments: an edit as a diff, a written
file as its lines, a command as the command, the todo list as a checklist,
reads and searches on one line, other tools as their arguments. What the tool
answered shows inside its card, cut at 16 KiB and 200 lines; the confirmations
of Edit, Write and TodoWrite show only when they failed. A failed call is
outlined and opens on its error, and a call still waiting for its answer says
running….

## Approvals and controls

The composer's permission mode is Claude Code's: **Ask before edits**,
**Accept edits** (the default) or **Plan mode**, from the next message; there
is no bypass. Sectile's own MCP tools, which every skill relies on, are always
allowed. A tool call the owner's Claude Code rules do not allow waits in its card
for **Allow**, **Always allow** (when Claude proposes a rule, which is then
saved where Claude says) or **Deny**, as in Claude Code; the composer reads
Waiting for your approval. The decision stays on the card. A question Claude
asks through `AskUserQuestion` shows in its card with its options, radio
buttons or checkboxes as the question allows, and a field for an answer of
one's own; **Answer** sends the answers, keyed by question text in the call's
input as Claude reads them, and **Skip** denies the call.

While Claude answers, a message sent joins the answer in progress, written on
the turn's stdin, and Claude reads it at its next request, as in Claude Code; a
message sent as the turn ends starts the next turn. **Stop answer** appears
beside **Send**, and Esc in the message box does the same: the answer stops and
the conversation stays open, the next message resuming the session.
**Terminal** opens a plain terminal window on the conversation's directory, in
the terminal the project uses, running the user's own shell; no Sectile session
is attached to it. **Stop execution** ends the conversation; a ticket
discussion then completes on the server.

The model picked in the composer, the conversation's own or one of the Claude
models of Settings, is sent with the message as `--model`. The effort is sent
as `--effort`;
**Default effort** leaves the CLI to decide. The ring beside the send button
shows how much of the model's context window the latest main-thread request
used. **Add folder…** attaches a folder to the project, and Claude is given it
from the next message. A message joining an answer in progress keeps that
turn's effort and folders.

## How it runs

Each message starts `claude -p --input-format stream-json --output-format
stream-json --verbose --include-partial-messages --permission-mode <mode>
--permission-prompt-tool stdio --allowedTools=mcp__sectile`, with `--resume`
once Claude has reported a session, `--model`, `--effort` and one
`--add-dir=<path>` per folder of the project, read afresh for each message
(#676). Nothing goes through a shell. The agent writes an `initialize` request
and the message on stdin, answers Claude's `can_use_tool` requests there,
writes there any message sent meanwhile, and closes stdin on the result, three
seconds later when a message was sent after Claude's last request. See
[ADR 0047](../adrs/0047-claude-conversations-speak-the-streaming-input-protocol.md).

History uses the bounded run trace and run store, with at most 2,000 retained
events; a reply in progress is never stored. After an agent restart the
transcript is read-only, and a new conversation starts a new Claude session.
Stop idle conversations before restarting the agent.

## Remaining work

- Tell an owner who looked away that a tool call waits for them.
- Restore a live conversation after an agent restart.
- Support providers other than Claude through a shared event contract.

See [ADR 0042](../adrs/0042-experimental-claude-conversations-use-process-pipes.md)
for the prototype's original scope and process ownership.
