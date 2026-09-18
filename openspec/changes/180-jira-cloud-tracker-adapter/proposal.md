# A Jira Cloud tracker adapter

## Why
Sectile's tracker abstraction (`internal/tracker.TicketingSystem`) has two
registered adapters, `github` and `local`. A project configured with
`issueTracker: jira` carries a project key, a site URL, an e-mail and a
write-only API token in its settings, and the interface offers a Jira setup
screen and a Jira sync button, yet every Jira operation ends in "Le support de
Jira a été retiré" or in a stub returning nothing. The previous Jira integration
was deleted in commit `e6e16ec` because it lived on the workstation side of the
server/agent split and fell back on the `acli` command-line tool, which the
server may not execute. The configuration, models and UI it left behind promise
a tracker that does not exist.

Ticket #180 asks for that tracker to exist again, over the Jira REST API alone.
The clarification (`docs/clarifications/180.md`, rounds 1 and 2) settled the
scope: Jira Cloud with an Atlassian API token, labels as the carrier of the
Sectile workflow stage, a Markdown ↔ ADF conversion, and sprints, teams and
epics both read and written.

## What Changes
- A `jira` adapter implementing `tracker.TicketingSystem` is registered in
  `trackerapi.NewDefaultRegistry`, speaking Jira Cloud REST v3 and Agile 1.0
  over `net/http` with Basic authentication (`email:token`), through the shared
  `trackerapi.Client` and its per-project credential resolution.
- `tracker.TicketingSystem` gains a **read side**: boards, board columns,
  sprints, statuses, issue types, epics, team search and team members, each
  gated by a capability and defaulting to `ErrUnsupported` in
  `BaseTicketingSystem`. The database stubs that returned nothing for boards,
  issue types, board columns, team search and team refresh route through the
  resolved adapter.
- Import derives the Sectile status from the workflow labels, then from the
  Jira status category; finishing or deleting a task runs the first
  `done`-category transition of the issue's workflow.
- Descriptions and comments are converted between Markdown and Atlassian
  Document Format for a bounded subset of constructs.
- Jira credentials join the per-request resolution of change 149: project
  `TrackerUrl` / `JiraProject`, then the user configuration, then the
  environment (`SECTILE_JIRA_URL`, `SECTILE_JIRA_EMAIL`, `SECTILE_JIRA_TOKEN`,
  `SECTILE_TRACKER_TOKEN`, `JIRA_API_TOKEN`). The tracker setup screen checks a
  Jira token against `GET /rest/api/3/myself` before saving it, and loses the
  "store the token in a file" checkbox that nothing honours.
- The `sync_jira` refusal, the `HandleSyncJira` refusal, the Jira refusal in
  project status listing, the `acli` wording in the sync view, the
  `jiraSearchFields` constant and the "Jira is unsupported" sentences in the
  documentation are removed.

## Capabilities

### New Capabilities
- `jira-tracker`: a project tracked in Jira Cloud is synchronised, read,
  created, updated, commented, assigned, transitioned, moved between sprints,
  attached to a team and to an epic from Sectile, with its credentials
  configured and verified from the interface.
- `tracker-read-side`: a tracker exposes what its projects are made of
  (boards, columns, sprints, statuses, issue types, epics, teams and their
  members) through `TicketingSystem`, so the server never names a tracker to
  fetch them.
- `personal-tracker-credentials`: a tracker credential belongs to the person who
  uses it, stored encrypted and bound to its owner, optionally sealed behind a
  passphrase only they know, and carried to the tracker by whoever asked for the
  operation. Added during implementation, once it became clear that a shared
  token makes a whole team sign as one integration account on a tracker that
  attributes its writes.

### Modified Capabilities
None: no `openspec/specs/` entry covers the tracker abstraction yet.

## Out of scope
Jira Server / Data Center authentication (Bearer personal access tokens, REST
v2, wiki markup); a GitLab adapter; a per-project Jira e-mail and token pair
(the project keeps overriding only the site URL and the project key); a token
file outside the database; an incremental JQL read in the auto-sync loop (the
loop keeps its per-task `GetIssue`); pull-request and merge-request handling.

## Impact
- `internal/tracker/ticketing.go`, `tracker.go` (read-side methods and
  capabilities, request types).
- `internal/trackerapi/client.go` (Jira credentials, `jira` request helper,
  token pagination, `CheckJira`), new `jira.go`, `jira_mapping.go`,
  `jira_fields.go`, `jira_teams.go`, `adf.go` and their tests; `adapters.go`
  (registration).
- `internal/secrets` (new), `internal/db/usercredentials.go` (new),
  `internal/handlers/usercredentials.go` (new), and the acting user carried in
  `context.Context` through `internal/tracker`.
- `internal/db/trackercredentials.go`, `db.go` (`processSyncJob`,
  `EnqueueSync`, project status listing, `enqueueTrackerUpdateUnsafe` wording),
  `board.go`, `teams.go`, `autosync.go` header comment.
- `internal/handlers/handlers.go` (`HandleSyncJira`, `HandleTrackerSetup`).
- `internal/runner/runner.go` (dead constant).
- `web/src/components/TrackerSetup.tsx`, `SyncView.tsx`.
- `README.md`, `docs/CAPABILITIES.md`, `docs/API_AND_DATA_SPEC.md`,
  `docs/ARCHITECTURE.md`, `docs/adrs/0006-independent-server-agent-runtimes.md`,
  a new ADR, `.agents/MEMORY.md` §2.
- No SQLite migration: the `jira_*` settings columns and the projects'
  `jira_project` / `tracker_url` / `board_id` / `sprints` / `tracker_columns`
  columns exist.
