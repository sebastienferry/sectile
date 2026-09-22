# #349 — Attribute a tracker ticket to a Sectile project with a label

- Ticket: https://github.com/sebastienferry/sectile/issues/349 (Feature, `#clarified`)
- Branch: `feat/349`
- Clarification: [`docs/clarifications/349.md`](../../docs/clarifications/349.md)
- Technical approach: [`plan.md`](./plan.md) — implementation checklist: [`tasks.md`](./tasks.md)

## Context

A Sectile project imports a whole tracker project: every work item of the configured issue
types of `sebastienferry/sectile`, or of the Jira key `PE`, lands on its board. Two teams
that share one Jira key therefore share one board, and there is no way to say "this ticket
is mine".

This feature adds a **membership label**: one free-text label configured on the Sectile
project. A ticket carrying it belongs to that project; the board shows only those tickets
by default, a quick filter shows the whole imported board, and two burger-menu entries add
or remove the label on a card — which writes it to the tracker, so the attribution is
visible to the team outside Sectile and survives a resync.

The clarification settled every product question over three rounds. Two decisions of the
owner shape what is specified here, and both reverse a Round 1 technical choice:

- **The label is not a sync filter.** The sync keeps importing the whole tracker project,
  filtered only by issue type as today. The narrowing happens at display time. Nothing on
  the tracker side is queried differently — `jiraJQL`'s `extra` clause stays as it is, and
  `GetEpics` is untouched.
- **The setting is not Jira-specific.** The ticket's title says "for Jira tracker"; the
  feature delivered is tracker-agnostic, on purpose. The column is `project_label`, the
  field `projectLabel`, and it lives in the project's general settings rather than in the
  Jira panel. All three registered adapters (`github`, `jira`, `local`) declare `CapLabels`,
  so the card action works on each; on a local project the label simply stays local.

Out of scope: pruning from the local board the tickets that stop matching (the import never
prunes, and nothing here changes that); several labels per project; any change to the
workflow stage labels of ADR 0013; any equivalent for the project's GitLab fields, which
have no registered adapter today.

## Decisions being specified

1. **`Project.ProjectLabel`** — one free-text label per project, stored in a new
   `projects.project_label` column. Empty means "no membership filter", which is what every
   existing project gets.
2. **Whitespace is refused at input**, in the UI and on the API. GitHub accepts a label
   with spaces, Jira does not, and `cleanLabels` silently rewrites them to `-` — which would
   make the value stored on the project differ from the value written to the tracker, and
   the filter match nothing.
3. **The membership filter runs server-side**, as one more parameter of `GET /api/tasks`
   and `GET /api/tasks/facets`. Every other board filter does, and the facets and counts
   come from the same scope; a client-side filter would leave them describing a list nobody
   sees.
4. **The quick filter is a toggle remembered per project**, like `pinnedOnly` and
   `activeOnly`, with one difference: its default is "this project only", so a project that
   configures a label sees its own slice without touching anything.
5. **The card action reuses `PATCH /api/tasks/:id`.** `UpdateTask` already diffs the
   submitted label set against the stored one and queues both the additions and the
   removals to the tracker. No new endpoint, no new code path for removal.
6. **Creation stamps the label locally and on the tracker.** Stamping only the tracker
   would filter the card the user just created out of its own board until the next sync.

## User stories

### US1 — Configure the membership label of a project (priority: P1)

As the owner of a project whose tracker project is shared with another team, I set one
label on my Sectile project so that Sectile knows which tickets are mine.

- **Given** I open the project settings, **when** the "Général" tab renders, **then** a
  "Label du projet" text input shows the project's current `projectLabel`, with a hint
  saying that an empty value shows every ticket of the board.
- **Given** I type `team-alpha` and save, **when** the project is reloaded, **then**
  `projectLabel` is `team-alpha`, spelled exactly as typed.
- **Given** I type a value containing a space or a tab, **when** I try to save, **then**
  the save is refused with a message naming whitespace as the reason, and the project is
  left unchanged. This holds whether the value came from the UI or straight from the API.
- **Given** a project created before this change, **when** it is read, **then** its
  `projectLabel` is empty and its board behaves exactly as it does today.
- **Given** the value has leading or trailing whitespace only, **when** it is saved,
  **then** it is trimmed rather than refused; whitespace *inside* the value is what is
  refused.

