# Plan #791 - Desktop: Merge request

## Stack

- Sectile Desktop, Electron renderer in plain JavaScript: `desktop/src`.
- No agent, server, database, tracker or MCP change. No child process is
  started, so the `agentexec.Hidden` rule does not apply.

## Data contract

None changes. The renderer keeps reading `pullRequests.get(taskId)`, filled by
`repositoryPullRequests(task)` in `desktop/src/pullRequests.mjs`: one link per
repository, `{url, repository, state, missingToken}`, the primary
repository's first. That list already excludes superseded pull requests, which
is what FR-003 asks for.

## Design decisions

### D1 - Markup of the toolbar

In the toolbar markup (`desktop/src/main.js`, the `#toolbar` template on line
63), `<span id="selected-pr-others" hidden></span>` is replaced by:

- `<button id="selected-pr-more" class="icon-button selected-pr-more"
  type="button" aria-haspopup="menu" aria-expanded="false"
  aria-controls="selected-pr-menu" aria-label="Pull requests of this task"
  title="Pull requests of this task" hidden>`, holding a `<span
  class="pr-count">` for `+N` and the same down-chevron SVG as
  `#worktree-folders`;
- `<div id="selected-pr-menu" class="folder-menu pr-menu" role="menu"
  aria-label="Pull requests of this task" hidden></div>`, placed beside it.

`#selected-pr` is untouched. The menu reuses `.folder-menu` for its fixed
positioning, surface, border, shadow and `[hidden]` rule.

### D2 - One menu helper for the toolbar's two menus

The open, close, dismiss, position and keyboard logic of the folders menu
(`openFoldersMenu`, `closeFoldersMenu`, the `onkeydown` handler, lines
534-583) is extracted into one function in `main.js`:

```js
// A chevron that opens a menu of buttons built at opening, fixed below it.
function toolbarMenu(button,menu,items){ ... return {open,close} }
```

`items()` returns the menu item elements, or an empty array to stay closed.
The helper owns `aria-expanded`, the viewport-clamped positioning, the
capture-phase `pointerdown` and window `blur` dismissal, Escape (closing and
refocusing the opener), ArrowDown on the closed opener, Tab, and
ArrowDown/ArrowUp/Home/End with wrapping. The folders menu is rewired on it
with no behaviour change; `folder-menu.ui.cjs`, `run-folders.ui.cjs`,
`attached-folders.ui.cjs` and `spec-folder.ui.cjs` stay green as its
regression guard.

Rejected: a second copy of the folders code for pull requests. It is about
forty lines of keyboard and dismissal handling that would drift apart.
Rejected: a native `<select>`, which cannot show the coloured state icon
(settled in round 1).

### D3 - Entries of the menu

A pure helper in `desktop/src/pullRequests.mjs`, unit-tested with `node
--test`:

```js
// The entries of the toolbar's pull request menu: every repository's current
// pull request, primary first. Empty below two, where the button suffices.
export function pullRequestMenuEntries(links)
// -> [{url, repository, name, label, link}]
```

- `name`: last segment of `repository` (what `repositoryName` computes today in
  `main.js`; move that function into `pullRequests.mjs` and import it).
- `label`: `PR #n` / `MR !n` (move `prLabel` the same way; its callers in
  `main.js` import it).
- `link`: the original link, given to `renderPullRequestIndicator`.

Each menu item is a `<button role="menuitem">`:

- a `<span class="pr-menu-icon">` rendered by
  `renderPullRequestIndicator(icon, link, label+' in '+repository)`, which sets
  the icon, the state colour and the tooltip;
- a `<span class="pr-label">` reading `name+' '+label` (`deploy MR !7`);
- the item's `title` and `aria-label` copied from the icon span, so the entry
  reads `Open MR !7 in gitlab.com/example/deploy`, the separator, then the
  state, exactly as today's other-repository button. The icon span then drops
  its own `title` and `aria-label`, and gets `aria-hidden="true"`.
- `onclick`: close the menu with focus back on the chevron, then
  `api.openPR(url).catch(error)`.

The primary entry carries its repository name like the others (round 2: each
entry shows the repository name), so `app PR #79` comes first.

### D4 - Rendering the toolbar

In the selected-task block (lines 831-850), the `selectedOthers` code is
replaced:

- `const entries=pullRequestMenuEntries(links)`;
- `more.hidden=!entries.length`;
- `more.querySelector('.pr-count').textContent='+'+(entries.length-1)`;
- if the menu is open and either the selected execution changed since it
  opened or `entries` is empty, close it (the `foldersRun` pattern, with a
  `prMenuTask` holding the task id it was opened for).

The menu content is built by the `items()` callback at opening (FR-006), never
on refresh.

### D5 - Styles

In `desktop/src/style.css`:

- remove the `#selected-pr-others` and `.selected-pr-other` rules (line 329);
- `.selected-pr-more`: the `#selected-pr` sizing (`gap:5px;width:auto;
  padding:8px 6px;font-size:12px;white-space:nowrap;color:var(--link)`), the
  chevron SVG at 12px;
- `.pr-menu button`: a single-row flex (`display:flex;align-items:center;
  gap:8px`) overriding the two-column grid of `.folder-menu button`;
  `.pr-menu-icon` 14px with the state colour; `.pr-label` in `var(--text)`.

`.pr-others` (the sidebar and tickets table badge) is untouched.

### D6 - Documentation

- `desktop/README.md`: after the paragraph on linked pull requests (line 832),
  one sentence: when a task has pull requests in several repositories, the
  toolbar shows the primary one and a `+N` chevron listing them all, each
  opening its pull request.
- `CHANGELOG.md`: one `Changed` line under `## [Unreleased]`.

## Target files

| File | Change |
| --- | --- |
| `desktop/src/pullRequests.mjs` | `pullRequestMenuEntries`, `repositoryName` and `prLabel` moved here |
| `desktop/src/main.js` | toolbar markup, `toolbarMenu` helper, folders menu rewired, pull request menu |
| `desktop/src/style.css` | chevron and menu styles, side-by-side rules removed |
| `desktop/tests/pullRequestStates.test.mjs` | unit tests of the entries helper and the moved functions |
| `desktop/tests/pr-repositories.ui.cjs` | rewritten for the chevron and menu |
| `desktop/README.md` | one sentence |
| `CHANGELOG.md` | one `Changed` line |

## Test plan

- Unit (`node --test desktop/tests/pullRequestStates.test.mjs`):
  `pullRequestMenuEntries` returns `[]` for zero and one link; for two it keeps
  the order, computes `name` and `label`; a GitLab URL gives `MR !n`; a URL
  with no number gives `PR / MR`; a repository with no segment gives
  `PR / MR` as its name, as `repositoryName` does today.
- UI (`desktop/tests/pr-repositories.ui.cjs`, fake agent): the sidebar badge
  assertions stay; `#selected-pr` reads `PR #79`; `#selected-pr-more` reads
  `+1` with `aria-expanded="false"`; `#selected-pr-others` no longer exists;
  opening shows two `menuitem`s in order with the expected text and accessible
  name; choosing the second opens its URL and closes the menu; reopening and
  pressing Escape closes it and focuses the chevron.
- UI, single pull request: a second test in the same file, or a second task in
  the same fake agent, asserts `#selected-pr-more` is hidden.
- Regression: the four folder UI tests pass unchanged after D2.
- The desktop UI tests need `npx vite build` first, run unsandboxed and
  serially; restore `webui/.gitkeep` if the build deletes it.
