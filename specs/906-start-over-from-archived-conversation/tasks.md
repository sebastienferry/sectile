# Tasks #906 - Start over from an archived conversation

Ordered; each task lists its tests. Run `go test ./internal/agent/...`,
`cargo test` in `desktop-tauri/src-tauri` and `npm test` in `desktop-tauri`
after each group.

## 1. Former context (Go)

- [ ] T001 Add `internal/agent/former_context.go`: `formerContextBudget`,
      `formerContext`, `buildFormerContext` (user/assistant only, newest-first
      whole messages within budget, single oversized message cut with a
      marker, omitted count, tools-omitted flag, delimited rendering).
- [ ] T002 Tests in `former_context_test.go`: order, filtering, budget
      boundary, oversized last message, empty and tool-only sources, a source
      that itself started from a former context.
- [ ] T003 Add `formerContextFor(projectID, sourceID)` reading the source the
      way `/desktop/conversation` does, with the unavailable / other project /
      empty errors; tests for each error and for a read that leaves the source
      record and journal bytes unchanged.

## 2. Agent endpoints

- [ ] T004 `GET /desktop/conversation/former-context` returning
      `{messages, omitted, toolsOmitted, characters}`; handler tests for
      success and each error status.
- [ ] T005 `desktopRun` gains `ContextRunID`, `ContextTitle`,
      `ContextOmitted`; persisted with the run store; round-trip test.
- [ ] T006 `POST /desktop/consoles` accepts `contextRunId` (conversation view
      only); builds the context before creating the run, creates nothing on
      error; stores the pending prefix on the run.
- [ ] T007 First `message` action sends prefix + instructions to the provider
      once, then clears it; journal records the notice and the instructions
      only. Tests with a fake provider: prefix on turn 1 only, journal
      content, source untouched.
- [ ] T008 `POST /desktop/tasks` accepts `contextRunId`; refuses it with a
      terminal view or autonomous mode; validates the source before calling
      the server; stores it in `taskLaunchPreference`. Tests: refusals, no
      server call on bad source, preference carries the ID.
- [ ] T009 Dispatch (`agent.go`, conversation branch): rebuild the context,
      fail the run with the reason when it cannot, else append it after the
      skill command and prompt, set the run's context fields and journal the
      notice. Tests: Claude `/` and Codex `$` command stays first; vanished
      source fails the run.

## 3. Tauri commands (Rust)

- [ ] T010 `create_conversation` gains `context_run_id`; `LaunchTask` gains
      `context_run_id` with the view/mode check in `launch_body`; new
      `former_context_preview` command registered in `lib.rs`.
- [ ] T011 Rust tests for the new validation paths.

## 4. Desktop UI

- [ ] T012 `lib/conversation.ts`, `lib/agent.ts`, `domain/execution.ts`:
      `contextRunId` plumbing and `formerContextPreview`; unit tests for
      `launchInput` carrying it only for interactive conversation launches.
- [ ] T013 Extract the engine/profile picker from `ProjectPrompt.tsx` if the
      dialog cannot reuse it as is.
- [ ] T014 `components/StartOverDialog.tsx` with both paths, preview, every
      state of US6, kept text on failure; styles in `App.css`.
- [ ] T015 `ProjectPrompt.tsx`: Start over button on archived conversations
      only (disabled with a reason while loading or disconnected); "Started
      from" line with link, fallback text and omitted count.
- [ ] T016 `App.tsx`: pass `archivedConversationIds`, open the dialog,
      navigate to the new run on success, never touch the archive list.
- [ ] T017 Vitest: button visibility, dialog states, skill disabled without
      a ticket, both launch calls with `contextRunId`, navigation, back-link
      rendering, archive list unchanged after a start.

## 5. Documentation

- [ ] T018 `docs/USER_GUIDE.md`, section "Use the Desktop conversation view":
      Start over, the two paths, what the former context contains, the
      100,000-character limit and its notice, and why skills need a ticket.
- [ ] T019 `CHANGELOG.md`, `## [Unreleased]` / `Added`: one line for Start
      over from an archived conversation.

## 6. Verification

- [ ] T020 Manual check in Desktop: start over on both paths from an archived
      conversation, confirm the source is still archived and unchanged, and
      check the disconnected and empty-history states.
