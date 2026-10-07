# Desktop conversations

Desktop has a conversation view for Claude and Codex, off by default. **Settings →
General → AI consoles** switches between **Terminal** (the PTY,
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
- **Claude chat (test)** or **Codex chat (test)** in the execution toolbar, an independent conversation
  in the selected execution's directory.

The engine must be Claude or Codex. A supported engine with a launch template
converses with the template's model and leaves the template's other options
aside; the first notice says so. Other engines keep the terminal, as
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
**Accept edits** (the default), **Auto mode** or **Plan mode**, from the next
message; there is no bypass. In **Auto mode** Claude Code's classifier approves
what the rules do not cover, and an action it refuses still waits for the owner.
A new conversation starts in the workstation's **Settings → Claude settings →
Conversation permission mode**, enabled only
in the conversation view (Accept edits when unset). It is the mode of the first
turn, which matters most for a skill or a project prompt launched in the
conversation view: its command is that turn and runs at once, before the owner
could pick a mode. Desktop keeps the value in its own file as
`conversationMode` and hands it to the agent (`PUT /desktop/conversation-mode`,
announced by the `conversation-mode-default` capability), which stores it as
`defaults.conversationMode` in its `settings.json`, accepts only the four modes
above and reads anything else as Accept edits. The conversation reports the mode
it starts in, and the composer adopts it. Sectile's own MCP tools, which every skill relies on, are always
allowed. A tool call the owner's Claude Code rules do not allow waits in its card
for **Allow**, **Always allow** (when Claude proposes a rule, which is then
added to the project's Sandbox allow rules rather than to a file of the
worktree, [ADR 0048](../adrs/0048-claude-sandbox-values-reach-claude-through-a-generated-settings-file.md))
or **Deny**, as in Claude Code; the composer reads
Waiting for your approval. The decision stays on the card. A question Claude
asks through `AskUserQuestion` shows in its card with its options, radio
buttons or checkboxes as the question allows, and a field for an answer of
one's own; **Answer** sends the answers, keyed by question text in the call's
input as Claude reads them, and **Skip** denies the call. While any of them
waits, the agent sets the run's `waitingSince`, so the sidebar marks the
conversation waiting and Desktop notifies, as for a terminal; the mark goes
with the last answer. A skill that declares a wait through `report_waiting`
marks the conversation the same way, and the owner's next message answers it
and tells the server, as Enter does in a terminal.

While Claude answers, a message sent joins the answer in progress, written on
the turn's stdin, and Claude reads it at its next request, as in Claude Code; a
message sent as the turn ends starts the next turn. **Stop answer** appears
beside **Send**, and Esc in the message box does the same: the answer stops and
the conversation stays open, the next message resuming the session.
**Terminal** opens a plain terminal window on the conversation's directory, in
the terminal the project uses, running the user's own shell; no Sectile session
is attached to it. **Stop execution** ends the conversation; a ticket
discussion then completes on the server.

A message starting with `!` runs in the shell, as Claude Code's bash mode,
which print mode lacks (it reads `!` as text). The agent runs it in the
conversation's directory with the user's shell and the task's environment,
hidden on Windows and stopped after two minutes, and shows it as a Bash card
with its output. The command and its output, as `<bash-input>` and
`<bash-stdout>`, go to Claude in front of the next message, bounded to 32 KiB.

A turn whose result comes with no message of Claude's before it ran a local
command (`/usage`, `/context`, `/cost`): its output, laid out for a terminal, is
kept as `command_output` and shown as such, a status line tinted by its mark.
`/mcp` alone is not sent to Claude, whose print mode only counts the servers:
the agent runs `claude mcp list` in the conversation's directory, which checks
each server, and shows its output; Claude is not told. The composer's
**Sectile MCP** chip shows whether Claude reaches the `sectile` server: the
agent checks it with `claude mcp get sectile` when the conversation opens or
the chip is clicked, and each turn's init frame refreshes it from its
`mcp_servers`.

Typing `/` at the start of a message completes the slash commands Claude offers
in the conversation's directory, the list its `initialize` response carries.
Before the first turn the agent reads it from a Claude started for that alone:
sent the initialize request only, it answers and stops without calling any
model. Each turn refreshes the list from its own initialize.

The model picked in the composer, the conversation's own or one of the Claude
models of Settings, is sent with the message as `--model`. The effort is sent
as `--effort`;
**Default effort** leaves the CLI to decide. The ring beside the send button
shows how much of the model's context window the latest main-thread request
used. **Add folder…** attaches a folder to the project, and Claude is given it
from the next message. A message joining an answer in progress keeps that
turn's effort and folders.

## How Claude runs

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

## How Codex runs

Codex uses one `codex app-server --listen stdio://` process per conversation.
The agent sends `initialize` and `initialized`, creates a thread, discovers
models with `model/list`, skills with `skills/list`, and MCP state with
`mcpServerStatus/list`. Opening the view starts this process without inference.
Each message uses `turn/start`; an active turn receives `turn/steer` with its
expected turn id, and interruption uses `turn/interrupt`. Models, effort,
collaboration mode and sandbox policy are resolved for each new turn.

The same event contract drives Desktop for both providers: text deltas become
the draft, completed items become messages or tool cards, and requests become
pending approvals. Command and file-change decisions retain Codex's wire ids
and session scope; questions translate the shared form's answers to native
question ids. An unknown request is answered with an error immediately.

A confirmed turn-boundary rejection queues a steering message for the next
turn. Transport errors do not replay it automatically. A process failure keeps
the thread id for `thread/resume` on the next message; stopping the execution
waits for process exit. Agent-restart restoration remains read-only.

The composer uses Codex's models and supported efforts, and completes native
skills with `$`. Its permission modes are Read only, Workspace edits and Plan
mode. No Claude permission rule or hook is installed for Codex. The integration
was checked against Codex CLI 0.157.1; see
[ADR 0054](../adrs/0054-codex-conversations-use-app-server.md).

## Remaining work

- Restore a live conversation after an agent restart.
- Support additional providers through the shared event contract.

See [ADR 0042](../adrs/0042-experimental-claude-conversations-use-process-pipes.md)
for the prototype's original scope and process ownership.

Codex approval review is selected in **Settings → Codex settings**: **Ask me**
or **Approve on my behalf**. It applies to the next message, preserves sandbox
limits, and does not change the native user configuration.
