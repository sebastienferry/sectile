# #472: Plan

References: [`spec.md`](spec.md), [`tasks.md`](tasks.md),
[`docs/clarifications/472.md`](../../docs/clarifications/472.md).

Stack: Go server (`internal/db`, `internal/taskmcp`, `internal/trackerapi`).
No migration, no web or desktop change, no new dependency.

## Architecture

The creation already has every piece it needs; three links are missing.

```
MCP create_task ──(no caller)──▶ DB.CreateTask ──▶ CreateTaskAs
                                                   │  guard: source ∈ {local, github}   ← refuses Jira
                                                   │  CreateIssueRequest{Title, Description,
                                                   │                     Priority, Labels} ← drops IssueType, ParentKey
                                                   ▼
                                    JiraAdapter.CreateIssue (ADF, parent, type, priority,
                                    required fields) ──▶ forProject(ctx): caller's token or
                                                         server credential when nobody
```

After the change:

1. `create_task` and `get_task` put the MCP caller on the context with
   `tracker.WithActingUser`, and call `CreateTaskAs` / `GetTaskCommentsAs`.
2. `CreateTaskAs` replaces the source allow-list with a check on the resolved
   adapter, and forwards `IssueType` and `ParentKey`.
3. `JiraAdapter.CreateIssue` completes a refusal with the required fields.

`JiraAdapter.forProject` is unchanged: with a caller it already requires a
personal token (`no personal Jira token for this user: …`) and surfaces a locked
one as `secrets.ErrSealed` (`credential is sealed: its owner must unlock it`);
with no caller it uses the server credential (ADR 0028).

## Target files

| File | Change |
| --- | --- |
| `internal/taskmcp/server.go` | `create_task`: `CreateTaskAs(tracker.WithActingUser(ctx, callerOf(resolve, req).UserID), …)`. `get_task`: `GetTaskCommentsAs` with the same context. |
| `internal/db/db.go` | `CreateTaskAs`: remote-creation check (below), forward `IssueType` and `ParentKey`, tracker display name, `%w` wrapping. |
| `internal/trackerapi/jira.go` | `CreateIssue`: on an HTTP 400 from `POST /rest/api/3/issue`, append the missing required fields. |
| `internal/db/quick_add_test.go` | Keep the "no local row" assertion; the reason becomes a source/tracker mismatch, not Jira itself. |
| `internal/taskmcp/server_test.go` | Rework `TestCreateTaskFailsRatherThanFilingLocally`; add the Jira and caller tests. |
| `internal/trackerapi/jira_test.go` | Required-field completion on refusal; lookup failure keeps the original error. |
| `internal/mcptest/contract.go` | Unchanged: it calls the store's `GetTaskComments` directly, not the tool. |
| `CHANGELOG.md` | One `Fixed` line under `[Unreleased]`. |

## Data contracts

### MCP tools

Input schemas of `create_task` and `get_task` are unchanged. `create_task`
keeps its description; its errors change as below. `get_task` keeps
`commentsError` as a string field.

### `CreateTaskAs` remote-creation check (FR1, FR2)

Replaces `internal/db/db.go` (currently line 2548):

```go
ts, tsErr := d.TrackerForProject(proj)
remote := req.Source != "local"
if req.RequireRemoteCreation && remote {
    switch {
    case tsErr != nil:
        return nil, fmt.Errorf("cannot create the task on tracker %q: %w", req.Source, tsErr)
    case ts == nil || !strings.EqualFold(ts.Name(), req.Source):
        return nil, fmt.Errorf("cannot create the task on tracker %q: the project uses %q", req.Source, name(ts))
    case !ts.Supports(tracker.CapCreate):
        return nil, fmt.Errorf("cannot create the task on tracker %q: it does not support creation", req.Source)
    }
}
```

The single `TrackerForProject` call moves above the check and is reused by the
creation block. Without `RequireRemoteCreation` the existing behaviour is kept
exactly (the web board still falls back to a local row when no adapter
resolves).

Cases this covers, and the test that pins each:

| Project tracker | Request source | Result |
| --- | --- | --- |
| `local` | `local` (default) | local row (unchanged) |
| `github` | `github` | GitHub issue (unchanged) |
| `jira` | `jira` | Jira issue (**new**) |
| `gitlab` (no adapter registered) | `gitlab` | error, no row |
| `local` (fallback first project) | `jira` (explicit) | error, no row (`quick_add_test`) |

### `tracker.CreateIssueRequest` (FR3)

```go
tracker.CreateIssueRequest{
    Project: proj, Title: req.Title, Description: req.Description,
    Priority: req.Priority, Labels: req.Labels,
    IssueType: strings.TrimSpace(req.IssueType), // new
    ParentKey: strings.TrimSpace(req.ParentKey), // new
}
```

