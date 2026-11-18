# Tasks #931 - Keep Send visible while queueing

- [x] T1 Keep one Send control with contextual tooltip and existing admission rules.
- [x] T2 Consume accepted drafts synchronously; preserve typing and queue Edit.
- [x] T3 Update UI expectations; cover idle/busy Send and Enter, duplicate events,
  intentional repeated text, refusal, availability and queue feedback/order.
- [x] T4 Update Tauri README and both Unreleased Fixed sections.
- [x] T5 Run Tauri check, unit tests, UI tests, build and Rust tests; review full diff.
- [x] T6 Publish and verify the task PR against feat/tauri-monorepo; record stages.

## Validation - 2026-11-18

- `npm run check`: exit 0 (`tsc --noEmit`).
- `npm test`: exit 0; 114 passed, 0 failed.
- `npm run test:ui -- --maxWorkers=2`: exit 0; 32 files and 311 tests passed.
- `npm run build`: exit 0; 112 modules transformed. Vite reports its chunk-size
  warning for the main bundle.
- `npm run test:rust`: exit 0; 48 passed, 0 failed. Four existing macOS objc
  macro cfg warnings; no Rust source changed.
- A regression probe temporarily restored state-based draft reads: both
  duplicate-submission tests failed (two extra follow-ups were queued). Restoring
  the synchronous ref made them pass in the complete suite.
- The first unrestricted UI run, concurrent with the initial Rust compilation,
  passed 310 tests and failed the board filter test waiting for its initial
  ticket. The complete two-worker run passed all 311 without a board/test change.
- Full diff reviewed against `origin/feat/tauri-monorepo`; `git diff --check`
  passes. Production changes only affect the Tauri conversation composer.

## Publication and review

PR: https://github.com/sebastienferry/sectile/pull/934

The PR targets `feat/tauri-monorepo` and is ready for human review. The complete
diff satisfies the specification; no further defect was found. PR comments,
reviews and inline feedback were retrieved successfully and were empty. The
branch is current with its remote base. GitHub reports no check runs for this
PR; the local validation above is the available check evidence.

The first push published the branch successfully but could not save local
upstream configuration because shared `.git/config` is protected. No retry of
that configuration write was made; publication uses the explicit remote branch.
