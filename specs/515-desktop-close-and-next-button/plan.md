# Plan #515 - Next-step and full-chain icon buttons in the desktop toolbar

Implementation choices for `spec.md`. Behaviour lives there; this file says
where and how. Line numbers refer to `origin/main` at `abc03dce`; merge it
first, since #510 reshaped `desktop/src/main.js`.

## Stack

- Electron desktop renderer only: `desktop/src/main.js` and
  `desktop/src/style.css`. No server, migration, MCP, agent or tracker change:
  `POST /desktop/tasks` already accepts `mode` and `force`, and
  `GET /desktop/project` already lists `server.skills`.
- Tests: the Playwright Electron UI tests (`desktop/tests/*.ui.cjs`, run after
  `npx vite build`) and `node --test` for the unit tests.

## What exists

- Toolbar markup (`main.js:42`): `... #stop, #next-step (text button),
  #mark-reviewed, #retry-next-step, #force-next-step`.
- `iconPaths` (`main.js:2065`) and the loop after it fill each listed id with
  a 24x24 stroke SVG (`currentColor`, `aria-hidden`), once, at start-up.
- `renderNextStep()` (`main.js:2663`) resets every control, returns early for
  no run, free console, loading and error, computes `busy`, `pending` and
  `current` (#500), then sets `button.textContent` to `Current: ...` or
  `Next: ...`. Setting `textContent` on an icon button would erase its SVG.
- `launchNextStep(force)` (`main.js:2719`) holds the busy guard, the
  freshness recheck (`readNextStep()` + `api.runs()`), the launch
  (`api.launchServerTask(projectId, taskId, skillId, '', undefined, force)`),
  the `submittedSteps` bookkeeping, the refusal handling
  (`refusedActiveRun()`, `forceableLaunches`) and the console selection.
- `forceableLaunches` (`main.js:65`) is a `Set` of task keys: it records that
  a launch was refused, not which one.
- `nextStepData.project` is not kept: `readNextStep()` reads the project to
  compute the step and drops it.
- `launch-server-task` (`electron/main.cjs:344`) adds `mode` to the body only
  when it is set, so `>` keeps sending none.

## Decisions

### 1. Markup and order

Keep the id `#next-step` for `>` (every existing test and style targets it),
add `#pickup-chain` for `>>` and `#next-step-label` for the badge:

```
#stop, #next-step, #pickup-chain, #next-step-label, #mark-reviewed, #retry-next-step, #force-next-step
```

- `#next-step` and `#pickup-chain` take the `icon-button` class; both start
  `hidden disabled`.
- `#next-step-label` is a `<span class="step-badge" aria-hidden="true" hidden>`.
- `#pickup-chain` gets `aria-label` and `title` `Pickup (full chain)` in the
  markup; they never change.

### 2. Icons

Add two `iconPaths` entries, the lucide glyphs the web card uses:

```js
'next-step':'<path d="m9 18 6-6-6-6"/>',
'pickup-chain':'<path d="m6 17 5-5-5-5"/><path d="m13 17 5-5-5-5"/>',
```

### 3. `renderNextStep()`

- Reset: also hide and disable `#pickup-chain`, hide `#next-step-label`.
- Replace `button.textContent=...` by a helper that sets the button's
  `aria-label` and `title` and the badge's `textContent` to the same string,
  and shows the badge whenever `button.hidden` is false (FR3).
- `>>` (FR5, FR6, FR10), after the loading and error early returns:

  ```js
  if(pickupAvailable(nextStepData.project)&&taskStage(nextStepData.task)!=='finished'){chain.hidden=false;chain.disabled=busy||pending}
  ```

  with `pickupAvailable(project)=Boolean(project?.configured&&project.server?.skills?.some(skill=>skill.id==='pickup'))`.
- `busy`, `pending` and `current` are unchanged: a `>>` launch in flight puts
  `pickup` into `submittingSteps` / `submittedSteps`, so `current` resolves to
  `Pickup` through the #500 path.

### 4. Keep the project in `nextStepData`

`readNextStep()` returns `{key, task, project, step}`. The project is already
fetched there; no new request.

### 5. One launch function for both buttons

Generalise `launchNextStep(force)` into `launchTaskWork(kind, force)` with
`kind` `'next'` or `'pickup'`, keeping a single guard, recheck and
bookkeeping path:

- Skill: `'next'` -> `fresh.step.skillId`; `'pickup'` -> `'pickup'`.
- Precondition on the displayed data: `'next'` needs `step.skillId`;
  `'pickup'` needs `pickupAvailable(project)` and a non-finished task.
- Recheck (FR8): both abandon on an active run in `latestRuns`. `'next'`
  also abandons when `fresh.step.skillId` differs from the displayed one (as
  today); `'pickup'` abandons when the fresh task is finished or pickup is no
  longer available, and ignores a stage step change.
- Launch: `api.launchServerTask(projectId, taskId, skill, '', kind==='pickup'?'autonomous':undefined, force)`.
- `submittingSteps.set(key, skill)` and
  `submittedSteps.set(key, {skillId: skill, runIds})` as today.
- `submittedSteps` cleanup in `renderNextStep()` drops an entry whose
  `skillId` differs from the stage step. For a pickup that would drop it at
  once and flash `Next:` until the run appears. Store `kind` in the entry and
  skip the stage-step comparison when `kind==='pickup'`; the "a new run
  appeared" condition still clears it.
- Error message on failure: `'Could not launch next step: '` for `'next'`,
  `'Could not launch full chain: '` for `'pickup'` (the footer is an English
  surface already).

`#next-step.onclick=()=>launchTaskWork('next',false)`,
`#pickup-chain.onclick=()=>launchTaskWork('pickup',false)`.

### 6. "Launch anyway" re-sends the refused launch (FR9)

`forceableLaunches` becomes a `Map` from task key to `kind`: the refusal
branch stores the kind that was refused, `#force-next-step.onclick` calls
`launchTaskWork(forceableLaunches.get(key)||'next', true)`. Its visibility
rule `force&&step.skillId&&forceableLaunches.has(key)` becomes
`forceableLaunches.has(key)&&(kind==='pickup'?chainVisible:step.skillId)`.
Every `.add` becomes `.set(key, kind)`; `.has` and `.delete` keep working.

### 7. Styles (`style.css`)

- `#next-step` keeps its `--next-step-*` colours; `#pickup-chain` shares
  them, so the two launch controls read as one group, distinct from the
  green closing button.
- `.step-badge`: `font-size:11px`, `color:var(--text-muted)`,
  `border:1px solid var(--border)`, `border-radius:999px`, `padding:2px 8px`,
  `white-space:nowrap`, `flex-shrink:0`. It uses existing tokens only, so it
  follows the dark and light appearances of #507.
- Add `#pickup-chain` to the `flex-shrink:0` and `:focus-visible` rules of
  `#next-step`.

## Rejected alternatives

- **Chevron plus the label inside the button**: recommended in round 1,
  rejected by the owner in round 2.
- **Hiding `>>` during a run**: rejected in the clarification (the toolbar
  would shift).
- **Leaving the mode to the configured precedence**: rejected in the
  clarification; a chain is autonomous by construction, as on the web.
- **A second, separate launch function for `>>`**: it would duplicate the
  busy guard, the recheck and the refusal handling that #500 and the
  concurrent-launch work tuned; one function with a `kind` keeps them in one
  place.
- **Making the badge the live region**: the footer `#next-step-status`
  already is one; announcing the same text twice is noise.

## Target files

| File | Change |
| --- | --- |
| `desktop/src/main.js` | markup (`#pickup-chain`, `#next-step-label`, `icon-button` on `#next-step`); two `iconPaths`; `readNextStep()` keeps the project; `renderNextStep()` label helper and `>>` rules; `launchTaskWork(kind, force)`; `forceableLaunches` as a `Map` |
| `desktop/src/style.css` | `.step-badge`; `#pickup-chain` joins the `#next-step` rules |
| `desktop/tests/next-step.ui.cjs` | `textContent` assertions move to `aria-label` and badge text; DOM order assertions; `>>` cases (see `tasks.md`) |
| `desktop/tests/concurrent-launch.ui.cjs` | "Launch anyway" after a refused `>>` re-sends `pickup` with `force` and `mode` |
| `CHANGELOG.md` | one `Changed` line under `[Unreleased]` |

## Risks

- `next-step.ui.cjs:67,70,82` read `button.textContent()`; with an icon
  button it is empty. They must read the `aria-label` (or the badge text).
  The `getByRole('button',{name:...})` lookups keep working through
  `aria-label`, in every `*.ui.cjs` file.
- `next-step.ui.cjs:150,153` assert the sibling order around `#next-step`;
  update them to decision 1's order.
- `skill-mode.ui.cjs` asserts the footer next-step button sends no mode; `>`
  must keep calling `launchServerTask` with `undefined` as mode.
- `desktop/tests/*.ui.cjs` load `dist/index.html`: run `npx vite build`
  first, and restore `internal/webui/dist/.gitkeep` if a build removed it.
- Worktrees may have no `node_modules`; see the repository memory before
  running the desktop tests.
