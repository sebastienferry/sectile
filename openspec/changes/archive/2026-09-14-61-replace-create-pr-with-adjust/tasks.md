# Implementation checklist

Implementation complete. Verification evidence and test-strategy details are recorded in `validation.md`. The existing PR remains draft for the separate adjustment/review stage.

## 1. Identity, compatibility, and dispatch

- [x] Add canonical `adjust` / `adjust-issue` and centralized legacy normalization in `internal/models/models.go` and `internal/db/skilltemplates.go`.
- [x] Update `internal/db/board.go`, job dispatch/completion/postback maps in `db.go`, managed-run detection in `skillresult.go`, and `internal/runner/runner.go`; retain workflow states and reviewed autonomous stop.
- [x] Test canonical and legacy IDs across queued jobs, stage routing, command preparation, managed guards, and repeat adjustment; ensure reviewed's next action remains Handoff.

## 2. Customization and generated skills

- [x] Implement deterministic override resolution and visible reconciliation for inherited legacy content in `projectskills.go` and the editor model/API; preserve all saved entries and history.
- [x] Keep existing custom-prompt storage compatible; update the fallback prompt and ensure reconciled customization carries the new stage contract.
- [x] Generate adjustment skills and safe legacy forwarding commands for every supported installation layout; test divergent files, both legacy override keys, conflicts with canonical overrides, and reinstall idempotence.
- [x] Replace review/creation instructions in single and batch pickup and all framework variants; update generated project context and manifests using their source generator.

## 3. Earlier PR creation and recovery

- [x] Update project policy instructions for both timings: specification owns early creation; implementation owns default creation before adjustment; persist PR URL at either successful owner stage.
- [x] Add policy-aware PR verification to managed validation/completion and standalone stage handling; retain specification artifact and implementation check requirements.
- [x] Implement missing-PR recovery through the configured owner with explicit recovery context, preserving attained stages and accepted work.
- [x] Test both timing policies, default settings, draft creation, existing draft/ready reuse, URL persistence, missing URL with discoverable PR, lookup failure, push/create failure, retry without duplicates, and recovery without downgrade or reviewed advancement.

## 4. Adjustment quality gate

- [x] Verify the matching open PR before launch; retain its identity for managed completion and reject mismatched/closed/merged PRs.
- [x] Implement full branch/default-base review, feedback retrieval and dispositions, corrective work, final checks, push, same-PR update, and readiness instructions in templates and runner fallback.
- [x] Extend injectable forge evidence to verify branch, URL, open state, and readiness; preserve branch/clean-checkout/check validation and managed ownership rules.
- [x] Extend `internal/db/skillresult_test.go`, `skills_test.go`, `pr_policy_test.go`, and runner tests for success without feedback, feedback failure, failed checks, draft/missing/replacement PR rejection, legacy aliases, and repeat adjustment.
- [x] Test single and batch pickup with both timing policies, one combined PR, partial completion, and the human merge boundary.

## 5. UI and documentation

- [x] Update `web/src/lib/workflow.ts`, TaskCard, TaskDetailModal, CopyTaskSkillMenu, ProjectModal, ProfileModal, ActivitiesView, and affected translations; preserve PR links and historical records.
- [x] Add frontend regression coverage for next-stage selection, copy command, missing-PR owner action, explicit repeat Adjust, and customization reconciliation display.
- [x] Update README, CHANGELOG, server-agent contract and relevant workflow guides; add an ADR for earlier PR ownership and compatibility handling. Author all additions in English.

## 6. Verification and review evidence

- [x] Run focused Go tests for changed database/models/runner behavior, then `go test ./internal/...` and `go vet ./...`.
- [x] Run `npm test`, `npx tsc --noEmit -p tsconfig.app.json`, and `npx oxlint src` from `web/` (the frontend checks used by `make test`).
- [x] Run `make binary-build` to verify the frontend and embedded Go binary together.
- [x] Run `openspec validate 61-replace-create-pr-with-adjust --strict` and review the final branch diff against every scenario.
- [x] Record actual commands/results and any baseline failures; exercise both policy paths, missing-PR recovery, native legacy invocation, and managed adjustment using deterministic integration fixtures (see validation.md). Confirm no merge/closure/cleanup is triggered.
- [x] Update the existing draft PR with implementation evidence and retain it as draft until the complete adjustment/review gate succeeds.
