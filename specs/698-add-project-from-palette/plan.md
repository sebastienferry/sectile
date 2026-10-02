# Plan #698 - Add a project from the command palette

## Stack

Sectile Desktop renderer only: `desktop/src/main.js`, Playwright Electron UI
tests in `desktop/tests/*.ui.cjs` run by Node's test runner. No server, web or
agent change, no new module.

## Design

### 1. A named Add project function (FR3, FR5)

The Add project flow is today the anonymous `onclick` of `#add-project`
(`desktop/src/main.js:2032`). Turn it into a named async function, keeping its
body unchanged:

```js
async function openAddProject(){
 showDialog('Add project')
 // ...current body, unchanged...
}
document.querySelector('#add-project').onclick=openAddProject
```

Both entry points then run the same code, so the dialog cannot drift between
them (FR3). Calling `.click()` on the button from the palette was rejected: the
button is hidden by opacity until hovered and sits in a sidebar that can be
collapsed, and a synthetic click on UI chrome hides the dependency.

Function declarations are hoisted, so its position relative to `COMMANDS` does
not matter; keep it where the handler is today to keep the diff small.

### 2. The palette row (FR1, FR2, FR4, FR6)

Add one row at the end of `COMMANDS` (`desktop/src/main.js:2975`):

```js
const COMMANDS=[
 {label:'Quick add task',run:()=>quickAdd()},
 {label:'Tasks list',run:()=>openTicketsFromPalette()},
 {label:'Add project',run:()=>openAddProject()}
]
```

`openCommandPalette` already builds a button per row, filters on the label and
runs the first visible one on Enter, so FR2 and FR4 need nothing else.
`openAddProject` begins with `showDialog`, which replaces the palette's content,
as the other two commands do. It reads neither the selection nor the sidebar
state, which gives FR6.

### 3. Documentation

- `desktop/README.md:786`: the palette paragraph names its commands; add
  **Add project** ("choose **Quick add task**, **Tasks list** or **Add
  project**"). The section is titled "Desktop Quick add"; leave the title.
- `CHANGELOG.md`, `[Unreleased]` / `Added`: "**Add a project from the Desktop
  command palette.** Cmd+K (Ctrl+K) now offers **Add project**, which opens the
  same dialog as the `+` next to PROJECTS, also when the sidebar is collapsed.
  (#698)"

## Tests

A new `desktop/tests/palette-add-project.ui.cjs`, modelled on
`project-open-tasks.ui.cjs` (a local `http` server standing for the agent, an
`agent-connection.json` in a temporary `SECTILE_DESKTOP_DATA_DIR`, Electron
launched with `SECTILE_DESKTOP_TEST=1`):

- `/desktop/projects` returns two projects; the desktop data marks one as added
  (check how `console.ui.cjs` gets "Example project · Already added" and reuse
  that setup).
- Control+K, fill "Search commands" with "tasks": **Add project** hidden.
- Fill it with "project", press Enter: the dialog heading "Add project" shows,
  with the added project's button disabled and labelled "· Already added", the
  other enabled.
- Close; collapse the sidebar with `#toggle-sidebar` (the Cmd+B / Ctrl+B
  mapping depends on the platform the test runs on); Control+K, click
  **Add project**: the dialog opens, the sidebar stays collapsed.
- Close; click `#add-project` after expanding the sidebar: same dialog (FR5).

Scenarios 3, 4 and 6 run the unchanged body of the dialog, already covered by
`console.ui.cjs`, `disconnect.ui.cjs` and the hide-from-sidebar tests; they are
not duplicated.

`project-open-tasks.ui.cjs:61` filters on "tasks" and only asserts that **Quick
add task** is hidden: it stays green with the new row.

## Gates

- `cd desktop && npx vite build` before the UI tests (they load
  `dist/index.html`), then `node --test tests/`, with the sandbox off for the
  Electron suites.
- Restore `internal/webui/.gitkeep` if a build removed it.
- `git diff origin/main --stat`: only `desktop/src/main.js`, the new test,
  `desktop/README.md`, `CHANGELOG.md` and the spec files.

## Rejected alternatives

- Dispatching a click on `#add-project` from the palette (see design 1).
- Labelling the command "Add a remote project", the `+` tooltip: the dialog,
  the README and the changelog all say "Add project".
- A palette command creating a project on the server: out of scope by the
  owner's decision.
