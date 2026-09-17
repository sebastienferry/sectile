# Design

## Next-step placement
The next-step control moves from `footer#task-status` to `#toolbar`, rendered as the last controls before `#stop`, so the order becomes execution history, pull request, Relaunch, Export log, next step, Retry, Stop. `#retry-next-step` moves with `#next-step`: it relaunches the failed metadata read for that same action and is meaningless apart from it.

`#next-step-status` stays in the footer. It is text, not a control: it carries the task key, the stage and the failure message, it is a `role=status` live region, and the toolbar has no room for a wrapping sentence next to a fixed-height row of buttons.

`renderNextStep()` keeps its current logic unchanged — the same hidden/disabled rules, the same generation guard against late responses. Only the nodes it writes into have moved. Keeping the footer markup and hiding it with CSS was rejected: it leaves a dead landmark and two places to reason about.

## Dialog close affordance
`#close-dialog` becomes an `.icon-button` holding the cross already used by the execution queue's close control (`<path d="m6 6 12 12M18 6 6 18"/>`, `desktop/src/main.js:178`), coloured with the theme red `#ffb7c3` that `#shutdown` already uses, on a transparent background instead of `#202a38`. No new colour or icon set enters the theme.

Red on hover only, and a filled macOS-style red disc, were both rejected during clarification: the first hides the affordance the ticket asks to make obvious, the second introduces a shape that exists nowhere else in the desktop.

The button keeps `aria-label="Close"`, its sticky position, its click handler and its place in the dialog's focus order; the SVG is `aria-hidden`. `#dismiss-dialog` in the dialog footer is untouched.

## Verification
Existing UI tests under `desktop/tests/` load `dist/index.html`, so `npx vite build` runs before them. `next-step.ui.cjs` and `workflow.ui.cjs` query the moved controls and are updated to their new container; their assertions on behaviour do not change.

No requirement is left open and no external dependency is involved. The assigned branch is `feat/157`.
