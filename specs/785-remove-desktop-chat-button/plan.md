# Plan #785 - Desktop: remove the Claude chat (test) button

## Stack

Desktop only: the renderer (`desktop/src/main.js`) and the Electron main and
preload scripts. No agent, server, migration or API change.

## Design

- `desktop/src/main.js`: drop `conversationButton` (creation, `title`,
  placement before `#save-log`, `onclick`) and its two lines in `render()`.
  The `consoleView` state and the comment above it stay: the conversation view
  still reads them.
- `desktop/electron/main.cjs`: drop the `create-conversation` handler. It was
  the only caller of `POST /desktop/conversation` with `sourceRunId` from
  Desktop; the agent keeps the endpoint, so a Desktop older than this change
  keeps working against a newer agent.
- `desktop/electron/preload.cjs`: drop `createConversation`.
- Tests: `desktop/tests/conversation.ui.cjs` already selects a completed
  terminal execution with a directory (`source`) and switches to Conversation;
  it asserts no `chat (test)` button right after the switch, while `source` is
  still selected, which is where the button showed before. The terminal-mode
  assertions in `conversation.ui.cjs` and `console.ui.cjs` match both labels.
- Documentation: `README.md`, `desktop/README.md` and
  `docs/experiments/desktop-conversation.md` drop the button; the Desktop
  README's permission-mode paragraph drops "the first message of a Claude
  chat".
- `CHANGELOG.md`: per FR5.

### Rejected alternatives

- Hiding the button only while a terminal execution runs (round 1, reading
  (a)): the owner chose removal in round 2.
- Removing the agent's `sourceRunId` path: it would break an older Desktop
  against a newer agent for no user gain; the clarification keeps it.

## Target files

- `desktop/src/main.js`
- `desktop/electron/main.cjs`
- `desktop/electron/preload.cjs`
- `desktop/tests/conversation.ui.cjs`
- `desktop/tests/console.ui.cjs`
- `README.md`
- `desktop/README.md`
- `docs/experiments/desktop-conversation.md`
- `CHANGELOG.md`
