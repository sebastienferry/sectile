# Plan #663 - Roadmap: ordered epic todos, mirrored on the tracker

Implementation plan for `spec.md`. The behaviour lives there; this file says
where it goes and how.

## Stack and surfaces

- Server: Go. `internal/db/macros.go` (todo saves), a new
  `internal/db/macrotodosmirror.go` (eligibility, rendering, scheduling, the
  write), `internal/db/trackerops.go` (new queued kind),
  `internal/db/migrations.go` (columns), `internal/handlers/handlers.go`
  (republish route).
- Tracker: `internal/tracker/ticketing.go` (optional marked-comment
  interface), `internal/trackerapi/jira.go` (its Jira implementation),
  `internal/trackerapi/github.go` (milestone read and description write).
- MCP: `internal/taskmcp/server.go` (`get_macro`, `update_macro_todos`).
- Skill: `internal/skills/fragments/refine_macro/*.md` and the four goldens
  `internal/skills/testdata/golden/refine_macro.*`.
- Web: `web/src/components/RoadmapView.tsx`, `web/src/types/index.ts`,
  `web/src/context/AppContext.tsx` (or wherever the macro calls live),
  `web/src/locales/planning.ts`.
- Desktop: nothing of its own; it serves the same web build.
- Docs: `CHANGELOG.md`, a new ADR `docs/adrs/0045-macro-todos-are-mirrored-one-way.md`,
  and the MCP tool list in `docs/ARCHITECTURE.md` if it enumerates tools.

