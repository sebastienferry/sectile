# Specification #639 - Switch to a ticket's new execution

- Ticket: https://github.com/sebastienferry/sectile/issues/639
- Branch: `feat/639`
- Clarification: `docs/clarifications/639.md` (rounds 1 and 2, confirmed by
  the owner on 2026-09-29)
- Framework: Spec Kit

## Summary

Sectile Desktop groups the executions of a ticket in one sidebar row. When a
new execution of the ticket whose row is selected appears, the console keeps
showing the earlier execution the user had open, while the row already shows
the new one as queued or running. With this change, the desktop switches the
console to the new execution as soon as it appears, whatever launched it.

## Scope

In scope:

- the ticket row selected in the sidebar, whose console shows one of its
  executions;
- every source of a new execution of that ticket: the Relaunch dialog, the
  Tickets pane, the Next step and Full chain buttons, the web board and any
  other client of the server;
- executions that appear queued, preparing or running.

Out of scope:

- rows that are not selected: they keep updating their state as today, and the
  console does not move to them;
- free consoles and macro runs: a new macro run does not take the console
  over from the macro run on display;
- the server, the local agent and the web board: no API change;
- the order of the sidebar rows.

## User stories

### US1 (P1) - The console follows a new execution of the displayed ticket

As a desktop user watching a ticket, I see its newest execution without
looking for it in the execution history.

1. Given ticket #1 selected with its finished execution A on display, when a
   new execution B of #1 appears in the run list, then the console shows B,
   the toolbar title names B's skill, and the execution history drop-down
   selects B.
2. Given ticket #1 selected with execution A on display, when B is launched
   from the Relaunch dialog, from the Tickets pane, from the Next step or Full
   chain buttons, or from the web board, then the console shows B in each
   case.
3. Given ticket #1 selected with execution A still running in an interactive
   console, when B appears, then the console shows B. A keeps running and
   stays reachable from the execution history drop-down.
4. Given ticket #1 selected, and the user picked its older execution A from
   the execution history drop-down, when B appears, then the console shows B.
5. Given ticket #1 selected, when B appears queued, then the console shows B's
   notice "Execution queued. Waiting for a console."; when B starts, then its
   console attaches without any user action.
6. Given ticket #1 selected, when two new executions of #1 appear in the same
   update, then the console shows the one the ordering of the sidebar puts
   first among them: an active one before a queued one before a finished
   one, then the most recently submitted.

### US2 (P1) - Nothing else moves

As a desktop user, the console does not jump away from what I chose.

1. Given ticket #1 selected, when a new execution of ticket #2 appears, then
   the console keeps showing #1's execution, and #2's row shows its new state.
2. Given a free console selected, when a new execution of any ticket appears,
   then the free console stays on display.
3. Given a macro run selected, when a new run of the same macro appears, then
   the macro run on display stays.
4. Given the desktop starts, or the local agent restarts, when the first run
   list arrives, then the console shows what it shows today (the first
   visible execution), not an execution chosen for being new.
5. Given ticket #1 selected, when an update brings no new execution of #1,
   then the console keeps the execution on display, including one the user
   picked from the history.
6. Given ticket #1 selected in a project that is disconnected (hidden), when a
   new execution of #1 appears, then the console does not switch to it.

### US3 (P2) - The switch does not interrupt the user

As a desktop user busy elsewhere in the window, the switch does not disturb
what I am doing.

1. Given the Tickets pane open over the console, when a new execution of the
   selected ticket appears, then the Tickets pane stays open.
2. Given a rename field or a project menu open in the sidebar, when a new
   execution of the selected ticket appears, then the rename or the menu
   stays open, and the sidebar catches up once it closes.
3. Given a launch from the Next step or Full chain buttons, when the new
   execution appears, then the console shows it once, without attaching it
   twice.

### US4 (P3) - The change is announced

1. Given `CHANGELOG.md`, when read, then `[Unreleased]` carries one line under
   `Fixed` saying that the desktop console switches to a ticket's new
   execution when that ticket is on display.

## Functional requirements

- **FR-001** On every update of the run list, the desktop finds the
  executions that were not in the previous list and belong to the same ticket
  as the execution on display.
- **FR-002** When FR-001 finds at least one, the console shows the one ranked
  first by the sidebar ordering (active, then queued, then finished, then
  newest submission).
- **FR-003** The switch happens whatever launched the execution, whatever its
  state (queued, preparing, running), and whether the execution on display
  was picked from the history or is still running.
- **FR-004** The switch applies to ticket rows only: not to free consoles, not
  to macro runs, not to executions of a hidden project or archived
  executions.
- **FR-005** The first run list after the desktop starts or the agent
  restarts is a baseline: it switches nothing.
- **FR-006** The switch keeps the Tickets pane open and defers the sidebar
  redraw while a rename or a menu is in progress, like the other selections
  made by the run list updates.
- **FR-007** A queued or preparing execution shows its console notice and
  attaches once it starts, as a selected queued execution does today.
- **FR-008** A launch from Next step or Full chain shows its execution once.

## Success criteria

- In a desktop test where the agent's run list gains an execution of the
  displayed ticket between two polls, the console switches to it within one
  poll, for a finished, a running and a history-picked execution on display.
- In the same test, a new execution of another ticket, of a macro or a free
  console on display switches nothing.
- The existing desktop tests keep passing.

## Open requirements

None. Every product decision was settled during clarification.
