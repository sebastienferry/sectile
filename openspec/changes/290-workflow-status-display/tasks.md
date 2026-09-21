# Implementation checklist

- [x] 1. Write and validate the OpenSpec change with `openspec validate 290-workflow-status-display --strict`.
- [x] 2. Add the `common.grouping` entries to `web/src/locales/translations.ts` for every locale.
- [x] 3. Add `web/src/lib/boardGrouping.ts` with the ordered option descriptors.
- [x] 4. Add `web/src/components/BoardGroupingToggle.tsx` consuming those options and `AppContext`.
- [x] 5. Replace the inline toggle in `web/src/components/BoardView.tsx` with `<BoardGroupingToggle size="md" />`.
- [x] 6. Replace the inline toggle in `web/src/components/ListView.tsx` with `<BoardGroupingToggle size="sm" />` and drop the now-unused imports.
- [x] 7. Add `web/tests/boardGrouping.test.mjs` covering order, ids and descriptors.
- [x] 8. Run `make test` (Go tests, web tests, `tsc --noEmit`, `oxlint`) and fix until green.
- [x] 9. Review the diff, commit, push and update the pull request.
