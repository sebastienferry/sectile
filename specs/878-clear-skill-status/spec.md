# Spec #878 - Clear, outcome-focused skill status in Sectile Desktop

Ticket: https://github.com/sebastienferry/sectile/issues/878
Clarification: `docs/clarifications/878.md` (rounds 1 and 2, settled)
Branch: `feat/878`
Follow-up kept out: #879 (show the question a waiting skill asks)

## Problem

For a task's current execution, Sectile Desktop shows two indicators side by
side: the run state of the process ("Running", "Finished", "Stopped",
"Failed") and a skill badge ("Skill completed", "Skill stopped", "Skill
failed", "Awaiting stage validation", "Execution ended · skill completion
unconfirmed"). They answer different questions with overlapping words, can
disagree ("Running" beside "✓ Skill completed"), never say what was achieved,
and never say why something stopped or failed, although the server records it.

## Scope

In:

- The **task row** of the Desktop sidebar (the row of a task's current
  execution).
- The **task status bar** at the bottom of the execution view, and the
  conversation status row that hosts the same indicator in a conversation.
- The local agent's skill-result lookup, which must carry the recorded reason.
- `desktop/README.md` "Skill result indicator", `docs/USER_GUIDE.md` (link
  kept valid), `CHANGELOG.md` `[Unreleased]`.

Out:

- The web UI's activity badges, and `shared/runStates.ts`.
- Desktop notifications: they keep announcing run-state transitions.
- The sidebar execution queue and its wording, the tickets pane rows, the
  orchestration overview, the command palette entries.
- The question text of a wait (#879).
- How the server validates a stage, and the skills' own `finish_run` notes.
- Free consoles and discussions: they run no skill, so they have no skill
  status to merge; their rows keep showing the run state as today, plus
  "Stopping" while a requested stop has not taken effect.

## Vocabulary

- **Primary status**: the one status Desktop shows for a task's current
  execution. It has a kind, a label, an optional detail and an optional next
  step.
- **Kinds**: Working, Needs your answer, Done, Needs verification, Could not
  finish, Stopped, Stopping.
- **Skill record**: what the server recorded for the execution's skill
  (`running`, `completed`, `failed`, `canceled`) and its note or error.
- **Run state**: the state of the process, as the shared run states name it.
- **Expected stage**: the stage a workflow skill leaves its task on: clarify
  `clarified`, specify `specified`, implement `implemented`, adjust and review
  `reviewed`, handoff `finished`.
- **Successor**: another skill started later in the same open console (#586).
- **Reason**: the first line of the recorded error, or of the recorded note
  when there is no error.
- **Diagnostics**: the run state, the skill record and the task stage, shown
  verbatim.

## User stories

### US1 - One primary status (P1)

- **US1.1** Given a task row or the task status bar of a skill execution, then
  exactly one status is shown: its glyph (from its kind), its label and, on
  the status bar, its detail. The run-state glyph and the separate skill badge
  are no longer shown there.
- **US1.2** Given the task row, then its tooltip and accessible label carry the
  primary status, its detail, then the diagnostics.
- **US1.3** Given the task status bar, then a details line under the status
  carries the diagnostics.
- **US1.4** Diagnostics read
  `Execution: <run state label> · Skill record: <status or "none"> · Task stage: <stage>`,
  so nothing inspectable today is lost.
- **US1.5** Labels are in English, the language of the Desktop interface.

### US2 - Precedence of conflicting signals (P1)

The first matching row wins. "Skill" means the successor when one exists,
otherwise the execution's own skill record.

| # | Signals | Primary status | Detail / next step |
|---|---------|----------------|--------------------|
| 1 | A stop was requested and the process has not exited | Stopping | "Waiting for the session to exit" |
| 2 | Skill record `completed`, and the expected stage reached, or the skill has no expected stage | Done · *outcome* (US3) | Next step (US3.5); an open console does not change this |
| 3 | Skill record `completed`, expected stage not reached | Needs verification | "*Skill* reported completion; the task is still at *stage*" |
| 4 | Skill record `failed` | Could not finish | Reason (US4), next step "Open the console" or "Relaunch *skill*" |
| 5 | Skill record `canceled` | Stopped | Stop reason (US5) |
| 6 | Process running and a wait declared | Needs your answer | "Answer in the console" |
| 7 | Process queued, preparing or running | Working · *skill* | "Queued" / "Preparing" / none; "No output for a while" when the run is silent |
| 8 | Process exited normally with no skill record | Needs verification | "The session ended without reporting a result" |
| 9 | Process failed or canceled with no skill record | Could not finish / Stopped | Reason when known, else "reason not recorded" |

- **US2.1** Each row is an acceptance criterion and has its own test.
- **US2.2** Given a console still open after its skill completed and the stage
  reached (rows 2 and 7), then the status is Done, not Working.
- **US2.3** Given a process that exited normally without its skill reporting
  (row 8), then the status is Needs verification, never Done.
- **US2.4** Given a reported completion before the stage is validated (row 3),
  then the status is Needs verification, and becomes Done once the task
  reaches the expected stage.
- **US2.5** Given a launch parked until its ticket is pinned to a repository,
  then the status is Needs your answer with the detail "Choose a repository
  for this ticket".
- **US2.6** A skill record belonging to another execution (other id, task or
  skill) is ignored, as today.
- **US2.7** Row 9 is a deliberate change: a failed or canceled process with no
  skill record now shows a status, where it showed no skill badge.

### US3 - Done names its outcome (P1)

- **US3.1** Done is shown only when the server recorded the skill as
  `completed` and, for a workflow skill, the task reached its expected stage.
  A process exit or a reported completion alone never yields Done.
- **US3.2** The outcome names the stage reached: clarify "Clarified", specify
  "Specification ready", implement "Implemented", adjust and review "Review
  complete", handoff "Handed off".
- **US3.3** Done never says the task is finished unless the task's stage is
  `finished`.
- **US3.4** For a pickup (`pickup`, `pickup_issues`), the outcome is the stage
  the task is at when the pickup completes, when it moved forward during the
  run ("Done · Now implemented"). When it did not move forward, or when
  Desktop cannot tell the stage the run started from, the status is Needs
  verification: "Pickup reported completion; the task is at *stage*".
- **US3.5** For a skill with no stage (`create_pr`, `rewrite_story`,
  `refine_macro`, `realign_macro`), a reported completion shows "Done ·
  reported by the skill".
- **US3.6** Done carries a text hint of the next workflow step ("Next:
  Specify"), the same one the next-step button would launch. It is not a
  button. No hint when the task is finished or has no next step.

### US4 - Failures say why (P1)

- **US4.1** Given a failed skill record, then the detail is the reason: the
  first line of the recorded error, otherwise of the recorded note, shown as
  recorded, without translation.
- **US4.2** The reason is truncated to about 120 characters, with an ellipsis,
  in the status bar and the tooltip; the full text appears in the status bar's
  details line.
- **US4.3** Given no recorded reason, then the detail says "reason not
  recorded".
- **US4.4** Given a local agent too old to return reasons, then the detail
  says "reason not available".
- **US4.5** The next step of Could not finish is "Open the console" while the
  console is still open, and "Relaunch *skill*" once it has exited.
- **US4.6** Server bookkeeping sentences (the silence observation) are never
  shown as a reason.

### US5 - Stopped says why, honestly (P1)

- **US5.1** "Stopped by you" only when Desktop recorded a stop request for
  this execution.
- **US5.2** "Session disconnected" only when the server recorded that it closed
  the run because its client disconnected.
- **US5.3** Otherwise the recorded reason (US4.1, US4.2), or "reason not
  recorded". No actor or cause is ever stated without a record of it.

### US6 - Needs your answer is distinct (P1)

- **US6.1** Needs your answer has its own glyph, label and colour, distinct
  from Could not finish, Stopped and Needs verification.
- **US6.2** It is shown only while the process is running; a wait mark left on
  a queued or ended run is ignored.
- **US6.3** A skill verdict (rows 2 to 5) outranks a wait mark.

### US7 - Successor skill in the same console (P2)

- **US7.1** Given a console where another skill started after the launched one
  ended, then the primary status speaks only for that successor, following the
  same precedence.
- **US7.2** The launched skill's outcome stays in the diagnostics.
- **US7.3** Given the console exited while the successor's record is still
  `running`, then the status is Needs verification (row 8) for the successor.

### US8 - Documentation (P2)

- **US8.1** `desktop/README.md` "Skill result indicator" describes the primary
  status, its kinds, its precedence and the diagnostics, and says the reasons
  require the updated local agent.
- **US8.2** `docs/USER_GUIDE.md` links to that section and the link resolves.
- **US8.3** `CHANGELOG.md` `[Unreleased]` carries one `Changed` line for the
  user.

## Functional requirements

- **FR1** One function computes the primary status from the execution, the
  skill result and the task; the task row and the status bar render only its
  output. No other code decides a status.
- **FR2** The function is pure and deterministic; it is unit tested row by row
  against the precedence table, including the cases of US2.2 to US2.7, US3,
  US5 and US7.
- **FR3** The local agent's skill-result lookup returns the recorded reason
  fields of the matched execution and of its successor, additively: a Desktop
  that ignores them keeps working, and a Desktop facing an agent that lacks
  them shows "reason not available".
- **FR4** No server API, database or migration change.
- **FR5** The refresh policy of skill results (`skill-result-refresh.mjs`) is
  unchanged.
- **FR6** Selection, keyboard focus and the terminal are preserved when a
  status changes, as today.

## Acceptance criteria

- [ ] A skill execution's task row and status bar show one primary status and
      no run-state glyph or skill badge beside it (US1).
- [ ] Every precedence row 1 to 9 has a passing unit test (US2).
- [ ] An open console after completion shows Done; an exit without report and
      a completion before stage validation show Needs verification (US2).
- [ ] Done names the outcome and never implies the task is finished unless it
      is (US3).
- [ ] Failed and stopped work show the recorded reason, or say it is not
      recorded or not available; "Stopped by you" and "Session disconnected"
      appear only on record (US4, US5).
- [ ] Needs your answer is visually and verbally distinct (US6).
- [ ] A successor skill owns the status; the previous outcome stays in the
      diagnostics (US7).
- [ ] Diagnostics show the run state, skill record and task stage (US1.4).
- [ ] README, user guide link and changelog updated (US8).
- [ ] `make test` (or the Desktop and Go test targets it runs) passes.

## Open points

None. Every point was settled in the clarification (round 2, 2026-10-27).
