# ADR 0040: Consoles survive an agent restart, on the workstation

Status: Accepted. Amends ADR 0003 and ADR 0016 (the trace of an autonomous
run).

## Context

The desktop sidebar lists the executions the local agent supervises, and each
one replays what its console showed. ADR 0003 accepted that "a daemon restart
loses sessions and the in-memory run index", and ADR 0016 that the trace of an
autonomous run "is forgotten with the run". In practice the agent restarts
often: the desktop restarts it after an update, the person restarts it after a
settings change, the machine reboots. Each time the sidebar comes back empty,
and the conversation a run held, which is the only record of what the engine
said and what the person answered, is gone. Issue #588 asks for it to survive.

The run itself cannot survive: the engine process is a child of the agent's
PTY, and reattaching a PTY across processes is a different feature. What can
survive is what the sidebar shows.

## Decision

**The agent keeps a private copy of every run on the workstation, and reloads
it at start.** One JSON file per run, in `runs/` next to
`agent-connection.json` (`~/.taskflow/runs/` by default), holding the run
record the desktop lists, the console output (the last 64 KiB, the bound of
the PTY history) or the trace (the last 2000 lines), and when the run
finished. The directory is `0700`, the files `0600`, and every write goes
through a temporary file and a rename.

**Every kind of run is kept**: skill runs, headless runs, macro runs,
discussions and free consoles.

**A restored run is read-only.** It is listed with `restored: true` and no
session; its terminal route replays the stored output and ends the stream,
through the path the headless trace already uses. A conversation is not
continued: that would need each engine's own resume option and is out of
scope.

**A run killed with the agent comes back canceled.** A live run is written
every 5 s while it changes and at once when it finishes, so a crash loses at
most that period. At start, a run stored while `running` is restored as
`canceled`; a `queued` or `preparing` run had not started and is dropped.

**Retention is the user's gesture, with a cap.** "Clear finished consoles"
deletes the files it clears. Beyond 100 finished runs, the oldest finished run
is deleted first; a live run never is.

**Nothing moves to the server.** The console output stays on the workstation,
as ADR 0016 decided for the trace: no table, no migration, no upload.

## Consequences

- After a restart, the sidebar shows the same consoles, readable as they
  ended. The skill result is still read from the server, which already stores
  the activity.
- Console output can hold whatever the engine printed, secrets included. It
  was already in the agent's memory; it is now also on disk, readable by the
  user only. "Clear finished consoles" is the way to remove it.
- A desktop older than this change does not know `restored` and shows its
  "no console" notice for a restored interactive run, which is today's
  behaviour after a restart minus the empty list.
- A run dropped by the cap stays listed until the next restart: retention
  applies to the store, not to the list in front of the person.
- The run store is local to one workstation. The same task run from another
  machine shows that machine's consoles only.
