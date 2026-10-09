# Plan #805 - Desktop: Add task / Edit task

## Stack

- Desktop renderer: plain JavaScript modules in `desktop/src/`, bundled by Vite,
  no framework. Unit tests with `node --test` (`desktop/tests/*.test.mjs`), UI
  tests with Playwright on Electron (`desktop/tests/*.ui.cjs`) against
  `desktop/tests/fake-agent.cjs`.
- Desktop main process: `desktop/electron/main.cjs` and `preload.cjs` (IPC to
  the agent, capability gates).
- Agent: Go, `internal/agent/agent_desktop.go` (the `/desktop/*` routes),
  tests in `internal/agent/*_test.go`.
- Server: unchanged. The page uses `POST /api/tasks`, `GET /api/tasks/<id>`,
  `PUT /api/tasks/<id>` and `GET /api/tasks/<id>/assignable` as they are.
- No migration.

## Editor library

**Choice: `@milkdown/kit` 7.22.x, without the Crepe preset.**

Milkdown is ProseMirror with remark as its Markdown parser and serializer, so
Markdown is the document's native format, which the round trip (US3.7) and the
server's Markdown/Jira conversion rely on. The kit gives every brick the page
needs as headless plugins: `preset-commonmark`, `preset-gfm` (tables,
strikethrough, task lists), `plugin-history`, `plugin-listener`,
`plugin-clipboard` (HTML paste through the schema's parse rules, Markdown paste),
`plugin-slash` (US3.3), `plugin-tooltip` (US3.4), `plugin-block` (US3.5),
`plugin-trailing`, `plugin-indent`.

Rejected:

- **`@milkdown/crepe`**: it ships the Notion-like UI ready-made but depends on
  Vue, KaTeX (math is outside the supported subset), CodeMirror with
  `@codemirror/language-data` (dozens of lazily loaded language chunks) and
  DOMPurify; most of its 3.4 MB unpacked is weight the page does not need, and
  its math and image-upload blocks would have to be switched off anyway. The
  slash menu, toolbar and block handle are written instead as small vanilla DOM
  views on the kit plugins, styled with the Desktop CSS variables.
- **Tiptap 3 with `@tiptap/markdown`**: viable, and the fallback. Its Markdown
  support is an extension over a non-Markdown document model (parsing through
  `marked`, its own serializer), which makes fidelity a property to test rather
  than one built in.

Gate before writing the page (task T1): build the editor alone and check that
(1) it runs under the unchanged CSP with no violation, (2) the round-trip
fixtures pass, (3) the renderer bundle growth is recorded in the pull request
description (`du -k desktop/dist/assets` before and after). If (1) or (2)
fails and cannot be fixed by configuration, switch to Tiptap; the module
boundary below keeps the switch local to `markdown-editor.mjs`.

## Modules

### `desktop/src/markdown-editor.mjs` (new)

The only module that imports Milkdown. API:

```js
const editor = await createMarkdownEditor(root, {
  markdown,            // loaded description, '' in creation mode
  openLink,            // (href) => Promise, the existing external opener
  onChange,            // () => void, on every document or source change
  placeholder,         // 'Write a description, or type / for blocks'
})
editor.markdown()      // current Markdown (serialized, or raw source in source mode)
editor.changed()       // FR4: document !== baseline, or source edited
editor.setSourceMode(on)
editor.sourceMode()
editor.focus()
editor.destroy()
```

- Baseline: the document parsed from `markdown` at creation, kept as a
  ProseMirror node; `changed()` is `!state.doc.eq(baseline)` in WYSIWYG mode.
  Undoing back to the loaded content therefore reads as unchanged (US3.7).
- Source mode (US3.6): entering it fills a `<textarea>` with the original
  `markdown` while the document still equals the baseline, else with the
  serialization; `changed()` is then `textarea.value !== enteredSource ||
  changedBeforeEntering`. Leaving it parses the text into the editor (replacing
  the document in one transaction) and, when the text was not edited, restores
  the previous state so that the baseline comparison still holds. While the
  document equals the baseline, `markdown()` returns the original text, so raw
  HTML and footnotes survive a save that did not touch the description.
- Links (US3.8): a ProseMirror `handleClickOn`/`handleDOMEvents.click` prevents
  navigation; with Cmd/Ctrl it calls `openLink(href)` when
  `isExternalLink(href)` (imported from `markdownView.mjs`) is true.
- Images (US3.10): an image node view renders `<img>` only for `data:` sources,
  else a placeholder `span.md-image-placeholder` with the alt text and URL. The
  node and its Markdown are untouched.
- Paste (US3.9): rely on the schema parse rules (unknown elements dropped);
  add a `transformPastedHTML` that strips `script`, `style`, `iframe`,
  `object`, `embed` before parsing, as defence in depth.
- Slash menu, inline toolbar, block handle: vanilla DOM views under
  `.md-slash`, `.md-toolbar`, `.md-block-handle`, keyboard-driven (arrow keys,
  Enter, Escape), `role="listbox"`/`role="toolbar"` with labels.
- Keymap (US3.4): the commonmark/gfm commands bound to Mod-b, Mod-i,
  Mod-Shift-x, Mod-e, Mod-k (Mod-k opens a small URL input in the toolbar),
  plus history's Mod-z / Mod-Shift-z.
- Styles in `desktop/src/style.css` under `.md-editor`, using the existing
  colour variables so light and dark follow the appearance (US3.11). The
  ProseMirror base CSS is imported from the package (inline style, allowed by
  the CSP).

### `desktop/src/task-form.mjs` (new, pure, unit-tested)

No DOM. Holds the rules the page applies:

- `validatePullRequestUrl(value, links)` → `{ok, url}` or `{ok:false, reason}`
  (US5.2: empty, not absolute http(s), credentials, duplicate).
- `addLink`, `removeLink(links, i)`, `moveLink(links, i, delta)`; loaded link
  objects are kept as they are, added ones are `{url}` (US5.3).
- `currentLink(links)`: the last one.
- `taskChanges(loaded, form, {descriptionChanged})` → the update payload with
  only the changed keys among `title`, `description`, `assignee`,
  `assigneeAccountId`, `assigneeAvatar`, `prLinks` (US2.5, FR4, FR5).
- `validateTitle`, `validateDescription` (FR5 messages).
- `assigneeLookup(provider)`: true for `jira` and `gitlab`, as the web's
  `trackerHas(source,'assigneeLookup')`.
- `initialProject({openedFrom, selected, remembered, known})` (US1.3), moved
  from the quick add code.
- `leaveMessage(key)` (US6.2).

### `desktop/src/task-page.mjs` (new)

Builds the page in a given container; exported
`openTaskPage(container, {mode, projectId, taskId, deps})` returns a handle
`{dirty(), destroy()}`. `deps` injects `api`, `openLink`, `launchClarify`,
`openTickets`, `refresh` and `confirm`, so the module has no global from
`main.js`. It owns the fields, the save sequence, the post-creation actions
and the status line.

Save sequence in creation mode:

1. `api.createTask({projectID, trackerID, title, description})`.
2. Switch the page to edition mode on the returned task (baseline = what was
   sent).
3. When an assignee or links were entered: `api.updateTask(projectID,
   task.id, {assignee, prLinks})`; on failure, keep them unsaved (US1.8).
4. Show the post-creation strip (US1.6).

Save sequence in edition mode: `taskChanges(...)`; when empty, nothing;
else `api.updateTask(...)`, then `api.task(...)` to reload, rebuild the
baseline, `Saved`.

### `desktop/src/main.js` (edited, in small hunks)

- Remove `quickAdd`, its `QUICK_ADD_PROJECT` handling moves to
  `task-form.mjs` (the `localStorage` key stays `quickAddProject` so the last
  used project is kept across the upgrade).
- `openTaskPage` wrapper that mounts the page in the workspace area like
  `openTickets`, remembers the previous view for **Done**, and registers the
  page as the current guard.
- Navigation guard: one `confirmLeave()` (async, resolves `true` when there is
  nothing unsaved or the user discards) called first by the sidebar run and
  project selection, `openTickets`, the palette actions, settings, the Cmd+N
  handler, and `openTaskPage` itself. A run that starts in the background and
  would normally take the workspace does not replace a dirty page.
- Entry points: `create.onclick`, the palette `New task` entry and the Cmd+N
  handler call `openTaskPage({mode:'create', projectId})`. The Tickets row menu
  gains **Edit task**; the run header gains **Edit task** when the run has a
  server `taskId` and is neither a free console nor a macro run.
- `beforeunload`: sets `event.returnValue` when the page is dirty (US6.3).

### `desktop/electron/main.cjs` and `preload.cjs`

- New IPC, each gated on `task-page` with the outdated-agent message of US7.1:
  `task` (`GET /desktop/tasks/detail`), `update-task`
  (`PUT /desktop/tasks/detail`), `assignable`
  (`GET /desktop/tasks/assignable`).
- `create-task` passes `trackerID` through (the agent already accepts it).
- `will-prevent-unload` on the main window: show a native message box
  (`Discard unsaved changes?`, buttons **Keep editing** / **Discard**) and call
  `event.preventDefault()` on Discard to let the unload proceed.

### Agent (`internal/agent/agent_desktop.go`)

- Capability `task-page` (constant `taskPageCapability`) added to
  `/desktop/status`.
- `/desktop/projects` GET entries gain `trackers: [{id, name, provider}]`,
  read from the project configuration the agent already fetches
  (`fetchConfig`); empty when unknown.
- `GET /desktop/tasks/detail?projectId=&taskId=`: `fetchConfig`, then
  `readAPI("/api/tasks/<id>")`, refuse with 400 when
  `!taskInProject(task, projectID)`, answer the task.
- `PUT /desktop/tasks/detail?projectId=&taskId=`: body limited to 128 KiB,
  decoded into a struct of pointers (`title`, `description`, `assignee`,
  `assigneeAccountId`, `assigneeAvatar`, `prLinks`); every other key is
  ignored. Validate: title non-empty when present, every `prLinks[].url`
  absolute `http`/`https` without user info (FR6). Check the task belongs to
  the project, then forward a `models.UpdateTaskRequest` with those fields to
  `PUT /api/tasks/<id>` with the paired token, and relay status and body as
  `desktopCreateTask` does.
- `GET /desktop/tasks/assignable?projectId=&taskId=&q=`: same project check,
  relay `GET /api/tasks/<id>/assignable?q=`.
- The server sets `prUrl` from the last link when `prLinks` is sent; a test
  pins that the agent sends `prLinks` alone and the server answer carries the
  expected `prUrl`. If it does not, the agent also sends `prUrl` = last URL
  (empty when the set is empty).

## Data contracts

`PUT /desktop/tasks/detail?projectId=<p>&taskId=<t>`

```json
{
  "title": "New title",
  "description": "Markdown…",
  "assignee": "Jane Doe",
  "assigneeAccountId": "5b10ac8d82e05b22cc7d4ef5",
  "assigneeAvatar": "https://…",
  "prLinks": [{"url": "https://github.com/o/r/pull/1", "state": "merged", "branch": "feat/1"}, {"url": "https://github.com/o/r/pull/2"}]
}
```

Every key optional; absent means unchanged. Answer: the server's task JSON.
Errors: 400 (validation, task not in project), 405, 502 (server unreachable),
and the server's own status otherwise.

`GET /desktop/projects` entry: `{id, name, path, configured, disconnected,
trackers: [{id, name, provider}]}`.

Documented in `docs/contracts/server-agent-v1.md` next to
`/desktop/create-task`.

## Target files

| File | Change |
| --- | --- |
| `desktop/package.json`, `desktop/package-lock.json` | add `@milkdown/kit` |
| `desktop/src/markdown-editor.mjs` | new |
| `desktop/src/task-form.mjs` | new |
| `desktop/src/task-page.mjs` | new |
| `desktop/src/main.js` | remove quick add, mount page, guard, entry points |
| `desktop/src/style.css` | page and editor styles; remove `.quick-add*` |
| `desktop/electron/main.cjs`, `preload.cjs` | IPC, gates, unload guard |
| `internal/agent/agent_desktop.go` | capability, three routes, trackers |
| `internal/agent/agent_desktop_test.go` (or the file holding the create-task tests) | route tests |
| `desktop/tests/task-form.test.mjs` | new |
| `desktop/tests/fake-agent.cjs` | new routes, capability, trackers |
| `desktop/tests/task-page.ui.cjs`, `markdown-editor.ui.cjs` | new |
| UI tests that open the quick add (`grep -l quick-add desktop/tests`) | move to the page |
| `docs/USER_GUIDE.md`, `docs/contracts/server-agent-v1.md`, `CHANGELOG.md` | docs |

## Risks

- `desktop/src/main.js` is large; edit it in small hunks, keep the logic in the
  new modules (memory: subagents stall on it).
- Round-trip normalization: covered by document comparison (FR4) and the
  "original text while unchanged" rule; fixtures pin it.
- Milkdown needs a DOM: its tests run in Electron (`markdown-editor.ui.cjs`),
  not under `node --test`.
- Window-close guard: `will-prevent-unload` must not block the quit path used
  by the autostart and update flows when nothing is dirty; covered by US6.4.
