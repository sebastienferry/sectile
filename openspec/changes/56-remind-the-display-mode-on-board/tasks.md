# Implementation checklist

- [ ] 1. Create `web/src/lib/boardDisplayMode.ts` defining `BoardCardDisplayMode`, storage keys, `loadBoardCardDisplayMode`, `saveBoardCardDisplayMode`, and `toggleBoardCardDisplayMode`.
- [ ] 2. Add unit tests in `web/tests/boardDisplayMode.test.mjs` verifying default mode, reading, saving, toggling, and legacy fallbacks.
- [ ] 3. Update `web/src/types/index.ts` to export `BoardCardDisplayMode`.
- [ ] 4. Connect `boardCardDisplayMode` and `toggleBoardCardDisplayMode` to `AppContext.tsx` and `useApp()`.
- [ ] 5. Update `BoardView.tsx` to consume `boardCardDisplayMode` from `useApp()`, wire the toggle button, and pass the resolved condensed boolean to `TaskCard`.
- [ ] 6. Run `npm test`, `npm run build`, and verify all tests and builds pass.
