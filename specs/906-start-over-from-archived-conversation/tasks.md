# Tasks #906 - Start over from an archived conversation

Ordered; each task lists its tests. Run `go test ./internal/agent/...`,
`cargo test` in `desktop-tauri/src-tauri` and `npm test` in `desktop-tauri`
after each group.

## 1. Former context (Go)

- [x] T001 Add `internal/agent/former_context.go`: `formerContextBudget`,
      `formerContext`, `buildFormerContext` (user/assistant only, newest-first
      whole messages within budget, single oversized message cut with a
      marker, omitted count, tools-omitted flag, delimited rendering).
- [x] T002 Tests in `former_context_test.go`: order, filtering, budget
      boundary, oversized last message, empty and tool-only sources, a source
      that itself started from a former context.
- [x] T003 Add `formerContextFor(projectID, sourceID)` reading the source the
      way `/desktop/conversation` does, with the unavailable / other project /
      empty errors; tests for each error and for a read that leaves the source
      record and journal bytes unchanged.

## 2. Agent endpoints

- [x] T004 `GET /desktop/conversation/former-context` returning
      `{messages, omitted, toolsOmitted, characters}`; handler tests for
      success and each error status.
- [x] T005 `desktopRun` gains `ContextRunID`, `ContextTitle`,
      `ContextOmitted`; persisted with the run store; round-trip test.
- [x] T006 `POST /desktop/consoles` accepts `contextRunId` (conversation view
      only); builds the context before creating the run, creates nothing on
      error; stores the pending prefix on the run.
- [x] T007 First `message` action sends prefix + instructions to the provider
      once, then clears it; journal records the notice and the instructions
      only. Tests with a fake provider: prefix on turn 1 only, journal
      content, source untouched.
- [x] T008 `POST /desktop/tasks` accepts `contextRunId`; refuses it with a
      terminal view or autonomous mode; validates the source before calling
      the server; stores it in `taskLaunchPreference`. Tests: refusals, no
      server call on bad source, preference carries the ID.
- [x] T009 Dispatch (`agent.go`, conversation branch): rebuild the context,
      fail the run with the reason when it cannot, else append it after the
      skill command and prompt, set the run's context fields and journal the
      notice. Tests: Claude `/` and Codex `$` command stays first; vanished
      source fails the run.

## 3. Tauri commands (Rust)

- [x] T010 `create_conversation` gains `context_run_id`; `LaunchTask` gains
      `context_run_id` with the view/mode check in `launch_body`; new
      `former_context_preview` command registered in `lib.rs`.
- [x] T011 Rust tests for the new validation paths.

## 4. Desktop UI

- [x] T012 `lib/conversation.ts`, `lib/agent.ts`, `domain/execution.ts`:
      `contextRunId` plumbing and `formerContextPreview`; unit tests for
      `launchInput` carrying it only for interactive conversation launches.
- [x] T013 Extract the engine/profile picker from `ProjectPrompt.tsx` if the
      dialog cannot reuse it as is.
- [x] T014 `components/StartOverDialog.tsx` with both paths, preview, every
      state of US6, kept text on failure; styles in `App.css`.
- [x] T015 `ProjectPrompt.tsx`: Start over button on archived conversations
      only (disabled with a reason while loading or disconnected); "Started
      from" line with link, fallback text and omitted count.
- [x] T016 `App.tsx`: pass `archivedConversationIds`, open the dialog,
      navigate to the new run on success, never touch the archive list.
- [x] T017 Vitest: button visibility, dialog states, skill disabled without
      a ticket, both launch calls with `contextRunId`, navigation, back-link
      rendering, archive list unchanged after a start.

## 5. Documentation

- [x] T018 `docs/USER_GUIDE.md`, section "Use the Desktop conversation view":
      Start over, the two paths, what the former context contains, the
      100,000-character limit and its notice, and why skills need a ticket.
- [x] T019 `CHANGELOG.md`, `## [Unreleased]` / `Added`: one line for Start
      over from an archived conversation.

## 6. Verification

- [ ] T020 Manual check in Desktop: start over on both paths from an archived
      conversation, confirm the source is still archived and unchanged, and
      check the disconnected and empty-history states.

## Implementation notes

- T009: the command-first ordering and the journal are tested through
  `startLaunchConversationWithContext`; the dispatch failure when the source
  vanished between launch and dispatch is implemented but has no test of its
  own (the dispatch path needs the whole server contract fixture).
- T013: no extraction was needed; the dialog has its own profile/engine select
  with the same options as the new-prompt composer.
- T008: the view/mode refusal runs before the ticket is read, and the agent
  also refuses a source that belongs to another ticket.
- T005: the run store persists `desktopRun` as a whole, so the new fields need
  no store change.
- T020 (manual check in Desktop) is left to the reviewer.
