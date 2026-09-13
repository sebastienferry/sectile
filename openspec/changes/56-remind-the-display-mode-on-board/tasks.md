# Implementation checklist

- [x] 1. Create `web/src/lib/boardDisplayMode.ts` defining `BoardCardDisplayMode`, storage keys, `loadBoardCardDisplayMode`, `saveBoardCardDisplayMode`, and `toggleBoardCardDisplayMode`.
- [x] 2. Add unit tests in `web/tests/boardDisplayMode.test.mjs` verifying default mode, reading, saving, toggling, and legacy fallbacks.
- [x] 3. Update `web/src/types/index.ts` to export `BoardCardDisplayMode`.
- [x] 4. Connect `boardCardDisplayMode` and `toggleBoardCardDisplayMode` to `AppContext.tsx` and `useApp()`.
- [x] 5. Update `BoardView.tsx` to consume `boardCardDisplayMode` from `useApp()`, wire the toggle button, and pass the resolved condensed boolean to `TaskCard`.
- [x] 6. Run `npm test`, `npm run build`, and verify all tests and builds pass.

Implementation completed. All unit tests pass and production build succeeds.
