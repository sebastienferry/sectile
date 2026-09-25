# Tasks #481 - Git project setup on desktop

Ordered checklist. Each step leaves the tree building and its tests green.
References: `spec.md` (behaviour), `plan.md` (choices).

## 1. Agent: folder state and initialization

- [ ] T1 Add `internal/agent/agent_desktop_git_init.go` with
      `gitFolderState(ctx, path) (top, state string)` returning `ready`,
      `unborn`, `folder` or `missing` (plan, "Agent endpoint").
- [ ] T2 Add the `GET /desktop/git-init?path=` handler returning
      `{path, state}`.
- [ ] T3 Add the `POST /desktop/git-init` handler under `d.prepareMu`:
      validation and root/home refusal (400, also for a folder removed since
      the offer),
      `git init` + `symbolic-ref HEAD refs/heads/main` for `folder`,
      `commit --allow-empty --no-verify -m "Initial commit"` for `folder` and
      `unborn`, nothing for `ready`, then `excludeTaskWorktrees`; Git failures
      as 422 with Git's text, nothing rolled back.
- [ ] T4 Route `/desktop/git-init` in `agent_desktop.go` and add `git-init` to
      the `/desktop/status` capabilities.
- [ ] T5 Tests in `internal/agent/agent_desktop_git_init_test.go`, with a
      controlled Git configuration (plan, "Test strategy" / Go): the four
      states on GET; `folder` initialized with an empty commit on `main`, files
      untracked, `/.tasks/` excluded, no `.gitignore`, no remote, a worktree
      addable from `HEAD`; `unborn` on `trunk` committed without re-init and
      with its config kept; failing `commit-msg` hook bypassed; `ready`
      untouched; subfolder of an `unborn` checkout commits the checkout, no
      nested `.git`; 400 for relative, absent, file, `/`, `$HOME`; 422 without
      identity, folder then `unborn`; capability listed.

## 2. Electron bridge

- [ ] T6 `desktop/electron/main.cjs`: `git-state` (answers `{state:'unknown'}`
      when the agent lacks `git-init`) and `git-init` (throws the "Update and
      restart the local agent to initialize a Git repository." message when it
      lacks it).
- [ ] T7 `desktop/electron/preload.cjs`: expose `gitState` and `gitInit`.

## 3. Renderer

- [ ] T8 `desktop/src/git-init.mjs`: `offerFor(state)` and the texts of plan,
      "Renderer"; unit tests in `desktop/tests/git-init.test.mjs` (every state,
      the explanation's four points).
- [ ] T9 `desktop/src/main.js`: `attachGitOffer` under the Local repository
      row; examined at open, after *Browse*, on `change`; stale answers
      discarded; "Initialize" and "Not now" per spec US1 and US4.
- [ ] T10 Attach the same offer to the Specifications folder input, examined
      only while it is shown; stored plain folder examined at open (US2);
      `onReady` refreshes `renderSpec` with the new value.
- [ ] T11 `desktop/src/style.css`: offer block, reusing the existing notice and
      button styles.
- [ ] T12 `desktop/tests/git-init.ui.cjs` (fake agent modelled on
      `spec-folder.ui.cjs`): the five scenarios of plan, "Test strategy" /
      Playwright. Run after `npx vite build`.

## 4. Documentation

- [ ] T13 `desktop/README.md`: the offer, the empty first commit and its
      consequence on worktrees, under Local repository and Specifications
      folder.
- [ ] T14 `CHANGELOG.md`: the `[Unreleased]` / `Added` line of plan,
      "Documentation".

## 5. Checks before the implemented transition

- [ ] T15 `go test ./internal/agent/...` (with `-race` as CI does); `go vet`.
- [ ] T16 `cd desktop && npm test`, then `npx vite build` and the new UI test
      plus `spec-folder.ui.cjs` and `repository-choice.ui.cjs`, which share the
      settings panel.
- [ ] T17 Manual check on a scratch folder: initialize from the Local
      repository field, save, launch a task and see its worktree created;
      decline on a Specifications folder and see it stay a plain folder.
- [ ] T18 Restore `web/webui/.gitkeep` if a build removed it; the checkout must
      be clean before the transition.
