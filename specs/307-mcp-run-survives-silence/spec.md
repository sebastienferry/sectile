# #307 — An active MCP run survives a long silence

- **Ticket**: [#307](https://github.com/sebastienferry/sectile/issues/307) · Bug · high
- **Branch**: `feat/307`
- **Clarification**: `docs/clarifications/307.md` (Round 2, owner-confirmed, zero open product question)

This file states behaviour only. Technical choices live in `plan.md`, the ordered
work in `tasks.md`.

## Problem

A client-to-server silence of fifteen minutes ends an MCP session. Every run that
session adopted is then canceled, the agent's later `finish_run` is refused with
*remote run not found or already finished with another status*, and — because a
hand-back enqueues nothing on a non-`completed` status — the whole autonomous
chain dies without a word. The fifteen minutes were harmless only while the stdio
bridge pinged inside the window; the recommended declaration is now a direct HTTP
endpoint with no keepalive, so any stage that compiles, tests, or waits for its
owner for a quarter of an hour loses its run.

## Users and value

- **An agent running a long skill** keeps its run and can report its real outcome.
- **A chain owner** sees the chain continue past a slow stage instead of stopping silently.
- **An operator on the board** can still tell a suspiciously quiet run from a busy one.
- **The owner of the three runs already canceled on 2026-09-21** can recover them.

## User stories, by priority

### US1 — A silent run is not killed (must)

As an agent that spent a long time without emitting an MCP call, I keep my run
open so that `finish_run` records the outcome I actually reached.

- **Given** a session that adopted a run through `start_run`,
  **when** no client-to-server message reaches the server for longer than the
  configured inactivity bound,
  **then** the run keeps status `running`, the session is not closed on that
  ground, and no cancellation is recorded.
- **Given** that same run,
  **when** the agent finally calls `finish_run` with `completed`,
  **then** the call succeeds and the run ends as `completed`.
- **Given** an autonomous run that is part of a full chain and went silent,
  **when** it later finishes as `completed`,
  **then** the hand-back enqueues the next stage exactly as for a run that never
  went silent.

### US2 — A long silence is visible without changing the verdict (must)

As an operator watching the board, I can see that a run has gone quiet, without
the board pretending it died.

- **Given** a run whose session exceeded the inactivity bound,
  **when** I read the run,
  **then** its status is still `running` and its summary carries one appended
  sentence naming the silence and its duration.
- **Given** the same run,
  **when** the silence continues past a second bound,
  **then** no further sentence is appended: the observation is recorded once per
  silent stretch.
- **Given** a run whose session went silent and then received a client message,
  **when** the session later goes silent again for longer than the bound,
  **then** a fresh sentence is appended for that new stretch.
- **Given** any silent run,
  **when** it is finished by its agent afterwards,
  **then** the reported note is appended to, not substituted for, the silence
  sentence, so both halves of the story remain readable.

### US3 — A run canceled by a disconnection is recoverable by its owner (must)

As the agent that owns a run a disconnection canceled, I can still report its real
outcome, which unblocks the chain instead of duplicating a step.

- **Given** a run with status `canceled` whose summary carries the disconnect
  note,
  **when** its owner calls `finish_run` with `completed`, `failed` or `canceled`,
  **then** the run takes the reported status and note, and the hand-back runs on
  that corrected status.
- **Given** the same run,
  **when** a user who is neither its owner nor an administrator calls
  `finish_run`,
  **then** the call is refused exactly as it is today for a run that belongs to
  someone else.
- **Given** a run an operator canceled from the user interface — a `canceled` run
  *without* the disconnect note,
  **when** its owner calls `finish_run`,
  **then** the call is refused and the run stays canceled: a deliberate human
  cancellation is final.
- **Given** a run already ended as `completed` or `failed`,
  **when** `finish_run` reports a different status,
  **then** the behaviour is unchanged from today: the mismatch is refused.
- **Given** a run recovered this way,
  **when** the hand-back replays,
  **then** at most one next step is enqueued for that run.

### US4 — Clean closures still cancel (must)

As the server, I still close the runs of a client that genuinely went away.

- **Given** an adopted run,
  **when** its session ends by an explicit termination (DELETE) or a broken
  connection,
  **then** the run is canceled immediately with the existing disconnect note, as
  today.
- **Given** an adopted run,
  **when** the server restarts,
  **then** the ADR 0007 sweep applies unchanged.
- **Given** a run reused through `SECTILE_RUN_ID`,
  **when** the session ends by any means,
  **then** the run is not adopted and therefore not closed, as today.

### US5 — The documented contract matches the code (must)

As a reader of the documentation, I am not told about a protection that no longer
exists.

- **Given** `README.md`, `docs/contracts/server-agent-v1.md` and `.env.sample`,
  **when** I look up `SECTILE_MCP_SESSION_TIMEOUT`,
  **then** they describe a bound that *marks* a silence rather than ending a
  session, state the four-hour default, and no longer justify it by a bridge
  keepalive.
- **Given** ADR 0007,
  **when** I read its consequences,
  **then** an amendment records that silence is no longer treated as proof of a
  dead client, the decision itself being unchanged.
- **Given** `CHANGELOG.md`,
  **when** I read the unreleased section,
  **then** the fix is listed.

## Functional requirements

| # | Requirement |
| --- | --- |
| FR1 | A client-to-server silence must never cancel an adopted run, whatever its duration. |
| FR2 | Sectile owns the inactivity bound; the transport must be given no session timeout of its own. |
| FR3 | Any client-to-server MCP message marks its session as active, whichever tool or method it invokes. |
| FR4 | Exceeding the bound appends exactly one sentence to each run the session adopted, naming the silence and its duration, and leaves status and every other field untouched. |
| FR5 | The sentence is emitted once per silent stretch; a later message rearms the observation. |
| FR6 | `SECTILE_MCP_SESSION_TIMEOUT` keeps its name and parsing rules and means "how long a silence goes unremarked". Its default is four hours. A non-positive or unparseable value keeps the default. |
| FR7 | `finish_run` accepts a run that is `canceled` **and** carries the disconnect note, when the caller is its owner or an administrator, and applies the reported status and note. |
| FR8 | A `canceled` run without that note stays unrewritable, as today. |
| FR9 | A rewrite replays the hand-back on the corrected status, and the run hands back at most once. |
| FR10 | Termination, connection loss, restart sweep, adoption rules and the `completed`/`failed`/`canceled` vocabulary are unchanged. |
| FR11 | The disconnect note is a single shared constant, not a literal duplicated between packages. |
| FR12 | No schema change: nothing here may require a SQLite or PostgreSQL migration. |

## Out of scope

Reviving the ADR 0012 "waiting for input" signal; reinstating the stdio bridge;
adding a status value, a column or a board indicator; enabling
`ServerOptions.KeepAlive`; changing adoption semantics; the ADR 0007 restart sweep.

## Open points

None blocking. One corroboration remains pending and does not gate delivery: the
ticket asks to confirm from the deployment's access logs that no POST reached
`/mcp` between `start_run` and the expiry. It needs access logs unavailable from
the worktree; the code path is conclusive on its own.
