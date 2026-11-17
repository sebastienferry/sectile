# Tasks #921 - Shared Git worktree permissions

All implementation items remain unchecked. Preserve accepted clarification, existing artifacts and branch `feat/921` on retry.

## 1. Establish evidence (US1, US2)

- [ ] T1 Build a disposable real linked-worktree fixture and record canonical common/per-worktree Git paths and relevant configuration.
- [ ] T2 Reproduce through built-in app-server conversations on macOS; capture sanitized trace, exact lock requester/trigger, versions, requested/effective policy, stderr/status and resulting Git state.
- [ ] T3 Qualify built-in autonomous CLI separately with the same evidence contract. Record non-reproduction explicitly and resolve gaps before claiming a correction.
- [ ] T4 Establish permission precedence and protected hooks/configuration/unrelated-path behavior with disposable boundary fixtures. On a guard denial, preserve the refusal and stop the intention pending owner direction.
- [ ] T5 Document causal attribution and supported correction feasibility per launch path before selecting implementation.

## 2. Deliver the supported outcome (US1 or US3)

- [ ] T6 If locally correctable, apply the narrow supported scope change to both affected built-in paths, preserving approvals, read-only behavior and unrelated protections.
- [ ] T7 Otherwise, document upstream evidence, remaining restriction and actionable supported remedy; provide the clear French diagnostic on the affected runtime surface. Evaluate this outcome separately for each path.
- [ ] T8 Preserve honest primary/auxiliary outcome reporting; do not retry, suppress denial, disable maintenance or bypass enforcement.

## 3. Regression and live qualification

- [ ] T9 Extend real linked-worktree regression coverage to exercise the identified lock operation and verify staged content, commit/ref state and stderr/status. Cover root resolution edge cases if changed.
- [ ] T10 Assert emitted policy/CLI arguments independently, including unchanged protected paths, read-only behavior and custom templates. These assertions do not substitute for live tests.
- [ ] T11 Qualify actual effective enforcement on macOS for both launch paths. For a local correction require denial-free staging, committing and the reproducing operation plus protected-boundary checks. For an upstream-only outcome preserve the limitation and verify diagnostic/remedy evidence without claiming a fix.
- [ ] T12 Run focused Go agent tests and required repository build/lint/test checks for changed files; add Windows hidden-process coverage if a subprocess is introduced or its visibility changes.

## 4. Documentation and implementation handoff

- [ ] T13 Record sanitized per-path evidence and supported reproduction/recovery steps in relevant launch/testing documentation. Update README/configuration documentation only if affected; add an English Unreleased entry for a user-visible change.
- [ ] T14 Review all FR1-FR7 criteria and state each path's disposition and platform limits. Resolve investigation gaps or report them as blockers.
- [ ] T15 At implemented stage only, follow configured PR creation checks and preserve any existing PR/stage on retry. This specification does not advance to reviewed.
