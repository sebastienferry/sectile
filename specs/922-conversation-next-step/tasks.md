# Tasks #922

- [x] Clarify scope and acceptance criteria; zero open product questions.
- [x] Write Spec Kit specification, plan and checklist.
- [x] Reuse shared header icon-button presentation and remove redundant CSS.
- [x] Add Fixed entries and update Tauri README.
- [x] Run typecheck, unit tests, UI tests, build and Rust tests (failures below).
- [x] Verify header appearance and interaction states in the browser.
- [x] Review the complete diff and commit implementation.
- [ ] Verify current remote base, publish a draft PR and record implemented.
- [ ] Separate review stage (outside this code-issue invocation; do not advance to reviewed).

## Validation - 2026-11-17

- `npm run check`: passed (exit 0).
- `npm run test:ui`: 31 files and 282 tests passed, including the existing
  next-step tooltip and eligible-skill callback test (exit 0).
- `npm run build`: passed (exit 0); existing large-chunk warning remains.
- `npm test`: 105 passed, 1 failed. Node 26.7.0 aborts in
  `tests/local-update.test.ts` with
  `Assertion failed: (env_->execution_async_id()) == (0)`.
  Reproduced using the original test and helper exported with `git show HEAD`
  to `/tmp/issue-922-baseline/`; neither file was changed by this ticket.
  The installed `node@24` path also reports 26.7.0, and a bounded attempt with
  `--test-isolation=none` reproduces the native abort (exit 134). No tests
  were skipped or modified to hide it.
- `npm run test:rust`: 41 passed, 7 failed (exit 101). All seven fail at
  fixture `TcpListener::bind("127.0.0.1:0")` with
  `Os { code: 1, kind: PermissionDenied, message: "Operation not permitted" }`.
  Rust code is unchanged. Do not retry these denied operations without the
  user's decision under the session guardrail.
- `git diff --check`: passed. Production diff is one class replacement and
  deletion of the obsolete button-specific CSS rule; documentation has one
  additional line in each changelog and the README. No new test was added
  for the reversible styling-only change; existing UI tests cover behavior.

## Pending recovery

- `git fetch origin` was refused while writing `FETCH_HEAD`; user approval
  was requested before any retry. Cached refs show this branch forks from
  `origin/feat/tauri-monorepo` at `c88fed19`, rather than `origin/main`.
- Pinchtab could not start because its service log under `~/.pinchtab` was
  denied; user approval was requested. Browser verification at 800, 1050 and
  1280 pixels, including hover/focus/disabled appearance, remains incomplete.
- A process-status diagnostic was refused (`operation not permitted: ps`)
  and stopped; it is unnecessary for completing this ticket.
- Ask permission for an elevated test retry, resolve the unit runtime abort,
  finish browser verification, fetch the current base and rerun affected
  checks before publishing. The failing suites prevent PR creation under the
  pickup skill. The task remains specified; do not mark implemented/reviewed.

## Authorized recovery validation - 2026-11-18

- Preserved the accepted implementation and specification. Fetched origin and
  rebased onto `origin/feat/tauri-monorepo` at `0645ecff`; resolved the root
  changelog conflict by retaining both user-visible fixes.
- `npm run check`: exit 0.
- `npm run test:ui`: exit 0; `Test Files 31 passed (31)`,
  `Tests 282 passed (282)`. Existing next-step interaction coverage passed.
- `npm run build`: exit 0; existing 500 kB chunk warning remains.
- `npm run test:rust` with authorized elevation: exit 0;
  `48 passed; 0 failed`; existing objc macro cfg warnings remain.
- Node 26.7.0 `npm test` still has the previously reproduced baseline native
  assertion in local-update.test.ts (105 passed, 1 failed). Supported Node
  22.23.5 passes the complete suite with local socket access:
  `npm exec --yes --package=node@22 -- node --experimental-strip-types --test tests/*.test.ts`;
  `tests 114`, `pass 114`, `fail 0`. No tests were skipped or changed.
- PinchTab UX fixture on port 1423: temporarily marked the task conversation
  completed and renameable, then disconnected the agent for disabled checks.
  Restored both fixture files byte-for-byte before committing.
- At 800/1050/1280 px, Next step, Rename and Archive are all 32 by 32 px,
  top 87 px, with centered 18 by 18 px next-step chevron; all controls fit.
  Next-step left positions: 630/880/1110 px. Hover uses the shared background;
  keyboard focus has a 2 px solid accent outline. Disconnected next-step is
  disabled with opacity 0.45 and unchanged bounds. Tooltip remains
  `Next step: Implement`. Initial active conversation hides the next-step.
- Visual inspection confirmed consistent alignment and sizing at minimum and
  desktop widths; browser geometry verified the intermediate responsive width.
- `git diff --check`: exit 0. No production logic or test changes needed.
