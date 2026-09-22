# Design

## Context
The clarification settled every product decision; this document records the
technical ones and the alternatives set aside. Two constraints frame everything:
the server reaches trackers over HTTP only (`.agents/MEMORY.md` §1,
`cmd/server/runtime_boundary_test.go`), and `internal/db` routes every tracker
operation through the resolved `TicketingSystem` rather than testing the
tracker's name (§2).

A reference implementation exists. Commit `e6e16ec` deleted
`internal/runner/jira_rest.go`, `jira_teams.go`, `comments.go` and
`internal/tracker/jira.go`, readable at `e6e16ec^`. They already cover Basic
auth, `GET /rest/api/3/search/jql` with `nextPageToken`, field discovery, board
and sprint listing, board configuration to columns, transitions by target
status, team members through the site gateway, user bulk lookup, assignable
search, set team, move to sprint, issue types, identity and creation with
mandatory fields. They are ported, not rewritten.

## Decisions

### One adapter in `internal/trackerapi`, on the shared `Client`
`JiraAdapter` embeds `tracker.BaseTicketingSystem` like `GithubAdapter` and
holds the `*Client`. Every request goes through a `jira(ctx, method, path,
query, payload, result)` helper on `Client` mirroring `github(...)`: the
`Authorization` header is `Basic base64(email:token)`, the redirect, size and
same-origin guards of `Client.request` apply, and a missing e-mail or token
answers "configure Jira credentials" before any network call. Paginated Agile
reads (`startAt` / `isLast`) and the platform search (`nextPageToken`) each get
a small loop with the repeated-page guard of `githubPages`.

Rejected: a separate `JiraRESTClient` type as before. It duplicated the HTTP
plumbing and cannot receive per-project credentials from `Client.For`.

### Credentials extend `trackerapi.Credentials`
`Credentials` and `Client` gain `JiraURL`, `JiraEmail`, `JiraToken`.
`db.trackerCredentials` fills them from `Settings.JiraUrl`, `JiraEmail`,
`JiraAPIToken`, then overrides the URL with `Project.TrackerUrl` when set.
`NewClient` reads `SECTILE_JIRA_URL`, `SECTILE_JIRA_EMAIL` and the token chain
`SECTILE_JIRA_TOKEN` → `SECTILE_TRACKER_TOKEN` → `JIRA_API_TOKEN` through
`trackerToken`. `withoutTrackerTokens` sets `JiraAPITokenFromEnv` from
`d.trackers.JiraToken`, which today is declared and never filled.

Rejected: a per-project Jira e-mail and token. One Atlassian API token is valid
on every site of the account, so the only per-project need, the site URL, is
already covered by `TrackerUrl`. Deferred, not refused.

### The site URL is the base URL; the API prefix is added per call
`Settings.JiraUrl` / `Project.TrackerUrl` hold `https://acme.atlassian.net`,
which the external-URL builder already suffixes with `/browse/<KEY>`. The
adapter appends `/rest/api/3/...`, `/rest/agile/1.0/...`, `/_edge/tenant_info`
or `/gateway/api/v4/...` itself, after trimming a trailing slash and any
`/rest/api/...` a user may have pasted (`normalizeJiraBaseURL` in the retired
code).

### Identity
`FormatTaskID(projectID, key, rawID)` returns `jira-<projectID>-<KEY>` with the
key upper-cased and trimmed, `jira-<KEY>` when the project is empty or
`default`. `task.Key` is the Jira key, `task.Source` is `jira`, `task.ID` is
left to `FormatTaskID` by the sync job, which already calls it.

### Status: labels first, category second, transitions on close
`jiraTask(item)` applies the same label switch as `githubTask`, then, when no
workflow label decided, maps `fields.status.statusCategory.key == "done"` to
`StatusFinished`. `TrackerStatus` receives the Jira status name so
`StageForTrackerStatus` and the board mapping keep working.

`UpdateIssue` with a finished status, and `DeleteIssue`, call
`GET /rest/api/3/issue/{key}/transitions`, pick the first transition whose
`to.statusCategory.key` is `done`, and `POST` it. No candidate is an error
carrying the key and the available transition names. `Transition(ctx, key,
status)` and `UpdateIssueRequest.TargetStatus` pick by `to.name` instead,
case-insensitively. A transition is never sent for a stage change that only
moves a label.

Rejected: a transition on every stage change through `TrackerStatusForStage`.
It forces every Jira project to map its columns before Sectile can move a
card, and it is what round 1 of the clarification ruled out; the mapping stays
an optional refinement the existing `board.go` helpers already implement.

