# Spec #841 - Always provide the ticket link in WebUI toasts

Ticket: https://github.com/sebastienferry/sectile/issues/841
Clarification: `docs/clarifications/841.md` (rounds 1 and 2, settled)
Branch: `feat/841`
Follows: #840 (merged in #845)

## Problem

#840 added an "Open #N" link to the success toasts of ticket creation,
cloning, macro stories and ordinary edits. Every other WebUI toast that names a
ticket still offers no way to reach it: a run that finished or failed, a stage
or status move, a sync, a comment, a sprint or team change, and every error.
The user reads "Skill failed for #123" and has to search the board for #123,
which may not even be on the current board.

## Scope

In: every WebUI toast whose subject is one or several existing tickets,
whatever its type (success, info, warning, error), including the run outcomes
announced by the activity poller, the credential offer that replaces an error,
and toasts raised by components (cards, list view, timeline backlog, run
badges); the "+N more" line; the changelog line.

Out (unchanged): toasts whose subject is not a ticket (settings, projects,
board views, credentials, trackers, global and per-tracker sync, sprints as
such, the activity log, skill editor, command palette installs), macro-only
toasts (macro framing, labels, horizons, roadmap axes, macro rename), the
toasts #840 already links, the Desktop app's native notifications, and the toast
presentation, lifetime and hover/focus pause.

## Definitions

- **Ticket link**: the #840 link. When the ticket is known to Sectile, a button
  "Open {key}" (`Ouvrir {key}` in French) that opens the ticket's detail view,
  plus the tracker icon when the ticket has an external URL. When the ticket
  exists only on the tracker, a link "Open {key} in tracker" to its external URL.
- **Subject tickets**: the ticket or tickets the action that raised the toast
  was made on, in the order the action received them.

## User stories

### US1 - Single-ticket toasts link their ticket (P1)

- **US1.1** Given an action on exactly one existing ticket raises a toast, then
  the toast offers that ticket's link, whether it reports a success, an
  information, a warning or an error.
- **US1.2** This applies at least to the toasts listed in the inventory below
  (FR6). A toast this spec does not list but whose subject is exactly one
  existing ticket follows the same rule.
- **US1.3** Clicking "Open {key}" opens the ticket's detail view and closes the
  toast, as in #840. Clicking the tracker icon opens the external URL in a new
  tab with `noopener noreferrer`.
- **US1.4** The toast title and description are unchanged.

### US2 - Run outcomes link their ticket, even off the board (P1)

- **US2.1** Given a run of a ticket completes or fails while the WebUI is open,
  then the "Skill completed" or failure toast offers "Open {key}".
- **US2.2** Given that ticket is on the loaded board, then clicking opens it at
  once, with its tracker icon when it has an external URL.
- **US2.3** Given that ticket is not on the loaded board (another project, a
  filter, a saved view), then the toast still offers "Open {key}"; clicking it
  reads the ticket from the server and opens its detail view. No ticket is
  invented from the run's data.
- **US2.4** Given the ticket cannot be read when clicked (deleted, no access,
  server unreachable), then the detail view does not open and an error toast
  says the ticket could not be opened, naming its key.
- **US2.5** Given a run has no ticket (no ticket id), then its toast has no
  ticket link.

### US3 - Errors with the credential offer show both actions (P1)

- **US3.1** Given an action on one ticket is refused because the user has no
  token of their own for the tracker, then the toast shows the credential offer
  first and the ticket link second.
- **US3.2** The same holds for the credential offer raised by a failed run in
  the activity poller (US2), and for an action on several tickets (the offer
  first, then the US4 links).
- **US3.3** The credential offer's label, behaviour and wording are unchanged.

### US4 - Toasts about several tickets (P2)

- **US4.1** Given an action on 2 to 5 tickets raises one toast, then the toast
  offers one ticket link per ticket, in the order of the subject tickets.
- **US4.2** Given an action on more than 5 tickets, then the toast offers the
  links of the first 5 and then a line "+{N} more" (`+{N} de plus`) without a
  link, N being the number of remaining tickets.
- **US4.3** A subject ticket whose key is not known to the WebUI (not on the
  loaded board and not returned by the server) gets no link and is counted in
  N. When no ticket can be linked and N is 0, the toast has no ticket line.
- **US4.4** Given an action on exactly one ticket through a multi-ticket path
  (a bulk action with one selected ticket), then US1 applies: one link, no
  "+N more".

### US5 - Deletion offers no link (P2)

- **US5.1** Given a ticket is deleted, then the deletion toast offers no link,
  whether the deletion was of one ticket or a bulk deletion.
