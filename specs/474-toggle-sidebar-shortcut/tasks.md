# #474: Implementation checklist

References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. Shared rule (FR1-FR4, FR7)

- [ ] T1.1 Add `shared/sidebarShortcut.mjs` and `shared/sidebarShortcut.d.mts`:
      `isMacPlatform`, `sidebarShortcutAction`, `sidebarShortcutLabel`,
      `sidebarShortcutAria`.
- [ ] T1.2 Add `web/tests/sidebarShortcut.test.mjs`: the decision table of
      `plan.md`, `isMacPlatform` on both navigator shapes, both labels.

## 2. Web (US1, US2, US3, US4, US5.1, US5.3, FR5-FR8)

- [ ] T2.1 Persist `sidebarCollapsed` under `sectile_sidebar_collapsed` in
      `AppContext.tsx`, both reads and writes inside `try`.
- [ ] T2.2 Add the chord branch to the global keyboard handler, before the
      `isInputActive` checks; add `isCloneModalOpen` to the modal test and the
      effect dependencies.
- [ ] T2.3 Tooltips and `aria-keyshortcuts` on both toggle buttons in
      `Sidebar.tsx`.
- [ ] T2.4 `toggle_sidebar` command in `CommandPalette.tsx`.

## 3. Desktop (US1.3-US1.5, US2.1-US2.3, US3.2, US5.2, FR5)

- [ ] T3.1 Extract `toggleSidebar()` from the `#toggle-sidebar` handler.
- [ ] T3.2 Capture-phase chord listener beside the Cmd/Ctrl+K one.
- [ ] T3.3 Tooltip and `aria-keyshortcuts` in `renderSidebarToggle`.

## 4. Changelog (FR9)

- [ ] T4.1 One line under `[Unreleased]` / `### Added` in `CHANGELOG.md`,
      naming both clients and the web persistence, with `(#474)`.

## 5. Verification

- [ ] T5.1 Add `web/tests/sidebar-shortcut.browser.mjs`.
- [ ] T5.2 Add `desktop/tests/sidebar-shortcut.ui.cjs`.
- [ ] T5.3 `npm run build`, `npm run lint`, `npm test` in `web/`; run
      `sidebar-shortcut.browser.mjs` and `board-views.browser.mjs`.
- [ ] T5.4 `npm test` in `desktop/`; `npx vite build`, then run
      `sidebar-shortcut.ui.cjs` and `sidebar-alignment.ui.cjs`; restore
      `webui/.gitkeep` if the build removed it.
- [ ] T5.5 By hand: Ctrl+B in Firefox does not open the bookmarks sidebar.
- [ ] T5.6 Re-read the diff.

## Test plan

| Acceptance | Covered by |
| --- | --- |
| US1.1, US1.2 | `sidebarShortcut.test.mjs`; `sidebar-shortcut.browser.mjs` (macOS and Linux) |
| US1.3, US1.4, US1.5 | `sidebar-shortcut.ui.cjs` (button state, stored value, terminal width) |
| US1.6 | `sidebar-shortcut.browser.mjs` (chord in the search field) |
| US1.7 | `sidebarShortcut.test.mjs` (`'toggle'` implies `preventDefault`); T5.5 by hand |
| US2.1, US2.2 | `sidebar-shortcut.ui.cjs` (terminal data on Ctrl+B / none on Cmd+B) |
| US2.3 | `sidebarShortcut.test.mjs` (Ctrl+B on macOS ignored) |
| US2.4, US2.5 | `sidebar-shortcut.browser.mjs` (bold inserted, bare B opens Board) |
| US2.6, US2.7 | `sidebarShortcut.test.mjs` |
| US3.1 | `sidebarShortcut.test.mjs`; `sidebar-shortcut.browser.mjs` (quick add open) |
| US3.2 | `sidebar-shortcut.ui.cjs` (command palette dialog open) |
| US4.1, US4.2 | `sidebar-shortcut.browser.mjs` (reload keeps the state) |
| US4.3 | `sidebar-shortcut.browser.mjs` (no stored value starts expanded) |
| US5.1, US5.3 | `sidebar-shortcut.browser.mjs` (tooltip, palette command) |
| US5.2 | `sidebar-shortcut.ui.cjs` (button title) |
