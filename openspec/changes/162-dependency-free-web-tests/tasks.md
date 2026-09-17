# Tasks

## 1. Convert the five test files
- [x] 1.1 `web/tests/commandTemplate.test.mjs`: replace the `typescript` import, the `readFile` of
      `../src/lib/commandTemplate.ts`, the `transpileModule` call and the `data:` URL import with a
      single named import of `TEMPLATE_MODE_PLACEHOLDER`, `commandPreview`, `dropModelSlot`,
      `modelArgs`, `resolveTemplateMode` and `templateCarriesMode` — whichever of those the file
      actually uses — from `../src/lib/commandTemplate.ts`. Drop the now-unused `readFile` import.
- [x] 1.2 `web/tests/runStates.test.mjs`: same conversion against `../../shared/runStates.ts`.
- [x] 1.3 `web/tests/remoteRunIndicator.test.mjs`: same conversion against
      `../src/lib/remoteRunIndicator.ts`.
- [x] 1.4 `web/tests/terminalSkillCommand.test.mjs`: same conversion against
      `../src/lib/terminalSkillCommand.ts`.
- [x] 1.5 `web/tests/workflowAdjustment.test.mjs`: same conversion against `../src/lib/workflow.ts`.
- [x] 1.6 Leave every helper, fixture and assertion in the five files untouched.

## 2. Verify
- [x] 2.1 `grep -rn "typescript" web/tests/` returns nothing.
- [x] 2.2 Run `npm test --prefix web` in this worktree, which has no `web/node_modules`: it exits 0
      and reports 78 tests, 0 failures.
- [x] 2.3 Run `npm run lint --prefix web` if its dependencies are available; otherwise record that it
      could not run here and why.
- [x] 2.4 Confirm `web/package.json` still declares `typescript` as a devDependency, since
      `npm run build` (`tsc -b`) needs it.

## 3. Close out
- [x] 3.1 Update `CHANGELOG.md` if the repository keeps one for this kind of change.
- [x] 3.2 Report the test count and exit status in the implementation note.

## Notes
- 2.3 `npm run lint --prefix web` could not run in this worktree: `sh: oxlint: command not found`,
  since `git worktree add` provisions no `web/node_modules`. Tracked as the separate non-goal.
- 3.1 The repository keeps no `CHANGELOG.md`.
- 3.2 `npm test --prefix web`: 78 tests, 78 pass, 0 fail, exit 0, with no installed dependencies.
