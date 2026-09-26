# #439: A failed agent_launch rename leaves no active run behind

Follow-up of #407 (PR #433), under macro #397. Clarification:
[`docs/clarifications/439.md`](../../docs/clarifications/439.md).

## Problem

A queued stage skill is launched on the local agent by renaming its row to
`agent_launch`, which takes it out of the one-run index, then inserting the remote run.
When the rename fails on a database error, the final update that should close the
launch repeats the rename and fails too. The row stays `running` under its stage skill
and every later launch on the task is refused with 409 until the row is reclaimed.

## User stories

### US1 (P1): a launch that could not start frees its task

- **Given** a queued launch whose rename to `agent_launch` fails,
  **then** its activity ends `failed`, with the summary "Agent launch failed" and the
  database error as its reason,
  **and** a new launch on the same task is accepted at once.
- **Given** the close itself fails transiently,
  **then** it is retried a few times before giving up, and a final failure is logged
  with the activity id.
- **Given** the launch was canceled meanwhile,
  **then** it stays `canceled`.

## Functional requirements

- **FR1**: When the rename failed, the closing update sets the terminal status, summary,
  error and completion time without touching the skill id.
- **FR2**: The closing update is retried a bounded number of times with a short pause.
- **FR3**: A canceled activity is never overwritten.

## Acceptance criteria

- A failed rename ends the queued row in a terminal status (`failed`, with a readable
  reason), so it no longer matches the index predicate.
- A test forces the rename to fail and shows that the task accepts a new launch
  afterwards.

No CHANGELOG line: the case needs a database error, and what a user sees (an activity
that says why the launch failed) is the existing failure path.
