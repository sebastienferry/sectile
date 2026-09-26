# Plan #530 - Sprint timeline translation

## Approach

Move Sectile-owned literals to the `sprints` namespace
(`web/src/locales/sprints.ts`, D1), grouped as `sprints.timeline`,
`sprints.batch`, `sprints.dialogs`, `sprints.close`, `sprints.backlog`,
`sprints.feedback`. Replace `formatDateFR(value)` with
`formatDate(locale, value)`; remove `formatDateFR` once unused (or keep it as
a thin wrapper only if another caller needs it).

## Data contracts

None: `sprintApi.ts` payloads are unchanged.

## Target files

`web/src/locales/sprints.ts`, `SprintTimelineView.tsx`, `lib/sprints.ts`,
`web/tests/sprintsCatalog.test.mjs`, `web/tests/sprintManagement.test.mjs`
(updated only if it used `formatDateFR`).

## Tests

Catalog test: close dialog strings and plural forms in both languages;
`sprintManagement.test.mjs` keeps passing; a test that the display date of
`2026-09-01` is 1 September in `en` and `fr`.
