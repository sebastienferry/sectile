# #651 — Plan

Spec: [`spec.md`](spec.md). Checklist: [`tasks.md`](tasks.md).

## Stack and target files

- `web/tests/browserRoot.test.mjs` — the two tests asserting on `root` (FR1, FR2).
- `web/tests/skillLaunchModel.test.mjs` — its `read` helper (FR3).
- Read, not changed: `web/tests/browserRoot.mjs` (the forward-slash contract of `root`),
  `web/src/components/TaskCard.tsx` (the `modeActions` fragment), `web/package.json` (the
  `test` script, `node --test`).

## Decisions

- **D1 — Forward-slash the expectation.** A local `slashes = path => path.replace(/\\/g, '/')`
  in `browserRoot.test.mjs` converts the expected directory: `slashes(join(dir, 'web'))` in
  the first test, `slashes(web)` in the relaunch test. `root` is compared as returned, so a
  root with backslashes still fails (FR1). The `realpathSync` comparisons, the `script`
  assertions and the relaunch expectations are left as they are, since `path` functions
  accept both separators on Windows (FR2).
  - Rejected: `path.normalize(root)` or `realpathSync(root)` on the actual value. They would
    hide a helper that stopped returning forward slashes, which the browser fixtures need.
- **D2 — Normalise in `read`.** `read` becomes
  `readFile(…, 'utf8').then(text => text.replace(/\r\n/g, '\n'))`, so every assertion of
  the file sees LF (FR3).
  - Rejected: rewriting the anchored regexes to `\r?\n`, which leaves the next regex written
    against LF to break again; and a `.gitattributes` pinning LF, which renormalises every
    Windows contributor's checkout (owner's answer, Round 2).
- **D3 — No platform branch.** On Linux the paths hold no `\` and the sources no `\r`, so
  both conversions change nothing (FR4, US3).

## Side effects

None: test-only change, no production code, no fixture.

## Verification

1. From `web/`, in this worktree (no `#` in its path): `node --test tests/browserRoot.test.mjs`
   and `node --test tests/skillLaunchModel.test.mjs`.
2. The same two commands in a temporary `git worktree` of the branch whose path holds `#`,
   removed afterwards.
3. `npm test` in `web/` once, on Windows.
4. CI runs the full suite on Linux on the pull request.
