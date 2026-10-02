# Plan #630 - Roadmap: go from a ticket to its epic and back

## Stack

React 19 + TypeScript web app (`web/`), Tailwind classes, `lucide-react`
icons, `node --test` unit tests (`web/tests/*.test.mjs`) and Playwright
browser component tests (`web/tests/*.browser.mjs`, `useApp` mocked through
`window.ctx`). No Go change, no migration, no route.

## Design

### 1. Pure rules: `web/src/lib/roadmapFocus.ts`

Everything decidable without React lives here, so it is unit tested:

```ts
import type { Project, Task, ViewMode } from '../types'
import type { HorizonTab, MacroRow } from './roadmap'

export const TICKET_VIEWS: ViewMode[] = ['board', 'list', 'triage', 'timeline']
export const isTicketView = (view: ViewMode): boolean

/** FR1: a parent key, and a project that enables the roadmap. */
export function canOpenEpicInRoadmap(task: Pick<Task, 'parentKey'>, project: Project | null | undefined): boolean

/** FR4 / FR5: where the epic sits, or null when the roadmap does not hold it. */
export function locateEpic(rows: MacroRow[], key: string):
  { tab: HorizonTab; closed: boolean } | null

/** FR6 / FR7: the origin view when it is enabled, the board otherwise. */
export function returnView(origin: ViewMode | null, project: Project | null | undefined): ViewMode
```

- `canOpenEpicInRoadmap` trims the key and uses `isViewAvailable(project,
  'roadmap')` from `optionalViews.ts`.
- `locateEpic` matches `row.key === key` exactly (the key the ticket carries
  is the one the rows are built from). `tab` is `row.horizon || 'unclassified'`
  (`hidden` included as a tab); `closed` is `row.closed`.
- `returnView` answers `origin` when it is a ticket view and
  `isViewAvailable(project, origin)`; `'board'` otherwise.
- `projectOfTask(task, projects, currentProject)` resolves a ticket's project
  by id or slug, or the project on screen for a ticket without an id; the
  context and the three entry points share it.

### 2. App context: request, origin and query

In `web/src/context/AppContext.tsx`:

- **Origin view (FR7).** A `roadmapOriginView` ref (`ViewMode | null`, not
  stored). `setActiveView(view)`: when `view === 'roadmap'` and the current
  view is a ticket view, record the current view. Read the current view from
  a ref mirroring `activeView`, since the callback is memoized. Entering the
  roadmap from a non-ticket view (activities, sync) leaves the recorded one.
- **Focus request (FR3, FR4).** State
  `roadmapFocus: { projectId: string; epicKey: string; from: ViewMode } | null`
  and two actions exposed on the context type:
  - `openEpicInRoadmap(task: Task)`: when `canOpenEpicInRoadmap` fails, do
    nothing. Otherwise set `roadmapFocus` with `from = activeView`, call
    `setSelectedProjectId(task.projectId)` when a view is selected
    (`selectedViewId`) or `selectedProjectId !== task.projectId`, then
    `setActiveView('roadmap')` (which records the origin as above).
    `canOpenEpicInRoadmap` is given the ticket's project from `projects`,
    not `currentProject`, so it works under "all projects" (FR1, US1.5).
  - `consumeRoadmapFocus()`: sets `roadmapFocus` to null.
  - `openEpicTickets(epicKey: string)` (FR6): `setParentFilter(epicKey)`, then
    `setActiveView(returnView(roadmapOriginView.current, currentProject))`.
- **Query (FR8).** In `buildTaskQuery`, send `macro` only when
  `activeView !== 'roadmap'`, next to the existing search exception, with the
  same kind of comment. `activeView` is already in its dependencies. The
  context also filters the loaded tickets on the parent (`filteredTasks`); that
  filter skips the roadmap too, or the query change alone would not be seen.

### 3. Roadmap arrival

In `web/src/components/RoadmapView.tsx`:

- Track that the macros of the current project are loaded: store the project
  id with the fetched list (`macrosFor` state set in the existing
  `fetchProjectMacros(...).then(...)`), so a list from the previous project
  never counts.
