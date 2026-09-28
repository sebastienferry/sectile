# Specification #594 - Five parallel executions by default

- Ticket: https://github.com/sebastienferry/sectile/issues/594
- Branch: `feat/594`
- Clarification: `docs/clarifications/594.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

A project that uses worktrees runs as many executions at a time as its
parallelism allows. When nobody set that number, neither for the project nor
in the workstation defaults, a project runs one execution at a time today.
With this change it runs up to five, on every workstation, with or without the
desktop app. A number someone set explicitly, including 1, keeps applying.

## Scope

In scope: the local agent's fallback for an unset parallelism, the values the
desktop settings show for it, the contract document and the changelog.

Out of scope:

- The upper bound, which stays 10.
- The rule that a project without worktrees runs one execution at a time.
- Any value already set explicitly, in the project section or in the
  workstation defaults.
- The server and the web interface, which neither store nor read parallelism.

## User stories

### US1 (P1) - An unset parallelism runs five executions

As a person who never touched the parallel executions setting, I get up to
five executions of a project at a time, and the sixth waits in the queue.

1. Given a workstation whose defaults and project section leave parallelism
   unset, and a project that uses worktrees, when six executions of that
   project are requested, then five run and the sixth is queued.
2. Given the same workstation and a project that does not use worktrees, when
   two executions are requested, then one runs and the other is queued.

### US2 (P1) - An explicit value wins

As a person who set a number, I keep the number I chose after the upgrade.

1. Given workstation defaults set to 1 and no project value, when the limit is
   computed, then it is 1.
2. Given a project value of 3 and workstation defaults unset, when the limit
   is computed, then it is 3.
3. Given workstation defaults set to 2 and no project value, when the limit is
   computed, then it is 2.

### US3 (P2) - The desktop shows the default

As a desktop user, I read in the settings what an unset value means.

1. Given workstation defaults with no parallelism, when the Execution defaults
   panel opens, then the slider reads "5 executions" and the hint reads
   "Default · 5 executions".
2. Given the same defaults, when the reset control of the parallel executions
   row is used after moving the slider, then the slider returns to 5.
3. Given a project inheriting the workstation value, when the project's
   Execution panel opens, then it shows the value the agent reports, 5 when
   nothing is set.

## Functional requirements

- FR1. When neither the project section nor the workstation defaults set a
  parallelism, the effective limit of a project that uses worktrees is 5.
- FR2. An explicit parallelism from 1 to 10 in the project section, else in
  the workstation defaults, is the effective limit, as today.
- FR3. A project without worktrees has an effective limit of 1, whatever the
  settings say.
- FR4. The desktop app and a headless agent compute the same limit from the
  same settings file.
- FR5. The desktop workstation panel shows 5 as the default value of parallel
  executions; the project panel shows the inherited value the agent reports.
- FR6. The contract document states the new default.
- FR7. `CHANGELOG.md` has one line under `Changed` in `[Unreleased]` saying
  that an unset parallelism now means 5, and that setting 1 restores one
  execution at a time.

## Acceptance criteria

- A workstation with no parallelism in its defaults nor in the project section
  runs up to 5 executions of a project that uses worktrees; the sixth queues.
- A project without worktrees still runs one execution at a time.
- An explicit value (1 to 10) in the defaults or the project section wins over
  the new default.
- The desktop workstation panel shows 5 as the default value and hint; the
  project panel shows the inherited 5.
- The contract document and the changelog state the new default.

## Open points

None. Both product questions were answered by the owner in round 2 of the
clarification.
