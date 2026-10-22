# Plan #855 - A personal tracker credential signs only the ticket actions its owner performs

Behaviour: `spec.md`. Decisions: `docs/clarifications/855.md`. Line numbers
are those of `origin/main` at `a6a5c60a` and only locate the code; re-find them
before editing. PR #807 is readable at `refs/pull/807/head` (`a96fb06b`); fetch
it with `git fetch origin refs/pull/807/head:refs/remotes/origin/pr-807`.

## Stack and surfaces touched

- Go server: `internal/tracker`, `internal/trackerapi`, `internal/forgeoauth`
  (new, from #807), `internal/db`, `internal/taskmcp`, `internal/handlers`,
  `cmd/server/main.go`.
- One migration: `jira_oauth_flows.tracker` (#807's migration, renumbered to the
  next free version; 55 on `a6a5c60a`, re-check at implementation).
- Web: *Profile → Tracker credentials* connect panel and Administration OAuth
  app panel, generalised from the Jira ones (#807).
- No Desktop change.
- Docs: ADR 0058, ADR 0029 and 0044 status lines, `docs/API_AND_DATA_SPEC.md`,
  `docs/USER_GUIDE.md`, `.env.sample`, `CHANGELOG.md`.

## 1. The routing rule: `internal/tracker` + `internal/trackerapi`

### 1.1 A credential class on the context

Today the context carries an acting user (`tracker.WithActingUser`) and an
unattended marker (`tracker.WithUnattended`). Add a third, orthogonal marker:

```go
// internal/tracker/tracker.go
// WithServiceCredential marks a write Sectile makes by itself on behalf of the
// acting user: the activity keeps naming that user, the tracker call uses the
// server credential (ADR 0058).
func WithServiceCredential(ctx context.Context) context.Context
func ServiceCredential(ctx context.Context) bool
```

It does not clear the acting user: activities and `TrackerOp.UserID` still
record who caused the write.

### 1.2 `ForWrite` and a new `ForRead`

`trackerapi.Client.ForWrite` (`client.go:262`) becomes:

| Context | Personal credential | Result |
| --- | --- | --- |
| `ServiceCredential` set (user named or not) | ignored | server client, or `MissingServerCredentialError` |
| user named, no marker | usable | personal client |
| user named, no marker | none / sealed / disconnected / wrong site | `MissingPersonalCredentialError` (existing reasons, plus the forge site reason from #807) |
| no user, `Unattended` | - | server client, or `MissingServerCredentialError` |
| no user, no marker | - | `ErrNoActingUser` |

`MissingServerCredentialError{Tracker}` is new, typed, with a code
`tracker_server_credential_missing` (sibling of `CredentialMissingCode`), and
the message: `no server <Provider> credential: an administrator sets it in
Administration → Trackers`. "Missing" means the resolved server credential has
no token (GitHub/GitLab) or no token or e-mail (Jira); it is detected before any
HTTP request.

Add `(*Client).ForRead(trackerID string) *Client`, which is `For(trackerID)`
and nothing else, and use it on every read path (§3). A read with no server
credential goes out unauthenticated as today (US3); `ForRead` never errors.

`ForActingUser` stays, used only by `ForWrite` for the personal branch and by
the *Profile* check (§5). Its doc comment says so.

### 1.3 Adapters

- `GithubAdapter.forProject` (`adapters.go:93`), `GitlabAdapter`'s read
  resolver (`gitlab.go:62`) and `JiraAdapter.forProject` (`jira.go:51`) use
  `ForRead`. The Jira read refusal for a named user without a personal
  credential (`jira.go:61-67`) is removed: reads never need one.
- Their `forWrite` keep `ForWrite`; the class comes from the context.
- GitHub `transferIssue` (GraphQL, `github.go`) is a write: it goes through the
  write client.

## 2. Classifying every write in one place: `internal/db`

### 2.1 Queued operations

In `internal/db/trackerops.go`, a single table:

```go
// trackerOpRoute is the credential each operation kind uses (ADR 0058).
var trackerOpRoute = map[TrackerOpKind]credentialRoute{
    TrackerOpAssign: routeDirect, TrackerOpSetParent: routeDirect,
    TrackerOpMoveToEpic: routeDirect, TrackerOpTransition: routeDirect,
    TrackerOpSetTeam: routeDirect, TrackerOpSetSprint: routeDirect,
    TrackerOpStage: routeService,
    TrackerOpEpicHorizon: routeService, TrackerOpEpicLabels: routeService,
    TrackerOpPushHorizons: routeService, TrackerOpEpicPriority: routeService,
    TrackerOpEpicQuarter: routeService, TrackerOpEpicReadiness: routeService,
    TrackerOpEpicTodos: routeService, TrackerOpEpicFraming: routeService,
}
```

- `routeDirect`: personal, unless the enqueuing context is
  `ServiceCredential` (a skill run), then service.
- `routeService`: always service.
- `TrackerOp` gets `ServiceCredential bool`, set by `EnqueueTrackerOp` /
  `enqueueTrackerOpUnsafe` from the route and the context, persisted in the job
  payload with the other fields, and re-applied by the worker
  (`trackerops.go:392`) with `tracker.WithServiceCredential` next to
  `WithActingUser`.
- `TrackerOpMoveToEpic`: the epic created on the fly (title only) is created
  under `WithServiceCredential`; the ticket moves keep the op's class.
- A kind missing from the table fails `buildTrackerOpJob` (programming error),
  and a test iterates over every declared kind constant (parse the const block
  or keep an `allTrackerOpKinds` slice checked against the table both ways).

### 2.2 Direct write sites

Mark with `tracker.WithServiceCredential` at their entry point:

- stage transitions: `db/stage.go` builds the `TrackerOpStage` job: covered by
  the table; check the report comment (`runStageOp` → `AddTaskCommentAs`)
  inherits the marker from the worker context;
- managed run post-back (`db/postback.go`): the whole post-back context;
- macro lifecycle and mirrors on GitHub (`db/macros.go`: `UpdateMacro` 417,
  `CreateMacro` 1273, `DeleteMacro` 1329, `MigrateMacro` 1486 for the milestone
  writes; `db/macrotodosmirror.go:667` `writeMilestoneTodosBlock`;
  `db/sddslicing.go:357` todos mirror) and on Jira (macro create/update through
  `taskmcp/macros.go:72,81` → `CreateMacro` / `UpdateMacroMetadata`, epic
  issue writes);
- `MigrateMacro` / `MigrateTasks` issue transfers stay direct (`macros.go:1491`,
  `1758-1762`): they are ticket edits;
- `applyTaskMacro` (`macros.go:1069`, attach a ticket) and `createStoryUnder`
  (`macros.go:1223`, create a ticket) stay direct.

The full inventory (grep `trackerForWrite(`, `forWrite(`, `ForWrite(`,
`EnqueueTrackerOp`, `enqueueTrackerOpUnsafe`, `AddTaskCommentAs`,
`CreateTaskAs`) goes in a table test `TestEveryTrackerWriteSiteHasARoute` in
the spirit of #482's inventory: each site listed with its route; the test scans
the package sources for the call names and fails on an unlisted site.

### 2.3 Read sites

Switch to the server client:

- `db/board.go:116` `trackerClientAs` and `board.go:153` → `ForRead`; delete
  `trackerClientAs`;
- `db/trackercredentials.go:147` `trackerAs` → `ForRead`; keep the name only if
  callers read better, else inline `d.trackers.ForRead(ref)` (callers:
  `adjustment.go:310,322`);
- `db/prstates.go:24` → `ForRead`;
- `taskmcp/server.go:433` `GetTaskCommentsAs`: the acting user stays for the
  activity, the adapter's read client is the server's (§1.3).

## 3. Skill runs are service: `internal/taskmcp`

A helper in `taskmcp/server.go`:

```go
// writeContext is the context of a write tool: the caller as acting user, and
// the service marker when the call belongs to a skill run (ADR 0058): its
// request names a run (agentprotocol.RunIDHeader), or its session holds a run
// it started and has not finished (SessionRegistry.AdoptedRuns).
func writeContext(ctx context.Context, sessions *SessionRegistry, req *mcp.CallToolRequest, caller Caller) context.Context
```

Use it in every write tool: `create_task` (`server.go:729`), `update_task`,
`add_comment`, `transition_stage`, `create_macro` / `update_macro` /
`update_macro_todos` (`macros.go:72,81`, `server.go:629`). `requireCaller`
still runs first: an anonymous caller writes nothing (#482).

Check that `finish_run` releases the run from `AdoptedRuns` (`sessions.go:538`
`Release`) before the next call, and that a finished run's header on a launched
console still marks the call (a launched console only ever works for its run).

## 4. GitHub and GitLab OAuth: port from PR #807

Cherry-pick or re-apply, then adapt. Keep from #807:

- `internal/forgeoauth/oauth.go`, `forgeoauthtest/fake.go`, tests: authorize
  URL, code exchange, GitLab refresh, revocation. Scopes: GitHub `repo` only
  (drop `read:project`, used by ProjectsV2 reads that are now server reads);
  GitLab `api`. Re-derive both from the scope pin tests (§4.3).
- `internal/db/forgegrants.go` (+ tests): sealed grant under the server key,
  GitLab compare-and-set refresh on `version`, `invalid_grant` → disconnected,
  best-effort revoke on disconnect and on reconnect, account from `GET /user`.
- `internal/db/jiraoauthflows.go`, `jiragrants.go` generalisation, the
  migration adding `jira_oauth_flows.tracker` (renumbered).
- `internal/db/oauthapps.go`: one app per tracker, Administration first, then
  `SECTILE_{GITHUB,GITLAB}_OAUTH_CLIENT_ID/_CLIENT_SECRET/_REDIRECT_URL`;
  `handlers/jiraoauthapp.go` generalised to `/api/admin/{github,gitlab}-oauth`.
- `internal/handlers/forgeoauth.go`: `POST
  /api/me/tracker-credentials/{github,gitlab}/connect` (web session only),
  callbacks `/auth/{github,gitlab}/callback`; `cmd/server/main.go` wiring.
- `internal/db/usercredentials.go` / `trackercredentials.go`: list fields
  (`kind`, connected, disconnected) for the profile.
- `trackerapi/client.go`: `PersonalCredential.OAuth` / `OnUnauthorized`,
  the forge site gate (`api.github.com`, `gitlab.com`, https, default port) and
  the GitHub `401` → disconnect hook, **on the write client only**.
- Web: `TrackerConnectPanel.tsx`, `OAuthAppPanel.tsx`, `trackerOAuth.ts`,
  `oauthApp.ts`, `TrackerCredentialForm.tsx` / `TrackerCredentialsTab.tsx`
  changes, `AdminView.tsx`, translations (fr/en), web tests.

Drop from #807:

- the grant probe in `CheckTrackerCredentials` (`forgeGrantProbe`, the GitLab
  branch checking the caller's grant): see §5;
- any use of the grant on a read path, and the `401` hook on GraphQL read paths
  (board status, PR states): those are server reads now;
- ADR 0056 (number taken): rewritten as ADR 0058 (§7).

### 4.1 Conflicts to expect

`internal/db/migrations.go` (number), `trackerapi/client.go` (the routing of
§1.2 lands on the same functions), `web/src/context/AppContext.tsx`,
`translations.ts`, `CHANGELOG.md`. Port by hand where §1 changes the code
#807 touched; do not merge `origin/pr-807` as a branch.

### 4.2 Behaviour kept from #804's decisions

GitHub OAuth App (not a GitHub App); gitlab.com only; one personal credential
row per (user, tracker): a grant replaces a pasted token and vice versa; the
pasted token behind "Use a token instead"; no migration of existing tokens.

### 4.3 Scope pins

`trackerapi/github_scopes_test.go` and `gitlab_scopes_test.go` from #807,
restricted to the calls a **personal** client can make (the direct-route writes
of §2 and `GET /user`): issue create/update/delete, comments, labels,
assignees, milestone on an issue, `transferIssue`. The consent scopes must equal
the union; a new personal call without a listed scope fails the test.

## 5. Credential checks

- `CheckTrackerCredentials` (`trackercredentials.go:203`): remove every
  fallback to the caller's personal credential (GitHub `UserTrackerCredentialsFor`
  branch, Jira grant and token branches). With an empty token it checks the
  server credential, like `CheckServerTrackerCredentials` (`:381`); merge the two
  if nothing else differs. `recordTrackerAccount` is no longer called from it.
- With no server credential and no typed token, the check answers an error
  saying the provider has no server credential.
- The *Profile* check of one's own credential stays
  `ConfirmUserTrackerCredential` (`:284`, `handlers/usercredentials.go:68`),
  which now also handles a GitHub/GitLab grant (from #807).
- Verify which screens call `HandleTrackerSetup` `/check`
  (`handlers.go:2049`; web `TrackerCredentialForm.tsx:87` through
  `checkTrackerCredentials`): if the profile form uses it for a person's own
  pasted token, route that form to the `/api/me/tracker-credentials` check
  instead, so no screen ever checks a personal credential through the setup
  endpoint.

## 6. Missing server credential (US6)

- `MissingServerCredentialError` from `ForWrite` (§1.2).
- Stage op, macro writes and queued service ops already turn an error into a
  failed activity step with the local state kept (#482, FR-7 there); check the
  new error reaches that step with its message, and that REST/MCP direct calls
  that hit it answer `tracker_server_credential_missing` (HTTP 409, like
  `tracker_credential_missing`; re-check the status used there).
- Web: the activity step shows the message; no new screen.

## 7. Documentation

- `docs/adrs/0058-a-personal-tracker-credential-signs-only-direct-ticket-actions.md`:
  context (#804/#807, ADR 0029), decision (routing table of `spec.md`, skill
  runs, reads, OAuth for github.com and gitlab.com, the Administration check),
  consequences (service accounts need write access everywhere; private data
  only the person sees is no longer read; Jira included). Absorb what still
  holds of #807's ADR (OAuth app, refresh, revocation, site gate).
- ADR 0029 and ADR 0044: "Amended by ADR 0058" in their status line.
- `docs/API_AND_DATA_SPEC.md`, `docs/USER_GUIDE.md`, `.env.sample`,
  `CHANGELOG.md` per `spec.md` § Documentation acceptance.

## Test strategy

- Unit, `trackerapi`: `ForWrite` table (§1.2) × github/gitlab/jira, including
  grant disconnected, grant on the wrong site, server credential missing;
  `ForRead` never carries a personal token (fake server records the
  `Authorization` header); adapter reads with a person who has a personal token
  send the server token.
- Unit, `db`: the route table covers every `TrackerOp` kind both ways; the
  write-site inventory test; a queued direct op enqueued from a run context
  executes with the server token; stage op with a person holding a personal
  token posts label and comment with the server token; macro create/update on
  GitHub and Jira use the server token; `MoveToEpic` creates the epic with the
  server token and moves with the personal one.
- Unit, `taskmcp`: `add_comment` with no run → personal token; with the run
  header → server token; after `start_run` → server; after `finish_run` →
  personal again; anonymous caller refused.
- Unit, `handlers`: connect/callback (state single use, session-bound, TTL),
  admin OAuth app endpoints (admin only, secret never returned), setup check
  with empty token returns the server account even when the admin has a
  personal token/grant.
- Unit, `forgeoauth`, `forgegrants` (from #807), scope pins.
- Web: node tests from #807 adapted; `tsc`, `oxlint`.
- Gates: `gofmt`, `go vet ./...`, `go test ./...`, web `npm test`, `tsc`,
  `oxlint`.
- End-to-end (E3, after E1 and E2): connect GitHub and GitLab on the deployment,
  create/edit/comment as yourself (author is you), run a skill stage (author is
  the service account), let a GitLab token expire (refresh), disconnect.
