# #320 — Skills require a `CHANGELOG.md` entry in a repository that has none

Ticket: https://github.com/sebastienferry/sectile/issues/320
Type: Task — follow-up of #307.
Branch: `feat/320`.
Clarification: [`docs/clarifications/320.md`](../../docs/clarifications/320.md).

## Context

When #307 was specified, it carried the acceptance criterion "the fix is listed in
`CHANGELOG.md`". The repository had no changelog, so the implement and review stages
both reported it as unsupported. The ticket asked for a decision: keep a changelog, or
drop the requirement.

That decision has since been taken. #329 created `CHANGELOG.md` in Keep a Changelog
format and ADR 0018 records it; the file is embedded in the server and the desktop app
and served on `GET /api/changelog`. Two gaps remain, and this ticket closes them:

1. no line of `CHANGELOG.md` covers the #307 fix, which merged (PR #315) before the
   file existed;
2. nothing states **when** an entry is written. `AGENTS.md` writes the entries at
   release time, while recent pull requests add their own line under `[Unreleased]`.

Out of scope: the release procedure itself (tag, version number, manifests), how the
changelog is served, the wording of any other existing entry, the stage skills and the
`runner.go` handoff prompt (none of them requires a changelog unconditionally).

This file states behaviour and acceptance criteria only. Implementation choices are in
[`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

---

## Decisions being specified

Settled by the owner in Round 2 of the clarification, both in line with the Round 1
recommendation.

- **D1 — Keep the changelog.** ADR 0018 is the decision record. No new ADR, nothing
  removed from the skills.
- **D2 — #307 goes under `[0.1.0]`.** One `### Fixed` line under
  `## [0.1.0] - 2026-09-22`, because #307 merged before the first tag and `v0.1.0` has
  not been cut. Fallback: if `v0.1.0` is tagged before this ships, the line goes under
  `[Unreleased]` instead.
- **D3 — An entry is written in the pull request that makes the user-visible change**,
  under `## [Unreleased]`. The release procedure promotes that section and only adds
  the entries that are missing. A specification carries a changelog acceptance
  criterion only when the change is visible to users.

---

## User stories

### US1 (P1) — A reader of the changelog finds the #307 fix

As somebody who uses Sectile, I want the release notes of `0.1.0` to mention that a
quiet agent run is no longer canceled, so that I know the behaviour I rely on is
intended.

- **Given** `CHANGELOG.md`,
  **when** I read the `## [0.1.0] - 2026-09-22` section,
  **then** a `### Fixed` section holds one line saying that an agent run that stays
  quiet for a long time is no longer canceled after fifteen minutes, and that a run
  whose client disconnected can be recovered and finished, referencing `(#315)`.
- **Given** the web interface or the desktop app,
  **when** I open the changelog,
  **then** the `0.1.0` release shows a Fixed group with that line, next to its Added
  group.

### US2 (P1) — A contributor knows when to write an entry

As somebody who opens a pull request on Sectile, I want the repository instructions to
say whether my pull request writes its changelog line, so that neither I nor the
reviewer has to guess.

- **Given** `AGENTS.md`, section "Releases: cutting a tag", step 3,
  **when** I read it,
  **then** it says that the pull request making a user-visible change adds its line
  under `## [Unreleased]`, and that a change nobody using Sectile can see adds none.
- **Given** the same step,
  **when** I cut a release,
  **then** it tells me to promote `[Unreleased]`, to add only the entries that are
  missing for user-visible changes merged without one, and to leave correct lines as
  they are.
- **Given** a specification written for a change users cannot see,
  **when** I read its acceptance criteria,
  **then** `AGENTS.md` does not ask for a changelog criterion in it.

### US3 (P2) — A later ticket carries no changelog caveat

- **Given** a ticket specified after this change,
  **when** it carries a changelog criterion (user-visible change),
  **then** the criterion can be satisfied, because the file exists and the convention
  says where the line goes.

## Functional requirements

- **FR1** `CHANGELOG.md` keeps the Keep a Changelog structure that
  `desktop/src/changelog.mjs` parses: `## [version] - date`, then `### Section`, then
  bullet lines.
- **FR2** `[0.1.0]` gains exactly one `### Fixed` section, placed after `### Added`
  (Keep a Changelog order), holding the #307 line. No other line of the file changes.
- **FR3** The line is in English, written for users, and references the pull request
  (`(#315)`), never a commit sha.
- **FR4** `AGENTS.md` states the per-pull-request convention (D3) in step 3 and keeps
  the rest of the release procedure unchanged.

## Success criteria

- `desktop/tests/changelog.test.mjs` and `internal/handlers/version_test.go` pass.
- `GET /api/changelog` returns the file with the new line (the file is embedded as-is).

## Open requirements

None.