- **US5.2** Given a deletion fails, then the ticket still exists and its error
  toast follows US1 (or US4 for a bulk deletion).

### US6 - Nothing else changes (P1)

- **US6.1** Toasts out of scope keep no link, or the link they have today.
- **US6.2** A toast with at least one link keeps the #840 behaviour: it waits
  while hovered or focused, and its lifetime and look are unchanged.
- **US6.3** The #840 links (creation, clone, macro story, ordinary edit) are
  unchanged.

## Functional requirements

- **FR1** Every link uses the #840 labels and behaviour. No new label is
  introduced for a ticket link.
- **FR2** A ticket the handler holds as a returned `Task` is linked from that
  `Task`. A ticket the handler holds only by id (and key) is linked from the
  loaded board when it is there, and otherwise through a link that reads the
  ticket by id on click.
- **FR3** The lazy link of FR2 is only offered when both the ticket id and its
  key are known: the label needs the key, the read needs the id.
- **FR4** When a toast already carries a link (the credential offer), the ticket
  link or links come after it.
- **FR5** Two new strings exist in both languages: the "+N more" line
  (en `+{count} more`, fr `+{count} de plus`) and the failed read of US2.4
  (en `Ticket {key} could not be opened`, fr `Impossible d'ouvrir le ticket
  {key}`).
- **FR6** Inventory of toasts that gain links (handler, toast; S = single
  ticket, M = several):

  | Where | Toast | Kind |
  | --- | --- | --- |
  | Activity poller | skill completed, skill failed, credential offer of a failed run | S, lazy |
  | `runSkill` | skill queued; error | S |
  | `retryActivity`, `cancelActivity` | retried, canceled; error | S, lazy |
  | `RemoteRunBadge`, `ExecutionRecovery` | cancel or close failed | S, lazy |
  | `createTask` | priority not written, macro attachment failed | S |
  | `cloneTask` | error (links the source ticket) | S |
  | `updateTask` | error | S |
  | `syncSingleTask` | realigned; error | S |
  | `moveTaskToTrackerStatus` | transition queued; move impossible | S |
  | `getTaskComments`, `postTaskComment` | comments unavailable; posted; post failed | S |
  | `setTaskTeam`, `setTaskSprint`, `setTaskMacro` | changed or queued; error | S |
  | `transitionTaskStage` | stage applied; error | S |
  | `confirmInteractiveStep` | confirmed; error | S |
  | `togglePin` | error | S |
  | `convertTask` | exported; export failed | S |
  | `moveTask` | moved; error | S |
  | `moveTaskWorkflowStage` | workflow stage updated | S |
  | `deleteTask` | error only (US5) | S |
  | `TaskCard`, `CopyStepPromptButton` | key copied, prompt copied | S |
  | `TaskDetailModal` | key copied, spec copied, description updated, copy failed | S |
  | `TrackerBacklogView` | added to project; add failed | S |
  | `createBatchTasks` | batch created; error | M |
  | `setTasksSprint`, `setTasksTeam` | tickets moved; error | M |
  | `moveTasksToMacro` | move queued; error | M |
  | `migrateTasks` | tickets migrated; error | M |
  | `ListView` bulk actions | priority, stage, status, label added or removed; refused; errors; deletion error | M |
  | `SprintTimelineView` backlog | planned, assigned | M |

## Assumptions (derived from the settled decisions, reversible)

- A failed creation (`createTask`, macro story creation, batch creation before
  any ticket exists) has no existing ticket, so its error carries no link.
- A failed clone concerns its source ticket, which is linked.
- The activity log toasts (`deleteActivity`, `clearCompletedActivities`) are
  about runs as log entries, not about a ticket: they stay without a link.
- Sprint toasts (create, close, reopen, delete) are about the sprint: they stay
  without links even when the close moves unfinished tickets.
- Toasts raised inside the ticket's own detail view link it too, as the rule
  says "every"; clicking keeps that ticket open.

## Acceptance criteria

- **AC1** US1 to US6 hold in the WebUI, in English and in French.
- **AC2** Automated tests cover: a single-ticket success and error toast, a run
  outcome for a loaded and for an unloaded ticket (including the failed read),
  the credential offer followed by the ticket link, 3 tickets (3 links), 7
  tickets (5 links and "+2 more"), the deletion toast with no link.
- **AC3** `CHANGELOG.md` gains one `Changed` line under `[Unreleased]` saying
  that WebUI notifications about tickets now always link to them.
- **AC4** Typecheck, lint, `web/tests` unit tests and the toast browser test
  pass.

## Open points

None. The clarification settled every product question.
