# Tasks #398 - Recreate GitLab tracker support

Ordered checklist. Each group is one commit (Conventional Commits). Before
group 1, `git fetch origin` and merge `origin/main` (do not rebase a pushed
branch). References: FR-n and US-n in [`spec.md`](spec.md), §n in
[`plan.md`](plan.md).

## 1. Client plumbing (§3, FR-1, FR-12)

- [ ] T1.1 `internal/trackerapi/gitlab_client.go`: `gitlab(...)`,
  `gitlabPages(...)` (`X-Next-Page`, 100-page cap), project path encoding,
  GraphQL endpoint derivation, missing / unreadable credential errors.
- [ ] T1.2 `gitlabErrorText` ported from Taskativ; French wrapping per status;
  429 keeps `*HTTPError` in the chain.
- [ ] T1.3 Tests: pagination over three pages; `%2F` subgroup encoding;
  self-managed base URL and GraphQL endpoint; error texts (string, list, map
  `message`); no token in any error; `IsRateLimited` on 429.

## 2. Mapping (§4, FR-3, FR-4, FR-5, FR-11)

- [ ] T2.1 Extract `statusFromStageLabels` from `githubTask` in `mapping.go`;
  GitHub tests unchanged and green (FR-13).
- [ ] T2.2 `gitlab_mapping.go`: `GitlabIssueItem`, `gitlabTask`,
  `FormatTaskID` (`gl-<project>-<iid>`).
- [ ] T2.3 Tests: each stage label, closed → finished, no label → new;
  `macro:` / `parent:`; milestone never a macro; `team::`; iteration beats
  milestone for `Sprint`; two projects give two ids for `#12`.

## 3. Adapter core: issues, notes, labels, assignee, MRs (§5, US2, US3)

- [ ] T3.1 `gitlab.go`: `GitlabAdapter`, capabilities (FR-2), `forProject`
  (D9), registration in `NewDefaultRegistry`.
- [ ] T3.2 `gitlab_issues.go` + adapter methods: `SyncIssues` (full and
  `updated_after`), `GetIssue`, `CreateIssue`, `UpdateIssue` (label merge,
  stale stage labels removed, close / reopen), `DeleteIssue` (close fallback),
  `AddComment`, `GetComments` (system notes dropped), `UpdateLabels`,
  `Assign` + username resolution (§5.1), `SearchAssignable`,
  `IssuePullRequests`, `ListIssueTypes`, `RequiredCreateFields`.
- [ ] T3.3 Tests: sync window present / absent; create payload; update
  merge and stage swap; finish closes, unfinish reopens; delete 403 → close;
  notes round trip; assign by id, by username, unassign; members search;
  related MRs oldest first; registry `Get("gitlab")` and capabilities;
  credentials (server for sync, personal when stored, locked refuses).

## 4. Boards, columns, statuses (§7, US4, FR-8, FR-9)

- [ ] T4.1 `gitlab_boards.go`: `ListBoards`, `ListBoardColumns` (Open, label
  lists by position, Closed; non-label lists skipped), `ListStatuses`.
- [ ] T4.2 Column move shared by `Transition` and `UpdateIssue.TargetStatus`;
  sync fills `TrackerStatus` from the list-label set read once per run.
- [ ] T4.3 Tests: column order; statuses; list → list, list → Closed,
  Closed → list, → Open; stage label untouched; unknown column refused;
  project with no board.

## 5. Sprints (§6, US5, FR-6, FR-7)

- [ ] T5.1 Sprint id codec (`milestone:` / `iteration:`), group path
  resolution (path, or `namespace.full_path` for a numeric id), Premium probe
  cache (TTL 1 h, transport errors not cached).
- [ ] T5.2 `ListSprints`: milestones (`include_ancestors`) + iterations when
  available; state mapping; iteration naming fallback.
- [ ] T5.3 `SetSprint`: clear, milestone, iteration; iteration on Free →
  unsupported; malformed id refused.
