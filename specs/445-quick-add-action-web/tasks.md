# #445: Implementation checklist

References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. Rules (US2, FR3, FR4)

- [ ] T1.1 Add `web/src/lib/quickAdd.ts`: `QuickAddFollowUp`,
      `quickAddMacroOptions`, `initialQuickAddMacro`.
- [ ] T1.2 Add `web/tests/quickAdd.test.mjs`: closed macros dropped, natural
      key order, pre-selection by key and by title, `''` for no filter,
      `__no_macro__`, `none` and a macro absent from the options.

## 2. Creation and attachment (US2, US3, FR5, FR6)

- [ ] T2.1 `createTask` in `AppContext.tsx` accepts `macroKey`; after creation it
      posts `/tasks/{id}/macro`, replaces the task on success, warns on failure,
      and returns the created task either way.

## 3. Dialog (US1-US5, FR1-FR4, FR7, FR8)

- [ ] T3.1 Strings: `quickAdd.macro`, `noMacro`, `macroLoading`, `followUp`,
      `followUpNone`, `followUpRewrite`, `followUpClarify`, `attachFailed`,
      `projectLabel`, `issueType`, `sprint`, `creating`; drop `quickAdd.tracker`.
- [ ] T3.2 Rework `QuickAddModal.tsx`: two columns, `MarkdownEditor`, dialog
      role, destination removed, macro select with reload/reset/pre-selection,
      follow-up radio group reset on opening, follow-up acted upon after
      creation.

## 4. Changelog (FR9)

- [ ] T4.1 One `### Changed` line under `[Unreleased]` in `CHANGELOG.md`.

## 5. Verification

- [ ] T5.1 Add `web/tests/quick-add.browser.mjs` covering US1.1, US1.2, US2.1-2.6,
      US3.1-3.2, US4.1-4.4, US4.7 and US5.3.
- [ ] T5.2 `npm run build`, `npm run lint`, `npm test` in `web/`.
- [ ] T5.3 Run `quick-add.browser.mjs` and `board-views.browser.mjs`.
- [ ] T5.4 Re-read the diff.

## Test plan

| Acceptance | Covered by |
| --- | --- |
| US1.1, US1.2 | `quick-add.browser.mjs` (bounding boxes, wide and narrow) |
| US1.3 | `quick-add.browser.mjs` (Markdown toolbar present) |
| US2.1, US2.2 | `quickAdd.test.mjs`, `quick-add.browser.mjs` |
| US2.3 | `quick-add.browser.mjs` (reload request, selection reset) |
| US2.4, US2.6 | `quick-add.browser.mjs` (attachment request present / absent) |
| US2.5 | `quick-add.browser.mjs` (attachment refused, warning toast, task kept) |
| US3.1, US3.2 | `quick-add.browser.mjs` (no "Destination", body without `source`) |
| US4.1-US4.4, US4.7 | `quick-add.browser.mjs` (run-skill requests, detail modal, reset) |
| US4.5 | Covered by `runSkill`'s existing error toast; not re-tested. |
| US4.6 | `quick-add.browser.mjs` (creation refused, no run-skill, dialog open) |
| US5.1 | `board-views.browser.mjs` S15 |
| US5.2 | Unchanged `createTask` toast; not re-tested. |
| US5.3 | `quick-add.browser.mjs` (Escape closes) |
