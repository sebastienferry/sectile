# Plan #532 - Operations feedback translation

## Approach

1. Move Sectile-owned literals of FR1 to the `operations` namespace
   (`web/src/locales/operations.ts`, D1): `operations.activities`,
   `operations.sync`, `operations.notifications`, `operations.projects`,
   `operations.connection`.
2. `AppContext.tsx`: every `addToast` with a French literal title or
   description uses the catalog (`format` for parameters). Raw server errors
   (`err.message`, `body.error`) are appended as detail, never translated.
3. `web/src/lib/activityText.ts`:

```ts
export interface ActivityTemplate { key: string; pattern: RegExp }
export function localizeActivityText(text: string, locale: Locale): string
```

   A table of anchored patterns, one per server template, with named
   captures, and per-locale renderers. French returns the input unchanged;
   English renders the template from the `operations.activityTemplates`
   catalog with the captures. `ActivitiesView`, `TaskDetailModal` activity
   list and `RemoteRunBadge` call it on `action`, `summary` and each step.

## Template inventory (server, 2026-09-26)

From `internal/handlers/handlers.go`, `internal/db/db.go` and the agent:
`Exécution de %s sur l'agent local`, `Exécution en cours sur l'agent local (%s)`,
`Tâche ciblée : %s - %s`, `Déléguée à l'agent local (%s)...`,
`Étape de %s ➔ %s`, `Passage de %s à l'étape « %s » [%s] en file d'attente`,
`Cible : %s ➔ %s`, `Statut tracker visé : %s`,
`Poussée dans la file d'attente d'exécution...`, `Écriture tracker`,
`Synchronisation %s en file d'attente`, `Synchronisation terminée`,
`Synchronisation des tickets distants`, `Échec de la synchronisation tracker`,
`Échec de l'écriture sur le tracker`, `Tâche %s passée à l'étape « %s »`, and
the other templates the implementation inventory finds. Each is covered by a
test with a real sample.

## ADR

`docs/adrs/0035-activity-text-is-localized-by-the-web.md`: context, decision
(web recognizer), consequences (server wording is a contract tested by
fixtures; new server templates show in French until added), rejected
alternative (message IDs in storage) and when to revisit it.

## Target files

`web/src/locales/operations.ts`, `web/src/lib/activityText.ts`,
`ActivitiesView.tsx`, `SyncView.tsx`, `DegradedReadBanner.tsx`,
`ToastContainer.tsx`, `RunStateGlyph.tsx`, `AppContext.tsx`,
`web/tests/activityText.test.mjs`, `web/tests/operationsCatalog.test.mjs`,
the ADR.
