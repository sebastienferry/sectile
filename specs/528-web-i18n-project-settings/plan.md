# Plan #528 - Project settings translation

## Approach

Move Sectile-owned literals to the `projectSettings` namespace
(`web/src/locales/projectSettings.ts`, D1), grouped as
`projectSettings.tabs`, `.general`, `.repositories`, `.views`, `.tracker`,
`.providers`, `.workflow`, `.execution`, `.skills`, `.columns`,
`.feedback`. Existing keys already used by the modal (`t.skills`, `t.sdd`,
the `Project settings, AI provider, tracker and skills` subtitle) stay.
`lib/trackers.ts` and `lib/optionalViews.ts` display labels, if French,
move to the catalog; their identifiers stay.

## Provider copy (FR3)

- GitHub: two-way sync over the GitHub REST API; workflow stages are
  reflected as labels (`#new`, `#clarified`, `#specified`, …) and the
  Open/Closed state.
- GitLab: same over the GitLab REST API, labels and issue state.
- Jira: over the Jira Cloud REST API, workflow stages as labels, status
  transitions as configured.
- Background sync help: what the periodic sync reads and writes.

## Data contracts

None.

## Target files

`web/src/locales/projectSettings.ts`, `ProjectModal.tsx`,
`BoardColumnsEditor.tsx`, display helpers of `lib/trackers.ts` and
`lib/optionalViews.ts` when French, `web/tests/projectSettingsCatalog.test.mjs`.

## Tests

Catalog test: tabs and a provider description in both languages, no "CLI"
in the tracker provider descriptions; the French browser harnesses
(`optional-views`, `spec-artifacts-option`) and
`projectModalExecution.test.mjs` keep passing.
