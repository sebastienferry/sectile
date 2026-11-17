# Implementation tasks — #920

Implementation is preserved on feat/920. Verification and publication blockers are recorded below.

1. [x] Recheck the accepted clarification and current sidebar/status inventory against spec.md. Preserve the assigned branch, clarification and existing code; report any newly conflicting semantics before changing scope.
2. [x] Add the persistent footer help button and locally controlled accessible dialog. Keep it outside sidebar-scroll and independent of selected conversation and execution lifecycle.
3. [x] Add the seven skill and four conversation/fallback entries using production symbol classes. Explain identity versus state, session versus work, required response/checks, unknown reasons and skill completion versus ticket completion.
4. [x] Implement title/accessible name, initial focus, Tab/Shift+Tab containment, explicit Close, Escape and focus restoration, including sidebar disappearance. Keep decorative symbols out of the accessibility tree.
5. [x] Apply viewport-bounded wrapping/scrolling layout and existing reduced-motion behavior. Ensure footer access with an empty list and a long scrolling list.
6. [x] Add focused UI tests: keyboard/click opening; all inventory meanings; Close/Escape; focus behavior; no selected-run/draft changes or execution mutation; empty/long lists; sidebar unmount. Reuse existing status fixtures without altering reconciliation expectations.
7. [ ] Perform browser/native verification: compare all symbols with production rows, normal/reduced motion, 320 CSS pixel viewport, overflowing content, keyboard traversal and restoration, active execution continuing while help is open. Record evidence and limitations.
8. [x] Update docs/USER_GUIDE.md with access, contextual status explanations and completion caveat; add one Added entry under CHANGELOG.md [Unreleased]. Update other feature documentation only where affected.
9. [ ] Run the desktop-tauri required checks: npm run check, npm test, npm run test:ui, npm run build and npm run test:rust. Resolve failures attributable to the change and record external blockers truthfully.
10. [ ] At implemented stage only, follow configured creation-owner checks, commit/push policy and PR discovery/creation policy. Reuse an existing PR if present, preserve attained stage and never advance to reviewed as part of creation recovery. No PR is created during specification.

## Specification verification

- [x] Applied accepted clarification round 2; no open product decisions.
- [x] Rechecked all seven skill states and four conversation/fallback classes against App.tsx, App.css and skill-status.ts.
- [x] Kept observable behavior separate from implementation choices.
- [x] Defined target files and implementation validation.
- [x] Verified specification files are not ignored; deliver them in a local specification commit before reporting specified.

## Implementation recovery report (2026-11-17)

- Added `desktop-tauri/src/components/SidebarHelpDialog.tsx`: fixed-footer trigger and native modal with eleven contextual status entries, Close/Escape handling, focus return and sidebar-unmount fallback to Settings. No execution APIs or network access.
- Updated `desktop-tauri/src/App.tsx` to place help outside the scrolling list; `App.css` reuses production symbols and motion rules with viewport-bounded scrolling and theme colors.
- Added `tests/sidebar-help.ui.test.tsx` and workspace integration coverage; `vitest.setup.ts` supplies the native dialog DOM methods absent from jsdom. Native focus containment is not claimed as proven by jsdom.
- Updated `docs/USER_GUIDE.md` and the Added changelog section.
- `npm run check`: passed, `tsc --noEmit`.
- `npm run build`: passed, `built in 477ms`; existing large-chunk advisory remains.
- `npm run test:ui`: `Test Files 32 passed (32)`, `Tests 287 passed (287)`.
- `npm test`: `pass 105`, `fail 1`. The unchanged local-update test crashes Node 26.7.0 with `Assertion failed: (env_->execution_async_id()) == (0)`. A bounded retry reproduced it. The workstation's node@24 path also reports Node 26.7.0, so it is not a separate runtime. No test or script in that path was modified.
- `npm run test:rust`: `41 passed; 7 failed`. All seven failures occur at unchanged test-server `TcpListener::bind("127.0.0.1:0")` calls, with `PermissionDenied`, `Operation not permitted`. Per AGENTS.md, the denied intention was not retried via escalation or another route. No Rust code changed.
- Browser verification: computer-use tool reported `The browser access host is unavailable`. Actual 320px layout, overflow, Tab/Shift+Tab containment, reduced-motion rendering, retained draft and live execution continuity remain unverified in a browser/native WebView.
- An incidental process inspection returned `zsh:1: operation not permitted: ps`; no alternate process-inspection route was attempted.
- `git diff --check` passed. Read the complete production and test diff; accepted clarification and specification remain intact.
- `git fetch origin` succeeded with network escalation; HEAD is zero commits behind origin/main. GitHub open-PR lookup returned `[]` for feat/920. No PR created because required checks are not green and browser verification is unavailable. No workflow transition; task remains specified, never reviewed.
- Next action: obtain authorization/environment access for the denied local-server checks, resolve or baseline the Node crash, perform browser verification, then finish creation-owner checks, commit/push and create a draft PR before reporting implemented. Preserve this implementation and completed checklist items on retry.

Detailed command output is retained at `/tmp/920-ui-verified.log`, `/tmp/920-unit.log`, `/tmp/920-unit-retry.log`, `/tmp/920-rust.log` and `/tmp/920-build.log` on this workstation.
