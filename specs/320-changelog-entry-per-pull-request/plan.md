# #320 — Plan

References: [`spec.md`](spec.md), [`docs/clarifications/320.md`](../../docs/clarifications/320.md).

## Stack and surfaces

Documentation only. No Go, TypeScript or schema change.

| File | Change | Why |
| --- | --- | --- |
| `CHANGELOG.md` | One `### Fixed` section under `## [0.1.0] - 2026-09-22`, after `### Added`, with the #307 line | D2, FR1–FR3 |
| `AGENTS.md` | Step 3 "Write the changelog entries" states the per-PR convention and what the release adds | D3, FR4 |

## Data contracts

`CHANGELOG.md` is read by:

- `changelog.go` (`//go:embed CHANGELOG.md`), served verbatim on `GET /api/changelog`;
  `internal/handlers/version_test.go` checks the route.
- `desktop/src/changelog.mjs`, which splits the file on `## [` headings then `###`
  sections; `desktop/tests/changelog.test.mjs` pins the parser.

A new `### Fixed` section under an existing release is a shape the parser already
handles (`[Unreleased]` carries several sections), so no code changes.

## Wording

The #307 line, following the release procedure's rules (user-facing, one line, PR
reference):

> **Quiet agent runs are no longer canceled.** A run that stays silent for a long
> time — a long build, a question waiting for its owner — used to be canceled
> after fifteen minutes, which stopped an autonomous chain without a word. It now
> stays running, and its summary notes how long it has been quiet. A run
> canceled because its client disconnected can still be finished by the agent
> that owns it, so the chain carries on. (#315)

Source: `specs/307-mcp-run-survives-silence/spec.md`, US1–US3.

`AGENTS.md` step 3 gains a lead paragraph before the rules:

- the pull request that makes a user-visible change adds its line under
  `## [Unreleased]`, following the rules below; a change nobody can see adds none;
- a specification carries a changelog acceptance criterion only for a user-visible
  change;
- at release time the procedure promotes `[Unreleased]`, fills in the user-visible
  changes that merged without a line (read from the commit range of step 2) and
  leaves correct lines as they are.

"Then edit the file" keeps its three steps.

## Rejected alternatives

- A new ADR: ADR 0018 already records the decision; the convention is procedure, which
  lives in `AGENTS.md`.
- Editing the stage skills or the `runner.go` handoff prompt: none requires a changelog
  unconditionally, and the handoff prompt is already conditional.
- Putting #307 under `[Unreleased]`: rejected by the owner while `v0.1.0` is uncut (D2).

## Verification

- `git ls-remote --tags origin` shows no `v0.1.0` before committing (D2 fallback).
- `cd desktop && node --test tests/changelog.test.mjs`.
- `go vet ./...` and `go test ./internal/handlers/` (on Linux/WSL, see project memory).
