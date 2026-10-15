# Plan #841 - Always provide the ticket link in WebUI toasts

Spec: `spec.md` in this directory.

## Stack and boundaries

WebUI only (`web/`, React + TypeScript). No server change, no migration, no new
dependency, no new endpoint: the ticket read of US2.3 uses the existing
`GET /api/tasks/{id}`, which `runSkill` already calls for a ticket that is not
loaded. Desktop is untouched.

## Data contracts

`web/src/types/index.ts`:

```ts
export interface ToastMessage {
  // ...unchanged fields
  link?: ToastLink
  links?: ToastLink[]
  /** Subject tickets left without a link after the first five (#841). */
  moreLinks?: number
}
```

`ToastLink` is unchanged: a lazy link is an in-app link whose `onOpen` reads
the ticket, with no `externalUrl` (the activity carries none).

Locale catalog (`web/src/locales/translations.ts`, type and both languages,
under `toasts`):

- `moreTickets`: en `+{count} more`, fr `+{count} de plus`
- `ticketUnavailable`: en `Ticket {key} could not be opened`,
  fr `Impossible d'ouvrir le ticket {key}`

## Architecture

### Pure helpers - `web/src/lib/issueToast.ts`

Keep `issueToastLink(task, open, labels)` as is (FR1). Add:

- `issueRefLink(ref: { id?: string; key?: string }, openById: (id: string, key: string) => void, labels)`
  returns `{ label: "<internal> <key>", onOpen: () => openById(id, key) }`
  when both `id` and `key` are set, `undefined` otherwise (FR3).
- `cappedLinks(links: (ToastLink | undefined)[], max = 5): { links: ToastLink[]; more: number }`
  keeps the first `max` defined links in order; `more` counts every entry past
  them plus every `undefined` entry (US4.2, US4.3).
- `withTicketLinks(toast, ticket: { links: ToastLink[]; more: number })`
  returns the toast with `links = [...(toast.link ? [toast.link] : toast.links ?? []), ...ticket.links]`,
  `link` removed, and `moreLinks = ticket.more` when positive (FR4). With no
  ticket links and no remainder the toast is returned unchanged.

These are pure and unit-tested in `web/tests/issueToast.test.mjs`.

### Context wiring - `web/src/context/AppContext.tsx`

Next to `taskLink` (line ~1258):

- `openTaskById(id, key)`: a ticket found in `tasks`, `pinnedTasks` or
  `selectedTask` is selected at once; otherwise `GET /tasks/{id}` and
  `setSelectedTask` with the answer. A non-OK answer or a network error adds
  `{ type: 'error', title: t.toasts.error, description: format(t.toasts.ticketUnavailable, { key }) }`
  (US2.4). Never builds a `Task` from activity fields.
- `ticketLink(ref: Task | { id?: string; key?: string } | undefined)`:
  a `Task` with an id goes through `taskLink`; an id or key found on the board
  resolves to that `Task` and goes through `taskLink` (US2.2); otherwise
  `issueRefLink` with `openTaskById` (US2.3).
- `ticketLinks(refs)`: `cappedLinks(refs.map(ticketLink))`.
- `refusalToast(err, toast, tickets?)` gains an optional third argument (the
  result of `ticketLinks`, or a single `ticketLink` wrapped as one) and applies
  `withTicketLinks` to whichever toast it returns, the token offer included
  (US3.1). The context type at line ~371 is extended accordingly; existing
  callers without the argument are unaffected.
- `tokenOfferToast(tracker, taskKey?, tickets?)`: same optional argument, for
  the activity poller (US3.2).

Expose `ticketLink` and `ticketLinks` on the context value for components.

Then each handler of the spec inventory (FR6) passes its subject:

- Handlers holding a returned `Task` (`syncSingleTask`, `moveTaskToTrackerStatus`
  when `data.task`, `setTaskTeam`, `setTaskSprint`, `setTaskMacro`,
  `transitionTaskStage` when `data.task`, `convertTask`, `moveTask`,
  `moveTaskWorkflowStage`, `runSkill`, and `createTask` warnings with `created`)
  link that `Task` on success.
- Their error branches, and the success branches without a returned `Task`,
  link `{ id }` (or `{ id: taskIdOrKey }` / `{ key }` for `transitionTaskStage`,
  resolved on the board) through `ticketLink`.
- `postTaskComment`, `getTaskComments`, `togglePin`, `updateTask` (error),
  `cloneTask` (error, source id), `deleteTask` (error only) link `{ id }`.
- `confirmInteractiveStep` links `{ id: taskId, key: taskKey }` from
  `pendingInteractive`.
- Activity poller (line ~2290): `{ id: act.taskId, key: act.taskKey }` for
  completed, failed and the token offer. `retryActivity` and `cancelActivity`
  read the activity from `activities` before the call and link it the same way.
- Multi-ticket handlers pass their subject list: `createBatchTasks` the created
  `Task[]` (error: none, nothing exists), `setTasksSprint`, `setTasksTeam`,
  `moveTasksToMacro` and `migrateTasks` their `taskIds` mapped to `{ id }`.
- `deleteTask` success keeps no link (US5.1).

Note: `ticketLink` is called when the toast is built, so the board lookup sees
the board of that moment; the lazy branch covers a ticket that is not there.

### Components

- `TaskCard.tsx` (copy), `CopyStepPromptButton.tsx`, `TaskDetailModal.tsx`
  (key/spec copied, description updated, copy failed), `TrackerBacklogView.tsx`
  (added, add failed): `link: ticketLink(task)`.
- `RemoteRunBadge.tsx`, `ExecutionRecovery.tsx`: `link: ticketLink({ id: run.taskId, key: run.taskKey })`.
- `ListView.tsx` bulk toasts (priority updated/refused, stage, status, label
  added/removed, errors): `withTicketLinks(toast, ticketLinks(selectedTasks))`.
  The bulk deletion success toast keeps no link (US5.1); its error links.
- `SprintTimelineView.tsx` backlog planned/assigned: the moved ids through
  `ticketLinks`.

### Rendering - `web/src/components/ToastContainer.tsx`

After the `links.map(...)` block, when `toast.moreLinks > 0`, render one
`<div className="text-[11px] text-[var(--text-muted)] mt-1.5">` with
`format(t.toasts.moreTickets, { count })`. No other change: the pause and
lifetime rules already key on the presence of links (US6.2).

## Rejected alternatives

- Fetching every unloaded activity ticket when the run finishes, to build a
  `Task` link with the tracker icon: one extra request per finished run for a
  link that is rarely clicked. The lazy read happens on click only.
- A fabricated `Task` from `taskId`/`taskKey`: forbidden by the clarification.
- A collapsible list for many tickets: changes the toast design, out of scope.

## Target files

- `web/src/types/index.ts`
- `web/src/lib/issueToast.ts`
- `web/src/locales/translations.ts`
- `web/src/context/AppContext.tsx`
- `web/src/components/ToastContainer.tsx`
- `web/src/components/TaskCard.tsx`, `CopyStepPromptButton.tsx`,
  `TaskDetailModal.tsx`, `TrackerBacklogView.tsx`, `RemoteRunBadge.tsx`,
  `ExecutionRecovery.tsx`, `ListView.tsx`, `SprintTimelineView.tsx`
- `web/tests/issueToast.test.mjs`, `web/tests/issue-toast-links.browser.mjs`
- `CHANGELOG.md`

## Risks

- `AppContext.tsx` is about 4,800 lines: edit in small hunks (project memory:
  subagents stall on large files).
- Browser tests need `PLAYWRIGHT_MODULE` pointing at a `#`-free path and run
  outside the sandbox.
