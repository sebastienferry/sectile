# Plan #531 - Skill editor translation

## Approach

Move Sectile-owned literals to the `skillsEditor` namespace
(`web/src/locales/skillsEditor.ts`, D1): `skillsEditor.modes`,
`skillsEditor.list`, `skillsEditor.indicators`, `skillsEditor.editor`,
`skillsEditor.feedback`. The existing `t.skills` keys stay where they are
used. Display fallbacks of `lib/workflow.ts` (French stage names used when
`skillLabel` has none) take a catalog value; the stage identifiers stay.

## Data contracts

None.

## Target files

`web/src/locales/skillsEditor.ts`, `SkillsView.tsx`,
`CommandModePreview.tsx`, display fallbacks of `lib/workflow.ts` and
`lib/commandPresets.ts` when French, `web/tests/skillsEditorCatalog.test.mjs`.

## Tests

Catalog test: mode labels and the divergence tooltip in both languages;
`skillLaunchMode.test.mjs`, `commandPresets.test.mjs` keep passing.