### Search
`SyncIssues` runs `GET /rest/api/3/search/jql` with `jql = project = <KEY>`,
`AND issuetype IN ("A","B")` when `Project.IssueTypes` is set, `ORDER BY
updated DESC`; `fields` lists `summary,description,status,priority,assignee,
labels,issuetype,parent,created,updated` plus the discovered sprint and team
field ids; `maxResults=100`; the loop follows `nextPageToken` until `isLast`.
`GetIssue` reads `GET /rest/api/3/issue/{key}` with the same `fields`.

Rejected: `GET /rest/api/3/search` with `startAt`. Removed by Atlassian in
2025; it answers 410.

### Field discovery
`jira_fields.go` resolves the sprint and team custom field ids from
`GET /rest/api/3/field`: schema custom suffix `:gh-sprint`; schema type `team`
or suffix `:atlassian-team`; then by display name (`sprint`, `team`,
`équipe`). The result is cached per base URL on the `Client` copy for the life
of one operation and in a package-level `sync.Map` keyed by base URL across
operations, since field ids do not change within a site. A site with neither
field still syncs: sprint and team simply stay empty.

Sprint value parsing keeps the retired rule: several sprints on an issue, the
`active` one wins, else the last. Team value parsing accepts the object form
(`{id, name}`) and the bare id.

### Sprints, boards, columns
`ListBoards` → `GET /rest/agile/1.0/board?projectKeyOrId=<KEY>&type=scrum,kanban,simple`.
The three types are the ones exposing a column configuration; `simple` is what the
Agile API calls the board of a team-managed project, and leaving it out gave such a
project no board at all.
`ListSprints(boardID)` → `GET /rest/agile/1.0/board/{id}/sprint?state=active,future,closed`
paginated, mapped to `models.TrackerSprint` with `startDate` / `endDate`.
`ListBoardColumns(boardID)` → `GET /rest/agile/1.0/board/{id}/configuration`,
status ids resolved through `GET /rest/api/3/status`, empty columns dropped.
`SetSprint(sprintID, keys)` → `POST /rest/agile/1.0/sprint/{id}/issue` in
batches of 50, `POST /rest/agile/1.0/backlog/issue` when the id is empty.
`db.SyncProjectBoardColumns` merges the columns as its comment already
describes (preserve hand-assigned statuses and hidden columns), refreshes
`Project.Sprints`, and is called at the end of a Jira sync when
`Project.BoardID` is set.

### Teams
`SearchTeams(query)` → `GET /rest/api/3/jql/autocompletedata/suggestions?fieldName=Team&fieldValue=`,
`<b>` markup stripped. `TeamMembers(teamID)` → cloud id from
`GET /_edge/tenant_info`, then `GET /gateway/api/v4/teams/{id}/members?siteId=&first=50&after=`
(full members only, 20 pages at most), then `GET /rest/api/3/user/bulk?accountId=…`
by 40 for names, e-mails, avatars, activity. `SetTeam(key, teamID)` → `PUT
/rest/api/3/issue/{key}` with the team field set to the bare id, retried with
`{id}` when refused, `null` to clear.

The gateway path is not part of the documented Jira API. It is isolated in
`jira_teams.go`, and a failure there degrades to "members unknown" on the team
without failing the sync or the operation that triggered it. The
organisation-scoped public Teams API needs an org-admin token and is rejected
for that reason.

### Epics
`SetParent(key, parentKey)` → `PUT /rest/api/3/issue/{key}` with
`fields.parent = {key}` or `null`. Import fills `ParentKey`, `ParentTitle`
(`parent.fields.summary`) and `ParentType` (`parent.fields.issuetype.name`,
`Epic` in practice). `ListEpics` → `search/jql` with `project = <KEY> AND
issuetype = Epic`, returning tasks. `CreateIssue` accepts `IssueType` (else the
first of `Project.IssueTypes`, else `Task`), a parent key, and an extra
`Fields map[string]string` for instance-mandatory fields; `RequiredCreateFields`
is ported (`GET /rest/api/3/issue/createmeta/{key}/issuetypes/{id}`) and exposed
on the read side so the epic dialog can ask for them.

### Assign
`Assign(key, accountID)` → `PUT /rest/api/3/issue/{key}/assignee` with
`{accountId}` or `{accountId: null}`. `SearchAssignable` →
`GET /rest/api/3/user/assignable/search?issueKey=&query=&maxResults=`.
`UpdateIssue` with `Assignee` set forwards to `Assign`; the value is an
account id, never a display name, which `db.go:2430` already guarantees.

