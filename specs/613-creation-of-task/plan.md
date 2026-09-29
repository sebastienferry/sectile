# Plan #613 - The quick-add button no longer says "Création CLI"

## Stack

Web interface only: React and TypeScript in `web/`, tests run with
`node --test`. No server, agent, desktop or database change.

## Cause

`web/src/components/QuickAddModal.tsx:538` renders the hard-coded string
`Création CLI...` inside the submit button while `isSubmitting` is true. It
dates from the initial commit (`56eb340b`) and was never routed through the
translations. `handleSubmit` calls `createTask` (`web/src/context/AppContext.tsx`),
which sends `POST /api/tasks`; no CLI runs.

## Design

1. Add a `creating: string` key to the `quickAdd` block of the `Translations`
   type in `web/src/locales/translations.ts`, with a doc comment saying it is
   the submit button's label while a creation is in progress.
2. Fill it in both catalogs of the same file:
   - French `quickAdd.creating: 'Création…'`
   - English `quickAdd.creating: 'Creating…'`
   Both use the ellipsis character U+2026, as `macroLoading` already does.
3. In `QuickAddModal.tsx`, replace `<span>Création CLI...</span>` with
   `<span>{t.quickAdd.creating}</span>`. `t` is already in scope in the
   component.
4. Add a `Fixed` line under `## [Unreleased]` in `CHANGELOG.md`.

The key lives under `quickAdd`, not `taskModal`, because only this dialog uses
it (decision 1 of the clarification).

## Data contracts

None changed. The `Translations` type gains one required key; the existing
`web/tests/translationCatalog.test.mjs` already enforces that French and
English hold the same keys and that no value is empty.

## Target files

- `web/src/locales/translations.ts` - type and both catalog values.
- `web/src/components/QuickAddModal.tsx` - use the key.
- `web/tests/translationCatalog.test.mjs` or a new focused test - pin the two
  values and the absence of "CLI" in the component.
- `CHANGELOG.md` - `Fixed` line.

## Rejected alternatives

- Reusing `taskModal.create` ("Créer" / "Create") during submission: it would
  not tell that the creation is in progress.
- Naming the tracker in the label ("Création sur GitHub…"): the success toast
  already says where the ticket landed; the owner chose the short wording.
- "Création de la tâche…" / "Creating the task…": first proposal of the
  clarification, reversed by the owner.

## Note outside scope

`web/src/locales/translations.ts:1013` holds another "via CLI" wording
(`running: 'Exécution en cours via CLI...'`). It is outside this ticket's
scope, which the clarification limited to the quick-add dialog; it is left
unchanged and may deserve its own ticket.
