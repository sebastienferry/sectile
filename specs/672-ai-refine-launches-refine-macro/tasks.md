# Tasks #672 - "AI refine" launches the refine-macro skill

Order matters: each step leaves the tree building and green.

## 1. Run helpers (US3)

- [x] T1.1 `macroSkillId` and the `skillId` argument of `macroLaunchBlocker`,
      with `otherRunning` (D3).
- Tests (`web/tests/macroRuns.test.mjs`): aliases are matched; an active run
  of the button's skill gives `alreadyRunning`, another skill's gives
  `otherRunning`; without `skillId` the old behaviour holds.

## 2. Shared runs and the generic button (US1, US2, US3, US4)

- [x] T2.1 `useMacroRuns` with `onRunEnded` (D2).
- [x] T2.2 `MacroRealignButton` becomes `MacroSkillButton` (D1).
- [x] T2.3 Strings: `refine` block, shared strings, reworded tooltip (D5).
- [x] T2.4 `RoadmapView`: both buttons from one `useMacroRuns`, the refine
      precondition (blank framing, dirty draft), the refresh on run end (D4).

## 3. Removal of the deterministic generator (US5)

- [x] T3.1 Web: the modal, its state, `refineMacro`, the types, the strings.
- [x] T3.2 Go: the routes, `HandleMacroRoute`, the generators, the model and
      `TestRefineMacro`.

## 4. Tests and documentation

- [x] T4.1 Browser test `web/tests/roadmap-macro-skills.browser.mjs`: AI refine
      posts `run-skill` with `refine_macro`; blank framing warns and launches
      nothing; a dirty draft is saved before the launch; no agent disables it
      with the reason; a running realign run disables AI refine with "another
      skill" and shows on Realign only; the run ending re-reads the macros.
- [x] T4.2 `CHANGELOG.md`: a `Changed` and a `Removed` line.

## Test plan

- `cd web && node --test tests/*.test.mjs`
- `cd web && npx tsc -b && npx oxlint`
- `cd web && node tests/roadmap-macro-skills.browser.mjs` and the other
  `roadmap-*.browser.mjs`
- `go build ./... && go vet ./... && go test ./internal/db/ ./internal/handlers/`
- Manual: on a macro with a framing text, click **AI refine** with the agent
  connected; the run shows, the skill starts, its TODOs appear once it ends.
