# Specification #917 - Tauri: `@` mentions for files, folders, conversations and tickets

Ticket: https://github.com/sebastienferry/sectile/issues/917
Clarification: `docs/clarifications/917.md` (rounds 1 and 2).

## Scope

In: both Sectile Desktop (Tauri) conversation composers ("New prompt" and an
active conversation, queued messages included); an agent route listing the
project's files and folders; agent-side resolution of mentions on send;
chips in the transcript. Out: Electron, web, server, slash commands (#916),
other composers, cross-project mentions.

Deviation from T2 of the clarification: the composer is the CodeMirror 6
editor introduced by #916 (ADR 0019) rather than a hand-written
`contenteditable`. The behaviour T2 describes (plain-string value, atomic
chips, Backspace, Enter/Shift+Enter, limit) is unchanged; #916's owner
decision made the editor available, which removes the regression risk T2
accepted.

## User stories

### US1 - Mention autocomplete (P1)

1. **Given** a composer, **when** the user types `@` at the start or after
   whitespace, **then** a floating listbox lists the project's files and
   folders, its conversations and its tickets, each with its own icon, the
   matched part highlighted, at most 50 entries.
2. Files and folders come from the conversation's working folder and the
   project's attached folders, without Git-ignored entries, `.git` or
   `node_modules`, and without symlinks leaving their root; they rank by base
   name prefix, then path prefix, then substring, newest `mtime` first within
   a tier, a folder's `mtime` being its newest descendant's.
3. Conversations: the project's conversations except the current one, newest
   first after the text match, archived ones marked.
4. Tickets: the project's tickets of every stage, matched on key and title,
   key prefix first. Within a tier the board's order is kept: the Tauri
   client's ticket carries no update time (deviation from T7's "most recently
   updated").
5. Per-kind caps keep every kind visible: 25 files and folders, 10
   conversations, 15 tickets.
6. **Given** an agent without the listing route, **then** the menu still lists
   conversations and tickets.
7. Keyboard and mouse as for `/` (#916): arrows, Enter/Tab, Escape (closes
   without interrupting), hover, click.

### US2 - Chips and folders (P1)

1. Picking a file, a conversation or a ticket inserts
   `@[label](sectile:<type>/<id>) ` (`file` id: absolute path; `conversation`:
   run ID; `ticket`: task ID), shown as an inline chip with its type icon,
   deleted whole by Backspace.
2. Picking a folder inserts `@<relative path>/` as text and keeps the menu
   open on that folder's contents.
3. The draft, the queue and the journal keep the plain text with its tokens.

### US3 - Resolution on send (P1)

1. **Given** a message with mentions, **when** the agent receives it, **then**
   it appends a `<referenced-context>` block to the provider message only;
   the journal keeps the message as typed.
2. File: absolute path and path relative to its root (Q1 -> a), after the
   agent re-validates that the path is inside one of the roots; otherwise
   the mention is reported as unavailable.
3. Folder (`@path/` naming an existing folder inside a root): same as a file.
4. Ticket: key, full task ID, title, stage and external URL, and a pointer to
   the Sectile MCP `get_task` (Q2 -> a); a ticket of another project is
   refused.
5. Conversation: title and a transcript of its user and assistant messages,
   newest kept whole within 30 000 characters, tools omitted (Q2 -> a); one of
   another project is refused.

### US4 - Transcript chips (P2)

1. A sent message shows its mentions as chips, like the composer (Q3 -> a);
   the referenced-context block never shows.

## Functional requirements

- FR1. `GET /desktop/mentions/files?projectId=&runId=&query=` (agent) and the
  Tauri command `mention_files`.
- FR2. Bounded listing (20 000 entries, 2 s), cached 10 s per root; `git`
  through `gitLocal` (hidden on Windows).
- FR3. Mention resolution before the provider message, outside the queue lock
  for network reads.
- FR4. Both composers use the editor with mention completion.
- FR5. One `Added` line in `CHANGELOG.md`.

## Success criteria

Go tests: listing bounds, ignore rules, symlink escape, ranking, folder
mtime, resolution of each type, refusals, journal unchanged. Vitest: trigger,
filtering, chip serialization and rendering, atomic Backspace, folder browse,
submission with mentions, transcript chips.
