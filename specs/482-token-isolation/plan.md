# Plan #482 - The server tracker credential never signs a person's action

Behaviour: `spec.md`. Decisions: `docs/clarifications/482.md`. Line numbers are
those of `a9374a61` and only locate the code; re-find them before editing.

## Stack and surfaces touched

- Go server only: `internal/tracker`, `internal/trackerapi`, `internal/db`,
  `internal/taskmcp`, `internal/handlers`.
- No schema change and no migration: the acting user is already stored on
  queued operations (`TrackerOp.UserID`) and on activities
  (`TaskActivity.UserID`).
- No web or desktop change. The refusal travels in the existing error and
  activity-step channels; the web already shows both.
- Docs: new ADR, `CHANGELOG.md`, `README.md` (§ personal credentials, around
  line 304).

## 1. The rule, in one place: `internal/tracker` + `internal/trackerapi`

### 1.1 An explicit unattended marker

Today an empty acting user means "unattended", which is exactly how the leaks
happened: a person-caused write that lost its actor on the way looked like the
synchronisation. The context gets a second, explicit marker.

```go
// internal/tracker/tracker.go
func WithUnattended(ctx context.Context) context.Context
func Unattended(ctx context.Context) bool
```

A context carries at most one of the two: `WithActingUser` with a non-empty
user clears nothing but wins; `Unattended` is only consulted when
`ActingUser(ctx) == ""`.

### 1.2 One write-side resolver

`trackerapi.Client` gets

```go
// ForWrite resolves the client for a tracker write. A named user gets their
// personal credential or an error; an unattended context gets the server
// credential; a context naming neither is refused (FR-3).
func (c *Client) ForWrite(ctx context.Context, tracker, projectID string) (*Client, error)
```

built on `ForActingUser`:

| Context | Personal credential | Result |
| --- | --- | --- |
| user named | found | personal client |
| user named | none | `ErrNoPersonalCredential` (provider named) |
| user named | sealed, not unlocked | the `ResolveUser` error, never a fallback |
| no user, `Unattended` | - | server client (`For(projectID)`) |
| no user, not unattended | - | `ErrNoActingUser` |

`ErrNoPersonalCredential` is a typed error (`*MissingPersonalCredentialError{Tracker}`)
whose message is the one Jira gives today, generalised:
`no personal <Provider> token for this user: add one in Profile → Tracker
credentials, or the work would be attributed to the server account`. The
existing Jira string is replaced by it, not duplicated (FR-8, FR-10).
`ErrNoActingUser` says `tracker write with no acting user and not marked
unattended` - a programming error surfaced loudly, not a user message.

### 1.3 Adapters: split read and write resolution

- `JiraAdapter.forProject` (`jira.go:46`) stays for reads unchanged. A new
  `forWrite(ctx, p)` calls `ForWrite`. Write methods switch to it:
  `CreateIssue`, `setPriorityAfterCreate` (uses the client it is given),
  `UpdateIssue`, `DeleteIssue`, `AddComment`, `Assign`, `Transition`,
  `SetSprint`, `SetTeam`, `SetParent`, `UpdateLabels`, and
  `CreateSprint` / `UpdateSprint` / `DeleteSprint` (`jira_sprints.go`).
  Reads (`GetIssue`, `SyncIssues`, `GetComments`, `ListBoards`, `ListSprints`,
  `ListBoardColumns`, `ListStatuses`, `ListIssueTypes`, `ListEpics`,
  `SearchTeams`, `TeamMembers`, `SearchAssignable`, `RequiredCreateFields`,
  `optionFields`, `fieldsFor`) keep `forProject`.
- `GithubAdapter.forProject` (`adapters.go:93`) stays for reads (fallback kept,
  FR-9). New `forWrite` for `CreateIssue`, `UpdateIssue`, `DeleteIssue`,
  `AddComment`, `UpdateLabels`. Its doc comment about the shared-token fallback
  is rewritten to say it now covers reads only.
- GitLab has no write adapter today (only `prstates.go`, a read). Nothing to
  switch; `ForWrite` handles `"gitlab"` so the first GitLab write gets the rule
  for free. Stated in the ADR.

### 1.4 GitHub milestone and transfer writes on `*trackerapi.Client`

`CreateGithubMilestone`, `UpdateGithubMilestone`, `DeleteGithubMilestone`,
`SetGithubIssueMilestone`, `TransferGithubIssue` are client methods called on
`d.tracker(proj.ID)` (server credential by construction). They are called on a
client obtained from `ForWrite` instead (§3.4). `ListGithubMilestones` is a
read and keeps `d.tracker`.

## 2. Unattended callers are marked

Every caller that legitimately writes with nobody behind it wraps its context
with `tracker.WithUnattended`. Known ones:

