# Run the browser tests from task worktrees whose path contains '#'

Clarification: `docs/clarifications/417.md` (option 2 settled in round 2).

## User story (P1)

As a Sectile contributor working in a task worktree such as
`.tasks/worktrees/#387`, I run any `web/tests/*.browser.mjs` with the
documented command and it runs against the code of that worktree, so a failure
means a regression and not a path Vite cannot read.

## Requirements and acceptance scenarios

1. Given a checkout whose absolute path contains `#`, when a browser test runs
   with the documented command, then Vite serves the checkout's own `web/` and
   `shared/` sources and the test result depends only on the code.
2. Given a checkout whose path has no `#`, when a browser test runs, then Vite
   serves it from its real path, as before.
3. Given any browser test file, when it starts its Vite server, then it takes
   its root from the one shared helper, so a new test inherits the behaviour.
4. Given a task worktree, which has no `desktop/node_modules`, when a
   contributor follows the documented command, then Playwright is found
   (absolute `PLAYWRIGHT_MODULE`).
5. Given the test run ends, whether it passes or fails, then nothing it created
   is left behind in the checkout, and only a temporary link it created itself
   is removed.

## Out of scope

Renaming task worktrees (options 1 and 3 rejected), running the browser tests
from `npm test`, installing Playwright in worktrees. No changelog entry.
