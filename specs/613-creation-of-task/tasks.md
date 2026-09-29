# Tasks #613 - The quick-add button no longer says "Création CLI"

Ordered checklist. Each group is one commit and leaves the tree buildable.

## 1. Translated label (FR1, FR2, FR3, FR4)

- [ ] T1.1 `web/src/locales/translations.ts`: add `creating: string` to the
  `quickAdd` type, with a doc comment.
- [ ] T1.2 Same file: French `creating: 'Création…'`, English
  `creating: 'Creating…'` (U+2026).
- [ ] T1.3 `web/src/components/QuickAddModal.tsx`: render
  `{t.quickAdd.creating}` instead of the hard-coded `Création CLI...`.

## 2. Tests

- [ ] T2.1 A `node --test` case asserting `translations.fr.quickAdd.creating`
  is `'Création…'` and `translations.en.quickAdd.creating` is `'Creating…'`.
- [ ] T2.2 A case asserting `QuickAddModal.tsx` contains no `CLI` text and
  reads `t.quickAdd.creating`.
- [ ] T2.3 `web/tests/translationCatalog.test.mjs` stays green (same keys in
  both languages, no empty value).

## 3. Documentation (FR5)

- [ ] T3.1 `CHANGELOG.md`: one line under `### Fixed` in `[Unreleased]`.

## 4. Verification

- [ ] T4.1 `node --test web/tests/` in `web/`.
- [ ] T4.2 Type check and lint of `web/` (`npx tsc --noEmit`, `npx oxlint`),
  with the main checkout's `node_modules` if the worktree has none.
- [ ] T4.3 Manual check: open the quick-add dialog in French then in English,
  submit, and read the in-progress label.
