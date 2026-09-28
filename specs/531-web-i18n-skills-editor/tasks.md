# Tasks #531 - Skill editor translation

Needs #534 T1 and T2.

- [x] T1 `SkillsView.tsx`: modes and help, empty state, list indicators and
  tooltips, macro skill labels.
- [x] T2 `SkillsView.tsx`: editor placeholder, reset/save/regenerate/import
  feedback, update metadata with `formatDateTime`.
- [x] T3 `CommandModePreview.tsx`, display fallbacks of `lib/workflow.ts` and
  `lib/commandPresets.ts`.
- [x] T4 `web/tests/skillsEditorCatalog.test.mjs`.
- [x] T5 `npm run build`, `npm run lint`, `npm test`.

## Implementation notes

- The editor has no placeholder, no "updated by" author and no
  regenerate/import toasts today: nothing was invented; the footer date
  ("modifiée le {date}") uses `formatDateTime`.
- The next-step labels of `lib/workflow.ts` only show on the board card, so
  they moved with #526 (`shell.nextStep`).
- The command preview errors used to be English in the French interface;
  they now follow the language (`commandPreview` takes optional messages,
  English by default).
