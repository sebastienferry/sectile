# Tasks #737 - Agent: option to work in any repository, not only the project's

Order matters: each step leaves the tree building and green. Tests go with the
step they cover. Every new `git` command goes through `gitLocal` or
`agentexec.Hidden`.

## 1. Fetch before a new secondary branch (US5)

- [x] T1.1 `ensureLocalWorktree` gains `fetch bool` and returns a warning;
      with it, fetch `origin` and base a new branch on `origin/<branch>`, else
      the remote default branch, else local `HEAD` with a warning (D6).
- [x] T1.2 `repositoryWorktree` passes `fetch=true`; the launch's code worktree
      passes false. `models.RepositoryWorktree.Warning` carries the warning.
- Tests: on temporary repositories with a bare `origin`: stale clone on a
  feature branch gets a branch from the remote default; remote branch only;
  local branch reused unmoved; unreachable `origin` gives a warning and a
  worktree from `HEAD`. Existing `internal/agent` worktree tests stay green.

## 2. Settings (US9 data side)

- [x] T2.1 `ProjectSettings.AnyRepository` and `ClonesPath` with `isZero`,
      `overlay`, the edit helper and accessors
      `Settings.AnyRepository(id)` / `ClonesPath(id, projectRoot)` (D1).
- [x] T2.2 Desktop settings API reads and writes both, refusing a relative
      clones folder by name.
- Tests: round trip; a section holding only one of them is kept; emptying
  removes the key; relative path refused; effective clones folder defaults to
  the parent of the project checkout.

## 3. Path and clone in `prepare_repository_worktree` (US1, US2, US3)

- [x] T3.1 Protocol: `Operation.RepositoryURL` and `Operation.Path`;
      `RepositoryWorktree` gains `Source`, `PathChecked`, `Remembered`,
      `AddedToSession` (D3).
- [x] T3.2 New `internal/agent/any_repository.go`: `checkRepositoryPath`
      (absolute, directory, top level, `origin` identity), `cloneRepository`
      (D4, D5, temporary sibling), `rememberRepositoryFolder` (D2).
- [x] T3.3 `repositoryWorktree` follows the lookup order of FR3; with the
      option off, a `path` is refused and the not-found message names the
      option (D15).
- [x] T3.4 Server: `PrepareRepositoryWorktree` forwards URL and path, refuses
      an agent that did not echo `PathChecked` when a path was sent.
- [x] T3.5 MCP: `path` input and new description (the bridge relays schemas,
      so `agentmcp` and `mcptest` need no change).
- Tests: each US2 and US3 case on temporary repositories (clone from a local
  bare repository by file URL); option off refusals; existing known folder
  wins over `path`; mapping written once and found by another project;
  handoff removal of a repository reached through `path`
  (`removeRepositoryWorktrees`); server relays and the old-agent refusal;
  Windows test asserting `CREATE_NO_WINDOW` on clone and fetch.

## 4. Running session sees the new worktree (US4)

- [x] T4.1 After `repository_worktree` creates a tree, type `/add-dir` into the
      agent's running Claude Code sessions of the ticket (D14), reusing
      `typeAddDir`; set `AddedToSession`.
- [x] T4.2 MCP answer text: tell the caller to run `/add-dir <path>` when
      `AddedToSession` is false.
- Tests: a fake terminal manager receives the line once; a folder inside the
  session's directories is skipped; no session gives `false`.

## 5. Lazy code worktree and pull request scope (US6, US7)

- [x] T5.1 `prepareDispatchLocked`: skip the code worktree under D8; working
      directory per D8; folder map lists the code repository as `context`.
- [x] T5.2 Server: lift the primary refusal for the code repository (D9);
      compute the branch name when the task has none (D10).
- [x] T5.3 Agent: `repository_worktree` on the code repository creates or
      reuses the code worktree with `fetch=true`.
- [x] T5.4 `branch_changes` answers `lazyCode` (D11); `validateStagePRs` and
      `stagePRSet.primary` apply the exemption.
- Tests: launch with option on and `specArtifacts=drop` creates no code
  worktree and starts in the project checkout; with a distinct Issue folder,
  starts in its worktree; with specifications in the code checkout, creates
  the code worktree; an existing code worktree is reused. Stage: secondary-only
  ticket accepted with its secondary pull request as current; refused when
  `lazyCode` is false or the branch exists in the code repository; code
  repository changed later requires its pull request. Existing `stageprs`
  tests stay green.

## 6. Pin to an undeclared repository (US8)

- [x] T6.1 Server pin check accepts a remote identity (D12); `taskPin` returns
      it; `taskPullRequestScope` already handles a pin outside the project;
      removing a repository clears its pins (D16).
- [x] T6.2 `ResolvePrimaryRepository(any bool)` (D13); `primaryRoot` clones
      on `PrimaryUnmapped` with the option on, fails naming the option when
      off.
- [x] T6.3 Web task detail: the repository select gains "Other repository…"
      with a text input for a URL or `host/path`; locales.
- Tests: pin stored and read back; bare word refused; launch with option on
  clones and creates the primary worktree there; option off fails with the
  message; web component test for the free entry.

## 7. Desktop settings UI (US9)

- [x] T7.1 Project settings: "Any repository" toggle and "Clones folder" row
      with its effective value, next to the specifications folders.
- Tests: desktop UI test toggling the option and saving the folder (build
  with `npx vite build` first; run unsandboxed).

## 8. Skills, documentation, release notes

- [x] T8.1 `implement/steps.md` and `contracts/transition.md`: when a needed
      repository has no folder, find its checkout and pass `path`, or call
      without it to let the agent clone; a ticket launched without a code
      worktree prepares the code repository before changing it; update the
      goldens (`UPDATE_GOLDEN=1`).
- [x] T8.2 `docs/USER_GUIDE.md`, `docs/CAPABILITIES.md`.
- [x] T8.3 ADR `docs/adrs/0052-any-repository-option.md`: workstation option,
      session finds and agent clones, workstation-wide memory, lazy code
      worktree and its pull request rule, server accepts any pin.
- [x] T8.4 `CHANGELOG.md` under `[Unreleased]`: `Added` (the option, with
      clone and pin) and `Fixed` (new secondary branches start from the
      remote default branch).

## Test plan

- `go test ./internal/agent/... ./internal/agentconfig/... ./internal/db/...
  ./internal/taskmcp/... ./internal/agentmcp/... ./internal/mcptest/...
  ./internal/skills/...` outside the sandbox (httptest), with
  `SECTILE_TEST_POSTGRES_DSN` set for the PostgreSQL variants.
- Windows: `GOOS=windows go vet ./internal/agent/...` and the
  `CREATE_NO_WINDOW` test on a Windows runner (CI).
- Web: `npm test` and the component test of T6.3; desktop UI suite after
  `npx vite build`.
- Manual: on a sandboxed Claude Code launch, reproduce PE-1668 with
  `akamai-python` unattached: option on, the session passes `path` (then, in a
  second run, nothing, to exercise the clone), writes in the worktree, opens
  its pull request, and the transition to `implemented` accepts it without a
  pull request in the code repository.
