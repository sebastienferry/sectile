# Implementation checklist

- [ ] 1. Write and validate the OpenSpec change with `openspec validate 290-workflow-status-display --strict`.
- [ ] 2. Add the `common.grouping` entries to `web/src/locales/translations.ts` for every locale.
- [ ] 3. Add `web/src/lib/boardGrouping.ts` with the ordered option descriptors.
- [ ] 4. Add `web/src/components/BoardGroupingToggle.tsx` consuming those options and `AppContext`.
- [ ] 5. Replace the inline toggle in `web/src/components/BoardView.tsx` with `<BoardGroupingToggle size="md" />`.
- [ ] 6. Replace the inline toggle in `web/src/components/ListView.tsx` with `<BoardGroupingToggle size="sm" />` and drop the now-unused imports.
- [ ] 7. Add `web/tests/boardGrouping.test.mjs` covering order, ids and descriptors.
- [ ] 8. Run `make test` (Go tests, web tests, `tsc --noEmit`, `oxlint`) and fix until green.
- [ ] 9. Review the diff, commit, push and update the pull request.
