# Tasks #532 - Operations feedback translation

Needs #534 T1 and T2.

- [ ] T1 `AppContext.tsx` notifications and operation messages (title and
  body from the catalog, raw detail appended).
- [ ] T2 `DegradedReadBanner.tsx`, `ToastContainer.tsx`, `RunStateGlyph.tsx`.
- [ ] T3 `ActivitiesView.tsx` labels, states, dates.
- [ ] T4 `SyncView.tsx`.
- [ ] T5 `lib/activityText.ts` with the template inventory, wired in
  `ActivitiesView.tsx` (and the activity lists of #526/#527 components by
  their owners, through the exported function).
- [ ] T6 `web/tests/activityText.test.mjs` (every template, unmatched text,
  French identity), `web/tests/operationsCatalog.test.mjs`.
- [ ] T7 ADR 0035.
- [ ] T8 `npm run build`, `npm run lint`, `npm test`.
