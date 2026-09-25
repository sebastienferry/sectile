# #474: Implementation plan

References: [`spec.md`](./spec.md), [`tasks.md`](./tasks.md).

## Stack and scope

Frontend only: the web client (`web/`, React 19 + TypeScript) and the desktop
renderer (`desktop/src/main.js`, plain DOM + xterm 6), with one pure module in
`shared/` that both import. No Go change, no migration, no endpoint, no
Electron main-process change.

## Target files

| File | Change |
| --- | --- |
| `shared/sidebarShortcut.mjs` (new) | Pure rules: `isMacPlatform`, `sidebarShortcutAction`, `sidebarShortcutLabel`, `sidebarShortcutAria`. |
| `shared/sidebarShortcut.d.mts` (new) | Types, as `shared/mcpConfig.d.mts` does. |
| `web/src/context/AppContext.tsx` | Persisted `sidebarCollapsed`; chord in the global keyboard handler. |
| `web/src/components/Sidebar.tsx` | Tooltips and `aria-keyshortcuts` on both toggle buttons. |
| `web/src/components/CommandPalette.tsx` | `toggle_sidebar` command. |
| `desktop/src/main.js` | Toggle handler extracted to a function; capture-phase chord listener; tooltip. |
| `web/tests/sidebarShortcut.test.mjs` (new) | Unit tests for the shared module. |
| `web/tests/sidebar-shortcut.browser.mjs` (new) | Web browser regression. |
| `desktop/tests/sidebar-shortcut.ui.cjs` (new) | Desktop UI regression. |
| `CHANGELOG.md` | One `Added` line. |

## Decisions

### One shared rule

Both clients decide with the same pure function, so that the platform and
terminal rules cannot drift between them. It lives in `shared/`, where
`runStates.ts` and `mcpConfig.mjs` are already imported by both (`web/src/...`
through `../../../shared/`, `desktop/src/main.js` through `../../shared/`).
Plain `.mjs` with a `.d.mts`, like `mcpConfig`, so that `node --test` runs it
with no transpilation.

```js
// 'toggle' | 'ignore'
sidebarShortcutAction({
  key, metaKey, ctrlKey, shiftKey, altKey, repeat, defaultPrevented,
  mac,          // isMacPlatform(navigator)
  inTerminal,   // focus inside an .xterm
  modalOpen,    // a modal dialog is open
})
```

It returns `'toggle'` only when `key.toLowerCase() === 'b'`, the platform
modifier is held and the other one is not (`mac ? metaKey && !ctrlKey :
ctrlKey && !metaKey`), no Shift, no Alt, `!repeat`, `!defaultPrevented`,
`!modalOpen`, and `!(inTerminal && !mac)`. `'ignore'` means the caller leaves
the event entirely untouched, which is what lets Ctrl+B reach the terminal and
Ctrl+B on macOS reach whatever has focus.

`isMacPlatform(nav)` reads `nav.userAgentData?.platform ?? nav.platform` and
tests `/mac/i`. No platform helper exists in either client today; Electron's
renderer reports the host platform through the same `navigator`, so the desktop
needs no preload bridge.

`sidebarShortcutLabel(mac)` returns `⌘B` or `Ctrl+B`;
`sidebarShortcutAria(mac)` returns `Meta+B` or `Control+B` for
`aria-keyshortcuts`.

Rejected: accepting either modifier on every platform, as Cmd/Ctrl+K does. On
macOS that captures Ctrl+B, which the owner kept for the terminal (D1).

Rejected: `event.code === 'KeyB'`. The existing Cmd/Ctrl+K and the Markdown
editor both match on `event.key`; matching on the layout-independent code would
make the chord and the editor's bold disagree on non-QWERTY layouts.

### Web

**Listener.** A branch at the top of the existing global handler in
`AppContext.tsx` (line 3632), right after Cmd/Ctrl+K and before the
`isInputActive` checks, so that US1.6 holds in plain fields. On `'toggle'`:
`e.preventDefault()` then `setSidebarCollapsed(prev => !prev)`, the setter the
buttons already use (FR5).

`modalOpen` combines the state the handler already has (`isCommandPaletteOpen`,
`isQuickAddOpen`, `selectedTask`, `selectedActivity`, `isProfileOpen`) with
`isCloneModalOpen` and `document.querySelector('[aria-modal="true"]')`, which
covers the modals that keep their open state locally (project, changelog, board
view, batch pickup). `CommandPalette`, `TaskDetailModal` and `CloneTaskModal` do
not carry `aria-modal`, hence the state flags; adding the attribute to them is
left alone because `BoardView`'s Escape rule reads the same selector.
`isCloneModalOpen` joins the effect's dependency list.

`inTerminal` reuses the handler's existing `isXterm` test. The web has no
terminal today (no `new Terminal` under `web/src`), so the value is always
false in practice; passing it keeps the shared rule honest if one appears.

