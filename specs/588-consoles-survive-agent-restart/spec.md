# Specification #588 - Consoles survive an agent restart

- Ticket: https://github.com/sebastienferry/sectile/issues/588
- Branch: `feat/588`
- Clarification: `docs/clarifications/588.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

The desktop sidebar lists the executions the local agent supervises, and each
one shows the console it held: what the engine printed, what the user typed,
and the result it reached. Today all of it lives in the agent's memory, so
restarting or updating the agent empties the sidebar. With this change the
agent keeps a private copy of every execution on the workstation and reloads
it when it starts: after a restart, the person finds the same consoles in the
sidebar and can read each one, read-only, as it was when it ended.

## Scope

In scope: interactive skill runs, headless runs and their trace, macro runs,
discussions and free consoles; the local agent's run index, its terminal
sessions' output and the headless trace; the desktop's console pane for a
restored run; "Clear finished consoles"; the ADR amending ADR 0003 and the
trace ADR 0016; the changelog.

Out of scope:

- Keeping an engine process alive across a restart, or reattaching a PTY.
- Continuing a restored conversation (engine resume options).
- Storing anything on the server: no table, no migration, no API change; the
  web board is unchanged.
- The task's workflow stage, which the server already stores.
- Archive and local names of runs, which the desktop already keeps in its
  `localStorage`.

## Definitions

- **Run**: one entry of the agent's run index (`/desktop/runs`), whatever its
  kind: skill run, headless run, macro run, discussion, free console.
- **Finished run**: a run whose status is `completed`, `failed` or `canceled`.
- **Live run**: a run whose status is `queued`, `preparing` or `running`.
- **Console output**: the bytes a run's terminal session printed, bounded as
  the terminal history is today (64 KiB, the most recent bytes kept).
- **Trace**: the rendered lines of a headless run's reasoning stream, bounded
  as today (2000 lines).
- **Run store**: the directory under the agent's state directory where runs
  are kept between two agent processes.
- **Restored run**: a run the agent loaded from the run store at start rather
  than launched itself.

## User stories (prioritised)

### US1 - The sidebar keeps its consoles after a restart (P1)

As a person using the desktop, when I restart or update the local agent, I
find the same executions in the sidebar, with their task, skill, status,
engine, branch and directory, instead of an empty list.

Acceptance:

1. **Given** an agent with finished runs of every kind (skill run, headless
   run, macro run, discussion, free console), **when** the agent restarts from
   the desktop, **then** `/desktop/runs` of the new process lists each of them
   with the same id, task, macro, project, skill, directory, branch, status,
   engine, provider, model, prompt and timestamps.
2. **Given** a restored run, **when** the desktop lists it, **then** it is
   marked `restored: true`, carries no `sessionId`, no `waitingSince` and no
   `cancelRequested`.
3. **Given** an agent started with no run store (the store directory cannot
   be created or read), **when** it starts, **then** it runs as today with an
   empty index and logs why the store is unavailable.

### US2 - A restored console shows what it showed (P1)

As a person, when I select a restored run, I read what its console showed when
it ended, in the same pane, and cannot type into it.

Acceptance:

1. **Given** a restored interactive run (skill run, macro run, discussion,
   free console), **when** the desktop attaches to it on
   `/desktop/terminal?id=<runId>`, **then** it receives the stored console
   output, the stream ends, no PTY is spawned, and any input it sends is
   discarded.
2. **Given** a restored headless run with a trace, **when** the desktop
   attaches to it, **then** it receives the stored trace lines and the stream
   ends.
3. **Given** a restored run, **when** the desktop shows it, **then** the pane
   attaches read-only (not focused) instead of showing the "No console is
   available" notice.
4. **Given** a restored run with a task, **when** the desktop asks for its
   skill result on `/desktop/run-result`, **then** the agent answers from the
   server as for any run it knows.
5. **Given** a desktop from before this change talking to a new agent, **when**
   it selects a restored interactive run, **then** it shows today's notice for
   a run with no console, and nothing breaks.

### US3 - A run killed with the agent comes back canceled (P2)

As a person, when the agent dies while a run is live (crash, `SIGTERM`,
reboot, update), I find that run canceled after the restart, with what its
console showed at the last write.

Acceptance:

1. **Given** a running run whose record and output were last written while it
   was `running`, **when** the agent starts, **then** the run is restored with
   status `canceled` and the output of that last write.
2. **Given** a `queued` or `preparing` run in the store, **when** the agent
   starts, **then** it is not restored and its file is deleted.
3. **Given** a live run, **when** it prints output, **then** the store holds
   that output at most a few seconds later (the flush period, 5 s), so a crash
   loses at most that period.
4. **Given** a live run, **when** it finishes, **then** its final status and
   output are written at once, not at the next flush.

### US4 - Clearing and relaunching stay the user's gestures (P2)

As a person, I decide when consoles go away, and a restored run can be
relaunched like any finished run.

Acceptance:

1. **Given** restored and finished runs, **when** I press "Clear finished
   consoles", **then** they leave the sidebar and their files leave the run
   store; live runs and their files are kept.
2. **Given** more than 100 finished runs in the store, **when** a run finishes
   or the agent starts, **then** the store keeps the 100 most recently
   finished and deletes the older files; a live run is never deleted to make
   room.
3. **Given** a restored finished skill run, **when** I press "Relaunch",
   **then** the skill is launched again for the same task, as for a finished
   run of the current process.

### US5 - The store is private to the user (P1)

As a person, what my consoles printed, secrets included, stays readable by me
only.

Acceptance:

1. **Given** the run store, **when** the agent creates it, **then** its
   directory has mode `0700` and each file mode `0600` (on Unix).
2. **Given** a write interrupted halfway, **when** the agent next starts,
   **then** it reads either the previous complete file or the new complete
   one, never a partial one (temporary file then rename).
3. **Given** a file in the store that cannot be parsed, or whose name is not a
   valid run id, **when** the agent starts, **then** it skips that file, logs
   it, and restores the others.

## Functional requirements

- **FR1** The agent keeps one file per run in `<state dir>/runs/`, where the
  state directory is the one holding `agent-connection.json` (`~/.taskflow/`
  by default). No file is written outside that directory.
- **FR2** A file carries the run record the desktop lists, the run's console
  output (bounded to 64 KiB) or trace (bounded to 2000 lines), and the time
  the run finished. It carries a format version; a file of an unknown version
  is skipped.
- **FR3** Every kind of run is stored: skill runs, headless runs, macro runs,
  discussions and free consoles.
- **FR4** A run's console output is captured independently of its terminal
  session, so it is kept even when the session closes before the run is
  stored (the shell exits, "Clear finished consoles" closes it).
- **FR5** A live run is written at most every 5 s while its output or status
  changed, and at once when it finishes. A graceful stop writes every run
  before the terminal sessions close.
- **FR6** At start, before the desktop can list runs, the agent loads the
  store: finished runs keep their status, `running` runs become `canceled`,
  `queued` and `preparing` runs are dropped with their file.
- **FR7** A restored run is marked `restored`, has no session, no waiting
  mark and no cancel request, and its terminal route replays the stored
  output or trace read-only and ends the stream.
- **FR8** "Clear finished consoles" deletes the files of the runs it removes.
- **FR9** The store keeps the 100 most recently finished runs; older files are
  deleted when a run finishes and at start. Live runs do not count and are
  never deleted by retention. A run dropped by retention stays listed until the
  agent restarts.
- **FR10** The store directory is `0700`, files are `0600`, and every write is
  atomic. A run id that is not a safe file name is never written or read.
- **FR11** A store failure (directory, write, read) is logged and never fails,
  delays or stops a run.
- **FR12** No server change: no new table, migration, MCP tool or API field.
  The desktop contract gains the optional `restored` field on `/desktop/runs`.
- **FR13** The desktop attaches read-only to a restored run, and keeps
  today's notice for an agent that does not send `restored`.
- **FR14** ADR 0040 records the decision and amends ADR 0003 and the trace
  ADR 0016; `CHANGELOG.md` gains one line under `[Unreleased]` / `Added`.

## Edge cases

- Two agents on one workstation: already refused by `localAgentAvailable`, so
  one process owns the store at a time.
- A run id reused by the server after a restart: the in-memory registration
  refuses an id already registered, restored runs included, as it refuses any
  duplicate today.
- The server reports a restored run as not owned: restored runs are finished,
  so they are not reported by `pull_tasks` and a cancel from the server gets
  the existing `RunNotOwned` answer.
- A stop request on a restored run finds it exited and answers as for any
  finished run.
- The console output holds the run control token typed by `agent-exec`; it is
  worthless once its run has ended, and the file is private (US5).
- A store written by a newer agent (unknown version) is skipped, not deleted.

## Success criteria

- After a desktop restart of the agent, every finished run of the previous
  process is listed and its console readable.
- `go test ./internal/agent/...`, `go vet ./...` and the desktop unit tests
  pass.
