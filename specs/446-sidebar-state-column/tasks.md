# Tasks: aligned run indicators in the Desktop sidebar

1. [ ] Rebase `feat/446` on `origin/main` and check nothing on `main` already
       reorders the sidebar row.
2. [ ] `desktop/tests/sidebar-alignment.ui.cjs`: write the alignment test first
       (glyph column, title column, long key, free console, first child, click
       targets, disabled macro key); confirm it fails on the current layout.
3. [ ] `desktop/src/main.js`: move `span.run-state` out of `button.run` as the
       row's first child, with `onclick=()=>select(run)`; wrap the key in an
       always-present `span.task-key-slot`, filled only for non-free consoles.
4. [ ] `desktop/src/style.css`: glyph left padding, `.task-key-slot` (`5ch`
       min-width, no shrink, nowrap), tabular figures on the key, retarget the
       `.local-task>.task-number` rules.
5. [ ] Grep the UI tests for selectors assuming the glyph is inside `.run` and
       adjust them without weakening their assertions.
6. [ ] `CHANGELOG.md`: one line under `## [Unreleased]` → `### Changed`
       (`(#446)`).
7. [ ] Gate: desktop `npm test`; `npx vite build` then the desktop UI tests
       (new one included, plus `run-state-animation`, `free-console`,
       `task-order-render`, `task-order-hold`, `tooltips`); restore
       `webui/.gitkeep` if the build removed it.
8. [ ] Visual check in the running Desktop app: glyphs and titles in columns
       for mixed keys and a free console.
