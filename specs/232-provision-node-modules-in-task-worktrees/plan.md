# Implementation plan - #232

Behaviour and acceptance criteria are in [`spec.md`](spec.md). This file records the technical
choices and the files to touch. The ordered checklist is in [`tasks.md`](tasks.md).

## Stack

Go agent (`internal/agent`), server-side operation budget (`internal/db/agentoperations.go`),
tests with `go test ./internal/agent/ ./internal/db/`.

## Where the step runs

Worktrees are created by `ensureLocalWorktree` (`internal/agent/agent_config.go`), called from
`agentDaemon.prepareDispatch`. `prepareDispatch` has two callers:

- the launch path (`internal/agent/agent.go`, after `awaitRunSlot`);
- the `prepare_workspace` operation (`internal/agent/agent_operations.go`), which the server
  sends for `POST /api/tasks/{id}/checkout-branch` through `DB.EnsureTaskWorktree`.

Provisioning is added to `prepareDispatch`, so both callers get it.

### Outside `prepareMu`

`prepareDispatch` holds `d.prepareMu` for its whole body, and so do the console, desktop and
`sync_config` paths. An `npm ci` of several minutes under that mutex would stall every other
preparation on the agent (NFR2). The body therefore moves to `prepareDispatchLocked`, which also
returns the resolved project root, and `prepareDispatch` becomes a wrapper: take the lock, call
`prepareDispatchLocked`, release the lock, then run `provisionWorktree(ctx, root, workDir)` when
preparation succeeded.

Two preparations of the **same** worktree are serialised by a per-directory mutex held by the
provisioning step, so two installs never run in one folder at once.

### Existing deadlock in `prepare_workspace`

The `prepare_workspace` case locks `d.prepareMu` and then calls `prepareDispatch`, which locks
it again. `sync.Mutex` is not reentrant, so this operation deadlocks today, and every later
preparation on the agent deadlocks behind it. The ticket requires provisioning on every
`prepare_workspace`, so the redundant outer lock is removed. `prepareDispatch` already takes
the lock itself.

### Server budget

`operationTimeout` gives `prepare_workspace` the 45 s default. A first install exceeds it, so
`prepare_workspace` gets a 17 minute budget. That stays above the agent's
total provisioning timeout (15 minutes) plus the git work.

## The provisioning step (`internal/agent/provision.go`)

```go
// Timeouts are package variables so tests can shorten them.
var (
	provisionFolderTimeout = 10 * time.Minute
	provisionTotalTimeout  = 15 * time.Minute
)

// npmInstall runs the install in dir. Tests replace it.
var npmInstall = func(ctx context.Context, dir string) error

func provisionWorktree(ctx context.Context, root, workDir string)
func packageFolders(workDir string) []string
func installReason(dir string) (install bool, skip string)
```

- **Main-checkout guard:** `sameDirectory(workDir, root)` returns early.
- **Discovery:** `workDir`, then the `os.ReadDir(workDir)` entries that are directories, whose name
  does not start with `.`, and are not `node_modules`. A folder qualifies when `package.json`
  is a regular file in it. `os.ReadDir` sorts, so logs and tests are deterministic.
- **Lockfile:** no `package-lock.json` → `log.Printf` a skip naming the folder.
- **Link guard:** `os.Lstat(node_modules)`. Missing is fine. Present and `!fi.IsDir()` or
  `fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0` → skip with a log line. Since Go 1.23,
  Windows reports junctions (mount points) as `ModeIrregular` rather than `ModeSymlink`, so
  both bits are checked.
- **Stamp:** `node_modules/.install-stamp`. Install when it is missing or when the mtime of
  `package.json` or `package-lock.json` is after it. After a successful install the stamp is
  written and its mtime set to now, like `touch`.
- **Install:** `npmInstall` runs `exec.CommandContext(ctx, "npm", "ci")` in the folder, with output
  captured and `WaitDelay` set so a surviving grandchild holding the pipes cannot hang the
  launch after the timeout. On Windows, `exec.LookPath` resolves `npm.cmd` through `PATHEXT`.
  On failure the error carries the tail of the output.
- **Timeouts:** a `context.WithTimeout(ctx, provisionTotalTimeout)` for the whole step, and a
  per-folder `context.WithTimeout(total, provisionFolderTimeout)`.
- **Logs:** the agent's existing `[Agent] …` log lines are in English, so these are too.

## Documentation

`docs/REIMPLEMENTATION_GUIDE.md` step 2: replace the `node_modules` / `.env*` symlink bullet with
the per-worktree `npm ci`, stamp and skip rules, and note that it runs in the agent.

## Tests (`internal/agent/provision_test.go`)

The injected `npmInstall` records the folders it was called for and creates `node_modules`.

- installs in the root and in `web/` and `desktop/` subfolders, writes stamps, and leaves the
  main checkout untouched;
- no `package.json` anywhere → no call;
- `package.json` without a lockfile → no call;
- root with `package-lock.json` only → root skipped;
- hidden directories and `node_modules` are not searched;
- a fresh stamp → no call; a newer `package-lock.json` → called again;
- a `node_modules` symlink → no call, link untouched (skipped when the platform cannot create
  a symlink);
- `workDir` equal to `root` → no call;
- a failing install → no stamp, the other folders are still attempted;
- the context passed to the install carries a deadline.

`internal/db/agentoperations_test.go`: the `prepare_workspace` row expects 17 minutes.

## Rejected alternatives

- **Linking the main checkout's `node_modules`:** rejected by the owner (shared state, and
  `npm ci` through a link empties the main checkout).
- **Provisioning under `prepareMu`:** it would serialise every preparation on the agent behind
  an install.
- **Making provisioning a separate agent operation:** the launch path must wait for it anyway,
  and a second round trip adds nothing.
