# #312 — Plan

## Stack

Go, package `internal/trackerapi`. No schema change, no migration, no interface or web change.

## Design

### D1 — `GithubAdapter.forProject` returns an error

`forProject(ctx, p) *Client` becomes `forProject(ctx, p) (*Client, error)`, the signature the Jira adapter already
uses. It returns the error of `Client.ForActingUser` as-is. `ForActingUser` already distinguishes the cases the spec
needs:

| Case | `ForActingUser` answers | `forProject` answers |
| --- | --- | --- |
| no actor | project client, no error | project client |
| actor, no personal token | project client, `personal == false` | project client (unlike Jira: decision 3) |
| actor, unlocked token | personal client | personal client |
| actor, locked token | `ErrCredentialLocked` | the error |

A locked credential surfaces from `db.UserTrackerCredentialsFor`, injected as `Client.ResolveUser`, as
`ErrCredentialLocked`, whose message already says the credential is sealed. The error is returned unwrapped, as the
Jira adapter does, so the activity shows the same text for both trackers.

### D2 — Every caller propagates it

The nine callers in `adapters.go` (`IssuePullRequests`, `CreateIssue`, `GetIssue`, `UpdateIssue`, `DeleteIssue`,
`SyncIssues`, `AddComment`, `GetComments`, `UpdateLabels`) take the error and return it before touching the client, so
no request reaches GitHub (FR-2).

### D3 — The background pass needs no change

`runAutoSyncPass` already queues under the owner and the activity already carries `user_id` (ADR 0018, covered by
`TestTheAutoSyncPassQueuesUnderTheProjectOwner`). A failing adapter call already turns into a failed activity. The
only missing piece is the adapter refusing.

### D4 — ADR 0018 amended

A consequence line records that the GitHub adapter refuses a locked credential like Jira, and that the fallback on the
project or server GitHub token is kept, on purpose, for an actor without a personal GitHub token and for an ownerless
project.

## Rejected alternatives

- **Refusing an actor without a personal GitHub token**, as Jira does: breaks every deployment running on
  `SECTILE_GITHUB_TOKEN` or a project token (decision 3).
- **Refusing only in background passes**: the context does not carry a "background" flag, and the same misattribution
  applies to a person's request (decision 4).

## Target files

- `internal/trackerapi/adapters.go`: `forProject` and its callers.
- `internal/trackerapi/adapters_test.go`: tests.
- `docs/adrs/0018-the-background-synchronisation-runs-as-the-project-owner.md`: consequence lines.
