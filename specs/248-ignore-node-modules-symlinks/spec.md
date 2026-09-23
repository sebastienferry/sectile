# #248 — A `node_modules` symlink can still be committed

Ticket: https://github.com/sebastienferry/sectile/issues/248
Type: Bug — "A desktop/node_modules symlink was committed to main and breaks fresh clones".
Branch: `feat/248`.
Clarification: [`docs/clarifications/248.md`](../../docs/clarifications/248.md).

## Context

`desktop/node_modules` was committed as a symlink to an absolute, machine-specific path
through the squash merge of #242. Commit `726ce0a` has since removed it from `main`, so a
fresh clone no longer carries it. The exposure that let it in is still open: the root
`.gitignore` rule `node_modules/` matches directories only, and git records a symlink as a
file, so a `node_modules` symlink recreated anywhere outside `web/` shows as untracked and
is swept in by the next `git add`.

This ticket closes that exposure. Out of scope: removing the symlink (already done by
`726ce0a`), rewriting history, provisioning dependencies in task worktrees (#232), and any
automated regression guard (test, `make` target, hook or CI job).

This file states behaviour and acceptance criteria only. Implementation choices are in
[`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

## Decisions being specified

Settled by the owner in Round 2 of the clarification, each matching the Round 1
recommendation.

1. **Scope reduced.** The ignore rule and its verification only; no `git rm --cached`.
2. **Worktree dependencies deferred to #232.** No Go change here; #232 was told that
   `desktop/node_modules` must join its provisioning path list.
3. **No regression guard.** The ignore rule is the whole protection.

## User stories

### US1 (P1) — A `node_modules` entry is never offered to `git add`

As a contributor, when my checkout or task worktree holds a `node_modules` entry of any
kind, git ignores it, so I cannot commit it by accident.

- **Given** a checkout where `desktop/node_modules` is a symlink,
  **When** I run `git status` or `git add desktop`,
  **Then** `desktop/node_modules` is neither listed as untracked nor staged.
- **Given** a checkout where `desktop/node_modules` or `web/node_modules` is a directory,
  **When** I run `git check-ignore -v --no-index <path>`,
  **Then** git reports an ignore rule for that path.

### US2 (P2) — A fresh clone installs the desktop dependencies

As a contributor on any machine, a fresh clone installs the desktop app's dependencies
without meeting a pre-existing `node_modules` entry.

- **Given** a fresh `git clone` of the branch,
  **When** I run `npm ci --prefix desktop`,
  **Then** it completes successfully and `desktop/node_modules` is a real directory.

## Functional requirements

- **FR1** — The repository ignore rules match a `node_modules` entry whether it is a
  directory, a regular file or a symlink, at any depth.
- **FR2** — `desktop/node_modules` and `web/node_modules` are both reported as ignored by
  `git check-ignore -v --no-index`.
- **FR3** — No `node_modules` entry is tracked: `git ls-files` lists none.
- **FR4** — Every other ignore behaviour of the repository is unchanged.

## Open requirements

None.