### US2 — See only my project's tickets (priority: P1)

As a user of a project that has a membership label, I open the board and see my team's
tickets, not the whole shared tracker project.

- **Given** a project whose `projectLabel` is `team-alpha` and a board holding tickets with
  and without that label, **when** I open the board, the list, the roadmap or a sprint view,
  **then** only the tickets carrying `team-alpha` are shown, in every view.
- **Given** a ticket carrying `Team-Alpha`, **when** the filter runs, **then** it is shown:
  the comparison is case-insensitive locally, as every other label comparison in the
  codebase already is.
- **Given** a ticket carrying `team-alphabet` and none carrying `team-alpha`, **when** the
  filter runs, **then** nothing is shown: the match is on the whole label, not on a prefix.
- **Given** a project whose `projectLabel` is empty, **when** I open the board, **then**
  every imported ticket is shown, exactly as today.
- **Given** the "all projects" scope (no project selected, so the bookmarked projects),
  **when** the board renders, **then** each project contributes its own slice: a project
  with a label contributes the tickets carrying it, a project without one contributes all
  of its tickets.
- **Given** the membership filter is active, **when** the sprint, team, label, assignee,
  status, issue-type and search filters are used, **then** they compose with it: the
  membership filter narrows the scope and none of them widens it back.
- **Given** a project that has just configured a label and no ticket carries it yet,
  **when** the board renders, **then** it is empty — which is the expected reading, and the
  reason the quick filter of US3 exists.

### US3 — Show the whole board (priority: P1)

As a user of a project that has a membership label, I flip one toggle to see every ticket
of the imported board, so I can find the ones that are not attributed yet.

- **Given** a project whose `projectLabel` is set, **when** the filter bar renders, **then**
  a toggle offers to show every ticket of the board, off by default.
- **Given** I turn the toggle on, **when** the tasks are fetched, **then** the membership
  filter is not applied and every imported ticket is shown, the label carriers and the rest.
- **Given** I turn the toggle on, **when** I leave the project and come back, or reload the
  page, **then** the toggle is still on: it is remembered per project in `localStorage`
  under `sectile_filters_<projectId>`, like `pinnedOnly` and `activeOnly`.
- **Given** a project whose `projectLabel` is empty, **when** the filter bar renders,
  **then** the toggle is not shown: there is nothing for it to widen.
- **Given** the toggle is on or off, **when** the facets are fetched, **then** the sprints,
  teams, assignees, labels, statuses and counts describe exactly the list being shown.

### US4 — Attribute a ticket to the project from its card (priority: P1)

As a user looking at an unattributed ticket, I add it to my project from the card's burger
menu, and the label is written to the tracker.

- **Given** a project with a `projectLabel` and a card that does not carry it, **when** I
  open the card's burger menu, **then** an "Ajouter au projet" entry is offered.
- **Given** I click it, **when** the update completes, **then** the task's labels contain
  the project label, `PATCH /api/tasks/:id` was called with the full new label set, and the
  addition is queued to the tracker as an `add` operation, leaving every other label alone.
- **Given** a project with a `projectLabel` and a card that carries it, **when** I open the
  menu, **then** a "Retirer du projet" entry is offered instead, and clicking it removes the
  label locally and queues its removal on the tracker.
- **Given** the card carries the label spelled with a different case, **when** the menu
  renders, **then** "Retirer du projet" is offered, and removing it drops that spelling.
- **Given** a project whose `projectLabel` is empty, **when** I open any card's menu,
  **then** neither entry is shown.
- **Given** the membership filter is active and I remove a card from the project, **when**
  the list refreshes, **then** that card leaves the board — which is the point, and is
  reversible through the "show all" toggle.

### US5 — A ticket created in the project belongs to it (priority: P2)

As a user creating a ticket from a project that has a membership label, I get a ticket that
is already mine, without having to attribute it by hand right after creating it.

- **Given** a project whose `projectLabel` is `team-alpha`, **when** I create a task in it,
  **then** the created work item carries `team-alpha` on the tracker and the local task
  carries it too, so it appears on its own board immediately.
- **Given** the same project, **when** a local task is pushed to a tracker
  (`PushTaskToTracker`), **then** the created work item and the local task both carry the
  label.