No new child process (NFR2). No change to story creation (#634).

## Data

Migration 43 (39 to 42 landed on main for #635 and #654), `macros.todos_mirror`, in `internal/db/migrations.go` only,
never in the frozen baseline:

```sql
ALTER TABLE macros ADD COLUMN todos_mirror_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE macros ADD COLUMN todos_mirror_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE macros ADD COLUMN todos_mirror_error TEXT NOT NULL DEFAULT '';
ALTER TABLE macros ADD COLUMN todos_mirror_at TIMESTAMP NULL;
```

- `todos_mirror_ref`: the Jira comment id; empty on GitHub, where the
  milestone number is the key itself.
- `todos_mirror_hash`: SHA-256 of the last body written successfully (FR9,
  "up to date").
- `todos_mirror_error`: the last failure, cleared by a successful write.
- `todos_mirror_at`: time of the last successful write.

These columns are written only by the mirror job and are never part of
`saveMacroMetaKeys`' upsert, so a list save cannot clobber them. Before
numbering, check `origin/main` for a migration 39 landed meanwhile and
renumber.

Known fallout to handle in the same commit: the rewind helpers of the db tests
(`dropRepositoryColumns`-style) and the forget-and-replay fixtures must know
the new columns; run the PostgreSQL suite against a throwaway DSN.

`models.MacroMeta` gains, computed on read and never stored as such:

```go
// TodosMirror is where the todos are copied on the tracker (#663).
type MacroTodosMirror struct {
    Kind      string     `json:"kind"`             // "jira_comment" | "github_description" | ""
    Reason    string     `json:"reason,omitempty"` // why Kind is "", French sentence
    UpToDate  bool       `json:"upToDate"`
    Error     string     `json:"error,omitempty"`
    WrittenAt *time.Time `json:"writtenAt,omitempty"`
    URL       string     `json:"url,omitempty"`
}
TodosMirror *MacroTodosMirror `json:"todosMirror,omitempty"`
```

`UpToDate` compares `todos_mirror_hash` with the hash of the body rendered
from the current todos. The read paths that return a macro to a client
(`GetProjectMacros`, the PUT answer, `findMacroMeta` for MCP) fill it.

## Eligibility (FR12)

```go
// macroTodosMirror tells where a macro's todos are mirrored, or why they
// stay in Sectile.
func (d *DB) macroTodosMirror(proj *models.Project, key string) (kind string, reason string)
```

In order:

1. no project, or a local tracker: none, "projet local".
2. `strings.EqualFold(proj.IssueTracker, "gitlab")`: none, "une macro GitLab
   n'a pas de ticket qui puisse porter la liste" (D5).
3. `githubMilestoneMacros(proj)`: key must be `M-<n>` with `n` among the
   listed milestones (reuse the listing `GetProjectMacros` already does; in
   the job, a 404 on the milestone read is the refusal). Kind
   `github_description`.
4. Jira: `isForeignMacro` true: none, the ADR 0043 sentence. `isMilestoneKey`
   true: none, "macro locale, sans épic Jira". Else the tracker must expose
   `tracker.MarkedCommentWriter`: kind `jira_comment`, else none with
   `tracker.Unsupported(..., CapComment)`'s sentence.

## Rendering (FR14, FR15)

```go
// renderTodosMirror renders the mirror body of a list for one kind, within
// the kind's size budget.
func renderTodosMirror(kind string, todos []models.MacroTodo) string
```

Markdown, French runtime text (NFR3), heading in the convention of the
stage comments:

```markdown
### 📋 [Sectile] Todos

1. [ ] Import the Jira priorities - PROJ-41
2. [x] Show the quarter on the card
3. [ ] Remove the legacy column

_Liste tenue dans Sectile : une modification faite ici est remplacée à la prochaine mise à jour._
```

- Story key appended as ` - KEY` when attached.
- Empty list: Jira body is the heading plus "_Aucun todo pour l'instant._";
  GitHub renders "" and the block is removed.
- Budgets: Jira 30,000 characters (limit 32,767, kept clear of ADF
  overhead); GitHub 60,000 minus the length of the text outside the block
  (limit 65,536). Over budget: keep the first todos that fit and end with
  "… et N autres todos dans Sectile."
- Verify `MarkdownToADF` renders `1. [ ]` usefully. If it does not support
  task-list items inside ordered lists, render `☐` / `☑` prefixes for Jira
  instead; the hash is per rendered body, so either is fine.
- GitHub block markers: `<!-- sectile:macro-todos -->` and
  `<!-- /sectile:macro-todos -->`, preceded by a blank line.

```go
// splitTodosBlock separates a milestone description into the text Sectile
// does not own and its todo block. A description without a block returns
// it whole and "".
func splitTodosBlock(description string) (outside string, block string)
// joinTodosBlock writes outside, a blank line and the block; outside alone
// when block is "".
func joinTodosBlock(outside, block string) string
```

Malformed blocks (opening marker without closing): everything from the
opening marker to the end is Sectile's. Tested.

## Scheduling (FR6, FR7)

```go
// scheduleTodosMirror queues the mirror of a macro a few seconds after the
// last save of its todos on this instance, as the person ctx names.
func (d *DB) scheduleTodosMirror(ctx context.Context, projectID, key string)
```

- `DB` gains `todosMirrorTimers sync.Map` keyed `projectID + "\x00" + key`,
  holding a `*time.Timer` and the acting user id and unattended flag read from
  `ctx` at the time of the save (the last save wins).
- Delay: `todosMirrorDelay`, a package variable, 3 s; tests set it to a few
  milliseconds.
- On fire: `EnqueueTrackerOp` with a context rebuilt from the stored actor
  (`tracker.WithActingUser`), so the #482 rule holds.
- Ineligible macros are not scheduled (AC4: no tracker call).
- Multi-replica: each instance debounces its own saves; two instances may
  each queue a write, and FR9's hash check makes the second a no-op. No
  cross-instance lock is needed: Jira update by id is idempotent, and the
  GitHub write is a read-modify-write of Sectile's own block.
- A restart drops pending timers: the macro then reads "not up to date" and
  the panel offers **Republier**; the next save also republishes.

Call sites, each after a successful save, with the request's acting context:

- `UpdateMacro` when `todos != nil`;
- `recordLineStoryKey` (single creation) and `CreateStoriesFromMacroTodos`
  once at the end of the batch rather than per line;
- `TodosFromSDD` and `TodosFromMacroStories` (`internal/db/sddslicing.go`);
  `TodosFromMacroStories` takes no ctx today: add one;
- the MCP `update_macro_todos` tool, through `UpdateMacro`.

## Queued write (FR8 - FR11, FR13)

`TrackerOpEpicTodos TrackerOpKind = "epic_todos"` in `trackerops.go`, with
`EpicKey`, `TaskKey` and `ProjectID` set. Activity: action
`Todos de KEY ➔ tracker`, summary `Recopie des todos de KEY en file
d'attente`. Dispatch to `runEpicTodosOp`, which calls:

```go
// PushMacroTodosMirror writes the current todos of a macro on its tracker
// copy. Run from a queued activity only.
func (d *DB) PushMacroTodosMirror(ctx context.Context, projectID, key string) (string, error)
```

1. Re-check eligibility (an epic may have become foreign, a project may have
   changed tracker). Ineligible: return the reason as an error.
2. Read the current todos (not the ones at schedule time) and render.
3. Hash equal to `todos_mirror_hash` and (Jira) a stored ref: return
   "todos de KEY déjà à jour", no tracker call.
4. Write with `retryTransient(ctx, 3, func() error)`, pauses 1 s then 3 s,
   retrying only network errors, timeouts and HTTP 429 / 5xx (inspect the
   trackerapi error type; add a small `trackerapi.IsTransient(err)` if none
   exists).
5. Success: store ref, hash, `todos_mirror_at`, clear the error, in one
   `UPDATE macros SET todos_mirror_* ... WHERE project_id = ? AND key = ?`.
   Failure: store the error text only, return it wrapped as
   "todos gardés dans Sectile mais pas recopiés sur KEY : %w", keeping the
   credential-missing chain so #645's offer appears.

### Jira

Optional interface in `internal/tracker/ticketing.go`, type-asserted like
`PullRequestDiscoverer`:

```go
// MarkedCommentWriter keeps one comment Sectile owns on an issue, found
// again by its id or by a marker property.
type MarkedCommentWriter interface {
    UpsertMarkedComment(ctx context.Context, req UpsertMarkedCommentRequest) (commentID string, err error)
}

type UpsertMarkedCommentRequest struct {
    Project   *models.Project
    Key       string
    CommentID string // the id remembered, "" when none
    Marker    string // property key, "sectile.macroTodos"
    Body      string // Markdown
}
```

`JiraAdapter.UpsertMarkedComment`, through `forWrite`:

1. `CommentID` set: `PUT /rest/api/3/issue/{key}/comment/{id}` with
   `{"body": MarkdownToADF(body)}`. Success: return the id. 404: go on.
2. Search: `GET /rest/api/3/issue/{key}/comment?expand=properties`, paged as
   `GetComments`, for a comment carrying the property `Marker`. Found: PUT it.
3. Else `POST /rest/api/3/issue/{key}/comment` with the body and
   `"properties": [{"key": Marker, "value": {"macroKey": key}}]`, return the
   new id.

The property is the marker on Jira because ADF does not keep HTML comments.
A comment without the property is never touched (US3.8).

### GitHub

`internal/trackerapi/github.go` gains:

```go
// GetGithubMilestone reads one milestone.
func (c *Client) GetGithubMilestone(repo, repoPath string, n int) (*GithubMilestoneItem, error)
// SetGithubMilestoneDescription writes a milestone's description, the empty
// one included, which UpdateGithubMilestone cannot send.
func (c *Client) SetGithubMilestoneDescription(repo, repoPath string, n int, description string) error
```

The job: client from `d.trackerForWrite(ctx, "github", proj.ID)`, read the
milestone, `splitTodosBlock`, `joinTodosBlock(outside, rendered)`, write only
when the result differs from what was read.

### Description sharing (FR16)

- `UpdateMacro`, GitHub branch, when `description != nil`: send
  `joinTodosBlock(*description, renderTodosMirror(github, currentTodos))`
  with `SetGithubMilestoneDescription` (so an emptied description is sent
  too). The current todos are the stored ones, or `*todos` when the same
  request saves them. On success with a non-empty block, store the block's
  hash as `todos_mirror_hash` so no redundant job write follows.
- `GetProjectMacros`, milestone import: insert
  `splitTodosBlock(m.Description).outside` rather than `m.Description`.
- `MigrateMacro` copies the local description, which never holds the block:
  no change.

## Route (FR18)

`POST /api/projects/{id}/macros/{key}/todos-mirror` in
`internal/handlers/handlers.go`, beside `labels`: refuses an ineligible macro
with `400` and its reason; otherwise `EnqueueTrackerOp` at once and answers
`202` with the activity. `epics` alias as the other sub-routes.

## MCP (FR19 - FR21)

In `internal/taskmcp/server.go`, beside `prepare_macro_worktree`:

- `get_macro` (`macroInput{ProjectID, MacroKey}`): `database.GetMacro`, a new
  thin read built on `findMacroMeta` plus `FillMacroFlags` and the mirror
  status; unknown project: "projet non trouvé"; unknown macro:
  "macro KEY introuvable dans le projet".
- `update_macro_todos` (`macroTodosInput{ProjectID, MacroKey, Todos
  []macroTodoInput}`; item `{ID, Text, Done, TargetProjectID,
  TargetTrackerProject}`; no `storyKey` field, so US6.5 holds by
  construction): `requireCaller`; `database.ReplaceMacroTodos(ctx, ...)`.

```go
// ReplaceMacroTodos saves a full ordered list given by an agent. It refuses
// a blank text, an unknown id or a repeated id before saving anything, keeps
// the story key and origin of every known line, and schedules the mirror.
func (d *DB) ReplaceMacroTodos(ctx context.Context, projectID, key string, items []MacroTodoInput) (*models.MacroMeta, error)
```

It merges the input onto the stored lines (`SourceKind`, `SourceEntry`,
`StoryKey` from the stored line) and calls `UpdateMacro(ctx, ..., &todos,
...)`, so the race rules of `saveMacroMetaKeys` apply. Context:
`tracker.WithActingUser(ctx, caller.UserID)`.

## Skill (FR22)

`internal/skills/fragments/refine_macro/`:

- `read-first.md`: read the macro with `get_macro` (projectId, macroKey).
- `steps.md`: a final step: show the merged list (existing todos kept with
  their ids, new ones appended or placed where the owner says), ask for
  confirmation, then call `update_macro_todos` with the full ordered list;
  on refusal report it and the list, no other route.
- `guard.md`: replace "Do not overwrite existing todos or tasks without user
  confirmation in the UI" with "Do not save todos without the owner's
  confirmation in the session, and never drop an existing todo the owner did
  not ask to drop."
- `report.md`: add the saved order and the mirror status.

Regenerate the four goldens with the catalog's update flag, read the diff,
and check that the marketplace plugin is not regenerated (it is maintained by
hand).

## Web

- Types: `MacroTodosMirror` and `todosMirror?` on the macro type.
- Inline rewording: the todo text `<span>` becomes a button-like element
  that swaps to an `<input>` (local state `editingTodoId`, `draftTodoText`).
  Enter or blur commits through `persist(key, { todos })` with only that
  line's `text` changed; Escape, blank or unchanged restores. Disabled while
  `batchRunning`.
- Reorder: a handle per line (`draggable`) using native HTML5 drag events,
  as the epic rows already do; drop computes the new index and persists.
  Move up / down buttons and Alt+Up / Alt+Down on the focused line; focus is
  restored on the moved line after the save. Pure helper
  `moveTodo(todos, fromIndex, toIndex)` in `web/src/lib/roadmap.ts`, unit
  tested.
- Status line under the checklist from `todosMirror`, with the link when
  `url` is set and **Republier** calling the new route; missing-token
  failures reuse the existing credential offer (#645).
- After a save the panel already replaces the macro with the answer, which
  now carries `todosMirror` ("Publication en attente" until the job runs).
  Refresh the macro when the activity of kind `epic_todos` ends, through the
  existing activity polling, so the line turns to **Recopiée**.
- Strings in `web/src/locales/planning.ts`, French and English:
  `todoEdit`, `todoMoveUp` (Monter / Move up), `todoMoveDown` (Descendre /
  Move down), `todoDragHandle`, `mirrorUpToDate` (Recopiée sur {key} /
  Copied to {key}), `mirrorPending` (Publication en attente / Waiting to be
  published), `mirrorFailed` (Échec de publication : {reason} / Publishing
  failed: {reason}), `mirrorLocal` (Reste dans Sectile : {reason} / Stays in
  Sectile: {reason}), `mirrorRepublish` (Republier / Publish again).

## Rejected alternatives

- **Two-way sync** (parse the comment back): rejected by D1.
- **A pending flag in the database with a startup sweep**: a restart would
  have to replay writes for a person who may no longer be the one to sign
  them (#482). The hash comparison shows the macro as not up to date, which
  the panel can republish, without a new column.
- **An HTML comment marker on Jira**: ADF drops it; the comment property
  survives edits and is invisible.
- **A GitLab group epic or a carrier issue**: rejected by D5 and ADR 0030.
- **One write per save, no debounce**: a drag-then-reword burst would make
  several tracker writes and activities for one intent.

## Test plan

Go (`go test ./internal/...`, sandbox off for httptest; GOCACHE under
`$TMPDIR`):

- `macrotodosmirror_test.go`: eligibility table (GitHub listed / unlisted
  milestone, Jira own / foreign / `M-<n>`, GitLab, local); rendering (order,
  checkboxes, keys, empty list per kind, truncation line at the budget);
  `splitTodosBlock` / `joinTodosBlock` round trips, missing and malformed
  blocks; hash no-op.
- Scheduling: several saves within the delay queue one op; the op renders
  the last list; ineligible macros queue nothing; the actor of the last save
  signs the op.
- Job with a fake tracker: Jira create then update by id; 404 then search by
  property; no property then create; a comment without the property is never
  updated; transient failure retried then success; permanent failure stored
  and reported with the credential chain; success clears the error.
- GitHub with an httptest server: block appended after existing text; block
  replaced, outside text untouched; empty list removes the block; description
  push keeps the block; milestone import strips it.
- `UpdateMacro`, `recordLineStoryKey`, batch creation, `TodosFromSDD`,
  `TodosFromMacroStories` each schedule the mirror.
- `ReplaceMacroTodos`: order kept, story key and origin kept, omitted line
  removed, new line gets an id; blank text, unknown id, duplicate id refused
  with nothing saved; input story key ignored.
- MCP (`server_test.go`): both tools listed; `get_macro` answer shape;
  `update_macro_todos` anonymous refusal, unknown macro refusal, success
  answer with `todosMirror`.
- Handler: `todos-mirror` route 202 / 400.
- Migration: the db suite on SQLite and on PostgreSQL with a throwaway DSN.
- Skill goldens updated and `catalog_test.go` green.

Web:

- `moveTodo` unit tests (first, last, same index, out of range).
- Browser component test (`*.browser.mjs`, Playwright from
  `desktop/node_modules`) for the panel: reword commit, Escape, blank;
  Alt+Down moves and keeps focus; status line variants and **Republier**.
- `tsc` and `oxlint` with the main checkout's `node_modules` linked when the
  worktree has none.
