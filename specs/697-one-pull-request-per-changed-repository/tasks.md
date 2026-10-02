# #697: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Shared model (FR1, FR2, FR3)

- [x] T1.1 `TaskPullRequest.Repository` and `MissingToken`; `PullRequestRepository`; `NormalizePullRequestLinks` fills the repository and keeps the mark.
- [x] T1.2 `AcceptRepositoryPullRequest`, `KeepPrimaryLast`, `AddPullRequestLink`.
- [x] T1.3 Table tests for the three.

## 2. Server write paths (US1, FR2)

- [x] T2.1 `taskPullRequestScope`; `encodePullRequestLinks` drops the repository.
- [x] T2.2 Adjustment prerequisite and evidence check per repository; primary kept last.
- [x] T2.3 Post-back and `PATCH prUrl` keep the primary last.
- [x] T2.4 Discovery per repository.
- [x] T2.5 Tests for T2.1 to T2.4.

## 3. `record_pull_request` (US1, FR4)

- [x] T3.1 `DB.RecordPullRequest`.
- [x] T3.2 MCP tool, bridge whitelist and count, `mcptest` catalog.
- [x] T3.3 Tests: appends, refuses an unrecognized URL and a foreign repository, leaves the stage alone.

## 4. Backfill and state marks (US3, US4, FR5, FR6)

- [x] T4.1 Secondary discovery in the sync pass.
- [x] T4.2 `HasForgeToken`; refresh marks `missingToken`, no warning, cleared on a read.
- [x] T4.3 Tests.

## 5. Agent and skills (US1)

- [x] T5.1 Adjust dispatch uses the per-repository guard.
- [x] T5.2 `adjust`, `create_pr` and pickup fragments; goldens regenerated.

## 6. Web and desktop (US2, US4)

- [x] T6.1 Web types, `repositoryPullRequests`, tooltip wording; tests.
- [x] T6.2 Task detail grouped by repository; board card `+N`.
- [x] T6.3 Desktop grouping helper and tests; toolbar indicators; sidebar and ticket list `+N`.

## 7. Documentation (FR7)

- [x] T7.1 `CHANGELOG.md` `Added` line; MCP tool listed where the docs list the catalog.

## 8. Verification

- [x] T8.1 `go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./...` (PostgreSQL DSN when available).
- [x] T8.2 Web `tsc`, `oxlint`, `node --test`; desktop `node --test`.
