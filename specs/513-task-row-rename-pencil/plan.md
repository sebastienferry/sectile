# Implementation Plan: Pencil rename instead of the task row "…" menu

**Spec**: [spec.md](spec.md)
**Ticket**: [#513](https://github.com/sebastienferry/sectile/issues/513)
**Branch**: `feat/513`

## Technical context

- **Scope**: Sectile Desktop renderer only (`desktop/src/main.js`,
  `desktop/src/style.css`) and its UI tests. No server, API, database
  migration, Electron main-process or tracker change.
- **Stack**: plain DOM JavaScript in the renderer, Playwright-driven Electron
  UI tests (`desktop/tests/*.ui.cjs`, run by `npm run test:ui` after
  `npx vite build`, see the repository memory on desktop UI tests).
- **State**: the local name is `localTasks[taskKey(run)].name`, persisted by
  `saveLocalTasks()` into `localStorage`. `taskState(run)` reads it. Nothing
  changes in that storage format.

## Current code

- `desktop/src/main.js`, `render()` (around line 493): the task row loop
  builds `row` (`.local-task`) from `state` (`.run-state`), `keySlot`
  (`.task-key-slot`), `button` (`.run`, holding the `<strong>` title and the
  skill status), an optional `.pr-indicator`, `archive` (`.task-archive`) and
  `menu` (`.task-menu`, the "…" button, around line 544).
- `taskMenu(run)` (around line 2434): the dialog with Relaunch, Detach,
  Rename locally (a `<form>` with an `aria-label="Local task name"` input,
  `maxLength=120`) and Archive.
- `sidebarBusy()` (line 109) defers `render({deferrable:true})` while the
  pointer is over a `.local-task` or the focus is inside one; the periodic
  `refresh()` uses that deferral, but about thirty other `render()` calls are
  not deferrable (a launch, a stop, a selection, a PR link load...).
- `window` `keydown` listener (around line 802) closes the ticket pane on
  Escape when no dialog is open.
- `desktop/src/style.css`: `.task-menu` shares the hover/focus/touch opacity
  rules of the project row buttons; `.task-archive` has its own identical set.

## Design

### Edition state survives re-renders

Because non-deferrable `render()` calls rebuild the whole sidebar, the inline
field cannot live only in the DOM. A module-level variable holds the edition:

```js
// The task row whose title is being edited inline, and what was typed so far.
let renaming=null // {key:taskKey(run),draft:string}
```

- The pencil sets `renaming={key,draft:<displayed name>}` and calls
  `render()`.
- In the row loop, when `renaming?.key===taskKey(run)`, the row renders an
  `<input>` (`.task-rename`) **in place of the `.run` button** (an input
  cannot nest inside a button). The state glyph, the key slot, the PR
  indicator, the archive button and the pencil stay. The input gets
  `value=renaming.draft`, `maxLength=120`, `aria-label='Local task name'`,
  and on `input` updates `renaming.draft`.
- After the row is appended, the input is focused. When the edition has just
  started, its text is selected; when the row is only being rebuilt, the caret
  is restored to the end instead, so a background render does not reselect
  what the user is typing over. A flag in `renaming` (`fresh:true`, cleared
  after the first render) distinguishes the two.
- Removing a focused element does not reliably fire `blur`, but when it does
  during a rebuild, the save must not happen. The rebuild sets a
  `rebuilding` guard around `list.replaceChildren()` and the blur handler
  ignores events while it is set, or while the input is no longer the one
  attached for `renaming`.
- If the row being edited is no longer rendered (task archived, project
  disconnected), `render()` clears `renaming` without saving.

### Ending the edition

A single `finishRename(run,commit)`:

```js
function finishRename(run,commit){
 if(!renaming||renaming.key!==taskKey(run))return
 const value=renaming.draft.trim(),current=displayedName(run)
 renaming=null
 if(commit&&value&&value!==current){localTasks[taskKey(run)]={...taskState(run),name:value};saveLocalTasks()}
 render()
 // Focus back on the row's pencil, looked up by task key after the rebuild.
}
```

- `keydown` on the input: Enter → `preventDefault()`, `finishRename(run,true)`;
  Escape → `preventDefault()`, `stopPropagation()` (the window listener would
  otherwise close the ticket pane), `finishRename(run,false)`.
- `blur` on the input → `finishRename(run,true)`, subject to the rebuild
  guard above. Pressing another row's pencil blurs the field first, which
  saves it, then starts the new edition.
- `displayedName(run)` factors the expression the row already uses for its
  title: `taskState(run).name||taskTitles.get(run.taskId)||runLabel(run)`.
  The pencil label uses the same name, so the row, the tooltip and the field
  agree.
- The pencil is found again after the rebuild by a `data-task-key` attribute
  (the JSON task key) on the row, then `.task-rename-button` inside it.
- `click` and `pointerdown` on the input stop propagation, so the row never
  treats them as a selection.

### The pencil button

- Built where `menu` is built today, appended after `archive`:
  `className='task-rename-button'`, `aria-label` and `title`
  `'Rename '+displayedName(run)`, an inline SVG drawn like the archive icon
  (`viewBox="0 0 24 24" width="16" height="16" fill="none"
  stroke="currentColor" stroke-width="1.6"`), path for a pencil, e.g.
  `<path d="M4 20h4L19 9l-4-4L4 16z"/><path d="m13.5 6.5 4 4"/>`.
- Its `onclick` starts the edition. It does not call `select(run)`.

### Removals

- Delete the `menu` button and `taskMenu(run)`. `requestArchive` and
  `archiveTask` stay (the archive button uses them).
- The toolbar's `#rerun` and `#detach-terminal` handlers are untouched.

### Stylesheet

- Replace `.task-menu` in the shared opacity rules by nothing, and give
  `.task-rename-button` the `.task-archive` rules: transparent, borderless,
  `opacity:0`, shown on `.local-task:hover` / `:focus-within`, always shown
  under `@media(hover:none)`, `color:var(--text-muted)`, hover
  `color:var(--text)`. Simplest: extend the three `.task-archive` selectors
  with `.task-rename-button`, then override only the hover colour.
- `.task-rename`: `flex:1;min-width:0;margin:0 0 0 8px;padding:5px 8px;
  font-size:12px`, keeping the row height (the PR display test checks that
  controls stay on one line of the row). Colours come from existing tokens
  (`--surface`, `--input-text`, `--border-strong`, focus outline
  `--accent`), so the appearance test that forbids colour literals keeps
  passing.
- Delete the now unused `.task-menu` rules (lines 174, 178, 179, 182).

## Tests

UI tests (`desktop/tests/*.ui.cjs`), rewritten where they drive the removed
dialog:

- `console.ui.cjs` (around lines 251-260): the rename through
  `Actions for #48` becomes: hover the row, press `Rename #48`, fill the
  `Local task name` textbox with `Local review`, press Enter, wait for the
  `Rename Local review` button. The check that the dialog also offers
  `Detach to native terminal` is removed; the toolbar check just above it
  stays. The later `Stop and archive Local review` step is unchanged.
- `task-header.ui.cjs` (around lines 122-124): the same rewrite with
  `Rename #82` / `Local title`; the tracker-title and restart checks stay.
- `pr-display.ui.cjs` (line 47): `.task-menu` becomes `.task-rename-button`
  in the list of controls that must stay within the row.
- The `Relaunch` clicks in `console.ui.cjs`, `free-console.ui.cjs` and
  `skill-mode.ui.cjs` already hit the toolbar's `#rerun` (the dialog button
  only existed once the dialog was open); verify they still resolve to a
  single element and leave them as they are.