- the synchronisation pass (`internal/db/autosync.go` and the sync entry point
  it and the manual *Sync* share): wrap once at the top of the pass, so every
  write it makes on its own (housekeeping labels, and the like) is unattended.
  A manual *Sync* is unattended too (#464, US1): the activity records who asked,
  the credential stays the server's;
- the tracker-op worker when `op.UserID == ""` **and** the op was enqueued by
  unattended code (see §3.1: an op enqueued from a person's context always has a
  user, so "empty user at execution" now only comes from unattended enqueuers,
  which must say so with `TrackerOp.Unattended bool`, set by `EnqueueTrackerOp`
  from `tracker.Unattended(ctx)`);
- `PostBackTask` for a run with no recorded launcher (§3.6).

Inventory step (tasks T2.1): list every write call site
(`grep -rnE '\.(CreateIssue|UpdateIssue|DeleteIssue|AddComment|UpdateLabels|Assign|Transition|SetSprint|SetTeam|SetParent|CreateSprint|UpdateSprint|DeleteSprint|CreateGithubMilestone|UpdateGithubMilestone|DeleteGithubMilestone|SetGithubIssueMilestone|TransferGithubIssue)\('`
outside `internal/trackerapi`) and classify each as person-caused or
unattended in the PR description. Anything unattended not in the list above is
marked explicitly; anything person-caused gets its actor (§3).

## 3. Person-caused paths that lose the actor today

Each item gets a regression test (§5).

### 3.1 Queued operations

`EnqueueTrackerOp` / `enqueueTrackerOpUnsafe` already copy
`tracker.ActingUser(ctx)` into `op.UserID` (`trackerops.go:102,123`) and the
worker re-injects it (`:305`). Add `op.Unattended` (§2) and, at execution,
`ctx = tracker.WithUnattended(ctx)` only when `op.Unattended`. An op with
neither now fails its activity with `ErrNoActingUser` instead of running under
the server credential.

### 3.2 Stage report comment (`runStageOp`, `trackerops.go:780`)

`_ = d.AddTaskComment(task.ID, commentBody)` becomes
`d.AddTaskCommentAs(ctx, task.ID, commentBody)` with the op's `ctx` (which
already names `op.UserID`). Its error is no longer dropped: on error, append a
failure step (`❌ Rapport d'étape non consigné sur %s : %v`) and return the
error so the activity ends in failure, while the local stage, already written
earlier in the op, is kept (FR-7). The success step is appended only on
success. The label update earlier in the same op already uses `ctx`; with §1.3
it now refuses for a person without a GitHub token, and its existing failure
handling reports it.

### 3.3 MCP `create_task` (`taskmcp/server.go:403`)

`database.CreateTask(req)` becomes
`database.CreateTaskAs(tracker.WithActingUser(ctx, caller.UserID), req)`,
matching the REST handler. Refused first when the caller is anonymous (§3.7).

### 3.4 Macros (`internal/db/macros.go`)

`UpdateMacro`, `CreateMacro`, `DeleteMacro`, `CreateStoryUnderMacro`,
`CreateStoryFromMacroTodo`, `MigrateMacro` have no `ctx`. Add
`ctx context.Context` as first parameter and thread it from the handlers
(`h.actingContext(r)`) and MCP tools. Inside:

- milestone writes (`:225`, `:494`, `:575`, `:607`, `:652`, `:827`, `:916`,
  `:923`, `:1013`, `:1026`) use `d.trackerForWrite(ctx, proj.ID)` instead of
  `d.tracker(proj.ID)`, where

  ```go
  func (d *DB) trackerForWrite(ctx context.Context, trackerName, projectID string) (*trackerapi.Client, error)
  ```

  wraps `d.trackers.ForWrite`. Milestone reads (`:118`, `:816`, `:1022`) keep
  `d.tracker`;
- errors that are dropped today with `_ =` on a write are returned (or, where
  the function has already written locally and the current contract is
  "best effort", surfaced in the returned note/step) so US2's "reported instead
  of dropped silently" holds. Keep the local behaviour of each function as it
  is on a failed tracker write today;
- `CreateStoryUnderMacro` (`:555`) calls `CreateTaskAs(ctx, …)` instead of
  `CreateTask`, and `SetParent` (`:581`) uses
  `tracker.WithProject(ctx, target.ID)` instead of `context.Background()`.

`SetTaskMacro` and `MoveTasksToMacro` already take `ctx`; check their milestone
writes use it once `trackerForWrite` exists.

`trackerAs` (`trackercredentials.go:87`) stays a read helper: its fallback to
the server client on error is correct for reads only. Its doc comment says so;
no write may use it (review rule, and T2.1 inventory).

### 3.5 Local-to-remote conversion (`db.go:5790`)

`ConvertTaskToRemote(taskID, target)` becomes
`ConvertTaskToRemote(ctx, taskID, target)`; the handler (`handlers.go:2365`)
passes `h.actingContext(r)`. `ts.CreateIssue(ctx, …)`. A refusal runs the
existing `release()` and returns the error, so the task stays local.

### 3.6 Managed-run post-back (`internal/db/postback.go`)

