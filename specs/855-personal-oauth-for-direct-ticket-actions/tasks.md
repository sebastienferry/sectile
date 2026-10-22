# Tasks #855 - A personal tracker credential signs only the ticket actions its owner performs

Ordered checklist. Each group is one commit (Conventional Commits). Before
starting, `git fetch origin` and integrate `origin/main` into `feat/855`; grep
`git log origin/main` for `(#855)`, `ForRead`, `WithServiceCredential` in case
part of it landed. Fetch PR #807 with
`git fetch origin refs/pull/807/head:refs/remotes/origin/pr-807`.

## 1. The routing rule (FR-1, FR-3, FR-5, FR-6, FR-7) - `feat(tracker)`

- [x] T1.1 `tracker.WithServiceCredential` / `tracker.ServiceCredential` in
  `internal/tracker/tracker.go`, with tests (keeps the acting user; nil context
  answers false).
- [x] T1.2 `MissingServerCredentialError{Tracker}` and its code
  `tracker_server_credential_missing`; `ForWrite` per plan §1.2; `ForRead`.
- [x] T1.3 `ForWrite` table test: every row of plan §1.2 × github, gitlab, jira.
- [x] T1.4 Adapters: GitHub, GitLab, Jira read resolvers on `ForRead`; remove
  the Jira read refusal; tests that a read by a person holding a personal token
  sends the server token.

## 2. Classify every write (FR-2, FR-8) - `feat(db)`

- [x] T2.1 `trackerOpRoute` table and `TrackerOp.ServiceCredential`, set at
  enqueue, persisted, re-applied by the worker; unknown kind fails the job build.
- [x] T2.2 Test: table covers every `TrackerOpKind` constant both ways.
- [x] T2.3 `MoveToEpic`: on-the-fly epic creation under the service marker.
- [x] T2.4 Mark the direct write sites of plan §2.2 (post-back, macro
  lifecycle and mirrors on GitHub and Jira); leave ticket transfers,
  `applyTaskMacro` and `createStoryUnder` direct.
- [x] T2.5 `TestEveryTrackerWriteSiteHasARoute` inventory test.
- [x] T2.6 Tests: stage op (label + comment) and macro writes use the server
  token for a person holding a personal one; a direct op from a run context
  uses the server token.

## 3. Reads on the server credential (FR-3) - `refactor(db)`

- [x] T3.1 `board.go`, `prstates.go`, `trackerAs` callers (`adjustment.go`) on
  `ForRead`; delete `trackerClientAs`.
- [x] T3.2 Tests: board statuses, PR states and comments read with the server
  token.

## 4. Skill runs (FR-4) - `feat(taskmcp)`

- [x] T4.1 `writeContext` in `taskmcp/server.go`; use it in `create_task`,
  `update_task`, `add_comment`, `transition_stage`, `create_macro`,
  `update_macro`, `update_macro_todos`.
- [x] T4.2 Tests: no run → personal; run header → server; after `start_run` →
  server; after `finish_run` → personal; anonymous caller refused.

## 5. Credential checks (FR-12, US5) - `fix(db)`

- [x] T5.1 `CheckTrackerCredentials`: no personal fallback on any provider;
  empty token checks the server credential; merge with
  `CheckServerTrackerCredentials` if equivalent.
- [x] T5.2 Verify which screens call the setup `/check`; route a profile
  check of a person's own token to `/api/me/tracker-credentials`.
- [x] T5.3 Handler test: an admin holding a personal token/grant re-checks the
  server credential with an empty token and gets the service account; no server
  credential → explicit error.

## 6. GitHub/GitLab OAuth, ported from PR #807 (FR-9, FR-10, FR-11, US4, US7) - `feat(oauth)`

- [x] T6.1 `internal/forgeoauth` + fake + tests; scopes GitHub `repo`, GitLab
  `api`.
- [x] T6.2 Migration `jira_oauth_flows.tracker` at the next free version;
  migration test and rewind fixtures.
- [x] T6.3 `oauthapps.go` per tracker and `/api/admin/{github,gitlab}-oauth`.
- [x] T6.4 `forgegrants.go` (sealed grant, GitLab CAS refresh, disconnect,
  revoke) + tests.
- [x] T6.5 Write client only: forge site gate, `OAuth` / `OnUnauthorized`,
  GitHub `401` → disconnect on REST writes and `transferIssue`.