- **Given** a task whose submitted labels already contain the project label, whatever its
  case, **when** it is created, **then** the label is not duplicated.
- **Given** a project whose `projectLabel` is empty, **when** a task is created, **then**
  its labels are exactly what they are today.

## Functional requirements

- **FR1** — `Project` carries `ProjectLabel` (JSON `projectLabel`), persisted in
  `projects.project_label` (TEXT NOT NULL DEFAULT ''), added with the same idempotent
  `ALTER TABLE ... ADD COLUMN` pattern as `jira_project`, and settable through
  `CreateProjectRequest` and `UpdateProjectRequest`.
- **FR2** — The API refuses a `projectLabel` containing whitespace with HTTP 400 and a
  message naming the reason; a value with only surrounding whitespace is trimmed and
  accepted. The UI refuses the same input before submitting. A value starting with `#` or
  spelling a workflow stage (`new`, `implemented`, …) is refused the same way:
  `SetWorkflowLabel` strips those at every stage change, which would take the ticket out
  of its project, and label comparisons drop the `#`.
- **FR3** — `GET /api/tasks` and `GET /api/tasks/facets` take a `membership` parameter,
  `project` (the default, also what an absent parameter means) or `all`. Under `project`,
  a task is returned when its project has no membership label, or when its labels contain
  that project's label.
- **FR4** — The label match is on the whole label and case-insensitive, and never matches a
  prefix, a suffix or a substring of another label.
- **FR5** — The membership condition composes with every existing filter as an additional
  `AND` term, and applies identically to a single-project scope and to the bookmarked
  "all projects" scope, each project answering with its own label.
- **FR6** — The facets are computed under the same membership scope as the task list, so
  the dropdowns and counts describe the list the user sees.
- **FR7** — The filter bar shows a "show the whole board" toggle for a project that has a
  membership label, default off, persisted per project in `localStorage` alongside the
  other filters, and hidden for a project that has none.
- **FR8** — The task card's burger menu offers "Ajouter au projet" on a card that lacks the
  project's label and "Retirer du projet" on a card that carries it, both going through
  `updateTask(id, { labels })`; neither is shown when the project has no label.
- **FR9** — `CreateTaskAs` and `PushTaskToTracker` add the project's membership label to the
  labels they send to the tracker and to the labels they store locally, without duplicating
  a label already present in any case.
- **FR10** — An empty `projectLabel` leaves every observable behaviour — sync, board,
  facets, menus, creation — exactly as it is before this change.

## Non-functional requirements

- **NFR1** — No change to any tracker query. `SyncIssues`, `GetEpics` and `GetIssue` send
  the same requests as before; the import keeps bringing in the whole tracker project.
- **NFR2** — The membership condition is expressed in SQL that both backends accept.
  `json_each` is SQLite-only and is not used; the pattern is built in Go, with the LIKE
  wildcards `%` and `_` escaped, and matched with an explicit `ESCAPE`.
- **NFR3** — The migration is additive and idempotent, and runs on an already-populated
  database of either backend without touching existing rows.
- **NFR4** — `projectLabel` is not a stage label: `SetWorkflowLabel` and
  `StaleWorkflowLabels` never see it, so a stage advance neither adds nor removes it.
- **NFR5** — User-facing strings added in Go are English, per the repository policy; the
  strings added in `web/src` follow the French copy of the panel they sit in.

## Acceptance criteria

The ticket is done when, on a project sharing a tracker project with another:

1. A membership label can be set in the project's general settings, and a value with a
   space is refused with an explicit message.
2. The board, list, roadmap and sprint views show only the tickets carrying it.
3. A toggle shows the whole imported board, and the choice survives a reload and a project
   switch.
4. The burger menu adds the ticket to the project or removes it, and the label appears on —
   or disappears from — the ticket on the tracker.
5. A ticket created from that project already carries the label.
6. A project with no membership label behaves exactly as it does today.
7. `gofmt -l .` is clean, `go build ./...` and `go test ./...` pass, and
   `npm test`, `npx tsc --noEmit -p tsconfig.app.json` and `npx oxlint src` pass in `web/`.

## Open requirements

None. The clarification closed every product question in Round 3 and the task is at
`clarified`; nothing in this specification is blocked.
