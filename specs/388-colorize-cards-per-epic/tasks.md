# Tasks — colour board cards per epic

1. [ ] `web/src/lib/epicColor.ts`: `fnv1a`, `epicColor`, `epicColorHex`.
2. [ ] `web/tests/epicColor.test.mjs`: null/empty/whitespace keys give `null`;
       trimming; determinism; the result is always a palette entry; a known
       key maps to a pinned palette entry (guards against an accidental
       change of the hash, which would repaint every board); several distinct
       keys spread over more than one colour.
3. [ ] `web/src/components/EpicMarker.tsx`: `EpicBar`, `EpicDot`.
4. [ ] Board — `TaskCard.tsx`: `EpicBar` in the root (both densities), `EpicDot`
       before the parent key button (expanded).
5. [ ] Backlog — `ListView.tsx`: `EpicBar` in the first cell of the row,
       `EpicDot` in the parent badge.
6. [ ] Sprint timeline — `SprintTimelineView.tsx`: `EpicDot` before the key on
       the three chip renderings; `EpicBar` + `EpicDot` on the three list
       renderings.
7. [ ] Roadmap — `RoadmapView.tsx`: `EpicBar` on `renderMacroRow` and
       `renderCondensedRow`.
8. [ ] `CHANGELOG.md`: one `Added` line under `[Unreleased]`.
9. [ ] Gate, in `web/`: `npm test`, `npx tsc --noEmit`, `npx oxlint src`,
       `npx vite build`; then `node --test tests/condensed-card.browser.mjs`
       (the existing condensed-card browser suite) to check the card still
       renders and behaves, if its prerequisites are available.

## Test plan

- Node unit test on the derivation (task 2): the only logic in the change.
- Components are markup-only additions guarded by `parentKey`; the no-parent
  path renders `null`, which keeps requirement 11. They are checked by the
  type checker, the linter, the build and the existing condensed-card browser
  suite rather than by a new browser suite.
