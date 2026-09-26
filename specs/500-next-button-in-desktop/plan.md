# Plan #500 - Current and Next labels on the desktop workflow button

Implementation choices for `spec.md`. Behaviour lives there; this file says
where and how.

## Stack

- Electron desktop renderer only: `desktop/src/main.js` and
  `desktop/src/workflow.mjs`. No server, migration, MCP, agent or tracker
  change: the run list the renderer already polls (`GET /desktop/runs`)
  carries `id`, `taskId`, `projectId`, `skill`, `status` and `createdAt`.
- Tests: `node --test` unit tests (`desktop/tests/*.test.mjs`) and the
  Playwright Electron UI tests (`desktop/tests/*.ui.cjs`, run after
  `npx vite build`).

## What exists

- `renderNextStep()` (`desktop/src/main.js`, around line 2359) computes
  `busy` (any active run of the task, `activeRun()`), `pending` (a key in
  `submittingSteps` or `submittedSteps`), then sets
  `button.textContent='Next: '+step.label` only when `step.skillId` is set,
  disabled on `busy||pending`.
- `launchNextStep()` adds the key to `submittingSteps`, rereads the step
  (`fresh`), and after a successful launch records
  `submittedSteps.set(key,{skillId:fresh.step.skillId,runIds})`.
  `submittingSteps` holds keys only: the skill being submitted is not stored.
- `nextTaskStep()` (`desktop/src/workflow.mjs`) returns
  `{stage,skillId,label,message}`; the `skills` table pairs ids and labels.
- `updateTicketRow()` picks the representative execution with
  `executions.find(activeRun)||...` sorted by `createdAt`; the toolbar needs
  the most recent active one, so it sorts active runs by `createdAt`
  descending.

## Decisions

### 1. Skill label helper in `workflow.mjs`

Export `skillLabel(skillId)`:

```js
const skillLabels={clarify:'Clarify',specify:'Specify',implement:'Implement',adjust:'Adjust',handoff:'Handoff',create_pr:'Create PR'}
export function skillLabel(skillId){
 const id=String(skillId||'').trim()
 return skillLabels[id]||(id?id[0].toUpperCase()+id.slice(1):'')
}
```

The `skills` table keeps its own labels, because the stage step `Create PR`
launches `implement` or `specify`: the step label and the skill label differ
on purpose (US2 scenario 2). `create_pr` is listed so the stage-neutral skill,
when launched from the Tickets menu, reads `Create PR`.

### 2. Remember the skill being submitted

Turn `submittingSteps` from a `Set` of keys into a `Map` from key to skill id:
`launchNextStep()` stores `displayed.step.skillId` when it starts, and
replaces it with `fresh.step.skillId` once the recheck returns (the recheck
aborts the launch when the two differ, so in practice they are equal). Every
`submittingSteps.has(key)` call keeps working on a `Map`; the `add` and
`delete` calls become `set` and `delete`.

### 3. Resolve the current skill in `renderNextStep()`

After the early returns for no run, free console, loading and error (FR6
keeps them first), compute:

```js
const active=runs.filter(item=>taskKey(item)===key&&activeRun(item)).sort((a,b)=>(b.createdAt||'').localeCompare(a.createdAt||''))[0]
const current=active?.skill||submittingSteps.get(key)||submittedSteps.get(key)?.skillId||''
```

- `current` set: `button.hidden=false`,
  `button.textContent='Current: '+(skillLabel(current)||step.label||'')`,
  `button.disabled=true`. This branch does not require `step.skillId` (FR7).
- `current` empty: the existing `Next:` branch, unchanged.

The fallback to `step.label` only covers a run with an empty `skill`, which
the server does not produce for a task run today; it avoids a bare
`Current: ` label. If both are empty the text reads `Current:` without a
name, which is acceptable for an impossible case.

`busy` and `pending` keep driving the footer message, "Mark reviewed" and
"Launch anyway" exactly as today (FR8). The `submittedSteps` cleanup line
stays where it is, before `current` is computed, so a submitted entry is
dropped as soon as its run appears, and the active run then provides the
label.

### 4. Refresh

No new polling: `renderNextStep()` already runs from `refresh()` whenever the
run list changes, and `refreshNextStep()` rereads the stage when a run ends.
US3 relies on that existing path; the test checks it rather than adding one.

## Rejected alternatives

- **Using the stage step for `Current:`**: rejected by the owner in the
  clarification (a pickup would read `Current: Clarify`).
- **Using `runLabel(run)`**: it appends the engine (`specify · opus`) and
  names free consoles; the toolbar needs the skill alone, in the workflow's
  casing.
- **A separate "current skill" element next to the button**: the ticket asks
  for the button's own label to change, and a second element would need its
  own layout and accessibility work.

## Target files

| File | Change |
| --- | --- |
| `desktop/src/workflow.mjs` | `skillLabel()` export |
| `desktop/src/main.js` | import `skillLabel`; `submittingSteps` becomes a `Map`; `Current:` branch in `renderNextStep()` |
| `desktop/tests/workflow.test.mjs` (new; no unit test file covers `workflow.mjs` today) | `skillLabel()` cases |
| `desktop/tests/next-step.ui.cjs` | `Current:` assertions (see `tasks.md`) |
| `CHANGELOG.md` | one `Changed` line under `[Unreleased]` |

## Risks

- Other UI tests (`concurrent-launch.ui.cjs`, `disconnect.ui.cjs`) look the
  button up by its `Next: <label>` name. Those lookups happen on idle tasks
  today, but any lookup made while a run is active must switch to the
  `Current:` name. Run the whole `test:ui` suite, not only `next-step`.
- `desktop/tests/*.ui.cjs` load `dist/index.html`: run `npx vite build`
  first, and restore `internal/webui/dist/.gitkeep` if the web build deletes
  it (see the repository memory).