- An effect runs when `roadmapFocus` is set, `roadmapFocus.projectId ===
  currentProject?.id`, `macrosFor === currentProject.id` and the context's
  `isLoading` is false. It calls `consumeRoadmapFocus()` first, then
  `locateEpic(allRows, epicKey)`:
  - found: `setSelectedKey(epicKey)`, `setTab(tab)`, `setShowClosed(true)` when
    closed, `setSearchQuery('')`, `setSelectedLabels([])`,
    `setPriorityFilter(null)`, `setOnlyIssues(false)`, `setIsPanelHidden(false)`.
    The search is the context's `searchQuery`, which the roadmap filters its
    rows with locally and which the ticket views share: clearing it only when
    it is non-empty is enough, and it is the roadmap search the spec means.
    The label filter (`selectedLabels`, #626) and the priority filter (#627)
    are local to the view. Grouping, sort, row mode, display mode and expanded
    panel are not touched.
  - not found: an error toast (`strings.focus.unknownTitle`,
    `format(strings.focus.unknown, { key })`), then `setActiveView(from)` when
    `from` is not `'roadmap'`. No other state changes.
- The existing "search jumps to a non-empty tab" effect must not fight the
  arrival: it only runs with a search, which the arrival clears.

### 4. Entry points

- `web/src/components/TaskCard.tsx`: in the card menu, beside the "Filter by
  parent" entry, a `Map` (or `Milestone`) icon entry "Open the epic in the
  roadmap", shown when `canOpenEpicInRoadmap(task, projectOf(task))`; it closes
  the menu and calls `openEpicInRoadmap(task)`. The parent key button is
  unchanged.
- `web/src/components/ListView.tsx`: in the action cell, before pin, an icon
  button with the same condition, title and action. The cell already stops the
  row click, so the detail does not open (US1.7).
- `web/src/components/TaskDetailModal.tsx`: after the parent key and its copy
  button, an icon button with the same condition and title; it closes the
  detail (`setSelectedTask(null)`), then calls `openEpicInRoadmap`.
- The project of a ticket comes from `projects.find(p => p.id ===
  task.projectId)`. `Task.projectId` is optional in the type: without it, the
  current project is the ticket's project, and under "all projects" the entry
  is not offered.

### 5. Return button

In the roadmap panel header (`RoadmapView.tsx`, beside "Open in tracker" and
"Copy link", around `:1546`): a "Open its tickets" button with a `ListFilter`
icon, shown when `selected.tasks.length > 0`, calling
`openEpicTickets(selected.key)`.

### 6. Header chip (FR9)

`web/src/components/Header.tsx`: read `activeView` from the context and render
the parent filter chip only when `activeView !== 'roadmap'`.

### 7. Strings

`web/src/locales/translations.ts` (type, `fr`, `en`):

The interface calls epics "macros" in both languages, so the strings do too.

| Key | fr | en |
| --- | --- | --- |
| `compactCard.openEpic` | Ouvrir la macro dans la roadmap | Open the macro in the roadmap |
| `planning.roadmap.panel.openTickets` | Ses tickets | Its tickets |
| `planning.roadmap.panel.openTicketsTitle` | Ouvrir les tickets de {key}, filtrés sur cette macro | Open the tickets of {key}, filtered on this macro |
| `planning.roadmap.focus.unknownTitle` | Macro introuvable | Macro not found |
| `planning.roadmap.focus.unknown` | {key} n'est pas une macro de la roadmap de ce projet. Relis les macros avec une synchro, ou vérifie que son projet tracker est déclaré. | {key} is not a macro of this project's roadmap. Re-read the macros with a sync, or check that its tracker project is declared. |

The list and the detail reuse `compactCard.openEpic` as their title, or get
their own key in their section if the catalog keeps one section per
component; follow what the file does for "Filter by parent".

### 8. Changelog

Under `## [Unreleased]`: `Added` "Open a ticket's epic in the roadmap from the
board, the list or the ticket detail, and go back to the tickets filtered on
it." `Changed` "The roadmap no longer applies the ticket views' parent filter,
so every epic keeps all its tickets."

## Rejected alternatives

- **Encoding the request in the URL.** The app has no router; a query string
  would be a first and would outlive the request.
- **Resolving the epic before leaving the ticket view.** The ticket side does
  not hold the macro list of another project, and under "all projects" no
  roadmap rows exist; the roadmap is the only place that knows what it holds.
- **Clearing the parent filter on entering the roadmap.** It would lose the
  user's ticket filter; not sending it from the roadmap's query keeps it for
  the way back.

## Target files

- `web/src/lib/roadmapFocus.ts` (new)
- `web/src/context/AppContext.tsx`
- `web/src/components/RoadmapView.tsx`
- `web/src/components/TaskCard.tsx`
- `web/src/components/ListView.tsx`
- `web/src/components/TaskDetailModal.tsx`
- `web/src/components/Header.tsx`
- `web/src/types/index.ts` (context type, if declared there)
- `web/src/locales/translations.ts`
- `web/tests/roadmapFocus.test.mjs` (new)
- `web/tests/roadmap-epic-navigation.browser.mjs` (new)
- `CHANGELOG.md`
