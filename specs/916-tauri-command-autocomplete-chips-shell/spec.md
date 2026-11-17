# Specification #916 - Tauri: `/` and `$` autocomplete, inline command chips, `!` shell indicator

Ticket: https://github.com/sebastienferry/sectile/issues/916
Clarification: `docs/clarifications/916.md` (rounds 1 and 2).

## Scope

In: the Sectile Desktop (Tauri) conversation composer, which becomes a
CodeMirror 6 editor (Q1 -> A). Out: the local agent and its command discovery,
the Rust bridge, the Electron client, the web UI, the "New prompt" composer
(no snapshot, no commands), the Supervisor composer, `@` mentions (#917).

## User stories

### US1 - Complete a command or a skill (P1)

1. **Given** the conversation snapshot lists commands, **when** the first
   token of the draft (leading whitespace allowed) starts with `/` and the
   caret is inside it, **then** a floating listbox above the composer box
   lists the commands whose name does not start with `$`, filtered by the
   typed text.
2. **When** the first token starts with `$`, **then** only the entries whose
   name starts with `$` (Codex skills) are listed.
3. Ranking: exact name, then name prefix, then name substring, then
   description substring, case-insensitive, stable within a tier; at most 50
   entries, scrollable; the matched part is highlighted in the accent colour;
   the argument hint is shown beside the name when present.
4. **Given** an empty command list, no match, a `/` or `$` that is not the
   first token, or a draft starting with `!`, **then** no menu is shown.
5. **Given** too little room above the composer box in the viewport, **then**
   the menu opens below it.

### US2 - Keyboard and mouse selection (P1)

1. While the menu is open: `ArrowDown`/`ArrowUp` move the active entry
   (cycling), `Enter` (without Shift) and `Tab` replace the token with the
   active entry followed by a space, `Escape` closes the menu and does not
   interrupt a running turn.
2. Hovering an entry makes it active; clicking it inserts it; leaving the
   editor closes the menu.
3. During IME composition keys pass through.
4. The editor is a `combobox` with `aria-autocomplete="list"`,
   `aria-expanded`, `aria-controls` pointing at a `listbox`, and
   `aria-activedescendant` naming the active `option` (`aria-selected`).

### US3 - Inline chips (P1)

1. **Given** the first token exactly matches a known command or skill and is
   followed by whitespace (always the case after a menu insertion), **then**
   it renders as an inline chip: monospace badge for `/` commands, a badge
   with a skill icon for `$` skills.
2. An unknown `/word` stays plain text.
3. `Backspace` right after a chip deletes the whole token at once.
4. What is sent, queued and stored is the exact draft text (`/name args`).

### US4 - Shell mode indicator (P1)

1. **Given** the trimmed draft starts with `!`, **then** the composer box has
   a distinct accent border and its footer shows a `Shell` badge with the
   terminal icon; the editor's accessible description says the command runs
   in this conversation's folder and the agent sees its output with the next
   message. The indicator also shows while queueing.

### US5 - The composer contract is kept (P1)

1. Accessible name `Next message`; 32 000-character limit; `Enter` sends or
   queues; `Shift+Enter` inserts a newline; `Escape` closes the menu first,
   then clears a recalled prompt (#913), then interrupts a busy turn.
2. Prompt history (#913), auto-grow with the dynamic cap (#913) and Edit from
   the queue strip (#914) keep working on the editor.

## Functional requirements

- FR1. `ConversationSnapshot.commands?: {name, description?, argumentHint?}[] | null`.
- FR2. Pure module for completion context, ranking and chip ranges.
- FR3. `ComposerEditor` (CodeMirror 6) with chips, atomic ranges, length
  filter, placeholder, and a highest-precedence key handler.
- FR4. ADR in `desktop-tauri/docs/adrs`; `THIRD_PARTY_NOTICES_JS.md`
  regenerated; README and changelogs updated.

## Success criteria

Unit tests for context, ranking and chips; UI tests for the menu (open,
filter, highlight, keyboard, mouse, Escape precedence, ARIA), chip rendering
and atomic deletion, plain-text submission, and the shell indicator.
