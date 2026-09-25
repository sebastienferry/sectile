# Tasks

- [x] Add `web/tests/browserRoot.mjs` and its unit test.
- [x] Use the helper in the ten browser tests; pass `resolve.preserveSymlinks`.
- [x] Correct the relative `PLAYWRIGHT_MODULE` run comments; add the README
      section.
- [x] Run every browser test from a path without `#` and from a copy under `#`.
- [x] Run `npm test` and `npm run lint` in `web/`.

## Validation

- `node --test tests/browserRoot.test.mjs`: 3 passed (path without `#`
  unchanged; path with `#` served through a link reaching the same `web/` and
  `shared/`; cleanup removes the link only; a temporary directory with `#` is
  refused).
- Browser suite from the batch worktree (no `#`): 10/10 pass.
- Browser suite from a copy of the branch under `.../wt/#417/web`: 10/10 pass
  (on `origin/main` the same layout fails: `UNLOADABLE_DEPENDENCY`, then
  `PARSE_ERROR` on the fixture). No `sectile-browser-*` directory left behind.
- `npm test` in `web/`: 192 passed, 0 failed. `npm run lint`: exit 0, no
  finding under `tests/`. `tsc -b --noEmit`: clean.
