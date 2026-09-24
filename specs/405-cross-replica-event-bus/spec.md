# #405: Cross-replica event bus for live board updates and cancellation

Parent macro: #397. Clarification: [`docs/clarifications/405.md`](../../docs/clarifications/405.md).

## Problem

A browser receives live updates (SSE) only for changes made through the server process
it is connected to. A cancellation only stops a job when it reaches the process running
it. And a job that finishes after being canceled overwrites `canceled` with its own
outcome.

## User stories

### US1 (P1): live updates whichever replica made the change

- **Given** a browser streams events from replica A,
  **when** a task changes through replica B (stage transition, post-back, run report),
  **then** the browser receives the `task_updated` event with the current task and
  activity.

### US2 (P1): a cancel stops the job where it runs

- **Given** a job runs on replica A,
  **when** a user cancels it through replica B,
  **then** replica A stops it, and its status stays `canceled`.

### US3 (P1): a canceled job stays canceled

- **Given** a job was canceled,
  **when** its execution ends or starts late,
  **then** its status is still `canceled`, on any engine and with one process too.

## Functional requirements

- **FR1** Every event a server sends to its browsers is also delivered by every other
  instance sharing the PostgreSQL database to its own browsers, except the per-chunk
  terminal output event.
- **FR2** A relayed event carries the task and activity as they are in the database when
  it is delivered.
- **FR3** An instance never delivers its own relayed events twice.
- **FR4** A cancellation reaches the instance holding the job, which stops it.
- **FR5** No job status update overwrites `canceled`.
- **FR6** With SQLite, delivery stays local and nothing else changes.
- **FR7** A lost listening connection is re-established without restarting the server.

## Out of scope

Agent and MCP routing (#406, #408); the web client's agent-status listener, which does
not receive named events (recorded for #406).

## Success criteria

A PostgreSQL test with two stores shows an event published by one delivered by the
other, a cancel from one stopping a job registered on the other, and no self-delivery;
SQLite tests show the status guard.
