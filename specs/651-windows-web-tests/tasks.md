# #651 — Implementation checklist

References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. browserRoot test (D1, FR1, FR2)

- [x] T1.1 Add the `slashes` helper to `web/tests/browserRoot.test.mjs` and compare `root`
      with `slashes(join(dir, 'web'))` in `a path without # is served as it is`.
- [x] T1.2 Compare `root` with `slashes(web)` in `a path with # relaunches the test through a
      link to the same checkout`; leave every other assertion unchanged.

## 2. skillLaunchModel test (D2, FR3)

- [x] T2.1 Normalise CRLF to LF in the `read` helper of `web/tests/skillLaunchModel.test.mjs`.

## 3. Verification (US1–US3, FR4)

- [x] T3.1 Run both files with `node --test` from `web/` in this worktree.
- [x] T3.2 Run both files from a temporary worktree whose path holds `#`, then remove it.
- [x] T3.3 Run `npm test` in `web/` once.

## 4. Pull request (FR5)

- [x] T4.1 Write the root cause of each failure in the pull request description.

## Test plan

- Automated: the two test files on Windows, from paths with and without `#`, and `npm test`
  in `web/`. CI runs the suite on Linux on the pull request (US3).
- Regression check: the corrected assertions still fail when `root` comes back with
  backslashes, and the fragment test still fails when `modeActions` is missing (US1, US2).
