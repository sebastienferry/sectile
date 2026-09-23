# Tasks

- [x] Clarify scope and inspect shared rendering and tracker availability rules.
- [x] Write behavior specification and implementation plan.
- [x] Capture baseline web build, lint and unit test results.
- [x] Reorganize the Story renderer into title, responsive content/metadata, pull requests.
- [x] Verify wide/narrow panel and modal layout and representative existing interactions in a browser.
- [x] Update the changelog and run web build, lint and unit tests.
- [x] Review complete diff against specification and current remote default branch.
- [x] Update the existing draft pull request, verify published head, and mark ready.

## Validation evidence — 2026-09-23

- Baseline and final `npm --prefix web run build`: passed (existing bundle-size warning).
- Baseline and final `npm --prefix web run lint`: exit 0, 62 warnings and no errors; warning categories/counts unchanged.
- Baseline and final `npm --prefix web test`: 159 passed, 0 failed.
- `PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs node web/tests/issue-detail-layout.browser.mjs`: passed against real components/styles in Chrome; panel and modal at 1440, 1024, 768 and 390 pixels, full-width title and PRs, content-first stacking, editor/save/rewrite/PR management, conditional team and creator.
- Visual inspection of the modal screenshot confirmed the requested hierarchy.
- Browser validation identified intrinsic flex widths on narrow views; both outer detail containers now allow shrinking. No handlers or data contracts changed.
- `git diff --check`: passed. Remote default branch had no commits missing from the issue branch; PR feedback and review-comment lists were empty when retrieved.
- Go checks were not rerun: no backend, API, generated contract or Go source changes; the affected WebUI build and suites were verified.
