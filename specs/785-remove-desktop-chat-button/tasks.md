# Tasks #785 - Desktop: remove the Claude chat (test) button

Order matters: each step leaves the tree buildable. Tests go with the step
they cover.

## 1. Renderer

- [ ] T1.1 Remove `conversationButton` and its `render()` lines from
      `desktop/src/main.js` (FR1).
- Tests (`desktop/tests/conversation.ui.cjs`): no `chat (test)` button in
  Conversation mode with the completed terminal execution `source` selected
  (US1.1, FR4); terminal-mode assertions in `conversation.ui.cjs` and
  `console.ui.cjs` match both labels (US1.3). The rest of the suite still opens
  a conversation (US1.4).

## 2. IPC plumbing

- [ ] T2.1 Remove the `create-conversation` handler and the `createConversation`
      preload entry (FR2); the agent is untouched (FR3).

## 3. Documentation

- [ ] T3.1 `README.md`, `desktop/README.md`,
      `docs/experiments/desktop-conversation.md` (US2.1).
- [ ] T3.2 `CHANGELOG.md` `Removed` line and the permission-mode clause (US2.2,
      FR5).

## 4. Checks

- [ ] T4.1 `git grep` finds no remaining button label or `createConversation(`
      outside specs and clarifications.
- [ ] T4.2 Desktop unit tests, `npx vite build`, the UI suites
      `conversation.ui.cjs`, `console.ui.cjs`, `codex-conversation.ui.cjs`.