`PostBackTask` resolves the run's launcher as it already does for pull-request
states (`postback.go:291`, `activity.UserID`) and builds the context used for
the stage transition, labels and report from it:
`tracker.WithActingUser(context.Background(), launcher)`, or
`tracker.WithUnattended(context.Background())` when no launcher is recorded.
The stage op then behaves as §3.1/§3.2: the local stage is kept, and a missing
personal credential fails the tracker step with the typed message (US3).

### 3.7 Anonymous callers (FR-6)

- MCP: a helper `requireCaller(caller) error` in `taskmcp` returns
  `tracker write refused: this key is not tied to a user; pair the desktop app
  or use a personal API key` when `caller.UserID == ""`. Called first by the
  write tools: `create_task`, `update_task`, `add_comment`,
  `transition_stage`. Read tools, `start_run`, `finish_run`,
  `report_waiting`, and worktree preparation are unchanged (they write nothing
  to a tracker).
- REST: handlers that cause tracker writes already go through
  `h.actingContext(r)`. With §1.2, an empty `webSessionUser` makes the write
  fail with `ErrNoActingUser` at the adapter. Map that error (and
  `MissingPersonalCredentialError`) to `403` with the messages above in the
  handler error helper, so REST callers get an explicit answer rather than a
  500. No per-handler change beyond passing `ctx` where §3.4/§3.5 add it.

### 3.8 Dead code

Delete `pushStageToTracker` (`interactive.go:48`), which has no caller.

## 4. Documentation

- ADR `docs/adrs/0029-server-tracker-credential-signs-unattended-work-only.md`
  (check the next free number on `origin/main`: two ADRs share 0027 and 0028
  already). Context: the leaks and ADR 0028's fallback. Decision: FR-2..FR-6,
  the explicit unattended marker, the read/write split. Consequences: breaking
  for shared-token deployments and for keys without a user. Alternatives
  rejected: see below. ADR 0028 gets a *Status: amended by 0029* line and its
  sentence at line 39 is annotated, not rewritten.
- `CHANGELOG.md`: the two lines of `spec.md` § Documentation acceptance.
- `README.md` ~line 307: "Jira accepts only a personal credential; GitHub
  accepts either, and falls back to the server token" becomes: every provider
  needs a personal credential to write; reads still use the server credential
  when you have none.

## 5. Test plan

Test double: a fake `tracker.TicketSystem` (and a fake GitHub HTTP server for
milestone client methods, as existing `trackerapi` tests do) that records, per
call, `tracker.ActingUser(ctx)`, `tracker.Unattended(ctx)` and the token the
HTTP request carried.

Unit (`internal/trackerapi`):

- `ForWrite` table: the five rows of §1.2, for `jira`, `github`, `gitlab`.
- Jira and GitHub adapters: each write method with a user without a token is
  refused and sends no HTTP request; each read method with the same user still
  succeeds with the server token (GitHub) / behaves as today (Jira).
- The error message names the provider and `Profile → Tracker credentials`.

Store (`internal/db`), one test per leaking path, each asserting the recorded
actor is the requesting user and no request carried the server token:

1. stage op with a note (`runStageOp`): comment made as `op.UserID`; with a
   user without a token, the local stage is recorded, the activity is failed,
   the failure step is present, and the success step is absent;
2. MCP `create_task` (in `internal/taskmcp`) as a resolved caller;
3. `ConvertTaskToRemote` with an acting context; refused conversion leaves the
   task local;
4. `CreateStoryUnderMacro` / `CreateStoryFromMacroTodo`: creation and
   `SetParent` as the user;
5. GitHub macro milestone writes (create, update, delete, set, migrate) as the
   user; milestone listing still uses the server token;
6. `PostBackTask` with a launcher (writes as launcher), a launcher without a
   token (stage kept, tracker step failed), no launcher (unattended, server
   token);
7. a queued op with neither user nor `Unattended` fails with
   `ErrNoActingUser`;
8. the sync pass: its writes are unattended and use the server token.

MCP (`internal/taskmcp`): each write tool with an unresolved caller returns the
refusal and makes no tracker call; read tools still answer.

REST (`internal/handlers`): a write through an agent credential without a
device answers 403 with the message; a web session without a GitHub token gets
403 with the missing-credential message on ticket creation.

Run: `go test ./internal/tracker/... ./internal/trackerapi/... ./internal/db/...
./internal/taskmcp/... ./internal/handlers/...`, plus PostgreSQL when a DSN is
available (the stage-op and post-back tests touch activities). `go vet ./...`.

## Alternatives rejected

- **Keep "empty actor = unattended" and only fix the six paths.** Fixes today's
  leaks but the next lost `ctx` leaks again silently. The explicit marker makes
  the loss fail loudly (FR-3).
- **Refuse reads too for people without a token.** Out of scope by the owner's
  decision: a read attributes nothing and refusing would blank screens.
- **Refuse anonymous callers at authentication.** Would also cut their reads
  and run reporting, which the decision does not ask for.
- **Fix `pushStageToTracker`.** It has no caller; deleting it removes a
  `context.Background()` write for good.
