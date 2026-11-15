# Tasks #908 - Tauri: show skill completion status in ticket conversations

Specification: `spec.md`. Plan: `plan.md`. Work in `desktop-tauri/` unless
stated otherwise. Write each test before or with its code; every task ends
with `npm test` (and `npm run test:ui` for UI tasks) green.

## Phase 1 - Domain (pure, unit tested)

- [x] T1 Widen `RunResult` in `src/domain/workflow.ts` with `SkillRecord`
      (`summary`, `error`, `disconnected`, `waitingReason`, `completedAt`) and
      the successor's `waitingSince` (FR2).
- [x] T2 Create `src/domain/skill-status.ts`: port `primaryStatus` and its
      helpers from `desktop/src/skill-result.mjs` with the adaptations of
      plan section 2 (free runs, zero-date wait mark, `asking`, record-only
      silence, Tauri skill labels), plus `isTicketSkillRun`,
      `skillStatusText`, `skillStatusTitle` (FR1, US1 to US7).
- [x] T3 Create `tests/skill-status.test.ts` with the cases listed in the
      plan's test plan: rows 1 to 9, US2.2 to US2.6, US3, US4, US5, US6, US7.
- [x] T4 Add `rememberLaunchStage` / `launchStageOf` (`sectile:launchStages`
      in `localStorage`, recorded once, pruned) and their tests (US3.3).
- [x] T5 Create `src/domain/skill-result-refresh.ts` (port of
      `desktop/src/skill-result-refresh.mjs`, Tauri live statuses, normalised
      wait mark) and `tests/skill-result-refresh.test.ts` (FR3).
- [x] T6 Remove `skillResultLabel` from `src/domain/workflow.ts` and its
      cases from `tests/workflow.test.ts`.

## Phase 2 - Shared reader

- [x] T7 Create `src/lib/skill-results.ts`: per-run store, `useSkillResult`
      (due check on each monitor snapshot, read on turn busy to idle, no
      doubled read, failed read keeps the last result, entry released with
      its last subscriber, no read for free runs) and `useSkillStatus`
      (FR3, FR4, US6.1).

## Phase 3 - Surfaces

- [x] T8 `ConversationStatus` in `src/components/ProjectPrompt.tsx`: pass
      `projectContext`, render the skill status in the execution menu
      summary with its kind mark and tooltip, and the run state, full reason
      and diagnostics in the panel; free conversations unchanged (US8, US6).
- [x] T9 `src/components/WorkflowTask.tsx`: replace the
      `workflow-run-status` line with the skill status of the selected run,
      drop the component's own result read, keep the 4 s task refresh, pass
      the conversation's pending-question flag (US9, US1.1).
- [x] T10 `ConversationNavigation` in `src/App.tsx`: indicator class
      `skill-<kind>`, tooltip and accessible name ending with the label, free
      rows unchanged (US10).
- [x] T11 `src/App.css`: kind classes for the sidebar indicator and the
      inline mark, reduced-motion rule, contrast check in both themes
      (US10.1, US8.4).

## Phase 4 - UI tests

- [x] T12 Create `tests/skill-status.ui.test.tsx`: strip summary and panel
      content, free conversation strip, workflow panel line equal to the strip
      text, sidebar dot class per kind and tooltip/accessible name, free row
      unchanged, one `runResult` call for two surfaces on the same run.
- [x] T13 Adapt `tests/workflow.ui.test.tsx` and `tests/workspace.ui.test.tsx`
      to the new status line and reader; enrich `runResult` in
      `tests/fixtures/ux-baseline.json`.

## Phase 5 - Documentation and checks

- [x] T14 `desktop-tauri/README.md`: "Skill status" section (plan section 11).
- [x] T15 `CHANGELOG.md` `[Unreleased]` / `Added`: one line for #908.
- [x] T16 Run `npm test`, `npm run test:ui`, `npm run check` in
      `desktop-tauri`; run the app with the UX fixture
      (`npm run dev:ux-fixture`) and check the strip, the panel and the
      sidebar in both themes.
      Done: `npm run check`, `npm test`, `npm run test:ui`, `npm run build`.
      Not done in this run: the visual check with `npm run dev:ux-fixture`
      (no browser in the implementation session); contrast was checked by
      computation instead, and the light theme got darker variants for the
      done, verify and failed kinds.