### Markdown ↔ ADF
`adf.go` converts a bounded subset both ways. Outbound `MarkdownToADF`:
paragraphs, headings 1-3, bullet and ordered lists (one level of nesting),
fenced code blocks with language, inline code, bold, italics, links; anything
else is emitted as a paragraph of text. Inbound `ADFToMarkdown`: the same
nodes, `mention` → `@displayName`, `hardBreak` → newline, unknown nodes
flattened to their text content. Both are pure functions with table-driven
tests; no third-party library.

Rejected: sending Markdown as plain text in a single paragraph, which Jira
accepts but renders reports as one block; REST v2 with wiki markup, which
needs a converter of the same size for a format Cloud is retiring.

### Read side of `TicketingSystem`
```go
ListBoards(ctx, req BoardsRequest) ([]models.TrackerBoard, error)
ListBoardColumns(ctx, req BoardRequest) ([]models.TrackerColumn, error)
ListSprints(ctx, req BoardRequest) ([]models.TrackerSprint, error)
ListStatuses(ctx, req ProjectRequest) ([]TrackerStatus, error)
ListIssueTypes(ctx, req ProjectRequest) ([]string, error)
ListEpics(ctx, req ProjectRequest) ([]models.Task, error)
SearchTeams(ctx, req TeamSearchRequest) ([]models.TrackerTeam, error)
TeamMembers(ctx, req TeamRequest) ([]models.TeamMember, error)
RequiredCreateFields(ctx, req CreateMetaRequest) ([]RequiredField, error)
```
Each request type carries `Project *models.Project` plus its key (`BoardID`,
`TeamID`, `Query`, `IssueType`). `BaseTicketingSystem` returns
`Unsupported(name, cap)` for all of them, with a new `CapBoard` for boards,
columns, statuses and issue types; sprints, epics and teams reuse `CapSprint`,
`CapEpic`, `CapTeam`. `GithubAdapter` and `LocalAdapter` change nothing and
keep compiling because they embed the base.

Rejected: optional interfaces asserted at the call site (`if r, ok :=
ts.(SprintReader)`). Consistent with Go idiom but not with this code base,
where every adapter embeds one base and callers ask `Supports`.

### Database routing
`ListProjectTrackerBoards`, `ListProjectIssueTypes`, `SyncProjectBoardColumns`,
`SearchTrackerTeams`, `RefreshTeamMembersNow`, `RefreshProjectTeamMembers` and
`GetProjectStatuses` resolve `d.TrackerForProject(proj)`, check the capability,
and call the read side. `processSyncJob` loses its `sync_jira` case: the
generic `sync_<tracker>` branch already handles any adapter with `CapSync`, and
the sync job's post-import step calls `RefreshProjectTeamMembers` and
`SyncProjectBoardColumns` when the adapter supports `CapTeam` / `CapBoard`.
`enqueueTrackerUpdateUnsafe` names the tracker from the resolved adapter's
`Name()` instead of `isJira` / `isGithub` booleans.

### Setup and verification
`CheckTrackerCredentials("jira", url, token)` needs the e-mail too:
`HandleTrackerSetup` passes `req.Email`, the signature gains an `email`
parameter used by the Jira case only. `CheckJira` → `GET /rest/api/3/myself`,
answers `displayName`. `SaveTrackerCredentials("jira", url, project, email,
token)` sets `JiraUrl`, `JiraEmail`, `JiraProject` (upper-cased) and
`JiraAPIToken` under the write-only rules. `TrackerSetup.tsx` drops
`storeInFile` and the `storeTokenInFile` payload field; the server ignores the
field if an old client still sends it.

### Errors
Jira answers refusals with a JSON body `{errorMessages, errors}`. The `jira`
helper appends the first message, truncated to 200 characters, to the
`tracker returned HTTP <code>` error, so an "Epic Type is required" refusal is
readable in the activity log. 429 answers surface a `RateLimited` error that
`autosync.isRateLimited` recognises.

## Risks
- The gateway teams endpoint may change without notice; its isolation and
  soft failure bound the damage to team membership.
- Field discovery on an instance with a custom "Team" field of another type
  may pick the wrong field; the name fallback runs after the schema match, and
  the sync log names the resolved ids.
- ADF coverage is a subset; a report using tables or images loses them in
  Jira. The converter is table-tested so extending it is local.

## Migration
None. Existing Jira-configured projects start working when a token is saved;
tasks previously imported by the retired implementation carried `Source:
"jira"` and Jira keys, so `Registry.ForTask` resolves them to the new adapter.
