# #472: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Store: strict remote creation (FR1, FR2, FR3, FR6)

- [ ] T1.1 In `DB.CreateTaskAs`, resolve `TrackerForProject` once, before the
  status defaults, and replace the `local`/`github` allow-list with the
  adapter check of the plan (unresolved, mismatched, or no `CapCreate`).
- [ ] T1.2 Forward `IssueType` and `ParentKey` (trimmed) in
  `tracker.CreateIssueRequest`. Leave `Assignee` and `Sprint` out.
- [ ] T1.3 Wrap tracker errors as `<Tracker> issue creation failed: %w`, with
  `trackerTitle` for `github` and `jira`, in both the error and the
  empty-response branches.
- [ ] T1.4 Tests in `internal/db`: rework `quick_add_test.go` so its comment
  states the mismatch case; add a `gitlab` project refused with zero rows; add a
  fake adapter registered under a test name that records the request and
  assert `IssueType`, `ParentKey` reach it; assert `errors.Is(err, secrets.ErrSealed)`
  survives the wrapping with a fake adapter returning it.

## 2. Jira adapter: required fields on refusal (FR7, FR8)

- [ ] T2.1 `missingRequiredFields(ctx, req, issueType) string` in
  `internal/trackerapi/jira.go`, from `RequiredCreateFields`, minus the ids in
  `req.Fields`, formatted `Name (id)`, empty on any error.
- [ ] T2.2 In `CreateIssue`, on an `*HTTPError` with status 400 from the POST,
  append `; fields this project requires on creation for <type>: <list>` when
  the list is not empty.
- [ ] T2.3 Tests in `jira_test.go`: the existing
  `TestJiraCreateUsesTheTypeFallbackAndQuotesRefusals` also asserts
  `Epic Type (customfield_10011)` in the refusal; a `createmeta` that answers
  500 leaves Jira's refusal as is; a successful creation records no
  `createmeta` request.

## 3. MCP tools act as their caller (FR4, FR5)

- [ ] T3.1 `create_task`: build `ctx := tracker.WithActingUser(ctx, callerOf(resolve, req).UserID)`
  and call `database.CreateTaskAs(ctx, …)`.
- [ ] T3.2 `get_task`: `database.GetTaskCommentsAs` with the same context.
- [ ] T3.3 Rework `TestCreateTaskFailsRatherThanFilingLocally`: the refused
  project becomes one whose tracker cannot create (`gitlab`); the Jira case
  moves to the new tests below.
- [ ] T3.4 New test: Jira-backed project against an `httptest` Jira site
  configured through `SECTILE_JIRA_URL`, `SECTILE_JIRA_EMAIL`,
  `SECTILE_JIRA_TOKEN`; `create_task` with `issueType`, `parentKey`, Markdown
  description, priority and a label returns key `PE-42` and its browse URL;
  the POST body carries `issuetype.name`, `parent.key` and an ADF `description`;
  the reread task is `to_clarify` with one workflow label and the custom label.
- [ ] T3.5 New test with `NewServerWithCallers` and a resolver naming a user:
  without a personal Jira token, `create_task` fails with the personal-token
  message, the fake site receives no POST, and no row is written; with a stored
  personal token, the POST authenticates with that token.
- [ ] T3.6 New test: `get_task` with a named caller reads comments with the
  caller's token; without a personal token it returns the task and a
  `commentsError`.
- [ ] T3.7 The existing GitHub `create_task` test passes with its assertions
  unchanged.

## 4. Documentation and verification (FR9)

- [ ] T4.1 `CHANGELOG.md`, `[Unreleased]` › `Fixed`: agents can file tickets on
  Jira-backed projects through `create_task`, under the caller's own Jira
  account, with the issue type and parent epic they give; `get_task` shows a
  Jira ticket's comments. Reference `(#472)`.
- [ ] T4.2 `go vet ./...` and `go test ./internal/...` green; a failure of the
  handlers keepalive test is rerun alone before being attributed to this change.
- [ ] T4.3 Manual check, only after the owner re-enters their Jira token from
  Profil > Trackers: the ticket's repro on a scratch Jira project, then delete
  the created issue by hand. Never run a branch server on the dev database
  (test on a copy).
