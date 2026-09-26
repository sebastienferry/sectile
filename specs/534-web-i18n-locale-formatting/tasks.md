# Tasks #534 - Locale formatting, document language, regression checks

Delivered first in the batch (T1, T2), since the companion tickets use them;
T3 onward after the companion tickets.

## 1. Shared helpers (FR1 to FR3)

- [ ] T1.1 `web/src/lib/i18n.ts` with the contract of the plan.
- [ ] T1.2 `web/tests/i18n.test.mjs`: format (repeated and unknown
  placeholders), plural 0/1/5 in both locales, date-only in a negative time
  zone (`TZ=America/Los_Angeles` sub-process or offset check), instants,
  invalid and missing values, number grouping, `resolveInitialLocale` order.

## 2. Catalog scaffolding (D1)

- [ ] T2.1 Empty surface modules (`shell`, `taskDetail`, `projectSettings`,
  `planning`, `sprints`, `skillsEditor`, `operations`, `signIn`) merged into
  `translations`, plus `app.documentTitle`.
- [ ] T2.2 `web/tests/translationCatalog.test.mjs`: parity of keys,
  non-empty values and placeholder sets for the whole catalog.

## 3. Document language (FR4)

- [ ] T3.1 `main.tsx` startup locale, `index.html` neutral title.
- [ ] T3.2 `AppContext.tsx` effect on `settings.language`.

## 4. Call sites (FR5)

- [ ] T4.1 `AdminView.tsx`, `UsersPanel.tsx` dates.
- [ ] T4.2 Check that the companion tickets migrated theirs
  (`grep toLocale` over `web/src` returns only `lib/i18n.ts`).

## 5. Regression checks (FR6 to FR8)

- [ ] T5.1 `web/tests/i18n-shell.browser.mjs`: representative components in
  English, accessible names asserted, a user-authored French title shown
  unchanged.
- [ ] T5.2 `docs/web-translation-checks.md` check matrix.

## 6. Verification

- [ ] T6.1 `npm run build`, `npm run lint`, `npm test` in `web/`.
