# Plan #529 - Planning views translation

## Approach

Move Sectile-owned literals to the `planning` namespace
(`web/src/locales/planning.ts`, D1), grouped as `planning.roadmap`,
`planning.macro`, `planning.triage`, `planning.curation`, `planning.team`.
`lib/roadmap.ts` horizon labels stay `NOW`/`NEXT`/`FUTURE`; its French hints
and descriptions (placement states, anomalies) move to the catalog or are
resolved by the component from returned keys. Library functions keep their
return types for identifiers.

## Data contracts

None.

## Target files

`web/src/locales/planning.ts`, the components of FR1, display helpers of
`lib/roadmap.ts`, `lib/macroRuns.ts`, `lib/labelAxes.ts`,
`web/tests/planningCatalog.test.mjs`.

## Tests

Catalog test: roadmap tabs, triage empty state, a plural count in both
languages; existing `roadmapProjects`, `roadmapDisplayMode`, `macroRuns`,
`labelAxes` tests keep passing (updated only where a helper now takes its
strings as a parameter).
