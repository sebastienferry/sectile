# #320 — Implementation checklist

References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. Changelog line (US1, D2, FR1–FR3)

- [ ] T1.1 Check `git ls-remote --tags origin`: no `v0.1.0`. If it exists, apply the
      fallback (line under `[Unreleased]`) and record it here.
- [ ] T1.2 Add `### Fixed` after `### Added` under `## [0.1.0] - 2026-09-22` in
      `CHANGELOG.md`, with the #307 line from `plan.md`, wrapped like its neighbours.

## 2. Convention (US2, D3, FR4)

- [ ] T2.1 Reword `AGENTS.md` step 3 "Write the changelog entries": per-PR entry under
      `[Unreleased]`, changelog criterion only for user-visible changes, release
      promotes and fills gaps.

## 3. Verification (success criteria)

- [ ] T3.1 `cd desktop && node --test tests/changelog.test.mjs` passes.
- [ ] T3.2 `go vet ./...` and `go test ./internal/handlers/` pass (Linux/WSL).
- [ ] T3.3 Re-read the diff: only `CHANGELOG.md` (one section) and `AGENTS.md` (step 3)
      change besides the spec files.

## Test plan

No new automated test: the change is documentation, and the existing parser test
(`desktop/tests/changelog.test.mjs`) already runs against the real `CHANGELOG.md`, so a
structural break would fail it.
