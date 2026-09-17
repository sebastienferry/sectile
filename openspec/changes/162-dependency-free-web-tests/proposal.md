# Make the web test suite runnable without installed dependencies

## Why
`npm test --prefix web` cannot be used as a gate. In any freshly created task worktree it fails
before running a single assertion, so every branch inherits failures it did not cause and a real
regression in those files is invisible.

The ticket reports three failing files. Five actually fail, and all five fail for the same single
reason: `ERR_MODULE_NOT_FOUND: Cannot find package 'typescript'`. These are exactly the five test
files that `import ts from 'typescript'` to `ts.transpileModule()` their subject and import the
result as a `data:` URL. A worktree is created by `git worktree add` alone, so it has no
`web/node_modules`, and the import fails.

The seven other test files in the suite already import their subject's `.ts` source directly —
Node 26 strips the types — and run with no dependencies at all. The five outliers are the only
reason the suite needs an install step. Aligning them on the convention the rest of the suite
already follows makes the gate work from a fresh clone or worktree, before anything is installed.

## What Changes
- Replace the `readFile` + `ts.transpileModule` + `data:` URL indirection in the five affected test
  files with a direct `import` of the subject module, as the other seven files already do.
- Remove the `typescript` import from those five files. No assertion is added, weakened or removed.

## Impact
- Changed: `web/tests/commandTemplate.test.mjs`, `web/tests/remoteRunIndicator.test.mjs`,
  `web/tests/runStates.test.mjs`, `web/tests/terminalSkillCommand.test.mjs`,
  `web/tests/workflowAdjustment.test.mjs`.
- Unchanged: every module under test, `web/package.json` (the `typescript` devDependency still
  serves `tsc -b` in the build), the test command, and the desktop and Go suites.

## Non-goals
- Provisioning `node_modules` in task worktrees. It is a real gap — `ensureTaskWorktree`
  (`internal/agent/agent_config.go`) runs `git worktree add` and nothing else, while
  `docs/REIMPLEMENTATION_GUIDE.md:36` states the intended behaviour is to symlink `node_modules`,
  `web/node_modules`, `.env` and `.env.local` — and it still breaks `npm run build` and
  `npm run lint` in a worktree. It is a Go change, tracked separately.
- Changing what the five test files assert, or the modules they exercise.
