# Tasks #809 - Web UI Roadmap: Clickable macro ID

Order matters: each step leaves the tree building and green.

## 1. Shared component (US1, US2, US3)

- [x] T1.1 Add `web/src/components/MacroKeyRef.tsx`: key, optional link with
      `ExternalLink` icon, copy button through `copyText`, check icon for
      2000 ms, success and failure toasts with the ticket detail's strings
      (D1).
- [x] T1.2 Isolate its events: `click` and `mousedown` propagation stopped on
      the outer span; the span is `draggable` and cancels and stops its
      `dragstart` (D2).
- Tests: `web/tests/macro-key-ref.browser.mjs` per the test plan (link or
  plain text, copy payload, toasts, no click or drag reaching the parent).

## 2. Roadmap (US1, US2, US3, US4)

- [x] T2.1 Render the key of `renderMacroRow` and `renderCondensedRow`
      through `MacroKeyRef` with `row.externalUrl`, keeping each row's text
      size and accent colour (D3).
- [x] T2.2 Render the side panel header key through `MacroKeyRef` with
      `selected.externalUrl`; delete the "Lien" `<a>`; keep "Copier" as is
      (D3).
- [x] T2.3 Remove the unused `panel.link` string from both locales of
      `web/src/locales/planning.ts` (D3).
- Tests: update `web/tests/roadmap-view.browser.mjs` for AC-1, AC-2, AC-4 and
  AC-5; adjust the selectors the new markup breaks; keep the existing
  "Copier" assertions passing. Run the other `roadmap-*.browser.mjs` tests as
  a regression guard.

## 3. Timeline (US5)

- [x] T3.1 Add `web/src/lib/macroLinks.ts` with `macroProjectIds`,
      `macroUrlKey` and `macroUrlIndex` (D4).
- [x] T3.2 In `SprintTimelineView.tsx`, fetch the macro lists of the projects
      shown on a change of project set, with a stale-answer guard, and expose
      `macroUrlOf(task)` (D4).
- [x] T3.3 Insert `MacroKeyRef` before the `parentTitle` chip in the sprint
      list rows, the sprint cards and the backlog cards, whenever
      `parentKey` is set (D5).
- Tests: `web/tests/macroLinks.test.mjs` (unit) and
  `web/tests/sprint-timeline-macro-key.browser.mjs` for AC-6 and AC-7.

## 4. Checks and documentation

- [x] T4.1 `CHANGELOG.md`: one `Added` line under `## [Unreleased]` (AC-8).
- [x] T4.2 Run `npx tsc --noEmit`, `npx oxlint`, `npm test`, the new and
      updated browser tests and `task-ref-copy.browser.mjs` (D6 guard).
