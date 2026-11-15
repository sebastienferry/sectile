# Spec #908 - Tauri: show skill completion status in ticket conversations

Ticket: https://github.com/sebastienferry/sectile/issues/908
Clarification: `docs/clarifications/908.md` (rounds 1 to 4, settled)
Branch: `feat/908`
Reference behaviour: `specs/878-clear-skill-status/spec.md` (Electron client)

## Problem

When a ticket skill runs in Sectile Desktop (the Tauri client), the user cannot
tell from the conversation how the skill ended. The conversation status strip
shows the turn ("Responding", "Ready to respond") and `Execution: <run state>`
("Active session", "Completed", "Stopped"), which describe the process, not the
skill. The ticket workflow panel shows a coarse, technical line ("Skill
completed · stage validation pending", "Process completed · skill result
unconfirmed"). The sidebar conversation row shows a dot that says whether the
session is live, never whether the work is done. The Electron client already
reconciles these signals into one outcome-focused status (#878); the Tauri
client must say the same thing.

## Scope

In:

- The **conversation status strip** of a conversation whose run executes a
  ticket skill.
- The **ticket workflow panel**, the status line under the run heading.
- The **sidebar conversation row**: its indicator dot, its tooltip and its
  accessible name.
- `desktop-tauri/README.md` (skill status paragraph) and `CHANGELOG.md`
  `[Unreleased]`.

Out:

- The server, the local agent and its `/desktop/run-result` payload, which
  already carry every signal needed.
- The web UI, the Electron client, desktop notifications.
- The tickets pane rows, the execution queue, the legacy (non-conversation)
  run rows of the sidebar, the command palette.
- The left-hand turn indicator of the status strip ("Responding", "Ready to
  respond", "Your answer needed"): it describes the conversation turn and is
  unchanged.
- Free conversations (no ticket), discussions (`discuss`) and free consoles:
  they run no skill and keep their current display (US6).

## Vocabulary

As in `specs/878-clear-skill-status/spec.md`: **skill status** (called the
primary status there), **kinds** (Working, Needs your answer, Done, Needs
verification, Could not finish, Stopped, Stopping), **skill record**, **run
state**, **expected stage**, **successor**, **reason**, **diagnostics**. In
addition:

- **Ticket skill conversation**: a conversation whose run has a ticket and a
  skill other than `discuss`.
- **Pending question**: the conversation shows at least one question or
  approval waiting for the user's answer.

## User stories

### US1 - One skill status, computed once (P1)

- **US1.1** Given a ticket skill conversation, then the conversation status
  strip, the ticket workflow panel and the sidebar conversation row all show
  the same skill status at the same time: same kind, same label.
- **US1.2** Labels, details, next steps and reasons are the English wording of
  the Electron client, word for word: `Working · <skill>`, `Needs your answer`,
  `Done · <outcome>`, `Needs verification`, `Could not finish`, `Stopped`,
  `Stopping`, and the details listed in US2 to US5.
- **US1.3** Diagnostics read
  `Execution: <run state label> · Skill record: <status or "none"> · Task stage: <stage or "unknown">`;
  with a successor, the skill record reads
  `<launched skill> <status>; now <successor skill> <status>`.

### US2 - Precedence of conflicting signals (P1)

The table of `specs/878-clear-skill-status/spec.md` (US2) applies unchanged:
the first matching row wins, and "skill" means the successor when there is
one, otherwise the run's own skill record. Summary:

| # | Signals | Status | Detail / next step |
|---|---------|--------|--------------------|
| 1 | Stop requested, process not exited | Stopping | "Waiting for the session to exit" |
| 2 | Skill record `completed`, expected stage reached or no expected stage | Done · *outcome* | Next step (US3.4) |
| 3 | Skill record `completed`, expected stage not reached | Needs verification | "*Skill* reported completion; the task is still at *stage*" |
| 4 | Skill record `failed` | Could not finish | Reason (US4); "Open the console" while the run is live, else "Relaunch *skill*" |
| 5 | Skill record `canceled` | Stopped | Stop reason (US5) |
| 6 | Process running and a wait declared (US2.3) | Needs your answer | "Answer in the console" |
| 7 | Process queued, pending, preparing or running | Working · *skill* | "Queued" / "Preparing" / none; "No output for a while" when the skill record carries the silence sentence |
| 8 | Process completed, no skill verdict | Needs verification | "The session ended without reporting a result" |
| 9 | Process failed or canceled, no skill verdict | Could not finish / Stopped | "reason not recorded", or the stop reason (US5) |

- **US2.1** Each row is an acceptance criterion with its own test.
- **US2.2** Given a ticket skill conversation whose session stays open after
  the skill completed and the task reached its expected stage, then the status
  is Done (row 2), not Working.
- **US2.3** A wait is declared when the run carries a wait mark
  (`waitingSince`, a zero date such as `0001-01-01…` counting as no mark), when
  its successor carries one, or when the conversation has a pending question.
  It counts only while the process is running (row 6); a skill verdict (rows 2
  to 5) outranks it.
- **US2.4** Given a launch parked until its ticket is pinned to a repository
  (run status `waiting`, or skill record waiting reason `repository`), then
  the status is Needs your answer with the detail "Choose a repository for
  this ticket".
- **US2.5** A skill record of another execution (other id, task or skill) is
  ignored.
- **US2.6** Given no skill result read yet, or a read that failed, then the
  status is decided by the process alone (rows 1 and 6 to 9).

### US3 - Done names its outcome and the next step (P1)

- **US3.1** Done is shown only when the server recorded the skill as
  `completed` and, for a workflow skill, the task reached its expected stage.
- **US3.2** Outcomes: clarify "Clarified", specify "Specification ready",
  implement "Implemented", adjust and review "Review complete", handoff
  "Handed off"; a skill with no expected stage "reported by the skill".
- **US3.3** Given a pickup (`pickup`, `pickup_issues`) that completed, when the
  task moved forward from the stage it was at when the run started, then the
  status is "Done · Now *stage*"; otherwise, or when Desktop does not know the
  start stage, Needs verification with "Pickup reported completion; the task
  is at *stage*". The start stage is remembered across a restart of Desktop.
- **US3.4** Done carries a text hint `Next: <step>`, the label of the step the
  existing next-step button would launch, when the task is not finished and
  that step can be launched. It is text, not a button.

### US4 - Failures say why (P1)

- **US4.1** The reason is the first line of the recorded error, otherwise of
  the recorded note, shown as recorded, without the server's bookkeeping
  sentences (launch placeholder, silence sentence).
- **US4.2** The detail shows the reason cut to 120 characters with an ellipsis;
  the full reason is available in the strip's execution menu panel.
- **US4.3** No recorded reason: "reason not recorded". A local agent too old
  to send reasons: "reason not available".

### US5 - Stopped says why, honestly (P1)

- **US5.1** "Stopped by you" only when a stop was requested for this run from
  Desktop.
- **US5.2** "Session disconnected" only when the skill record says the server
  closed the run because its client disconnected.
- **US5.3** Otherwise the recorded reason, or "reason not recorded".

### US6 - Free conversations are unchanged (P1)

- **US6.1** Given a conversation with no ticket, a `discuss` conversation or a
  free console, then the strip still reads `Execution: <run state>` exactly as
  today, except "Stopping" while a requested stop has not taken effect; the
  sidebar row keeps its current dot and tooltip; no skill result is read.

### US7 - Successor skill in the same conversation (P2)

- **US7.1** Given a conversation where another skill started after the
  launched one ended, then the status speaks only for that successor,
  following the same precedence, and names it (`Working · Specify`).
- **US7.2** The launched skill's outcome stays in the diagnostics.
- **US7.3** The successor is noticed without waiting for the slower refresh:
  while the launched skill has ended and the session is still live, the result
  is read at every poll of the run list.

### US8 - Conversation status strip (P1)

- **US8.1** Given a ticket skill conversation, then the strip's execution menu
  summary shows the skill status in place of `Execution: <run state>`: the
  label, then the detail or the next step when there is one (for example
  `Done · Clarified · Next: Specify`, `Could not finish · git push rejected`).
- **US8.2** Opening that menu shows, above its existing actions, the run state
  (`Execution: <run state>` as shown today), the full reason when there is
  one, and the diagnostics line.
- **US8.3** The summary's tooltip carries the label, the detail, the next step
  and the diagnostics.
- **US8.4** The summary carries a visual mark of the kind (US10.1), so Needs
  your answer, Needs verification, Could not finish and Stopped are told apart
  at a glance.
- **US8.5** The left-hand turn indicator and the workflow toggle are unchanged.
- **US8.6** When the agent is unavailable, the summary reads "Status
  unavailable", as today.

### US9 - Ticket workflow panel (P1)

- **US9.1** The status line under the run heading shows the same skill status
  as the strip for the selected run: label, then detail or next step.
- **US9.2** The former wording ("Skill completed · stage validation pending",
  "Skill failure recorded", "Process completed · skill result unconfirmed",
  "Skill result pending", …) is no longer shown anywhere.
- **US9.3** Selecting another execution in the panel shows that execution's
  status.

### US10 - Sidebar conversation row (P1)

- **US10.1** Given a ticket skill conversation, then its indicator dot, in the
  same place and size as today, reflects the kind:

  | Kind | Indicator |
  |------|-----------|
  | Working | blue, pulsing (as today's live dot) |
  | Needs your answer | orange (as today's needs-input dot) |
  | Done | green dot |
  | Needs verification | amber, distinct shape from a dot |
  | Could not finish | red, distinct shape from a dot |
  | Stopped | grey, filled |
  | Stopping | grey, pulsing |

  The meaning never rests on colour alone: Needs verification and Could not
  finish have shapes of their own, and every kind is named in the tooltip and
  accessible name. The pulse stops under reduced motion, as today.
- **US10.2** The row's tooltip and accessible name end with the status label
  (for example `… · Done · Clarified`) in place of the plain activity word.
- **US10.3** Rows of free conversations keep today's indicator and tooltip.
- **US10.4** A sidebar with many rows does not multiply reads of the server:
  each run's result is read through the same throttled reader as the strip and
  the panel (FR3).

## Functional requirements

- **FR1** One pure, deterministic function computes the skill status from the
  run, the skill result, the pending-question flag, the next step label and
  the pickup start stage. The strip, the panel and the sidebar row render only
  its output; no other code decides a status.
- **FR2** The Tauri skill result type carries the fields the local agent
  already sends: `summary`, `error`, `disconnected`, `waitingReason`,
  `completedAt` on the run's record and on its successor, and `waitingSince`
  on the successor.
- **FR3** One reader per run, shared by every surface showing it. It reads the
  result when it has never been read, when the run's status or wait mark
  changes, when the conversation turn goes from busy to idle, at every poll
  during the 30 seconds after the run changed, at every poll while the run's
  own skill has ended and its session is live (US7.3), and otherwise every
  30 seconds for a live run and every 120 seconds for an ended one. A reader
  is released when no surface shows its run any more.
- **FR4** A read in flight is never doubled; a failed read keeps the last
  known result until the next due read.
- **FR5** No server, agent, Rust command or API change.
- **FR6** Selection, keyboard focus, the composer content and the transcript
  scroll position are preserved when a status changes.

## Acceptance criteria

- [ ] A ticket skill conversation shows the skill status in the strip summary,
      in the workflow panel and in the sidebar row, and the three agree (US1,
      US8, US9, US10).
- [ ] Every precedence row 1 to 9 has a passing unit test, as do US2.2 to
      US2.6 (US2).
- [ ] Done names the outcome and the next step, and a pickup shows
      "Done · Now *stage*" only when it moved the task forward (US3).
- [ ] Failed and stopped work show the recorded reason, or say it is not
      recorded or not available; "Stopped by you" and "Session disconnected"
      appear only on record (US4, US5).
- [ ] A successor skill owns the status, and the launched skill's outcome
      stays in the diagnostics (US7).
- [ ] The strip's execution menu shows the run state, the full reason and the
      diagnostics (US8.2).
- [ ] The sidebar dot is green for Done, keeps blue and orange for Working and
      Needs your answer, and gives Needs verification and Could not finish a
      shape of their own (US10).
- [ ] Free conversations, discussions and consoles look as they do today
      (US6).
- [ ] The refresh rule has unit tests (FR3), and UI tests cover the strip, the
      panel and the sidebar row.
- [ ] `desktop-tauri/README.md` describes the skill status; `CHANGELOG.md`
      `[Unreleased]` carries one `Added` line (US1).
- [ ] In `desktop-tauri`, `npm test`, `npm run test:ui` and `npm run check`
      pass.

## Assumptions (reversible, not product decisions left open)

- **A1** A pending question in the conversation counts as a declared wait
  (US2.3). The Electron client has no conversation questions; without this, a
  ticket conversation asking a question would read Working in the strip and
  turn the sidebar dot from orange to blue, a regression of today's dot.
- **A2** The skill name in `Working · <skill>` and in the details uses the
  Tauri client's existing skill labels ("Specify", "Run chain", …) and falls
  back to the skill id, as the rest of the Tauri client does, where Electron
  falls back to "Skill".

## Open points

None. Every product point was settled in the clarification (round 4,
2026-11-15).