`Assignee` and `Sprint` are deliberately not forwarded: `CreateTaskRequest.Assignee`
is a display name on the web board while `JiraAdapter.assign` takes an account
id, and sprints are not part of this ticket. The GitHub adapter ignores both new
fields, so GitHub creation is unchanged.

### Error wrapping (FR6)

```go
return nil, fmt.Errorf("%s issue creation failed: %w", trackerTitle(ts.Name()), err)
```

`trackerTitle`: `github` → `GitHub`, `jira` → `Jira`, anything else as is. The
switch from `%v` to `%w` lets `errors.Is(err, secrets.ErrSealed)` hold end to
end. The empty-response branch uses the same title.

### Required fields on refusal (FR7, FR8)

In `JiraAdapter.CreateIssue`, around the `POST /rest/api/3/issue` call:

```go
if err := c.jira(ctx, http.MethodPost, "/rest/api/3/issue", nil, body, &created); err != nil {
    var httpErr *HTTPError
    if errors.As(err, &httpErr) && httpErr.Status == http.StatusBadRequest {
        if missing := j.missingRequiredFields(ctx, req, issueType); missing != "" {
            return nil, fmt.Errorf("%w; fields this project requires on creation for %s: %s", err, issueType, missing)
        }
    }
    return nil, err
}
```

`missingRequiredFields` calls `RequiredCreateFields` (same credential resolution
through `forProject`), drops the ids already present in `req.Fields`, and
formats `Name (id)` joined with `, `. Any error or an empty list returns `""`,
so Jira's refusal is returned as is. Only a failed creation pays the lookup.
`HTTPError.Status` is defined in `internal/trackerapi/client.go:228`.

### Caller resolution

`callerOf(resolve, req)` returns an empty `UserID` on a transport without
headers (the in-memory transport of the tests, `NewServer` without a resolver),
so `WithActingUser(ctx, "")` keeps today's no-caller behaviour there. The local
stdio bridge relays over HTTP with the device credential, which resolves to the
workstation's user, so an agent session is a named caller.

## Decisions

- **Check the adapter, not a list of names.** The allow-list was written before
  the Jira adapter was ported back; a capability check lets any future adapter
  with `CapCreate` work under the strict contract without touching this code.
- **Close the silent local fallback only under the strict contract.** The web
  board's lenient behaviour is out of scope and has no ticket asking to change.
- **Complete the refusal after the fact.** A pre-flight `createmeta` read would
  cost every creation two to three extra requests (FR8).
- **Reuse the Jira error text.** Error messages on this path are English today
  (`issue title is required`, `no personal Jira token …`); the new ones follow.

## Rejected alternatives

- **Add `jira` to the allow-list.** Fixes the symptom, keeps the silent local
  fallback for unresolved trackers, and still drops issue type and parent.
- **Create with the server credential from MCP.** Rejected by the owner
  (clarification Q1): the issue would carry the service account's name.
- **Fall back to the server credential for `get_task` when the caller has no
  token.** Rejected by the owner after #464 (clarification addendum): reads by a
  person follow the web detail view.
- **A `fields` input on `create_task`.** Deferred by the owner (Q3).

## Test plan

Unit and integration tests only, against `httptest` servers; nothing reaches a
real tracker.

1. `internal/db`: strict creation refused for a `gitlab` project and for a
   source/tracker mismatch, with zero rows; local project still created;
   `IssueType` and `ParentKey` reach a fake adapter registered in the registry.
2. `internal/taskmcp`: a Jira-backed project with a fake Jira site
   (`SECTILE_JIRA_URL`, `SECTILE_JIRA_EMAIL`, `SECTILE_JIRA_TOKEN` pointing at
   an `httptest` server, as `internal/trackerapi/jira_test.go` does) creates `PE-42`, returns key and browse URL, lands at
   `to_clarify` with one workflow label, and the POST body carries
   `issuetype`, `parent` and an ADF `description`.
3. `internal/taskmcp`: a server built with `NewServerWithCallers` and a
   resolver naming a user without a personal Jira token refuses with the
   personal-token message and writes no row; with a stored personal token the
   request to the fake site authenticates with that token.
4. `internal/taskmcp`: `get_task` with a named caller reads comments with the
   caller's token; without a personal token it returns the task and a
   `commentsError`.
5. `internal/trackerapi`: a 400 on creation with a mandatory field lists
   `Epic Type (customfield_10011)`; a failing `createmeta` returns Jira's
   refusal unchanged; a successful creation makes no `createmeta` request.
6. Full suites: `go test ./internal/...`.
