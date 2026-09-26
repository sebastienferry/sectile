# Plan #481 - Git project setup on desktop

Behaviour and acceptance criteria are in `spec.md`; this file holds the
implementation choices.

## Stack and placement

- **Local agent (Go)**: a new desktop endpoint that reports a folder's state
  and initializes it. Every Git call goes through `gitLocal`, like every other
  Git call made for the desktop. Nothing changes on the server, the tracker,
  the database or the server-agent contract.
- **Electron bridge** (`desktop/electron/main.cjs`, `preload.cjs`): two
  forwarding handlers, gated on a new agent capability, as `map-repository` is.
- **Renderer** (`desktop/src/main.js`): the offer under the two fields of the
  General panel of the project settings. Its texts and the decision of which
  offer to show live in a small pure module, `desktop/src/git-init.mjs`, so they
  are unit-tested without Electron.

## Agent endpoint

New file `internal/agent/agent_desktop_git_init.go`, routed from
`agentDaemon.ServeHTTP` next to `/desktop/repositories`:

```
GET  /desktop/git-init?path=<absolute path>
  200 {"path": "<examined path>", "state": "ready" | "unborn" | "folder" | "missing"}

POST /desktop/git-init   {"path": "<absolute path>"}
  200 {"path": "<repository top level>", "state": "ready",
       "initialized": <bool: a repository was created>,
       "committed":   <bool: the first commit was made>}
  400 text: refusal (not absolute, absent, not a directory, root, home),
            including a folder removed since the offer was shown
  422 text: Git's own output when `git init` or `git commit` fails
```

`state` is computed by one helper, `gitFolderState(ctx, path) (top, state)`:

1. empty, relative, absent or not a directory -> `missing`;
2. `git rev-parse --show-toplevel` fails -> `folder` (top = cleaned path);
3. `git rev-parse --verify --quiet HEAD` fails -> `unborn` (top = top level);
4. otherwise `ready` (top = top level).

POST, under `d.prepareMu` so it never races a workspace preparation:

1. Validate: absolute, existing directory; refuse `filepath.Dir(p) == p` and
   `sameDirectory(p, os.UserHomeDir())` with
   `Sectile does not initialize a Git repository in <path>: choose the project's own folder`.
