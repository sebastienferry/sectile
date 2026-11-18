# Tasks #921 - Shared Git worktree permissions

Implementation and publication complete. Focused regression, native macOS sandbox regression, both full launch paths, the full Go race suite, agent build, Go static analysis, repository formatting and diff checks passed. Baseline environment failures and two formatting-only repairs are documented in `docs/testing/shared-git-worktree-permissions-921.md`.

Draft PR: https://github.com/sebastienferry/sectile/pull/926, branch `feat/921`, base `feat/tauri-monorepo`. The draft state, base and implementation head were verified remotely. Preserve accepted artifacts, the worktree, branch and qualification fixtures. Record implemented only; do not advance to reviewed or merge.

## 1. Establish evidence (US1, US2)

- [x] T1 Build a disposable real linked-worktree fixture and record canonical common/per-worktree Git paths and relevant configuration.
- [x] T2 Reproduce through built-in app-server conversations on macOS; capture sanitized trace, exact lock requester/trigger, versions, requested/effective policy, stderr/status and resulting Git state.
- [x] T3 Qualify built-in autonomous CLI separately with the same evidence contract. Record non-reproduction explicitly and resolve gaps before claiming a correction.
- [x] T4 Establish permission precedence and protected hooks/configuration/unrelated-path behavior with disposable boundary fixtures. On a guard denial, preserve the refusal and stop the intention pending owner direction.
- [x] T5 Document causal attribution and supported correction feasibility per launch path before selecting implementation.

## 2. Deliver the supported outcome (US1 or US3)

- [x] T6 If locally correctable, apply the narrow supported scope change to both affected built-in paths, preserving approvals, read-only behavior and unrelated protections.
- [x] T7 Not applicable: a supported local correction exists. Otherwise, document upstream evidence, remaining restriction and actionable supported remedy; provide the clear French diagnostic on the affected runtime surface. Evaluate this outcome separately for each path.
- [x] T8 Preserve honest primary/auxiliary outcome reporting; do not retry, suppress denial, disable maintenance or bypass enforcement.

## 3. Regression and live qualification

- [x] T9 Extend real linked-worktree regression coverage to exercise the identified lock operation and verify staged content, commit/ref state and stderr/status. Cover root resolution edge cases if changed.
- [x] T10 Assert emitted policy/CLI arguments independently, including unchanged protected paths, read-only behavior and custom templates. These assertions do not substitute for live tests.
- [x] T11 Qualify actual effective enforcement on macOS for both launch paths. For a local correction require denial-free staging, committing and the reproducing operation plus protected-boundary checks. For an upstream-only outcome preserve the limitation and verify diagnostic/remedy evidence without claiming a fix.
- [x] T12 Run focused Go agent tests and required repository build/lint/test checks for changed files; add Windows hidden-process coverage if a subprocess is introduced or its visibility changes.

## 4. Documentation and implementation handoff

- [x] T13 Record sanitized per-path evidence and supported reproduction/recovery steps in relevant launch/testing documentation. Update README/configuration documentation only if affected; add an English Unreleased entry for a user-visible change.
- [x] T14 Review all FR1-FR7 criteria and state each path's disposition and platform limits. Resolve investigation gaps or report them as blockers.
- [x] T15 At implemented stage only, follow configured PR creation checks and preserve any existing PR/stage on retry. This specification does not advance to reviewed.
