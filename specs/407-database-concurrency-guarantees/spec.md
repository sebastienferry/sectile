# #407: Database guarantees instead of process-local locking

Parent macro: #397 (Multi-replica server). Clarification:
[`docs/clarifications/407.md`](../../docs/clarifications/407.md) (Rounds 1-4).
Depends on #403 (merged as #412); builds on #404 (#414) and #405 (#418).

## Problem

Every write that must not interleave with another is protected today by one mutex
in the server process. With one process that holds. With two server processes on
one PostgreSQL database it does not: two replicas can each read a task, change it
and write it back, and one change silently disappears. Pull-request links vanish,
run output loses chunks, two runs start on the same task, one project is
synchronized twice at once, a local task is turned into two tracker issues.

## User stories

### US1 (P1): nothing a user or agent records is lost when two replicas write at once

As a user of a multi-replica Sectile, what I, my agents and the synchronization
record on a task is all there afterwards, whichever replica served each request.

- **Given** two replicas append output to the same run at the same time,
  **then** the stored output contains every chunk, in the order each replica
  committed it, and the truncation notice appears at most once.
- **Given** two replicas attach different pull requests to one task at the same time
  (stage transition, pull-request discovery, state refresh, adjustment, post-back or
  task edit),
  **then** the task carries both links.
- **Given** two replicas append steps to one activity at the same time,
  **then** every step is kept.
- **Given** two users edit different fields of the same project, settings, user
  settings, saved view or macro at the same time,
  **then** both edits are kept. Two edits of the same field keep the last one.
- **Given** two replicas pin a task, bookmark a project or change team membership at
  the same time,
  **then** the result is one of the requested states, never a mix that neither
  request asked for.

### US2 (P1): stage transitions stay consistent

- **Given** two transitions of one task arrive on two replicas at the same time,
  **then** both apply one after the other, and the second sees the labels and links
  the first wrote.
- **Given** a managed run is active on a stage of the task,
  **when** transitions arrive on two replicas,
  **then** the running-stage rule refuses each of them exactly as on one replica.
- **Given** a transition to the stage the task already holds (to attach a follow-up
  pull request or post a note),
  **then** it is accepted, as today.

### US3 (P1): one ordinary run per task

- **Given** a task has a queued or running run,
  **when** anyone launches another ordinary run on it, from any replica, at the same
  moment or later,
  **then** the launch is refused with HTTP 409, the existing JSON body
  (`error`, `activeRunId`), and nothing is recorded for the refused launch.
- **Given** the active run is queued and has not started,
  **then** the refusal says it is queued instead of naming a start time.
- **Given** the owner of the active run, or an admin, chooses "Launch anyway",
  **then** the second run starts, as today.
- **Given** a CLI session declares its own run through MCP `start_run` without a
  launcher's run id,
  **then** it is never refused, as today.
- **Given** a full chain or a retry is enqueued on a busy task,
  **then** it is refused with the same 409 answer instead of the current 400.
- **Given** a chain step is refused because another run became active,
  **then** the chain stops and says so in the run summary, as today for any refused
  step.

### US4 (P1): one server-side worker per project across all replicas

- **Given** two replicas each hold a queued synchronization, field update or queued
  skill job for the same project,
  **then** they run one after the other, never at the same time. Tracker writes keep
  running without waiting, as today.

### US5 (P2): a local task becomes one tracker issue

- **Given** two replicas convert the same local task to a tracker issue at the same
  time,
  **then** exactly one issue is created, and the other request is refused.
- **Given** two replicas create local tasks in one project at the same time,
  **then** both are created with distinct keys and no error.

### US6 (P2): the single-replica deployment behaves as today

- **Given** a single SQLite or PostgreSQL server,
  **then** every behaviour above that exists today is unchanged, except that a queued
  run now makes a task busy (US3).

## Functional requirements

- **FR1** Every section of the store that the process mutex protects is classified,
  and each read-modify-write or read-check-write sequence is made correct across
  processes by the database itself, without depending on the mutex.
- **FR2** The classification is published in `docs/db-concurrency-audit.md`: one row
  per section, its class and what was done (converted, safe as is, covered by
  #404/#405, accepted with the reason).
- **FR3** Concurrent stage transitions of one task are serialized; the running-stage
  rule and the recomputation of labels, links and branch use the state written by
  the transition before.
- **FR4** At most one ordinary active run exists per task, enforced by the database.
  Active means queued, pending or running. A run is ordinary unless it was started
  with "Launch anyway", declared by a client through `start_run` without a
  launcher's run id, or reported by an agent without the server having created it.
- **FR5** The busy check counts every active run of the task, ordinary or not, queued
  included.
- **FR6** A refused ordinary launch, enqueue, full chain or retry answers HTTP 409
  with `{"error": <description>, "activeRunId": <id>}` and records nothing.
  `start_run` with a launcher's run id is unaffected.
- **FR7** Existing databases are upgraded without failing: tasks that already hold
  several active ordinary runs keep them all, the surplus being marked non-ordinary.
- **FR8** On PostgreSQL, at most one server-side job (synchronization, tracker field
  update, queued skill) runs per project across all replicas. A job waiting for its
  turn stays queued and holds no database connection while waiting. SQLite keeps the
  in-process rule.
- **FR9** A run that reached `completed`, `failed` or `canceled` is never moved back
  to `queued` or `running`. A terminal run may still change to another terminal
  status (ADR 0007, #307).
- **FR10** Converting a local task to a tracker issue creates at most one issue,
  whatever the number of concurrent requests.
- **FR11** No database lock is held while a tracker is called over the network.
- **FR12** Run output keeps at most 256 K characters, cut on a character boundary.

## Out of scope

SQLite with several processes (#403); cancellation relay and the job status guards
(#405, already merged); auto-sync claim and pacing (#404, already merged); agent and
MCP routing (#406, #408); keys unlocked by a passphrase, held in process memory
(#409); readiness probe, integration harness and the macro's ADRs (#410).

## Success criteria

- PostgreSQL tests opening two store handles on one database (two instance ids) and
  racing concurrent calls show: no lost run-output chunk, no lost pull-request link,
  no transition bypassing the running-stage rule, exactly one ordinary run per task,
  never two jobs of one project at once, one issue per conversion, distinct keys for
  concurrent local tasks, one default project.
- They run in the existing `test:postgres` CI job.
- The audit document lists all 137 sections.
- Every existing test passes, except those that insert two active ordinary runs on
  one task, which are rewritten to mark the extra run non-ordinary.
