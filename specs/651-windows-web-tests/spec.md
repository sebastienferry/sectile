# #651 — The browserRoot and skillLaunchModel web tests fail on Windows

Ticket: https://github.com/sebastienferry/sectile/issues/651
Type: Task — "Fix the browserRoot and skillLaunchModel web tests failing on Windows".
Branch: `feat/651`.
Clarification: [`docs/clarifications/651.md`](../../docs/clarifications/651.md).

## Context

`npm test` in `web/` gives 557/560 on Windows: two tests of `web/tests/browserRoot.test.mjs`
and one of `web/tests/skillLaunchModel.test.mjs` fail, on `origin/main` as well. PR #623
blamed the `#` of the task worktree path and CRLF line endings. The clarification reproduced
the three failures and found two causes, neither of them `#`:

- **Path separators** (both `browserRoot.test.mjs` failures). `browserRoot()` returns `web/`
  in forward slashes by contract, because the browser fixtures match Vite's module ids
  against it; the test builds its expected value with `path.join`, which uses `\` on Windows.
- **CRLF** (the `skillLaunchModel.test.mjs` failure). A Windows checkout with
  `core.autocrlf=true` has CRLF sources, and the regex that finds the shared `modeActions`
  fragment of `TaskCard.tsx` requires `)` directly followed by `\n`.

This ticket corrects the two test files. Out of scope: the behaviour of
`web/tests/browserRoot.mjs`, the TSX sources the assertions read, the browser tests
(`*.browser.mjs`), a repository line-ending policy (`.gitattributes`), the other web tests,
and the `EPERM` a Windows machine without Developer Mode or elevation would raise when the
`#` test creates its symbolic link.

This file states behaviour and acceptance criteria only. Implementation choices are in
[`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

## Decisions being specified

Settled by the owner in Round 2 of the clarification.

1. **Fix the tests, not the helper.** `browserRoot()` keeps returning forward slashes.
2. **Line endings are normalised in the test that reads the sources**, not pinned in a
   `.gitattributes`.
3. **No skip and no platform branch.** Both corrections are no-ops on Linux.

## User stories

### US1 (P1) — The browserRoot tests pass on Windows

As a contributor on Windows, I can run `node --test tests/browserRoot.test.mjs` from `web/`
and see it pass.

- **Given** a Windows checkout whose path holds no `#`,
  **When** the test file runs,
  **Then** its four tests pass, and `a path without # is served as it is` still checks that
  the returned root is the checkout's `web/` directory.
- **Given** a Windows checkout whose path holds `#`,
  **When** the test file runs,
  **Then** its four tests pass, and `a path with # relaunches the test through a link to the
  same checkout` still checks that the relaunched process gets the link's `web/` directory
  as its root.
- **Given** a root returned with backslashes or pointing at another directory,
  **When** the test file runs,
  **Then** the corresponding test fails.

### US2 (P1) — The skillLaunchModel tests pass on Windows

As a contributor on Windows, I can run `node --test tests/skillLaunchModel.test.mjs` from
`web/` and see it pass.

- **Given** a Windows checkout with CRLF line endings, with or without `#` in its path,
  **When** the test file runs,
  **Then** every test passes, `both card shapes share the model entry` included.
- **Given** a `TaskCard.tsx` whose shared `modeActions` fragment is missing, duplicated or
  no longer holds the model entry,
  **When** the test file runs,
  **Then** `both card shapes share the model entry` fails, whatever the line endings.

### US3 (P2) — Linux and CI are unchanged

- **Given** a Linux checkout (LF, forward slashes), as on CI,
  **When** `npm test` runs in `web/`,
  **Then** both files pass as before.

## Functional requirements

- **FR1** — `browserRoot.test.mjs` compares the root `browserRoot()` returns with the expected
  directory written in forward slashes, in its two tests that assert on `root`.
- **FR2** — `browserRoot.test.mjs` keeps every other assertion: the relaunch, the link
  resolving to the checkout, `shared/` reachable, the environment passed on, the cleanup, the
  two refusals.
- **FR3** — `skillLaunchModel.test.mjs` asserts on the sources with LF line endings, whatever
  the line endings of the checkout.
- **FR4** — No production file, fixture, `.gitattributes`, dependency or other test changes,
  and neither test is skipped or branched per platform.
- **FR5** — The pull request states the root cause of each of the three failures.

## Open requirements

None.
