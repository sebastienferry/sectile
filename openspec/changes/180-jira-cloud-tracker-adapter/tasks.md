# Tasks

## 1. Abstraction
- [x] 1.1 Add the read-side methods to `tracker.TicketingSystem` (`ListBoards`, `ListBoardColumns`, `ListSprints`, `ListStatuses`, `ListIssueTypes`, `ListEpics`, `SearchTeams`, `TeamMembers`, `RequiredCreateFields`) with their request types, `TrackerStatus` and `RequiredField`, and `CapBoard`; give `BaseTicketingSystem` `ErrUnsupported` defaults; extend `CapabilityLabel`.
- [x] 1.2 Add `ParentKey` and `Fields map[string]string` to `tracker.CreateIssueRequest`; verify `GithubAdapter` and `LocalAdapter` compile unchanged and `internal/tracker/tracker_test.go` still passes.

## 2. Client and credentials
- [x] 2.1 Extend `trackerapi.Credentials` and `Client` with `JiraURL`, `JiraEmail`, `JiraToken`; read `SECTILE_JIRA_URL`, `SECTILE_JIRA_EMAIL` and the token chain `SECTILE_JIRA_TOKEN` → `SECTILE_TRACKER_TOKEN` → `JIRA_API_TOKEN` in `NewClient`; apply them in `Client.For`.
- [x] 2.2 Add `Client.jira(ctx, method, path, query, payload, result)` with Basic auth, base-URL normalisation, Jira error-body extraction and a `RateLimited` error on 429; add `jiraAgilePages` (`startAt`/`isLast`) and `jiraSearchPages` (`nextPageToken`) with the repeated-page guard.
- [x] 2.3 Add `Client.CheckJira(ctx, siteURL, email, token)` against `GET /rest/api/3/myself`.
- [x] 2.4 In `internal/db/trackercredentials.go`, resolve Jira credentials (settings, then `Project.TrackerUrl`), fill `JiraAPITokenFromEnv`, add the `jira` cases to `CheckTrackerCredentials` (new `email` parameter) and `SaveTrackerCredentials`.

## 3. Adapter
- [x] 3.1 Create `internal/trackerapi/jira.go`: `JiraAdapter` embedding `BaseTicketingSystem` with capabilities create, update, delete, sync, get, comment, labels, assign, transition, sprint, team, epic, board; `FormatTaskID` → `jira-<projectID>-<KEY>`; register it in `NewDefaultRegistry`.
- [x] 3.2 Create `jira_mapping.go`: `jiraTask` (labels → status, then status category; priority by name; assignee account id; parent; sprint and team values; `TrackerStatus`; external URL `<site>/browse/<KEY>`), the JQL builder, and the `fields` list.
- [x] 3.3 Create `jira_fields.go`: sprint and team field discovery from `GET /rest/api/3/field` with schema then name matching, cached per base URL.
- [x] 3.4 Create `adf.go`: `MarkdownToADF` and `ADFToMarkdown` for the bounded subset, with table-driven tests covering headings, lists, code blocks, inline marks, links, mentions and unknown nodes.
- [x] 3.5 Implement `SyncIssues` (`search/jql`), `GetIssue`, `CreateIssue` (issue type fallback, parent, extra fields, ADF description), `UpdateIssue` (summary, description, labels merge with removals, priority, assignee, finished → done transition, `TargetStatus` → named transition), `DeleteIssue` (done transition, `CloseOnly` honoured), `AddComment` and `GetComments` (ADF both ways, created timestamps).
- [x] 3.6 Implement the `Writer` side: `Assign`, `SearchAssignable`, `Transition`, `UpdateLabels`, `SetParent`, `SetSprint` (batches of 50, backlog when empty), `SetTeam` (bare id, object retry, null to clear).
- [x] 3.7 Create `jira_teams.go`: cloud id from `/_edge/tenant_info`, gateway members pagination, `user/bulk` lookup, `SearchTeams` through JQL autocomplete; make member failures non-fatal.
- [x] 3.8 Implement the read side: `ListBoards`, `ListBoardColumns` (configuration + status names), `ListSprints`, `ListStatuses` (`project/{key}/statuses`), `ListIssueTypes` (createmeta), `ListEpics`, `RequiredCreateFields`.
- [x] 3.9 `httptest`-backed tests in `internal/trackerapi/jira_test.go`: Basic auth header, sync pagination and field mapping, create with fallback type and mandatory fields, label merge on update, finished → done transition and its "no transition" error, comments round trip, sprint batching, team members soft failure, credential resolution order, `FormatTaskID`, registry resolution of a `jira`-sourced task.

