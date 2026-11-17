# Tasks #913 - auto-expanding composer and prompt history

Work in `desktop-tauri/`.

## Phase 1 - Pure modules

- [x] T1 Move `cleanUserEventText` to `src/lib/user-event-text.ts`, re-export
      it from `ConversationTranscript.tsx` (FR3, US3.6).
- [x] T2 Create `src/domain/prompt-history.ts` (`userPrompts`,
      `navigatePromptHistory`) (FR3, FR4, US3, US4).
- [x] T3 Create `src/lib/composer-sizing.ts` (`composerMaxHeight`,
      `useAutoGrowTextarea`) (FR1, FR2).
- [x] T4 `tests/prompt-history.test.ts` for T2 and `composerMaxHeight`.

## Phase 2 - Wiring

- [x] T5 `src/App.css`: dynamic `max-height`, `line-height:1.4`,
      `overflow-y` (US1).
- [x] T6 `ProjectPrompt.tsx`: auto-grow on both composers (US1, US2).
- [x] T7 `ProjectPrompt.tsx`: history and Escape precedence on the
      conversation composer, IME tracking, reset on id change and send
      (US3, US4).

## Phase 3 - UI tests and docs

- [x] T8 `tests/composer.ui.test.tsx` per the plan's test plan.
- [x] T9 `CHANGELOG.md` `Added` line (FR5).
- [x] T10 `npm run check`, `npm test`, `npm run test:ui`, `npm run build`.
