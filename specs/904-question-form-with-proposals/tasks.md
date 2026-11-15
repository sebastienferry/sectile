# Tasks #904 - Desktop (Tauri): tabbed question form with proposals

## Phase 1: Question form (`desktop-tauri`)
- [x] 1.1 Add `SKIPPED_ANSWER` and `recommendedLabel()` to `QuestionForm.tsx`; unit-test the suffix detection (exact suffix, whitespace, no suffix, suffix mid-label).
- [x] 1.2 Extend per-question state with `skipped` and add the `active` tab; derive `state(i)`, `answers` and `valid` (FR-3, FR-11, FR-12).
- [x] 1.3 Render the tablist / tabpanel for two or more questions, no tablist for one; labels from `header` or `Qn`; state markers with accessible names; arrow-key navigation (FR-1 to FR-4, FR-6).
- [x] 1.4 Render recommended options (stripped label, badge, highlight, not preselected, original label submitted) (FR-8, FR-9).
- [x] 1.5 Add "Skip this question", clearing values and un-skipping on input (FR-10).
- [x] 1.6 Auto-advance on single-choice selection and on skip; never on multi-choice or typing (FR-5).
- [x] 1.7 Styles in `App.css`: tab strip, state markers, recommended highlight and badge, light and dark themes.
- [x] 1.8 UI tests in `desktop-tauri/tests/questions.ui.test.tsx` for scenarios 1-10, 15, 16; keep the existing tests green (secret handling, unsupported format, error message without echo).

## Phase 2: Agent answer path (`internal/agent`)
- [x] 2.1 Add `maskedAnswers(approval, answers)`; use it for the Claude trace detail and replace the inline Codex masking (FR-18).
- [x] 2.2 Tests: Claude request with an `isSecret` question shows `[hidden]` in the trace; `Skipped by the owner` passes `checkAnswers`; Codex sends the literal as a one-element answer list (scenarios 7, 8, 11).

## Phase 3: Run activity step
- [x] 3.1 Add `RunStepType` / `RunStep` to `internal/agentprotocol/message.go` and list it in the `Message.Type` comment.
- [x] 3.2 Agent: after a successful `answer` on a task-linked conversation, send `run_step` with the masked detail; log and ignore failures; nothing on `deny` or for free-standing conversations (FR-15 to FR-17).
- [x] 3.3 Server: handle `run_step` in the agent read loop with the ownership and status checks; append to `Activity.Steps`, persist, broadcast (add a narrow db append if needed).
- [x] 3.4 Tests: agent sends one `run_step` for a task-linked answer and none otherwise (scenarios 12, 13); server appends for an owned running activity and drops for another agent's, an unknown or a finished run; an unavailable link does not fail the answer (scenario 14).

## Phase 4: Documentation and checks
- [x] 4.1 `CHANGELOG.md`: one `Added` line under `[Unreleased]`.
- [x] 4.2 Run `npm run check` and `npm run test:ui` in `desktop-tauri`; `go vet ./...` and `go test ./internal/agent/... ./internal/handlers/... ./internal/agentprotocol/... ./internal/db/...`.
- [x] 4.3 Check that `desktop/` (Electron) and `web/` are untouched by the diff.