## 4. Database routing
- [x] 4.1 Remove the `sync_jira` case from `processSyncJob`; after a sync on an adapter supporting `CapTeam` / `CapBoard`, call `RefreshProjectTeamMembers` and `SyncProjectBoardColumns` when `Project.BoardID` is set.
- [x] 4.2 Route `ListProjectTrackerBoards`, `ListProjectIssueTypes`, `SyncProjectBoardColumns` (merge preserving hand-assigned statuses, hidden columns and sprint refresh), `SearchTrackerTeams`, `RefreshTeamMembersNow`, `RefreshProjectTeamMembers` and the Jira branch of `GetProjectStatuses` through the read side with capability checks.
- [x] 4.3 Replace the `isJira` / `isGithub` booleans of `enqueueTrackerUpdateUnsafe` with the resolved adapter's `Name()`; keep the activity text.
- [x] 4.4 Correct the `autosync.go` header comment (per-task `GetIssue`, no incremental JQL) and make `isRateLimited` recognise the `RateLimited` error.
- [x] 4.5 Tests in `internal/db`: sync job on a fake `jira` adapter imports tasks with `jira-<projectID>-<KEY>` ids and refreshes columns and teams; board and team stubs answer through the adapter; a `github` project still gets `ErrUnsupported` for boards.

## 5. Handlers and web
- [x] 5.1 `HandleSyncJira` enqueues a `sync_jira` job like `HandleSyncGithub`; `HandleTrackerSetup` checks and saves `jira` with the e-mail, ignoring `storeTokenInFile`.
- [x] 5.2 `TrackerSetup.tsx`: remove the file checkbox and payload field for every tracker; `SyncView.tsx`: replace the `acli` sentence with the REST wording; `AppContext.tsx`: drop `storeTokenInFile` from `TrackerCredentials`.
- [x] 5.3 Web tests for the setup screen (Jira fields shown, no checkbox) with the existing dependency-free test runner.

## 6. Cleanup and documentation
- [x] 6.1 Delete `jiraSearchFields` and the acli comment block in `internal/runner/runner.go`.
- [x] 6.2 Update `README.md`, `docs/CAPABILITIES.md`, `docs/API_AND_DATA_SPEC.md` (`/api/sync/jira`, Jira team refresh), `docs/ARCHITECTURE.md`, `docs/adrs/0006-independent-server-agent-runtimes.md` (Jira is supported over REST), and `desktop/README.md` where it repeats the limitation.
- [x] 6.3 Write ADR `0013-labels-carry-the-stage-on-jira.md`: labels as the stage carrier, transitions on close only, the rejected status-driven alternative.
- [x] 6.4 Update `.agents/MEMORY.md` §2 (the `jira` adapter, the read side, the ADF subset, the gateway teams path as a known soft dependency).
- [x] 6.5 Run `make test`, `gofmt` check and the web tests; record the output in the implementation report. Run `cmd/server/runtime_boundary_test.go` explicitly.

## 7. Personal tracker credentials (added during implementation)
- [x] 7.1 Add `internal/secrets`: AES-256-GCM sealing bound to `(user id, tracker)`, a server key from `SECTILE_SECRET_KEY` or a 0600 file beside the database, and an Argon2id passphrase mode; tests including a record moved between owners.
- [x] 7.2 Add `user_tracker_credentials` with the site, e-mail, encrypted record, sealed flag and salt, plus store, clear, unlock, lock and resolve.
- [x] 7.3 Carry the acting user in `context.Context` (`tracker.WithActingUser`), resolve the personal credential in `trackerapi.Client.ForActingUser`, and refuse a Jira operation a person asked for without their own token.
- [x] 7.4 Carry the acting user onto queued work: `SkillJob.ActingUser` for syncs and field updates, `TrackerOp.UserID` for operations, and put it back into the context in each worker.
- [x] 7.5 Add `/api/me/tracker-credentials` with its listing, store, delete, unlock and lock, guarded by the session; keep `/api/me` an exact public path.
- [x] 7.6 Add the Trackers tab with one activatable zone per tracker, sharing its form with the first-run connection screen.
- [x] 7.7 Start without the key, refusing only the operations that need it, and document the variable in the server image.
- [x] 7.8 Write ADR 0014 and update the README, the API specification and the project memory.