- [ ] T5.4 `SprintManager`: `CreateSprint` (milestones; iteration branch
  isolated in `createKind`, **blocked by spec O1**), `UpdateSprint`
  (milestone; manual iteration; automatic refused; `MoveOpenTo`),
  `DeleteSprint` (404 = deleted; automatic refused).
- [ ] T5.5 Tests: Premium list; Free list (no error); one probe for two
  lists; transport error retried; every `SetSprint` case; milestone CRUD;
  manual iteration update / delete; automatic cadence refusals.

## 6. Teams (US6, FR-5)

- [ ] T6.1 `SetTeam` (swap `team::*`), `SearchTeams` (project labels
  `team::`), `TeamMembers` (project members).
- [ ] T6.2 Tests: search filter; swap and clear; members mapping.

## 7. Database touch points (§8)

- [ ] T7.1 `teams.go`: `SetTasksTeam` gated on `CapTeam`, not on
  `Source == "jira"`; French message generalised.
- [ ] T7.2 `db.go`: assignee push for GitLab; `trackerops.go`
  `runAssignOp` username fallback for GitLab; external URL fallback;
  sync runner title uses `trackerDisplayName`.
- [ ] T7.3 `macros.go`: GitLab macro attach / create-under-macro writes
  `macro:` / `parent:` labels; `CreateMacro` GitHub milestone only for GitHub
  projects. `trackerinstance.go`: `gitlab` case (spec O2, GitHub's rule).
- [ ] T7.4 Verify `macrohorizons.go` skips GitLab macros and `#`-key
  heuristics stay behind an explicit `Source`; fix only if broken.
- [ ] T7.5 Tests (SQLite; PostgreSQL when `SECTILE_TEST_POSTGRES_DSN` points
  at a throwaway database): team write accepted on GitLab, refused on GitHub;
  same-instance cases; macro attach queues the label write; a
  `sync_gitlab` job imports through a fake adapter.

## 8. Web (US1)

- [ ] T8.1 `types/index.ts`: `IssueTracker` gains `'gitlab'`.
- [ ] T8.2 `lib/trackers.ts`: GitLab in `PROJECT_TRACKERS`, `hasAdapter`
  flag removed; code comments touched are rewritten in English (repository
  rule), user-visible strings stay in French.
- [ ] T8.3 `ProjectModal.tsx`: GitLab card, description, project path and
  instance URL inputs bound to `gitlabProject` / `gitlabUrl`, prefilled from
  settings; missing-credential warning as for GitHub.
- [ ] T8.4 `locales/translations.ts`: GitLab strings in French (and the other
  existing locales).
- [ ] T8.5 Sprint list shows "Jalon" / "Itération" for GitLab sprints from
  the id prefix (US5-1), only where the board lists sprints today.
- [ ] T8.6 Tests: `trackers` unit test; `tsc --noEmit`, `oxlint`,
  `node --test` in `web/` (symlink the main checkout's `node_modules` if the
  worktree has none, remove it afterwards).

## 9. Documentation (US7)

- [ ] T9.1 README: GitLab tracker setup (API URL, project path, PAT scope
  `api`, server vs personal token); remove "no GitLab adapter is registered".
- [ ] T9.2 `docs/CAPABILITIES.md`, `docs/API_AND_DATA_SPEC.md`
  (`issueTracker` values), `docs/ARCHITECTURE.md` (adapter list).
- [ ] T9.3 ADR `docs/adrs/0029-gitlab-tracker-mapping.md` (number checked
  against `origin/main`).
- [ ] T9.4 `CHANGELOG.md` `[Unreleased]` / `Added`: one line, for users, with
  `(#398)`.

## 10. Final checks

- [ ] T10.1 `gofmt -l .` empty, `go vet ./...`, `go test ./...` (outside
  the sandbox, `GOCACHE` under `$TMPDIR`); GitHub and Jira tests untouched.
- [ ] T10.2 Web checks green; `git status` clean (restore
  `internal/webui/dist/.gitkeep` if a build removed it).
- [ ] T10.3 Every FR and US acceptance mapped to at least one test or a
  documented manual check; open points O1 / O2 restated in the PR.
