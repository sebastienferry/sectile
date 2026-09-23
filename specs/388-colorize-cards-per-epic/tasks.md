# Tasks — colour board cards per epic

1. [x] `web/src/lib/epicColor.ts`: `fnv1a`, `epicColor`, `epicColorHex`.
2. [x] `web/tests/epicColor.test.mjs`: null/empty/whitespace keys give `null`;
       trimming; determinism; the result is always a palette entry; a known
       key maps to a pinned palette entry (guards against an accidental
       change of the hash, which would repaint every board); several distinct
       keys spread over more than one colour.
3. [x] `web/src/components/EpicMarker.tsx`: `EpicBar`, `EpicDot`.
4. [x] Board — `TaskCard.tsx`: `EpicBar` in the root (both densities), `EpicDot`
       before the parent key button (expanded).
5. [x] Backlog — `ListView.tsx`: `EpicBar` in the first cell of the row,
       `EpicDot` in the parent badge.
6. [x] Sprint timeline — `SprintTimelineView.tsx`: `EpicDot` before the key on
       the three chip renderings; `EpicBar` + `EpicDot` on the three list
       renderings.
7. [x] Roadmap — `RoadmapView.tsx`: `EpicBar` on `renderMacroRow` and
       `renderCondensedRow`.
8. [x] `CHANGELOG.md`: one `Added` line under `[Unreleased]`.
9. [x] Gate, in `web/`: `npm test`, `npx tsc --noEmit`, `npx oxlint src`,
       `npx vite build`; then `node --test tests/condensed-card.browser.mjs`
       (the existing condensed-card browser suite) to check the card still
       renders and behaves, if its prerequisites are available.

### Review revision

10. [x] Migration 3 `projects.epic_colors`, model and request fields, both read
        paths, insert and update; `forgetSchemaVersion` drops the column so the
        upgrade tests replay it.
11. [x] `internal/db/epiccolors_test.go`: off by default, on/off round trip
        through both read paths, untouched by an unrelated update, set at
        creation.
12. [x] `epicColorsEnabled`, `useEpicColors`; `EpicDot` removed; `EpicBar`
        becomes a full-size layer painting a 3px inset shadow, clipped to the
        card's corners.
13. [x] Views: the bar on every surface, gated by the task's project setting.
14. [x] Project settings: "Couleur par épic" checkbox under General.
15. [x] `web/tests/epicColor.test.mjs`: setting resolution per project and
        fallback.

## Test plan

- Node unit test on the derivation (task 2): the only logic in the change.
- Components are markup-only additions guarded by `parentKey`; the no-parent
  path renders `null`, which keeps requirement 11. They are checked by the
  type checker, the linter and the build.

## Implementation notes

- `tests/condensed-card.browser.mjs` does not run on `main` either (c2849ab):
  under Vite 8 its fixture page gets a 404 on `/@vite/client` and never
  renders the card, and from a path containing `#` (the task worktrees) Vite
  additionally fails to parse the fixture's JSX. It was therefore not extended.
  The card was instead verified with a throwaway server-side render of the real
  `TaskCard` (Vite `ssrLoadModule`, `AppContext` mocked): bar with the
  `#8b5cf6` colour of `#1` on both shapes, dot with `aria-label="Épic #1"` on
  the expanded shape only, running border class unchanged, no marker without a
  parent. Repairing the browser harness is a change of its own.
- Every `space-y-*` container is Tailwind v4, whose zero-specificity
  `margin-block-end` would shorten an absolute bar; `EpicBar` carries `m-0`.