- [x] T6.6 Connect and callback handlers, `cmd/server/main.go` wiring, profile
  list fields; handler tests.
- [x] T6.7 Scope pin tests limited to personal-client calls (plan §4.3).
- [x] T6.8 Web: connect panel, OAuth app panel, credential form/tab, admin
  view, fr/en strings, node tests.

## 7. Missing server credential surfaced (FR-6, US6) - `feat(db)`

- [x] T7.1 The error reaches the failed activity step with its message; REST
  and MCP answer `tracker_server_credential_missing`.
- [x] T7.2 Tests: stage transition with no server credential keeps the local
  stage and fails the activity; a direct action by a person with a personal
  credential still succeeds.

## 8. Documentation - `docs`

- [x] T8.1 ADR 0058; ADR 0029 and ADR 0044 status lines.
- [x] T8.2 `docs/API_AND_DATA_SPEC.md`, `docs/USER_GUIDE.md`, `.env.sample`.
- [x] T8.3 `CHANGELOG.md` `[Unreleased]`: Added, Changed (breaking), Fixed.

## 9. Gates

- [x] T9.1 `gofmt -l .` empty, `go vet ./...`, `go test ./...`.
- [x] T9.2 Web: `npm test`, `tsc --noEmit`, `oxlint`.

## Deviations from plan.md (implementation, 2026-10-22)

- **§1.2 / §6, server credential refused up front.** `ForWrite` refuses a
  write marked for the server credential when the deployment has none
  (`serverWriter`), because some macro writes swallow a failed call and keep
  the change locally: the typed error must reach them before any request.
- **§1.2, the marker needs an author.** A write marked for the server
  credential that names neither a person nor unattended work is still refused
  (`ErrNoActingUser`), so a queued stage that lost its author fails as before.
- **§2, skill runs past the request.** `Actor.ServiceCredential` and
  `SkillJob.ServiceCredential` carry a skill run's marker through
  `UpdateTaskBy`, `MoveTaskBy`, the pins and the queued field sync, which
  rebuilt their context from `context.Background()`.
- **§2, sprints and macro migration.** Sprint management (create, update,
  delete, move open work) is routed to the server credential: a sprint is a
  planning object, not a ticket. `MigrateMacro` is service as a whole, being
  part of the macro lifecycle; `MigrateTasks` stays a ticket action.
- **§5, the checks.** On main, Administration already checked the server
  credential only (`CheckServerTrackerCredentials`); `/api/setup/tracker/check`
  serves only the personal form. So the personal check was not stripped of the
  caller's credential, as §5 said, but made strictly personal: GitLab now
  checks the caller's own token or grant like GitHub and Jira, and a caller
  with neither is refused instead of shown the service account. The setup of a
  tracker's site keeps its server fallback and never probes a grant.
- **§4, PR #807 port.** Applied as one diff on top of the routing (its head
  already contained `a6a5c60a`); the migration kept version 55. The GitHub
  `401` hook stays on every GitHub call path, but only a personal client
  carries a grant, and reads never resolve one.
- **Web strings.** The new connection strings said "your writes are made under
  your account"; they now say tickets and comments.
- **spec.md US3.** A read without a server credential fails with the typed
  error; it never went out unauthenticated (the client refuses an empty token).

## External tasks (providers' administrators, before E3)

Reported at each stage like the code tasks.

- [ ] E1 Register the GitHub OAuth App under the organisation that owns the
  repositories, callback `https://sectile.internal.eqtv.dev/auth/github/callback`,
  scope `repo`; store its client id and secret in Administration (or
  `SECTILE_GITHUB_OAUTH_*`); have every organisation that enforces OAuth app
  access restrictions approve it.
- [ ] E2 Register the GitLab application on gitlab.com (group- or user-owned),
  callback `https://sectile.internal.eqtv.dev/auth/gitlab/callback`, scope
  `api`; store its client id and secret in Administration (or
  `SECTILE_GITLAB_OAUTH_*`).
- [ ] E2b Confirm the GitHub, GitLab and Jira server credentials have write
  access to every project Sectile serves (stage labels, comments, milestones,
  epics), and the GitHub one the scopes the ProjectsV2 board needs.
- [ ] E3 End-to-end on the deployment: connect GitHub and GitLab; create, edit
  and comment (author is you); run a skill stage (author is the service
  account); a GitLab refresh; disconnect.
