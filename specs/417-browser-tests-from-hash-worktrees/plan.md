# Implementation plan

## Helper: `web/tests/browserRoot.mjs`

`browserRoot(testUrl)` returns `{ root, preserveSymlinks }` for the Vite server
of the test whose `import.meta.url` is `testUrl`:

- the real `web/` directory (forward slashes, no trailing slash) is derived
  from `testUrl`, as each test does today;
- when it holds no `#`: `{ root: realWeb, preserveSymlinks: false }`;
- otherwise: create a directory with `mkdtempSync(join(tmpdir(), 'sectile-browser-'))`,
  symlink the checkout root (the parent of `web/`, so `../../../shared` still
  resolves) inside it, and return `{ root: <link>/web, preserveSymlinks: true }`.
  Vite would otherwise resolve the link back to its real path. The temporary
  directory is removed on process exit (`rmSync` removes the link, never its
  target). A temporary directory that itself holds `#` is refused with a clear
  error.

## Tests

Each of the ten `web/tests/*.browser.mjs` replaces its own
`const root = fileURLToPath(...)` line with the helper and passes
`resolve: { preserveSymlinks }` to `createServer`. The run comments that gave
a relative `PLAYWRIGHT_MODULE` (`board-views`, `optional-views`) name an
absolute one; the others already say "pointing to an installed Playwright
module" and the README carries the full command.

A unit test `web/tests/browserRoot.test.mjs` (run by `npm test`) covers: a path
without `#` is returned unchanged; a path with `#` yields a `#`-free root
whose `web/` and sibling `shared/` resolve to the real directories; the
temporary directory is removed while the checkout stays intact.

## Documentation

README: a short "Browser tests" section with the command, the absolute
`PLAYWRIGHT_MODULE`, and the note that paths with `#` are handled.

## Validation

All ten browser tests from the batch worktree (no `#`) and from a
`git archive` copy under a `#` directory; `npm test` and `npm run lint` in
`web/`.

## Follow-up (2026-09-25): relaunch through the link

Returning `<link>/web` as the root was not enough in a worktree with its own
`web/node_modules`: Vite serves its client (`/@vite/client`) from its own
installation, which Node had resolved to the real, `#`-holding path, and the
request answered 404. See `docs/clarifications/417.md`, round 3.

- From a path holding `#`, `browserRoot` creates the same link, then runs the
  same test file through it (`spawnSync(process.execPath, [...execArgv,
  '--preserve-symlinks', '--preserve-symlinks-main', <link>/web/tests/<file>,
  ...argv])`) with `SECTILE_BROWSER_TEST_RELAUNCHED=1`, and exits with the
  child's status. It does not return in that process.
- In the relaunched process the path holds no `#`; the marker makes the helper
  return `preserveSymlinks: true`. A relaunched process that still sees `#` is
  refused rather than relaunching again.
- The link and its directory are still removed when the parent exits.
- `relaunch` and `env` are injectable so the unit test drives both processes
  without spawning.
