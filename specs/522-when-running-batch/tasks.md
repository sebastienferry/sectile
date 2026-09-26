# Tasks #522 - Batch membership and member states

Ordered. Each task names the requirements it covers and its tests. Server first,
then contract, then web, then documentation.

## Phase 1 - Server data

- [ ] **T1** Migration 30 `batch_members` in `internal/db/migrations.go`
  (table + `idx_batch_members_task`). Add `DROP TABLE batch_members` to
  `forgetSchemaVersion` and to the drop lists of `activerun_test.go` and
  `specartifacts_test.go`. (FR1, FR2)
  - Test: the migration applies on a fresh database and on a replayed one
    (existing `migrations_test.go` suites), SQLite and PostgreSQL.
- [ ] **T2** `models.TaskBatch`, `Task.Batch`, `RunSkillRequest.BatchTaskIDs`.
- [ ] **T3** `internal/db/batch.go`: `RecordBatch`, `MarkBatchMemberProcessing`,
  `ActiveBatchOf`, `ActiveBatchesByTask`. (FR1, FR4, FR5)
  - Tests (`batch_test.go`): positions and initial states; moving the marker
    marks the previous member done; reporting a done member makes it processing
    again; reporting the processing member is a no-op; a non-member returns
    false; `ActiveBatchOf` returns nil once the batch run is completed, failed
    or canceled (US4).
- [ ] **T4** Fill `Task.Batch` in `GetTasks` and `GetTaskByID` with one query per
  list. (FR2, FR12)
  - Test: a running batch fills every member, an ended one fills none.

## Phase 2 - Busy rule and launch

- [ ] **T5** `ActiveRunOnTask` falls back to the batch run of a running batch;
  `ActiveBusyCause` reports the batch; `TaskBusyError.Batch`. (FR8, FR11)
  - Tests: a member is busy while the batch runs and free after; the lead
    reports its batch; a queued skill on a member through
    `EnqueueSkillOnTaskWithOverrides` is refused.
- [ ] **T6** `describeActiveRun` batch message and `batchLeadKey` in the 409
  body. (FR8)
  - Test (handlers): launch on a member answers 409 with the message naming
    member and lead, and records nothing on the member (US3.1, US3.2).
- [ ] **T7** Batch launch in the `run-skill` handler: validation (FR3), busy
  members (FR10), `StartAgentRun` then `RecordBatch`, failure closes the run,
  `task_updated` per member. `Force` with `batchTaskIds` is 400.
  - Tests (handlers, fake agent dispatcher): accepted batch records members in
    order; one busy member refuses the whole launch and records nothing
    (US3.3); duplicate IDs, a single ID, a foreign project, a first ID different
    from the path task and a skill other than `pickup_issues` answer 400.
- [ ] **T8** "Launch anyway" on a member: owner or admin only, concurrent run,
  batch untouched. (FR9)
  - Test: owner accepted with `concurrent = 1`, another user refused, batch
    states unchanged (US3.4).

## Phase 3 - Member report

- [ ] **T9** `startRemoteRun` accepts a batch run ID on a member and marks it
  processing; notify both changed members. (FR5, FR6, FR11)
  - Tests: member accepted, returns the batch run, creates no activity; the
    previous member becomes done; a non-member keeps the "does not match"
    error; a finished batch run is refused; the MCP `start_run` over HTTP
    transport accepts it (see memory: tracker writes need an actor).
- [ ] **T10** `start_run` description in `internal/taskmcp/server.go`; batch
  sentence in `contracts/transition.md`; `UPDATE_GOLDEN=1 go test
  ./internal/skills/` and review that only batch lines changed. (FR19)

## Phase 4 - Web

- [ ] **T11** Types and launch: `TaskBatch`, `Task.batch`; `runSkill` sends
  `batchTaskIds`; `confirmBatchPickup` passes `taskIds`. (FR1)
  - Test: extend `web/tests/batch-pickup-context.browser.mjs` to assert the
    request body carries `batchTaskIds` in order.
- [ ] **T12** `web/src/lib/batchMembership.ts` `batchIndicator(task)`. (FR12,
  FR13, FR14)
  - Unit test `web/tests/batchMembership.test.mjs`: waiting → amber + waiting
    label; processing → indigo + processing label; done → no tone, no label;
    no batch → null.
- [ ] **T13** Labels in `translations.ts`, French and English. (FR18)
- [ ] **T14** `BatchBadge.tsx`, and its use in `TaskCard.tsx` (badge + border
  override), `ListView.tsx` and `TaskDetailModal.tsx`. (FR12-FR16)
  - Browser test `web/tests/batch-members.browser.mjs`: a board with a 3-member
    batch in each state shows the badges `Lot #10`, the labels and borders of
    US2.2; the lead card in state done has no indigo border while its
    `RemoteRunBadge` is still shown; a task without batch is unchanged (US1.5);
    list rows and the detail panel show the same badge and label.
  - Run `web/tests/condensed-card.browser.mjs` and `backlog-batch.browser.mjs`
    to check nothing regresses.

## Phase 5 - Documentation and checks

- [ ] **T15** ADR `docs/adrs/0034-batch-membership.md`; `docs/contracts/
  server-agent-v1.md`; `docs/CAPABILITIES.md`.
- [ ] **T16** `CHANGELOG.md`, `## [Unreleased]` → `### Fixed`: every ticket of a
  batch launched from the web now shows its batch and its progress, and cannot
  be launched again while the batch runs (#522).
- [ ] **T17** Full checks: `go test ./...` (with `SECTILE_TEST_POSTGRES_DSN` on
  a throwaway server), `go vet ./...`, `npm test`, `tsc` and `oxlint` in `web/`,
  the browser tests above.

## Manual verification

1. With a local agent connected, select three tickets on the web board and
   launch a batch: the three cards show `Lot #<first>`, the first `en cours`,
   the others `en attente dans le lot`.
2. Reload the page, open the list view and a detail panel: same indicators.
3. Try launching any skill on the second ticket: refused, the toast names the
   batch.
4. Let the agent move to the second ticket: first ticket shows the badge only,
   second shows `en cours`.
5. Cancel the batch run: every indicator disappears; a launch on the second
   ticket is accepted.
