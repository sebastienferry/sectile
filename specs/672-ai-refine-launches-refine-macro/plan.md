# Plan #672 - "AI refine" launches the refine-macro skill

## Stack

Web front end (React, TypeScript) in `web/src`, Go server in `internal/` and
`cmd/server`. No migration, no agent change, no new route.

## Design

### D1 - `MacroSkillButton`

`web/src/components/MacroRealignButton.tsx` becomes
`web/src/components/MacroSkillButton.tsx`. Props:

- `projectId`, `macroKey`;
- `skillId` (`realign_macro` or `refine_macro`);
- `strings`: the skill's string block (`planning.macro.realign` or
  `planning.macro.refine`);
- `icon` and `tone` (`sky` with `GitCompareArrows`, `orange` with `Sparkles`);
- `runs` and `refresh`, from the shared hook (D2);
- `showsForeignRuns`: the button also shows a run no button owns (Realign);
- `beforeLaunch?: () => Promise<boolean>`: a precondition; `false` cancels;
- `onError`, `onLaunched`.

The launch, stop and force-close logic is the realign button's, unchanged.

### D2 - `useMacroRuns`

`web/src/hooks/useMacroRuns.ts` holds the polling the realign button held
(5 s while a run is active, 30 s otherwise, answers for a macro left since are
dropped) and returns `{ runs, refresh }`. It takes `onRunEnded`: when the
active run seen at the previous read is no longer active, it is called once
(D4).

### D3 - Matching a run to a skill

`web/src/lib/macroRuns.ts` gains:

- `macroSkillId(name)`: trims, lowercases, strips a `sectile:` prefix, turns
  `-` into `_`, then maps `refine` to `refine_macro` and `realign` to
  `realign_macro` (the catalog aliases of `internal/skills/catalog.go`).
- `macroLaunchBlocker(agents, projectId, runs, strings, userId, skillId)`: an
  active run of `skillId` gives `alreadyRunning`, any other active run gives
  `otherRunning`. Without `skillId` every active run gives `alreadyRunning`,
  as before.
- `MacroLaunchBlockerStrings.otherRunning`.

### D4 - Refresh on run end

`RoadmapView` uses `useMacroRuns` for the selected macro with
`onRunEnded = () => fetchProjectMacros(projectId).then(setMacroMeta)`.

### D5 - Strings

`planning.macro.refine` mirrors `planning.macro.realign` (French and English).
`planning.macro.otherSkillRunning` ("another skill is running on this macro")
and `planning.macro.foreignRunning` ("{skill} running") are shared.
`framing.refineTitle` is reworded; `framing.refineFailed` and
`framing.refineLaunched` mirror the realign toasts.

### D6 - Removal

- Go: both `/refine` branches of `internal/handlers/handlers.go`,
  `HandleMacroRoute` and its registration in `cmd/server/main.go`,
  `RefineMacro`, `GenerateMacroTodosFromFraming`,
  `GenerateProposedMacroTasksFromFraming` in `internal/db/macros.go`,
  `models.ProposedMacroTask`, `TestRefineMacro`. Each helper only used by them
  goes too.
- Web: `refineMacro` in `AppContext`, `RefineMacroResult` and
  `ProposedMacroTask` in `types`, the `refinePreview` modal, its state and
  `isRefining` in `RoadmapView`, the strings `refineModal.*`,
  `noGeneratedData*`, `operations.notifications.macros.refineRefused` and
  `refineFailed`.

### Rejected alternatives

- Two independent buttons each polling the runs: the realign button would show
  a refine run as "Realignment running".
- A third, generic run indicator for foreign runs: more UI for a case that
  only arises from a hand-started MCP run.

## Target files

- `web/src/components/MacroSkillButton.tsx` (renamed)
- `web/src/hooks/useMacroRuns.ts` (new)
- `web/src/lib/macroRuns.ts`
- `web/src/components/RoadmapView.tsx`
- `web/src/context/AppContext.tsx`, `web/src/types/index.ts`
- `web/src/locales/planning.ts`, `web/src/locales/operations.ts`
- `internal/handlers/handlers.go`, `cmd/server/main.go`
- `internal/db/macros.go`, `internal/db/macros_test.go`,
  `internal/models/models.go`
- `web/tests/macroRuns.test.mjs`, `web/tests/roadmap-macro-skills.browser.mjs` (new)
- `CHANGELOG.md`
