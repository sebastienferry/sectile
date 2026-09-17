# Tasks

## 1. Convert the five test files
- [ ] 1.1 `web/tests/commandTemplate.test.mjs`: replace the `typescript` import, the `readFile` of
      `../src/lib/commandTemplate.ts`, the `transpileModule` call and the `data:` URL import with a
      single named import of `TEMPLATE_MODE_PLACEHOLDER`, `commandPreview`, `dropModelSlot`,
      `modelArgs`, `resolveTemplateMode` and `templateCarriesMode` — whichever of those the file
      actually uses — from `../src/lib/commandTemplate.ts`. Drop the now-unused `readFile` import.
- [ ] 1.2 `web/tests/runStates.test.mjs`: same conversion against `../../shared/runStates.ts`.
- [ ] 1.3 `web/tests/remoteRunIndicator.test.mjs`: same conversion against
      `../src/lib/remoteRunIndicator.ts`.
- [ ] 1.4 `web/tests/terminalSkillCommand.test.mjs`: same conversion against
      `../src/lib/terminalSkillCommand.ts`.
- [ ] 1.5 `web/tests/workflowAdjustment.test.mjs`: same conversion against `../src/lib/workflow.ts`.
- [ ] 1.6 Leave every helper, fixture and assertion in the five files untouched.

## 2. Verify
- [ ] 2.1 `grep -rn "typescript" web/tests/` returns nothing.
- [ ] 2.2 Run `npm test --prefix web` in this worktree, which has no `web/node_modules`: it exits 0
      and reports 78 tests, 0 failures.
- [ ] 2.3 Run `npm run lint --prefix web` if its dependencies are available; otherwise record that it
      could not run here and why.
- [ ] 2.4 Confirm `web/package.json` still declares `typescript` as a devDependency, since
      `npm run build` (`tsc -b`) needs it.

## 3. Close out
- [ ] 3.1 Update `CHANGELOG.md` if the repository keeps one for this kind of change.
- [ ] 3.2 Report the test count and exit status in the implementation note.