2. `folder`: `git init` then `git symbolic-ref HEAD refs/heads/main` in that
   folder. The two-step form gives `main` on any Git version, where
   `git init -b main` needs Git 2.28; it is the clarification's `git init -b
   main` in effect. `initialized = true`.
3. `folder` or `unborn`: `git commit --allow-empty --no-verify -m "Initial commit"`
   at the top level. Nothing is staged. `--no-verify` because an empty commit
   has nothing for a pre-commit hook to check, and a commit-message linter must
   not block the setup. Signing and identity follow the workstation's Git
   configuration untouched. `committed = true`.
4. `ready` (a second click, or a folder made ready meanwhile): nothing is run,
   `committed = false`.
5. Any state reaching Git: `excludeTaskWorktrees(ctx, top)` (the existing
   helper of `agent_macro_worktree.go`, idempotent), which settles the
   clarification's "to be confirmed" point in favour of the exclusion.

A Git failure is returned as it is (`gitLocal` already carries Git's output).
Nothing is rolled back: a repository whose commit failed stays `unborn`, which
is what the retry offer expects (spec US4).

`/desktop/status` adds `"git-init"` to `capabilities`.

## Why the agent, not Electron

Electron never runs Git today; the agent does, with its hidden-process wrapper
(`agentexec.Hidden`) and its own error texts. Running `git init` in Electron
would add a second place where Git is spawned for the same folders. The agent
also owns `prepareMu`, which keeps an initialization from interleaving with a
worktree being created in the same folder.

## Electron bridge

```js
// main.cjs
async function requireGitInit(){ /* status.capabilities includes 'git-init' */ }
ipcMain.handle('git-state',async(_,path)=>{ if(!(await hasGitInit()))return {state:'unknown'}; return api('/desktop/git-init?path='+encodeURIComponent(path)) })
ipcMain.handle('git-init',async(_,path)=>{ await requireGitInit(); return api('/desktop/git-init','POST',{path}) })
// preload.cjs
gitState:path=>ipcRenderer.invoke('git-state',path),
gitInit:path=>ipcRenderer.invoke('git-init',path),
```

`state: "unknown"` from an outdated agent shows no offer (spec FR11). A missing
capability on `git-init` throws
`Update and restart the local agent to initialize a Git repository.`

## Renderer

`desktop/src/git-init.mjs` exports:

- `offerFor(state)` -> `null` for `ready`, `missing`, `unknown`; otherwise
  `{title, detail, action: 'Initialize a Git repository', dismiss: 'Not now'}`.
- The texts:
  - `folder` title: `This folder is not a Git repository.`
  - `unborn` title: `This Git repository has no commit yet.`
  - shared detail: `Sectile can initialize it with an empty first commit so it
    can create worktrees. The repository stays on this workstation: it is never
    pushed and needs no remote. Nothing in the folder is committed, so
    worktrees start without its existing files.`
  - success notice: `Git repository initialized in <path>. Save the local
    configuration to use it.`

`desktop/src/main.js`, in `openProject`, around the Local repository and
Specifications folder rows (currently lines ~1313-1365):

- `attachGitOffer(input, row, {onReady})` builds a hidden
  `<div role="group" aria-label="<field label> Git initialization">` appended to
  the row, with a paragraph for the title and detail, the two buttons and a
  `role="status"` line for errors and the success notice.
- `examine()` calls `api.gitState(input.value.trim())` and renders
  `offerFor(state)`, unless the user dismissed that exact value. It runs at
  open, after *Browse* returns, and on the input's `change` event. A stale
  answer (the value changed while waiting) is discarded by comparing values.
- "Initialize a Git repository" disables both buttons, calls `api.gitInit`,
  then sets the field to the returned `path`, hides the offer, shows the
  success notice and calls `onReady` (for the Specifications folder, the
  existing `renderSpec` with the new value; the kind shown next to the field
  stays the stored one and refreshes on save, as it does today). On failure,
  the error text goes to the status line and the folder is examined again (an
  `unborn` result then shows the commit-only offer).
- "Not now" records the value as dismissed and hides the offer.
- The Specifications folder offer is attached to the folder input and examined
  only while that input is shown (box unticked, or multi-repo); ticking the
  box hides it.
- `form.onsubmit` is unchanged: saving never initializes.

## Data contracts

No stored setting changes. `settings.json` still maps the project to a path;
an initialized folder is stored exactly as any Git checkout is (its top level).

## Target files

| File | Change |
| --- | --- |
| `internal/agent/agent_desktop_git_init.go` | new: `gitFolderState`, handler |
| `internal/agent/agent_desktop.go` | route `/desktop/git-init`; capability `git-init` |
| `internal/agent/agent_desktop_git_init_test.go` | new: endpoint tests |
| `desktop/electron/main.cjs` | `git-state`, `git-init` handlers |
| `desktop/electron/preload.cjs` | `gitState`, `gitInit` |
| `desktop/src/git-init.mjs` | new: offer texts and state mapping |
| `desktop/src/main.js` | offer under both fields |
| `desktop/src/style.css` | offer block styling, reusing the notice styles |
| `desktop/tests/git-init.test.mjs` | new: `offerFor` unit tests |
| `desktop/tests/git-init.ui.cjs` | new: Playwright scenario with a fake agent |
| `desktop/README.md` | Local repository and Specifications folder sections |
| `CHANGELOG.md` | `[Unreleased]` / `Added` line |

## Test strategy

- **Go**, in temporary directories, with a controlled Git configuration
  (`GIT_CONFIG_GLOBAL` pointing at a test file with `user.name`/`user.email`,
  `GIT_CONFIG_NOSYSTEM=1`):
  - `folder` with files -> `initialized && committed`, branch `main`, exactly
    one commit, its tree empty, the files untracked, `/.tasks/` in
    `.git/info/exclude`, no `.gitignore` created, no remote;
  - a worktree can then be added from `HEAD` (`git worktree add -b x <dir> HEAD`);
  - `unborn` on a branch `trunk` with a local config key set beforehand ->
    `committed`, not `initialized`, branch still `trunk`, key still there;
  - `unborn` with a `commit-msg` hook that always fails -> still committed;
  - `ready` -> nothing run, commit count unchanged;
  - subfolder of an `unborn` checkout -> the checkout is committed, no nested
    `.git` is created;
  - relative path, absent path, file, `/` and `$HOME` (via `t.Setenv("HOME")`)
    -> 400 and nothing on disk;
  - no identity (`user.useConfigOnly=true`, no `user.*`, `EMAIL` and
    `GIT_AUTHOR_*`/`GIT_COMMITTER_*` unset) -> 422 with Git's text, and the
    folder's state is then `unborn`;
  - GET reports each of the four states;
  - `/desktop/status` lists `git-init`.
- **Node unit** (`git-init.test.mjs`): `offerFor` for every state; the detail
  says "never pushed", "no remote", "worktrees", "existing files".
- **Playwright** (`git-init.ui.cjs`, fake agent like `spec-folder.ui.cjs`):
  - Local repository typed to a `folder` path -> offer shown with the
    explanation; "Initialize" posts once, the offer hides, the notice asks to
    save; saving posts the returned path;
  - "Not now" hides it; saving then shows the agent's refusal;
  - an `unborn` Specifications folder stored -> offer shown at open with the
    no-commit title;
  - a 422 from the agent -> Git's text under the field, offer kept;
  - an agent without the capability -> no offer anywhere.

The UI test needs `npx vite build` first and the Playwright module path (see
the project memory on browser and desktop UI tests).

## Documentation

- `desktop/README.md`: under the Local repository and Specifications folder
  descriptions, one paragraph on the offer, the empty commit and its
  consequence on worktrees.
- `CHANGELOG.md`, `[Unreleased]` / `Added`:
  `**Start a project on a folder that is not a Git repository yet.** The desktop project settings offer to initialize the Local repository or the Specifications folder with an empty first commit, so Sectile can create worktrees from it. The repository stays on your workstation and is never pushed; since nothing is committed, worktrees start without the folder's existing files. A Git folder with no commit yet is offered the first commit alone. (#481)`
- No ADR: the change adds a local endpoint in the existing desktop pattern and
  makes no architectural trade-off.

## Rejected alternatives

- **Initializing implicitly on save**: a Git repository appearing in a folder
  because a form was saved is a surprise the clarification rules out; the
  offer is an explicit action.
- **A native Electron message box**: the settings already live in the app's
  dialog; a second, native modal on top cannot be driven by the Playwright
  tests and separates the explanation from the field it concerns.
- **Renaming an unborn repository's branch to `main`**: it would change a
  repository the user already created; the commit is made where `HEAD` points.
- **Running Git in Electron**: see "Why the agent, not Electron".
