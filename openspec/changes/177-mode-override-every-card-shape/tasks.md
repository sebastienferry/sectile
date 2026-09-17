# Tasks

## 1. Share the mode entries between both card shapes
- [x] 1.1 Extract the two mode buttons from the `{isCondensed && …}` block of
      `web/src/components/TaskCard.tsx` into a single `modeActions` JSX fragment, keeping the
      handlers (`handleAdvance(false, 'interactive')` / `'autonomous'`), the icons and the
      disabled conditions unchanged. *(merged in `74b9a9a`)*
- [x] 1.2 Reference `{modeActions}` in the condensed branch at its original position, between
      `Avancer` and `Avancer toute la chaîne`. *(merged in `74b9a9a`)*
- [x] 1.3 Add a `{!isCondensed && …}` branch rendering `{modeActions}` and a separator, and
      nothing else. *(merged in `74b9a9a`)*
- [x] 1.4 Comment why both shapes carry the entries: the expanded card's inline chevrons carry
      no mode, so without them the override is unreachable. *(merged in `74b9a9a`)*

## 2. Pin the sharing with a test
- [x] 2.1 In `web/tests/skillLaunchMode.test.mjs`, extend the existing
      `the card ... menu offers both modes for a single launch` test with assertions that the
      mode entries are defined once as `modeActions`.
- [x] 2.2 Assert that the `isCondensed` branch references `{modeActions}`.
- [x] 2.3 Assert that a `!isCondensed` branch references `{modeActions}`, so the expanded case
      cannot be dropped.
- [x] 2.4 Assert the condensed order is preserved: `modeActions` between the `handleAdvance(false)`
      entry and the `handleAdvance(true)` full-chain entry.

## 3. Verify
- [x] 3.1 Run `make test` (Go suite, `npm test`, `tsc --noEmit`, `oxlint`) and quote the output.
- [x] 3.2 Run `openspec validate 177-mode-override-every-card-shape --strict`.
