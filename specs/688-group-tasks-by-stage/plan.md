# Plan #688 - Group a project's tasks by stage

## Stack

Sectile Desktop renderer only: plain ES modules in `desktop/src`, the
stylesheet `desktop/src/style.css`, Node's test runner for the `.test.mjs`
and `.ui.cjs` suites in `desktop/tests`. No server, web or agent change.

## Branch base

`feat/688` carries four commits of the unmerged
`feat/desktop-conversation-test` branch (owner's choice, see the
clarification). None of them touches the sidebar order, the project menu or
the key styling; the line numbers below are those of this branch. Before
opening the pull request, check `git diff origin/main -- desktop/src/main.js`
to keep the change readable against `main`.

## Design

### 1. Stage of a sidebar row (FR8, FR9)

`refreshPRs` (`desktop/src/main.js`, around line 2833) already fetches
`api.serverTasks(projectId)` for every project with listed executions, at most
every 15 s, and fills `taskTitles` and `pullRequests` per `taskId`. Add a
third cache next to them (line 83):

```js
const taskStages=new Map()
```

and, in the same loop, `taskStages.set(run.taskId,taskStage(task))` when the
task is found and `taskStage(task)` is one of the six stages, else
`taskStages.delete(run.taskId)`. A failed request leaves the map as it was
(the existing `catch{}`), which gives the "last known stage" edge case.

`taskStage()` can return a raw status or `'Unknown'` for a task outside the
workflow: only the six stage names are stored, anything else is unstaged.
Export the stage list from `workflow.mjs` (`STAGES`) and make
`task-list-order.mjs` import it instead of its private `STAGE_ORDER` copy, so
there is one ordering.

Free consoles are already filtered out of `refreshPRs`; macro runs have no
task in the list, so both stay unstaged.

### 2. Ordering (FR3, FR4)

`orderedTaskGroups(groups)` in `desktop/src/task-order.mjs` gains an optional
second argument:

```js
export function orderedTaskGroups(groups,{stageOf}={})
```

Without `stageOf` the result is exactly today's (FR4). With it, groups are
compared first on the rank of `stageOf(group.run)` in `STAGES` (unknown ranks
last), then by the existing `compareGroups`. The stage is read from the
group's lead run: every execution of a group shares one `taskId`, so one
stage.

`main.js` (line 571) passes `{stageOf:run=>taskStages.get(run.taskId)}` when
the project is in `stageGroupedProjects`, nothing otherwise.

### 3. The option (FR1, FR2)

A set beside `collapsedProjects` (line 128):

```js
const stageGroupedProjects=new Set(readJSON('stageGroupedProjects',[]))
```

using the same `JSON.parse(localStorage.getItem(...)||'[]')` idiom, guarded
the way the existing read is. A helper `toggleStageGrouping(projectID)`
flips membership, writes `localStorage`, and calls `render()`.

`projectMenu` (line 456) gets one item after "Show execution queue":

```js
{label:'Group by stage',checked:stageGroupedProjects.has(project.id),run:()=>toggleStageGrouping(project.id)}
```

The item loop learns `checked`: such an item gets `role="menuitemcheckbox"`
and `aria-checked`, and a leading check glyph styled like the menu text. The
keyboard handler selects `[role=menuitem]`; widen its selector (and the
`openAt` focus selector) to `[role^=menuitem]` so the new entry stays
reachable with the arrows.

`requestRemoveProject` drops the project from `stageGroupedProjects` and
rewrites the key once `api.removeProject` succeeds.

### 4. The tint (FR5, FR6, FR7)

On the key button built at line 576, when `taskStages.get(run.taskId)` is set:
`context.dataset.stage=stage` and `context.title='Open task in Sectile · Stage: '+stage`.
An unstaged row gets neither, so it renders as today.

`style.css`: six tokens per theme in the two token blocks (the dark `:root`
and the `prefers-color-scheme:light` one), the only place colour literals
belong:

```
--stage-new-bg  --stage-clarified-bg  --stage-specified-bg
--stage-implemented-bg  --stage-reviewed-bg  --stage-finished-bg
```

low-alpha tints of six distinct hues (for example slate, sky, violet, amber,
teal, green), and one rule:

```css
.local-task .task-key-slot>.task-number[data-stage]{padding:1px 5px;border-radius:4px}
.local-task .task-key-slot>.task-number[data-stage=new]{background:var(--stage-new-bg)}
/* … one per stage */
```

The key text keeps `--task-key`; the tint stays light enough for it to read
in both themes. The padding must not widen the `task-key-slot` (`min-width:5ch`)
beyond what `sidebar-alignment.ui.cjs` accepts; check it, and widen the slot
for every row rather than for tinted rows only, so the columns stay aligned.

### 5. Re-render (FR9)

`refreshPRs` ends with `render()`, which already honours the sidebar hold
(`sidebarBusy`, the deferred render). No new trigger is needed: a stage
change reaches the sidebar on the next refresh, held while the pointer or a
rename is in the sidebar.

### 6. Changelog (FR10)

Under `## [Unreleased]` → `### Added`:

> **Desktop groups a project's tasks by stage.** Choose **Group by stage** in
> a project's `…` menu to list its tasks from new to finished; every task key
> in the sidebar is tinted after its stage. (#688)

## Target files

- `desktop/src/workflow.mjs`: export `STAGES`.
- `desktop/src/task-list-order.mjs`: import `STAGES`.
- `desktop/src/task-order.mjs`: `stageOf` option.
- `desktop/src/main.js`: `taskStages`, `stageGroupedProjects`,
  `toggleStageGrouping`, the menu item and its checkbox role, the key's
  `data-stage` and title, removal cleanup.
- `desktop/src/style.css`: six tokens per theme, the tint rules.
- `desktop/tests/task-order.ui.cjs`: ordering with `stageOf`.
- `desktop/tests/workflow.test.mjs`: `STAGES` export.
- a new `desktop/tests/stage-grouping.ui.cjs`: menu, persistence, tint.
- `CHANGELOG.md`.

## Rejected alternatives

- **Section headers per stage**: rejected by the owner (clarification
  decision 2).
- **A global setting in Settings → Appearance**: rejected by the owner for a
  per-project menu entry (decision 4).
- **Tint only while grouped**: rejected by the owner (decision 3).
- **Fetching each task on render**: one request per row; the project task list
  is already fetched.
- **Reading the stage from the run**: a run records the skill it ran, not the
  task's current stage, and goes stale after the transition.

## Risks

- The 15 s throttle of `refreshPRs` delays a stage change by up to 15 s.
  Accepted: the same delay already applies to titles and pull requests.
- A tinted key changes the visual weight of the sidebar; the tints must stay
  subtle. Review both themes by screenshot.

## Test plan

- Unit (`task-order.ui.cjs`): without `stageOf` the existing expectations
  hold; with it, stages order new → finished, ties keep the current order
  (active before queued before finished runs), unstaged groups last, input
  not mutated.
- Unit (`workflow.test.mjs`): `STAGES` lists the six stages in order.
- UI (`stage-grouping.ui.cjs`, Electron, needs `npx vite build` first and the
  sandbox off): the menu shows the entry unchecked; checking it reorders the
  project's rows and sets `aria-checked="true"`; the choice survives a reload;
  a second project is unaffected; keys carry `data-stage` and the stage in
  their title; a free console's key slot has no `data-stage`.
- Existing UI suites (`task-order-render`, `task-order-hold`,
  `sidebar-alignment`, `workflow`) pass unchanged.