New UI test `desktop/tests/task-rename.ui.cjs`, following the harness of
`task-header.ui.cjs` (fake server, one tracker task, one free console, one
macro run):

1. No `.task-menu` and no button named `Actions for <task>` in any row.
2. Each of the three rows has a `Rename <name>` button; its opacity is 0
   until the row is hovered, then 1.
3. Pencil → textbox focused with the full name selected; type, Enter → row
   title and toolbar title show the new name; `localStorage.localTasks` holds
   it.
4. Pencil → type → click elsewhere → saved.
5. Pencil → type → Escape → previous name, ticket pane (opened beforehand)
   still open.
6. Pencil → clear → Enter → previous name, no `name` written.
7. Pencil → type a few characters → advance the fake server so that a
   non-deferrable render happens (e.g. a run status change) → the field is
   still open, focused, with the typed text.
8. After Enter or Escape, the row's pencil has the focus.
9. The field accepts at most 120 characters.
10. Free console and macro run rows rename too.

Run: `npx vite build` then `npm run test:ui` in `desktop/`, and `npm test`
(the stylesheet tests of `appearance.test.mjs`).

## Documentation

- `CHANGELOG.md`, `## [Unreleased]` → `### Changed` (create the section if
  absent, after `### Added`): one line such as "**Rename a task from its
  sidebar row.** In Sectile Desktop, the "…" button of a task row is replaced
  by a pencil that edits the task's local name in place: Enter or clicking
  away saves, Escape cancels. Relaunch and Detach stay in the toolbar of the
  selected task, Archive on the row. (#513)"
- `desktop/README.md`: line 53 ("Rename and archive are available through
  the console menu") and line 521 ("A task's **…** menu provides relaunch,
  local rename and archive actions") describe the removed menu; rewrite both
  to say that a task row has a pencil for the inline local rename and an
  archive button, relaunch and detach being toolbar buttons.

## Rejected alternatives

- **A dialog reduced to the name field**: rejected by the owner in the
  clarification (round 2).
- **Editing the `<strong>` with `contenteditable`** inside the `.run`
  button: interactive content inside a button is invalid, a click would also
  select the run, and `maxLength` does not apply to `contenteditable`.
- **Relying on `sidebarBusy()` alone** to protect the field: only the
  periodic refresh is deferrable; selecting a run, a launch or a stop call
  `render()` directly and would drop the edition.

## Risks

- A UI test that counts the buttons of a row or relies on `.task-menu` being
  last; `grep -rn "task-menu\|Actions for #" desktop/tests` after the change
  must return nothing.
- The focus return and caret restoration are timing-sensitive in Playwright;
  assert with `toBeFocused()` polling rather than immediate evaluations.
