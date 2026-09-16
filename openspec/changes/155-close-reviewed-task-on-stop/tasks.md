# Tasks

## 1. Closing rule
- [ ] 1.1 Export `closingStep(task, project)` from `desktop/src/workflow.mjs`,
      returning the `handoff` skill id for a `reviewed` task on a configured
      project that exposes it, and `null` otherwise.

## 2. Desktop stop handler
- [ ] 2.1 After a successful `api.stop`, read the stopped run's task and project
      and evaluate `closingStep`, swallowing read failures.
- [ ] 2.2 Add the closing dialog: explanation, confirm button, status line.
- [ ] 2.3 Launch `handoff` on confirm, disable the button while in flight, close
      the dialog and refresh on success, report failure in place.

## 3. Verification
- [ ] 3.1 Unit tests for `closingStep`: reviewed/eligible, other stages, missing
      project configuration, missing skill, free console.
- [ ] 3.2 UI test: stopping a reviewed execution offers closure and launches
      `handoff`; stopping a non-reviewed one does not.
- [ ] 3.3 `npm test` and `npm run test:ui` in `desktop`, `npm run build` in `web`,
      `go build ./...`, `go vet ./...`, `go test ./...`.
