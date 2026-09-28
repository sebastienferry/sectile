# Implementation checklist

- [x] Move selected run state and skill result into the task footer and adapt responsive styles.
- [x] Hide the redundant current-step badge while retaining action names and Next labels.
- [x] Extend Electron UI coverage for footer placement, selection, polling, and narrow layout.
- [x] Add a user-facing changelog entry.
- [x] Run desktop build, unit tests, UI tests, and diff checks.
- [x] Review the full diff, publish the branch, and verify the ready PR.

## Validation evidence

Desktop build, JavaScript syntax checks, diff checks, and strict OpenSpec validation passed. Unit tests: 145/145. Complete Electron UI suite: 59/59. After integrating current main, the build and unit checks passed again and all six affected UI regressions passed. Footer placement was visually inspected at 800 px. PR #562 was verified open and ready for review with the published implementation commit; no review feedback was present.
