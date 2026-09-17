# Tasks

## 1. Toolbar presentation
- [x] 1.1 Move `#stop` before `#next-step` in the toolbar markup (`desktop/src/main.js:25`), keeping `#retry-next-step` last.
- [x] 1.2 Replace `iconPaths.stop` (`main.js:813`) with a closing glyph; if none reads at 20px, render `#stop` as a text button labelled `Close` instead.
- [x] 1.3 Keep the `#stop` accessible name and tooltip on the literal effect (`Stop execution`): the same control serves free consoles, which have no next step to unlock.

## 2. Colours
- [x] 2.1 Restyle the `#stop` rule in `desktop/src/style.css` with the green closing accent, keeping its `focus-visible` outline.
- [x] 2.2 Restyle `#next-step` with a distinct blue accent, keeping its `focus-visible` outline.
- [x] 2.3 Check both controls against the dark background for legibility and non-colour distinguishability (glyph/label differ, not colour alone).

## 3. Behaviour left intact
- [x] 3.1 Verify the `#stop` handler (`main.js:404-411`) and `offerClosure` are unchanged.
- [x] 3.2 Verify both disabled computations (`main.js:152`, `main.js:356`) still gate on `running|queued|preparing`.
- [x] 3.3 Verify `Next: <skill>` becomes available once the execution ends, with no stage transition made by the desktop.

## 4. Tests
- [x] 4.1 Update the order assertion in `desktop/tests/next-step.ui.cjs:80` to the new toolbar order.
- [x] 4.2 Add coverage that the closing control precedes `Next: <skill>` and that the two are enabled in mutually exclusive run states.
- [x] 4.3 Run `npx vite build`, then the desktop UI tests, and confirm `desktop/tests/task-closure.ui.cjs` passes unmodified.
