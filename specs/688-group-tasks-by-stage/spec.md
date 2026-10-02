# Specification #688 - Group a project's tasks by stage

- Ticket: https://github.com/sebastienferry/sectile/issues/688
- Branch: `feat/688`
- Clarification: `docs/clarifications/688.md` (round 1, every recommendation
  accepted by the owner)
- Framework: Spec Kit

## Summary

In the Sectile Desktop sidebar, each project can order its local tasks by
workflow stage, from new to finished, through a "Group by stage" option of its
`…` menu. Whatever that option, the key of every staged task carries a light
tint of its stage, so the stage reads at a glance.

## Scope

In scope: the Desktop sidebar (the local tasks under each project), the
project `…` menu, the stage tint of the task key, and the changelog.

Out of scope:

- The web board and backlog, which already group by workflow.
- The Desktop "Open tasks" pane, which already sorts on a stage column.
- The execution queue view of a project ("Show execution queue").
- Section headers per stage inside a project.
- Filtering tasks by stage.
- Any server, API, database or tracker change.

## Vocabulary

- **Local task row**: one row of the sidebar under a project, standing for
  every visible execution of one task.
- **Stage**: the workflow stage of the row's task, one of new, clarified,
  specified, implemented, reviewed, finished, as Desktop already derives it
  for the next step and the tickets pane.
- **Unstaged row**: a row whose stage is not known: a free console, a macro
  run, or a task Desktop has not read yet or could not read.
- **Current order**: the sidebar order as it is today: active executions
  first, then queued, then the others, each by submission.
- **Stage tint**: a light background colour on the task key, one per stage.

## User stories

### US1 (P1) - Order a project's tasks by stage

As an owner following many tickets of one project, I turn on "Group by stage"
for that project and read its tasks in workflow order, so I see at once which
tickets wait for a clarification, a specification or a review.

1. Given a project whose rows have the stages specified, new and reviewed,
   when I choose "Group by stage" in its `…` menu, then its rows read new,
   specified, reviewed.
2. Given "Group by stage" on, when two rows share a stage, then they keep
   their current order relative to each other.
3. Given "Group by stage" on, when a project also lists unstaged rows, then
   they come after every staged row, in their current order.
4. Given "Group by stage" on, when I open the `…` menu again, then the entry
   shows as checked, and choosing it turns the option off and restores the
   current order.

### US2 (P1) - The choice belongs to the project and survives a restart

1. Given two projects, when I turn "Group by stage" on for one, then the other
   keeps its current order.
2. Given "Group by stage" on for a project, when I quit and restart Desktop,
   then the project is still grouped by stage.
3. Given a fresh installation, then no project is grouped by stage.

### US3 (P1) - The task key shows its stage

1. Given a row whose task is at the clarified stage, then its key carries the
   clarified tint, and hovering the key names the stage ("Stage: clarified")
   along with the existing "Open task in Sectile".
2. Given "Group by stage" off, then the keys still carry their stage tint.
3. Given an unstaged row, then its key carries no tint.
4. Given the light and the dark themes, then every tinted key stays readable.

### US4 (P2) - A stage change shows without a manual refresh

1. Given a task listed in the sidebar, when its stage changes on the board or
   the tracker, then its tint, and its place when grouped, follow at the next
   task refresh Desktop already performs, with no extra request per task.
2. Given the pointer over the sidebar or a rename in progress, then rows do
   not move under it, as the sidebar already guarantees for other updates.

## Functional requirements

- **FR1** The project `…` menu offers a "Group by stage" entry, with its
  checked state exposed to assistive technology.
- **FR2** The option is per project, off by default, stored on the
  workstation, and kept across restarts. Removing a project from Desktop
  forgets it.
- **FR3** With the option on, the project's rows are ordered by stage in the
  workflow order new, clarified, specified, implemented, reviewed, finished,
  then the unstaged rows; the current order breaks every tie.
- **FR4** With the option off, the order is exactly the current order.
- **FR5** Every staged row's key carries the tint of its stage, whatever the
  option; an unstaged row's key carries none.
- **FR6** The key's tooltip names the stage of a staged row.
- **FR7** The tints are distinct per stage and readable in both themes.
- **FR8** The stage comes from the task data Desktop already fetches for the
  sidebar; no request per task is added.
- **FR9** A row whose stage changes moves and recolours on the next sidebar
  render, subject to the existing hold while the sidebar is in use.
- **FR10** `CHANGELOG.md` gets one `Added` line under `[Unreleased]`.

## Acceptance criteria

- US1 to US4 scenarios pass.
- The sidebar order tests of today pass unchanged with the option off.
- The project menu keeps its keyboard navigation (arrows, Escape) with the new
  entry.

## Edge cases

- A task whose labels carry no stage falls back on its status, as
  `taskStage()` already does; a status that maps to no stage makes the row
  unstaged.
- A finished task still listed (its executions not archived) sorts last among
  staged rows and carries the finished tint.
- The task list request of a project fails: its rows keep the stage they last
  had; rows never read stay unstaged.
- A project shown in queue mode: the option is kept but has no effect until
  the task list is shown again.
- A collapsed project: nothing is rendered; the option applies on expansion.

## Open points

None. Every product question was settled in the clarification.
