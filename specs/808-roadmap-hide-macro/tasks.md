# Tasks #808 - Web UI: Roadmap, hide a macro

Spec: [spec.md](spec.md) · Plan: [plan.md](plan.md)

## Phase 1 - Test first

- [ ] T001 Write `web/tests/roadmap-hide-from-panel.browser.mjs` (or extend
      `roadmap-view.browser.mjs`) following the test plan: condensed list,
      hide from the panel, land in the Hidden tab, un-hide from the panel,
      suggestion shown with ` ?`, `aria-pressed` on the active button.
- [ ] T002 Run it and confirm it fails on the current code (no `HIDDEN`
      button in the panel).

## Phase 2 - Panel selector (US1, US2, US3)

- [ ] T003 In `web/src/components/RoadmapView.tsx`, extract the unfolded row's
      horizon button into a shared renderer taking the macro row, the horizon
      and the click handler; switch `renderMacroRow` to it with an identical
      DOM.
- [ ] T004 Replace the read-only horizon badge of the panel header with a
      `role="group"` of four buttons (NOW, NEXT, LATER, HIDDEN) built with
      the shared renderer, each calling `persist(selected.key, { horizon: h })`
      and carrying `aria-pressed`.
- [ ] T005 Leave `renderCondensedRow` and `CONDENSED_HORIZONS` unchanged.

## Phase 3 - Verification

- [ ] T006 Run the new browser test until green.
- [ ] T007 Run the existing Roadmap browser suites
      (`web/tests/roadmap-*.browser.mjs`) and unit tests
      (`node --test web/tests/roadmap*.test.mjs`).
- [ ] T008 Run `tsc` and `oxlint` on `web/`.

## Phase 4 - Documentation

- [ ] T009 Add a `Fixed` line under `## [Unreleased]` in `CHANGELOG.md`, for
      example: "**Hide a macro from the Roadmap panel again.** The side panel
      header shows NOW / NEXT / LATER / HIDDEN buttons instead of a read-only
      badge, so a macro can be hidden or brought back whatever the row shape,
      the condensed list included. (#808)"
