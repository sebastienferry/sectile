# ADR 0034: A batch records its members on its run

Status: Accepted (#522)

## Context

A batch pickup launched from the web (`pickup_issues`) is one run, recorded on
the first ticket of the batch. The other tickets appeared only in the prompt the
agent receives, so the board had no way to tell that they belonged to a running
batch: they looked idle, and nothing stopped a second agent from being launched
on a ticket the batch was about to process.

The clarification (`docs/clarifications/522.md`) settled that every ticket of a
running batch shows the batch and where it stands in it (waiting, processing,
done), that those tickets are busy, and that every indicator ends with the batch
run.

## Decision

- **Membership is a table hanging on the batch run.** `batch_members` (migration
  30) holds `(run_id, task_id, position, state)`. The web sends the tickets, in
  order, as `batchTaskIds` on `run-skill`; the server validates them (at least
  two distinct tickets of one project, the first being the task the launch is
  made on) and records them right after the batch run, before the agent hears of
  it. A launch refused for any reason records no membership.
- **A batch is running exactly while its run is.** Every read joins the batch
  run and keeps only an active one (queued, pending or running). Rows are never
  deleted when the batch ends: nothing needs cleaning up, and nothing can be left
  behind by a cleanup that did not happen.
- **Membership is exposed on the task** (`Task.batch`), filled by the task list
  and by the single task read. The activity list the web reads is capped to its
  newest rows, and a long batch writes enough activities for its run to fall out
  of it while still running; tasks are always loaded whole.
- **The processing mark moves on an explicit report only.** The agent reports
  the ticket it starts working on by calling `start_run` on that ticket with the
  batch run's id. The call returns the batch run and creates no run. The ticket
  becomes processing and the previous one done. Stage transitions are not used:
  a batch records `reviewed` on every ticket at its end, which would move the
  mark backwards.
- **The busy rule covers members.** A ticket of a running batch is busy through
  the batch run: every launch path refuses it, and the refusal names the batch
  lead. "Launch anyway" follows the rule of any busy ticket (the batch run's owner
  or an admin) and records a concurrent run without touching the batch. A batch
  cannot take in a ticket that is already busy.

## Consequences

- The agent contract of `pickup_issues` changes: the launch run id is reused on
  every ticket of the batch, and `finish_run` is called once, on the lead.
- Batch runs recorded before this change carry no membership and keep showing on
  their lead only.
- **Accepted race.** Membership is not covered by the partial unique index that
  settles the one-active-run race (`idx_activities_one_active_run`). A single
  launch on a ticket and a batch launch that takes it in can cross in the few
  milliseconds between their checks and the membership insert, leaving the
  ticket with its own run inside the batch. Closing it would need a lock across
  tables on both engines for a window this narrow; it is left open.
- Only the web shows the indicators. The desktop receives the refusal like any
  other client.