**Bold in the editor.** The Markdown textarea handles Cmd/Ctrl+B in its React
`onKeyDown` with `preventDefault()`. React dispatches from the root container
during bubbling, before the native event reaches `window`, so the global
handler sees `defaultPrevented` and ignores it (US2.4). No change to
`Markdown.tsx`.

**Persistence.** `sidebarCollapsed` gets the `hideDone` treatment: a lazy
`useState` initialiser reading `localStorage.getItem('sectile_sidebar_collapsed')
=== 'true'` inside `try`, and `setSidebarCollapsed` wrapped in a `useCallback`
that computes the next value (function or boolean, as the context type already
allows) and writes it inside `try`. The `sectile_` prefix follows the other
keys; there is no legacy `taskacao_` key to read since the state was never
stored.

**Tooltips.** Collapse button: `` `${t.nav.toggleSidebar} (${label})` ``;
expand button: `` `${t.app.title} - ${t.nav.toggleSidebar} (${label})` ``. Both
get `aria-keyshortcuts`. The `|| 'Replier'` / `|| 'Déplier'` fallbacks are
dropped: the key exists in both locales. No new translation string: the
shortcut label is not a word.

**Palette.** A `toggle_sidebar` entry in `generalActions`, after
`create_task`, titled `t.nav.toggleSidebar`, icon `PanelLeft` (lucide),
`shortcut: sidebarShortcutLabel(mac)`, keywords `['sidebar', 'menu', 'barre',
'laterale', 'replier', 'deplier', 'toggle', 'collapse', 'expand']`, action
`setSidebarCollapsed(prev => !prev)` then close the palette.

### Desktop

**One toggle path.** The body of `#toggle-sidebar`'s `onclick` (line 1149)
becomes `toggleSidebar()`: toggle `sidebar-hidden` on `#workspace`,
`renderSidebarToggle(hidden)`, persist `localStorage.sidebarCollapsed`,
`resize()`. The button calls it; so does the chord (FR5).

**Listener.** A `window` `keydown` listener in the capture phase, next to the
Cmd/Ctrl+K one (line 2213) and in the same style. Capture runs before xterm's
own handler on its helper textarea, so on macOS Cmd+B never reaches the
terminal (US2.2) once the listener calls `preventDefault()` and
`stopPropagation()`. For `'ignore'` it returns without touching the event, so on
Windows and Linux Ctrl+B reaches xterm as before (US2.1).

- `inTerminal`: `document.activeElement?.closest('.xterm')`.
- `modalOpen`: `dialog.open`, the single `<dialog>` every desktop dialog and
  the command palette use (`showDialog`).
- `defaultPrevented`: passed as is; nothing earlier handles B in capture.

**Tooltip.** `renderSidebarToggle` sets `title` to `Hide projects (⌘B)` /
`Show projects (Ctrl+B)` (the platform's label) and `aria-keyshortcuts`; the
`aria-label` stays without the shortcut, since `aria-keyshortcuts` carries it.

Rejected: an Electron menu accelerator (`CmdOrCtrl+B`). Windows and Linux have
no application menu (`Menu.setApplicationMenu(... : null)` in
`desktop/electron/main.cjs`), and a menu accelerator fires before the renderer
sees the key, so the terminal exception could not be expressed.

## Risks

- **Firefox Ctrl+B.** Firefox lets a page cancel Ctrl+B (bookmarks sidebar) with
  `preventDefault()`; checked by hand on Firefox, since the browser suite runs
  Chromium.
- **Future web terminal.** If one appears, it must not cancel the event itself
  for Cmd+B on macOS, or the global handler never sees it; the shared rule
  already expects to be told about the terminal.

## Test strategy

- Unit: `web/tests/sidebarShortcut.test.mjs` runs the shared rule as a table:
  each platform x modifier combination, Shift, Alt, repeat, `defaultPrevented`,
  modal, terminal on both platforms, other keys; plus `isMacPlatform` on
  `userAgentData` and legacy `platform`, and the two label helpers.
- Web browser: `sidebar-shortcut.browser.mjs` reuses the
  `board-views.browser.mjs` harness shape (real `App`, fake `fetch`). The
  platform is forced by overriding `navigator.platform` in an init script
  before the app loads, once per platform. It asserts the sidebar width class,
  the stored value across a reload, bold in the Markdown editor, bare B, the
  quick-add dialog blocking the chord, the tooltips and the palette command.
- Desktop UI: `sidebar-shortcut.ui.cjs` reuses the `sidebar-alignment.ui.cjs`
  harness (fake agent server, `SECTILE_DESKTOP_TEST`). It checks the button
  state, the stored value and that the terminal received no data on Cmd+B, and
  received `\x02` on Ctrl+B with the platform forced to Linux through an init
  script. Needs `npx vite build` first.
