# Tasks #851 - Macro keys link to the tracker

Order matters: each step leaves the tree buildable. Tests go with the step
they cover.

## 1. Data

- [ ] T1.1 `SeedLine` gains an optional `externalUrl`; `seedProposals` copies
      it from the row when present (US1.4).
- Tests: `web/tests/epicAxes.test.mjs`, with and without a URL.

## 2. Key link

- [ ] T2.1 Add the macro key render helper in `RoadmapView.tsx`: anchor with
      new tab, `noreferrer`, `draggable={false}`, click and mousedown
      propagation stopped, `openRemoteTitle` tooltip, hover underline; plain
      span without a URL (FR1-FR5).
- [ ] T2.2 Use it in `renderMacroRow` and `renderCondensedRow`, keeping each
      site's classes and style (US1.1, US1.2).
- [ ] T2.3 Use it in the side panel header (US1.3).
- [ ] T2.4 Use it in the seed table with `line.externalUrl` (US1.4).
- Tests: `web/tests/roadmap-view.browser.mjs` (links on `M-3`, plain key on
  `M-4`, click and Shift click leave the selection unchanged, popup
  intercepted); `web/tests/roadmap-epic-axes.browser.mjs` (seed key link,
  checkboxes unchanged); `web/tests/roadmap-grouping.browser.mjs` (drag from
  the key moves the row).

## 3. Panel button

- [ ] T3.1 Remove the panel's "Lien" button; drop the `ExternalLink` import if
      unused (US3.1, FR6).
- [ ] T3.2 Remove `panel.link` from the French and English catalogs; keep
      `openRemoteTitle` (FR6).
- Tests: `roadmap-view.browser.mjs` asserts no "Lien" button; the existing
  panel link assertion still passes; `tsc`, `oxlint`, the locale catalog test.

## 4. Documentation

- [ ] T4.1 `CHANGELOG.md`: one `Changed` line under `## [Unreleased]` naming
      the linked keys and the removed panel button (FR7).
