# Plan #398 - Recreate GitLab tracker support

Behaviour: [`spec.md`](spec.md). Checklist: [`tasks.md`](tasks.md).

## 1. Stack and shape

- Go, package `internal/trackerapi`, on the shared `Client` (guarded
  `request`: same-origin redirects, 16 MiB cap, `HTTPError` without body in
  its message, `IsRateLimited`). No new dependency.
- A **port** of Taskativ's GitLab client
  (`~/Sources/apps/taskativ/internal/runner/gitlab_rest.go`,
  `gitlab_graphql.go`, the milestone and member parts of `gitlab_epics.go`,
  `internal/tracker/gitlab.go`, `gitlab_writer.go`), rewritten onto
  Sectile's `Client` and `tracker.TicketingSystem`. Not ported: group epics
  (`GroupEpics`, `CreateEpic`, `UpdateEpic`, `DeleteEpic`, `EpicByIID`,
  `SetIssueEpic`, `SetEpicField`, `EpicField`), scoped `workflow::` /
  `priority::` label reading, Taskativ's `PROJECT-N` keys, and the
  "milestones first, then iterations" `sprintKind` probe.
- Web: TypeScript/React in `web/src` (tracker lists, project modal, types,
  translations). Desktop: nothing, it has no tracker choice.
- No migration: `gitlab_url`, `gitlab_project`, the server GitLab credential
  and personal GitLab credentials already exist (#464).

## 2. Files

| File | Change |
| --- | --- |
| `internal/trackerapi/gitlab_client.go` (new) | REST helpers on `Client`: `gitlab(ctx, method, path, query, payload, result)`, `gitlabPages(ctx, path, query)` (pagination), `gitlabGraphQL`, `gitlabErrorText`, project path encoding, GraphQL endpoint derivation. |
| `internal/trackerapi/gitlab_issues.go` (new) | Issues: list (full / `updated_after`), get, create, update, delete, notes, related MRs, members, labels. |
| `internal/trackerapi/gitlab_boards.go` (new) | Boards, board lists, statuses, column moves. |
| `internal/trackerapi/gitlab_sprints.go` (new) | Milestones (REST), iterations and cadences (GraphQL), sprint id codec, Premium probe cache. |
| `internal/trackerapi/gitlab_mapping.go` (new) | `GitlabIssueItem`, `gitlabTask`, stage / macro / team / sprint extraction, `FormatTaskID`. |
| `internal/trackerapi/gitlab.go` (new) | `GitlabAdapter`: capabilities, `forProject`, every `TicketingSystem`, `Writer`, `SprintManager`, `PullRequestDiscoverer` method. |
| `internal/trackerapi/gitlab*_test.go` (new) | `httptest` suites (§9). |
| `internal/trackerapi/adapters.go` | Register `gitlab` in `NewDefaultRegistry`; doc comment. |
| `internal/trackerapi/mapping.go` | Extract the stage-label → status switch of `githubTask` into `statusFromStageLabels(labels)` shared by both mappings (behaviour-preserving). |
| `internal/db/db.go` | External URL fallback for `gitlab`; assignee push for `Source == "gitlab"`; `taskSource` untouched (adapter always sets `Source`). |
| `internal/db/teams.go` | `SetTasksTeam` accepts tasks whose tracker supports `CapTeam` instead of `Source == "jira"`; message generalised in French. |
| `internal/db/trackerops.go` | `runAssignOp`: for GitLab, an unresolved account id falls back to the typed username; message no longer says "Jira" for GitLab. |
| `internal/db/macros.go` | `CreateStoryUnderMacro` / macro attach: GitLab writes `macro:` / `parent:` labels through `UpdateLabels`; `CreateMacro` creates a GitHub milestone only when `IssueTracker == "github"` (a GitLab project with a stale `GithubRepo` must not create one). |
| `internal/db/trackerinstance.go` | `sameTrackerInstance`: `gitlab` case, same API URL and same GitLab project (spec O2). |
| `internal/models/models.go` | Comments listing trackers (`"github", "jira", "gitlab", "local"`). |
| `web/src/types/index.ts` | `IssueTracker` gains `'gitlab'`. |
| `web/src/lib/trackers.ts` | `PROJECT_TRACKERS` gains `{ id: 'gitlab', label: 'GitLab' }`; GitLab loses `hasAdapter: false`; comments updated; `needsCredentialsFor` already handles it through `storedFor`. |
| `web/src/components/ProjectModal.tsx` | GitLab tracker card and description, GitLab project path and instance URL inputs bound to `gitlabProject` / `gitlabUrl`, prefilled from settings. |
| `web/src/locales/translations.ts` | French (and existing locales') strings for the GitLab card, fields and warnings. |
| `web/src/lib/trackers.test.ts` (or existing test) | `PROJECT_TRACKERS` and `personalTrackers` include GitLab. |
| `README.md`, `docs/CAPABILITIES.md`, `docs/API_AND_DATA_SPEC.md`, `docs/ARCHITECTURE.md` | GitLab tracker section, capability row, `issueTracker` values, adapter list. |
| `docs/adrs/0029-gitlab-tracker-mapping.md` (new) | ADR (§10). Check the next free number against `origin/main` first. |
| `CHANGELOG.md` | One `Added` line under `[Unreleased]`. |

## 3. Client plumbing (`gitlab_client.go`)

- Base URL: `c.GitlabURL` (already resolved per project and per user by
  `For` / `ForActingUser`), trimmed of `/`. Auth header `Bearer <token>`;
  empty token → `c.missingCredential("GitLab")`; `c.unreadable.gitlab` →
  the existing unreadable-credential error, like GitHub.
- Project segment: `url.PathEscape(strings.Trim(path, "/"))`, so
  `acme/platform/app` → `acme%2Fplatform%2Fapp`. The path comes from
  `c.GitlabProject` (project override, else settings, D11). An empty path is
  refused: `configurez le projet GitLab (groupe/projet) du projet Sectile`.
- Request builder: the shared `request` sends JSON; GitLab v4 accepts JSON
  bodies for every write used here. Query strings via `url.Values`.
- Pagination (`gitlabPages`): `per_page=100`, follow `X-Next-Page` while it is
  non-empty, hard stop at 100 pages (10 000 items) with an error rather than
  a silent truncation.
- GraphQL endpoint: base URL with a trailing `/api/v4` replaced by
  `/api/graphql` (`https://gitlab.example.org/api/v4` →
  `https://gitlab.example.org/api/graphql`). Uses `c.graphqlRequest`.
- Errors: `gitlabErrorText(body)` ported from Taskativ: it reads `message`
  (string, list or map of field → messages) or `error` / `error_description`,
  truncates to 300 characters, and wraps as
  `GitLab a refusé la requête (HTTP %d) : %s`. 401 → `jeton GitLab refusé`,
  403 → `accès refusé par GitLab`, 404 → `projet ou ticket GitLab introuvable`.
  429 keeps the `*HTTPError` in the chain (`%w`) so `IsRateLimited` still
  matches. No token is ever formatted.

## 4. Mapping (`gitlab_mapping.go`)

`GitlabIssueItem`: `id`, `iid`, `title`, `description`, `state`
(`opened`/`closed`), `web_url`, `labels []string`, `author{username,avatar_url}`,
`assignees[]{id,username}`, `milestone{id,title}`, `iteration{id,title}`
(present on Premium), `issue_type`, `updated_at`, `closed_at`.

`gitlabTask(projectPath, item)`:

- `Key` `#<iid>`; `ID` provisional `gl-<iid>` (rewritten by `FormatTaskID`).
- `Status`: `closed` → finished; else `statusFromStageLabels(labels)` (the
  switch extracted from `githubTask`, default `StatusToClarify`).
- `ParentKey` / `ParentTitle` from `parent:` / `macro:` labels only;
  `ParentType` `macro`. The milestone is **never** a macro here (unlike
  `githubTask`).
- `Team`: first label with prefix `team::` (case-insensitive), trimmed.
- `Sprint`: iteration title when present, else milestone title (`Task` has
  no sprint id; the id lives on the project's mirrored `TrackerSprint`s).
- `TeamID`: the team name too (GitLab has no team id, §5 `SearchTeams`).
- `TrackerStatus`: `closed` when closed, else the first label that is a list
  label of the project's boards when the caller supplies that set, else
  `opened`. (The sync passes the list-label set it read once per run.)
- `Priority` medium (GitHub behaviour, no weight mapping). `Source` `gitlab`,
  `ExternalURL` `web_url`, `Creator` / `CreatorAvatar` from `author`,
  `Assignee` first assignee's username.

`FormatTaskID(projectID, key, rawID)`: `gl-<projectID>-<iid>` (or `gl-<iid>`
for the default project), stripping `#`, `gl-`. Two GitLab projects are two
Sectile projects, hence two `projectID`s: identities never collide (D12).

## 5. Adapter (`gitlab.go`)

```go
type GitlabAdapter struct {
    tracker.BaseTicketingSystem
    client *Client
    tiers  *gitlabTierCache // §6
}
```

Capabilities: `CapCreate, CapUpdate, CapDelete, CapSync, CapGet, CapComment,
CapLabels, CapAssign, CapTransition, CapSprint, CapSprintManage, CapTeam,
CapBoard, CapPullRequests, CapIncrementalSync`. No `CapEpic`.

`forProject(ctx, p)`: identical to `GithubAdapter.forProject` with tracker
`"gitlab"` (project from `p` or `tracker.Project(ctx)`, acting user from ctx,
error on a locked personal credential, server credential otherwise). Writer
methods that only receive a key (`Assign`, `Transition`, `SetSprint`,
`SetTeam`, `UpdateLabels`, `SearchAssignable`) resolve through the context
project, as GitHub's `UpdateLabels` does.

| Method | GitLab call |
| --- | --- |
| `SyncIssues` | `GET /projects/:p/issues?scope=all&state=all&order_by=updated_at` (+ `updated_after=now-UpdatedWithinMin` in UTC RFC 3339 when > 0), paged; board list labels read once to fill `TrackerStatus`. |
| `GetIssue` | `GET /projects/:p/issues/:iid`. |
| `CreateIssue` | `POST /projects/:p/issues` with `title`, `description`, `labels` (request labels + `#new` when no stage label + `team::<Team>` + `macro:`/`parent:` when `ParentKey`), `issue_type` when set, `assignee_ids` when `Assignee` resolves (§5.1), `milestone_id` when `Sprint` is a milestone id; an iteration sprint is set right after through GraphQL. `RequiredCreateFields` → empty slice, nil. |
| `UpdateIssue` | `PUT /projects/:p/issues/:iid` with `title`, `description`, `add_labels` = `Labels`, `remove_labels` = `RemovedLabels` + stale stage labels (every `#<stage>` other than the new one), `state_event` from `isFinishedStatus` (close / reopen), and a `TargetStatus` column move (§7). |
| `DeleteIssue` | `CloseOnly` or no delete permission → `PUT state_event=close`; else `DELETE /projects/:p/issues/:iid`; 403 on delete falls back to close. |
| `AddComment` / `GetComments` | `POST /projects/:p/issues/:iid/notes {body}`; `GET …/notes?sort=asc&order_by=created_at`, paged, `system == true` dropped. |
| `UpdateLabels` | `PUT …/issues/:iid` with `add_labels`, `remove_labels`. |
| `Assign` | `PUT …/issues/:iid {assignee_ids: [id]}`; `""` → `assignee_ids: [0]`. |
| `SearchAssignable` | `GET /projects/:p/members/all?query=…&per_page=limit` → `tracker.Person{ID: strconv(id), DisplayName: name, AvatarURL, Active: state=="active"}`. |
| `Transition` | Column move (§7). |
| `SetSprint` / `CreateSprint` / `UpdateSprint` / `DeleteSprint` / `ListSprints` | §6. |
| `SetTeam` | Read issue labels, `remove_labels` = every `team::*`, `add_labels` = `team::<name>` (name = `teamID`, which for GitLab is the team name). |
| `SearchTeams` | `GET /projects/:p/labels?search=team::`, paged, keep names starting with `team::` whose remainder contains the query (case-insensitive) → `TrackerTeam{ID: name, Name: name}`. |
| `TeamMembers` | `GET /projects/:p/members/all`, paged → `TeamMember{TeamID, AccountID: strconv(id), DisplayName: name, AvatarURL, Active: state=="active"}`. |
| `ListBoards` / `ListBoardColumns` / `ListStatuses` | §7. |
| `ListIssueTypes` | `[]string{"issue","incident","task","test_case"}`. |
| `ListEpics`, `SetParent` | Base: `ErrUnsupported` (`CapEpic` not declared). Macro writes go through labels in the db layer (§8). |
| `IssuePullRequests` | `GET /projects/:p/issues/:iid/related_merge_requests`, sorted by `created_at` ascending → `TaskPullRequest{URL: web_url, Branch: source_branch}`. |

### 5.1 Assignee resolution

`Assign(key, personID)`: a numeric `personID` is the GitLab user id; a
non-numeric one is a username, resolved with
`GET /projects/:p/members/all?query=<username>` (exact `username` match),
error `aucun membre GitLab « %s » dans le projet` otherwise. `CreateIssue`
applies the same resolution to `req.Assignee`.

## 6. Sprints (`gitlab_sprints.go`)

- **Sprint id codec**: `milestone:<numeric id>` and
  `iteration:<numeric id>` (the GraphQL gid
  `gid://gitlab/Iteration/<id>` is rebuilt when needed). `parseSprintID`
  refuses any other shape with
  `identifiant de sprint GitLab illisible : %q (attendu milestone:<id> ou iteration:<id>)`.
  `TrackerSprint.ID` carries the encoded value, which is what the board and
  `SetSprint` hand back, so no probe exists (FR-6).
- **Group path**: `path.Dir(projectPath)` when the path has a `/`; for a
  numeric id, `GET /projects/:id` → `namespace.full_path` when
  `namespace.kind == "group"`. A `user` namespace has no group: no iterations.
- **Premium probe** (`gitlabTierCache`): key `baseURL + "|" + groupPath`,
  value `{iterations bool, checkedAt}`; TTL 1 h; `sync.Mutex`. Probe =
  GraphQL `group(fullPath) { iterationCadences(first: 1) { nodes { id } } }`;
  GraphQL errors, `null` group, 403/404 → `false`. A transport error or 5xx is
  **not** cached (next call retries) and is treated as `false` for that call.
- **ListSprints** (board id ignored: GitLab sprints are not per board):
  project milestones `GET /projects/:p/milestones?include_ancestors=true`
  (paged; `state` `active`/`closed`; dates `start_date`/`due_date`), plus,
  when the probe says so, iterations over GraphQL
  `group(fullPath){ iterations(includeAncestors:true, first:100, after:$c) { nodes { id title state startDate dueDate iterationCadence { id automatic } } } }`.
  States: milestone `active` → `active` when today ∈ [start, due] or no
  dates, `future` when start > today, `closed` for `closed`; iteration
  `current` → `active`, `upcoming` → `future`, `closed` → `closed`. Iteration
  names: `title`, else `<cadence title> <start> to <due>` (automatic cadences
  often have none). Each sprint's name is prefixed nowhere; its kind is in the
  id and exposed to the UI through the id prefix (spec US5-1: the sprint list
  shows "Jalon" / "Itération" from it).
- **SetSprint(sprintID, keys)**: `""` → `milestone_id: 0` and, when
  iterations are available, GraphQL `issueSetIteration(iterationId: null)`;
  `milestone:<id>` → `PUT milestone_id`; `iteration:<id>` → iterations
  unavailable → `tracker.Unsupported("gitlab", CapSprint)` wrapped with
  `les itérations ne sont pas disponibles sur cette instance GitLab`; else
  `issueSetIteration(projectPath, iid, iterationId)`.
- **CreateSprint**: milestone `POST /projects/:p/milestones {title,
  start_date, due_date}` (dates `YYYY-MM-DD` in UTC). Iteration creation is
  **not implemented until spec O1 is decided**; keep the branch point in
  code as a single `createKind(req) == milestone` function so O1 lands in one
  place.
- **UpdateSprint**: milestone `PUT /projects/:p/milestones/:id` (`title`,
  `start_date`, `due_date`, `state_event` `close`/`activate` from
  `patch.State` `closed`/other); iteration → read its cadence
  (`iterationCadence.automatic`); automatic → error
  `l'itération %s est planifiée automatiquement par sa cadence : modifiez-la sur GitLab`;
  manual → GraphQL `updateIteration(input:{groupPath,id,title,startDate,dueDate})`.
  `patch.MoveOpenTo` → `SetSprint(target, open issues of the sprint)` before
  closing, reading issues with `milestone_id` / `iteration_id` filters.
- **DeleteSprint**: milestone `DELETE /projects/:p/milestones/:id`, 404 →
  nil; iteration: automatic → same refusal; manual → GraphQL
  `iterationDelete(input:{id})`, not-found → nil.

## 7. Boards, columns, statuses (`gitlab_boards.go`)

- `ListBoards`: `GET /projects/:p/boards` → `TrackerBoard{ID: strconv(id),
  Name, Type: "gitlab"}`. Empty list → empty slice (the UI's "no board" path).
- `ListBoardColumns(boardID)`: `GET /projects/:p/boards/:id/lists`, sorted by
  `position` → `[{Name:"Open",Statuses:["opened"]}, {Name: label.name,
  Statuses:[label.name]}…, {Name:"Closed",Statuses:["closed"]}]`. Lists that
  are not label lists (assignee / milestone lists, Premium) are skipped.
- `ListStatuses`: `opened` (category `new`), every distinct list label of every
  project board (category `indeterminate`), `closed` (category `done`).
- **Column move** (`Transition(key, status)` and `UpdateIssue` with
  `TargetStatus`): compute the set `L` of list labels of all project boards.
  - `closed` → `state_event=close`, labels untouched.
  - `opened` → `remove_labels` = `L`, `state_event=reopen` when closed.
  - a label ∈ `L` → `remove_labels` = `L \ {label}`, `add_labels` = label,
    `state_event=reopen` when closed.
  - anything else → error `colonne GitLab inconnue : %q`.
  The `#<stage>` label is never in `L`'s arithmetic (D7). If a board uses a
  `#<stage>` label as a list, it is still only swapped by a move, which is the
  owner's configuration.

## 8. Database layer touch points

- **External URL** (`db.go` ~1120): `case "gitlab"`: when the task has no
  `ExternalURL`, build `<web base>/<projectPath>/-/issues/<iid>` from the
  API URL minus `/api/v4`. Normally unused, the adapter sets `web_url`.
- **Assignee push** (`db.go` ~3114): condition becomes
  `existing.Source == "jira" || existing.Source == "gitlab"`; for GitLab the
  op carries the username when no account id is known and `runAssignOp`
  passes it to `Assign` (§5.1) instead of failing with the Jira message.
- **Teams** (`teams.go` ~626): keep tasks whose project tracker supports
  `CapTeam` (Jira, GitLab); error `le champ Équipe n'existe pas sur ce tracker`.
- **Macros** (`macros.go`): `case task.Source == "gitlab"`: `ts.UpdateLabels`
  with `macro:<title>`, `parent:<key>`, removing the task's previous
  `macro:*` / `parent:*` labels; failure → notice kept locally, like Jira.
  `CreateMacro`: GitHub milestone only when `proj.IssueTracker == "github"`;
  a GitLab macro gets a local `M-n` key.
- **Same instance** (`trackerinstance.go`): `case "gitlab"`: same resolved
  GitLab API URL and same `gitlabProject` (case-insensitive), messages in
  French modelled on GitHub's.
- **Sync** needs no change: `EnqueueSyncWith`'s default branch creates
  `sync_gitlab` from the registry, the runner calls `SyncIssues`,
  `FormatTaskID` and `afterTrackerSync` (PR rediscovery via the
  `PullRequestDiscoverer` assertion). `trackerDisplayName` already spells
  "GitLab"; the per-tracker title in the sync runner is fixed to use it
  (`Gitlab` → `GitLab`).
- Verify, and fix only if broken: `macrohorizons.go` must skip GitLab macros
  (they are local), and every `Source == "github"` heuristic keyed on `#`
  keys stays behind an explicit `Source` check (GitLab tasks always carry
  `Source: "gitlab"`).

## 9. Test plan

`httptest.Server` fakes a GitLab instance at `/api/v4` and `/api/graphql`
with a recorded request log; the `Client` is built with `GitlabURL` pointing
at it (proves self-managed base URLs, FR-1).

- **Mapping**: stage labels → status for each stage, closed → finished, no
  label → new; `macro:` / `parent:`; `team::`; milestone vs iteration sprint;
  milestone never becomes a macro; `FormatTaskID` for two projects.
- **Sync**: three pages via `X-Next-Page`; `updated_after` present only with
  a window; project path with subgroup encoded as `%2F`; error body
  `{"message":"404 Project Not Found"}` surfaces in French without token.
- **Writes**: create payload (labels incl. `#new`, `team::`, `issue_type`,
  assignee resolved by username); update merges `add_labels` /
  `remove_labels` and strips stale stage labels; finish closes, unfinish
  reopens; delete falls back to close on 403; notes round trip, system notes
  filtered.
- **Boards**: columns order and Open/Closed; statuses; move list→list,
  list→Closed, Closed→list, →Open; unknown column refused.
- **Sprints**: list with Premium (milestones + iterations, kinds in ids,
  states); list on Free (GraphQL error → milestones only, no error); probe
  cached (one GraphQL call for two lists) and transport error not cached;
  `SetSprint` for each kind and for clear; iteration write on Free →
  `IsUnsupported`; malformed id refused; milestone create/update/delete, 404
  delete = nil; automatic-cadence iteration update/delete refused; manual
  iteration update.
- **Teams**: search filters `team::` labels; `SetTeam` swaps; members mapped.
- **Credentials**: sync (no acting user) uses the server token; acting user
  with a personal token uses it; locked personal token refuses; empty token →
  missing-credential message naming GitLab.
- **Registry**: `NewDefaultRegistry(...).Get("gitlab")` resolves and
  `Supports` matches FR-2; `IssuePullRequests` via the
  `PullRequestDiscoverer` assertion, oldest first.
- **Rate limit**: 429 → `IsRateLimited(err)`.
- **db**: `SetTasksTeam` accepts a GitLab task and refuses a GitHub one;
  `sameTrackerInstance` GitLab cases; macro attach queues label writes.
- **Web**: `trackers` unit test (GitLab in `PROJECT_TRACKERS` and personal
  trackers); `tsc --noEmit` and `oxlint` on `web/`.
- Full suite: `go test ./...` (outside the sandbox for `httptest`, GOCACHE in
  `$TMPDIR`), `gofmt -l`, `go vet ./...`, `node --test` for `web/`.
- Manual smoke (optional, needs a token): a throwaway gitlab.com project,
  **a copy** of the dev database, sync, create, comment, drag, close.

## 10. ADR

`docs/adrs/0029-gitlab-tracker-mapping.md` (number to check against
`origin/main`): Context (Taskativ's scoped-label mapping vs Sectile's
labels), Decision (D2 to D7 of the spec), Consequences (one mapping shared with
GitHub; `team::` the only scoped label; two sprint kinds in the id;
Premium probe with TTL; no epics), Rejected alternatives (`workflow::`
labels, group epics as macros, probing sprint kinds on write, milestones as
macros as on GitHub).

## 11. Risks

- GitLab's `iteration` field on REST issues and the GraphQL iteration API
  differ slightly between versions; the tests pin the shapes of GitLab 16+,
  and a missing field reads as "no iteration".
- `include_ancestors` on milestones exists since GitLab 13; older instances
  ignore it (project milestones only), which is acceptable.
- The `TrackerStatus` of imported tasks depends on the list-label set read at
  sync time; one extra call per sync, not per issue.
