# Specification #675 - A finished run deleted from the activities view stays deleted

- Ticket: https://github.com/sebastienferry/sectile/issues/675
- Branch: `feat/batch-675-690-693` (batch #675, #690, #693)
- Clarification: `docs/clarifications/675.md` (round 1, no product question)
- Framework: Spec Kit
- Type: bug

## Summary

A remote run that ended, then was deleted from the activities view or by
"clear completed activities", never comes back as `running` because the agent
that ran it keeps reporting it while its console is open. A run the agent
reports and the server never knew about is still recorded, as today.

## Scope

In scope: the server's record of the remote runs it deleted after they ended,
the agent report path that inserts unknown runs, the two deletion paths that
remove finished activities on request, their tests and the changelog.

Out of scope:

- Telling the agent that a run ended (agent protocol, #393 gap 4).
- A run deleted while it is still `running` or `queued`: an agent report
  recreates it, as today, because it is really running.
- Removing a whole task or project, which removes its activities with it.
- What Sectile Desktop shows while the console stays open.

## Vocabulary

- **Finished run**: a `remote_run` activity whose status is `completed`,
  `failed` or `canceled`.
- **Agent report**: one entry of an agent's `running_tasks` list, applied by
  `ApplyAgentRunningTasksFor` through `SyncRemoteRunStatusFor`.
- **Deletion record**: what the server keeps of a finished run it deleted.

## User stories

### US1 (P1) - A deleted finished run does not reappear

As an owner, I delete a finished run from the activities view while its
console is still open, and the board does not show it running again.

1. Given a launcher run adopted with `start_run` and closed with
   `finish_run(completed)`, when it is deleted and the agent then reports it
   as `running`, then no activity with that id exists.
2. Given the same run, when the agent reports it as `queued`, then no activity
   with that id exists.
3. Given several finished runs removed by "clear completed activities", when
   the agent reports one of them as `running`, then it is not recreated.

### US2 (P1) - A run the server never knew is still recorded

1. Given no activity and no deletion record for an id, when the agent reports
   it as `running`, then a `remote_run` activity is inserted as today: status
   `running`, concurrent, owned by the agent's user.

### US3 (P2) - A running run deleted is still running

1. Given a `remote_run` activity still `running`, when it is deleted and the
   agent reports it as `running`, then it is recreated as today.

## Functional requirements

- FR1. Deleting one finished run records its id with the time of deletion.
- FR2. "Clear completed activities" records the id of every finished run it
  removes.
- FR3. Deleting a run that is not finished, or an activity that is not a
  `remote_run`, records nothing.
- FR4. An agent report of an id that has a deletion record inserts nothing,
  returns no activity and no error, and broadcasts nothing.
- FR5. An agent report of an id with no row and no deletion record is inserted
  exactly as before.
- FR6. A deletion record older than 30 days is removed the next time a record
  is written.
- FR7. The record lives in the shared store, so it holds for every instance of
  a multi-replica deployment.
- FR8. `CHANGELOG.md` gets one `Fixed` line under `## [Unreleased]`.

## Acceptance criteria

- [ ] A test covers: launcher run finished, its row deleted, an agent report
  of the run as `running`. The run does not reappear.
- [ ] A run first reported by the agent, which the server never created, is
  still inserted.
- [ ] The same holds on SQLite and on PostgreSQL.
