# #232 - Provision node_modules in task worktrees so build and lint can run

## Context

A task worktree prepared by the local agent has no installed JavaScript dependencies, so
`npm run build --prefix web` fails on `tsc` and `npm run lint --prefix web` fails on `oxlint`.
`docs/REIMPLEMENTATION_GUIDE.md` describes a symlink step that was never implemented.

The clarification is recorded in [`docs/clarifications/232.md`](../../docs/clarifications/232.md).
Every product question it raised was answered by the owner (round 2), and none remains open.

This document states behaviour and acceptance criteria only. Technical choices are in
[`plan.md`](plan.md), and the ordered checklist is in [`tasks.md`](tasks.md).

---

## Scope

**In scope**

- A per-worktree `npm ci` in each package folder of a prepared task worktree, run before the
  agent session starts.
- Discovery of package folders at the worktree root and one level down.
- A stamp that skips the install when nothing changed.
- The skip cases: main checkout, no `package.json`, no `package-lock.json`, `node_modules` that is
  not a plain directory.
- The correction of `docs/REIMPLEMENTATION_GUIDE.md`.

**Out of scope**

- Linking or copying anything from the main checkout, `.env` and `.env.local` included.
- Package managers other than npm (folders with only `yarn.lock` or `pnpm-lock.yaml` are skipped).
- Repository-specific post-install steps, such as the `Makefile`'s
  `node node_modules/electron/install.js`.
- The `.gitignore` rule for `node_modules` links (#248).
- Removing a `node_modules` link a user created by hand.

---

## Decisions being specified

1. **Real install, never shared.** Each worktree gets its own `node_modules` from `npm ci` run
   inside it. The main checkout is never linked, read for dependencies or written to.
2. **Automatic discovery.** A package folder is the worktree root or one of its direct
   subdirectories, when it holds a `package.json`. Hidden directories and `node_modules` are not
   searched. Nothing is specific to this repository's layout.
3. **npm only.** A package folder is installed only when it also holds a `package-lock.json`.
   Otherwise it is skipped with a log line.
4. **Idempotent, on every preparation.** Provisioning runs every time the agent prepares a task
   workspace, both at launch and on `prepare_workspace`. An install is skipped when
   `node_modules/.install-stamp` is newer than both `package.json` and `package-lock.json`,
   the convention the `Makefile` already follows.
5. **The launch waits, but never fails because of provisioning.** The install runs before the
   session starts, with a bounded timeout. A failure, a timeout or a missing `npm` is logged, and
   the launch continues. A failed install writes no stamp, so it is retried at the next launch.
6. **A link is never installed through.** When a worktree's `node_modules` exists and is not a
   plain directory (a symlink, a junction, a file), nothing is run in that folder.

---

## User stories

### US1 - A new task worktree can build and lint (P1)

**As a** developer or agent working in a task worktree,
**I want** the worktree's JavaScript dependencies installed when it is prepared,
**So that** build and lint work without a manual `npm ci`.

- **Given** a repository whose `web/` and `desktop/` folders each hold a `package.json` and a
  `package-lock.json`, and a task whose worktree the agent prepares
- **When** the launch prepares the worktree
- **Then** `npm ci` runs in `web/` and `desktop/` inside the worktree before the session starts,
  `web/node_modules` and `desktop/node_modules` are real directories in the worktree, and each
  holds a `.install-stamp`.

- **Given** the same preparation
- **Then** the main checkout's `node_modules` folders are not read, linked or modified.

### US2 - Repeat launches cost nothing (P1)

- **Given** a worktree whose `node_modules/.install-stamp` is newer than its `package.json` and
  `package-lock.json`
- **When** the worktree is prepared again
- **Then** no install runs in that folder.

- **Given** a worktree whose `package-lock.json` is newer than its stamp (the branch changed its
  dependencies)
- **When** the worktree is prepared again
- **Then** `npm ci` runs again in that folder, and the stamp is refreshed.

- **Given** an existing worktree created before this change, without `node_modules`
- **When** it is prepared at its next launch
- **Then** it is provisioned like a new one.

### US3 - Nothing to provision is not an error (P1)

- **Given** a repository with no `package.json` at the root or one level down
- **When** a worktree is prepared
- **Then** no command runs and nothing is logged as a failure.

- **Given** a folder with a `package.json` but no `package-lock.json`
- **Then** it is skipped with a log line naming the folder, and nothing is installed.

- **Given** a preparation that resolves to the main checkout (worktrees disabled, or the branch
  checked out in the main checkout)
- **Then** nothing is installed. The main checkout stays managed by `make web-deps` and
  `make desktop-deps`.

- **Given** a root holding a `package-lock.json` but no `package.json` (this repository)
- **Then** the root is not a package folder and is skipped silently.

### US4 - A link is never installed through (P1)

- **Given** a worktree whose `desktop/node_modules` is a symlink or a junction
- **When** the worktree is prepared
- **Then** no install runs in `desktop/`, the link and its target are untouched, and a log line
  names the folder. The other package folders are provisioned as usual.

### US5 - A failed install does not block the launch (P1)

- **Given** `npm` missing from `PATH`, or `npm ci` exiting with an error, or exceeding its timeout
- **When** a worktree is prepared
- **Then** a log line names the folder and the cause, no stamp is written, the remaining folders
  are still attempted, and the launch continues into the agent session.

---

## Functional requirements

- **FR1** Provisioning runs after the worktree is resolved and before the agent session starts,
  on every launch and every `prepare_workspace`.
- **FR2** It runs only when the prepared directory is not the main checkout.
- **FR3** Package folders are the worktree root and its direct, non-hidden subdirectories other
  than `node_modules` that hold a `package.json`.
- **FR4** A package folder is installed with `npm ci` when it holds a `package-lock.json`, its
  `node_modules` is missing or a plain directory, and the stamp is missing or older than
  `package.json` or `package-lock.json`.
- **FR5** A successful install writes `node_modules/.install-stamp`. A failed one writes nothing.
- **FR6** Each install has a bounded timeout, and provisioning as a whole has a bounded total
  timeout. Both are in the order of minutes.
- **FR7** No provisioning outcome turns a launch or a `prepare_workspace` into a failure.
- **FR8** `.env` and `.env.local` are neither linked nor copied.
- **FR9** `docs/REIMPLEMENTATION_GUIDE.md` describes the per-worktree install and no longer lists
  symlinks of `node_modules` or `.env*`.

## Non-functional requirements

- **NFR1** The main checkout is never written to by provisioning.
- **NFR2** Concurrent preparations of different tasks are not serialised behind one install.
- **NFR3** Tests need neither the network nor `npm`: the install command is injected.

## Open requirements

None. The clarification left no open product question.
