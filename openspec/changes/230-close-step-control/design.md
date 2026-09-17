# Design

## Context
`#stop` is declared in the toolbar markup of `desktop/src/main.js:25`, after
`#next-step` and `#retry-next-step`. Its glyph comes from `iconPaths.stop`
(`main.js:813`, a cross path), it is injected by the shared icon loop
(`main.js:821-823`), its colours live in the `#stop` rule of
`desktop/src/style.css`, and its handler is `main.js:404-411`
(`api.stop` then `offerClosure`). Its disabled state is computed in two places:
`main.js:152` and `main.js:356`, both on `running|queued|preparing`.

`#next-step` is rendered by `renderNextStep` (`main.js:1061-1077`). It is
disabled while `busy` — an active run on the same task — or while a launch is
pending. Ending the running execution is therefore the actual mechanism that
makes the next step available.

## Decisions

### Presentation change, not a rewiring
The button keeps its id, its handler and `offerClosure`. Only its icon, colour,
label semantics and DOM position change.

Rejected: re-pointing the button at the `handoff` skill. `handoff` is the
closing step of a *reviewed* task, a different concept from ending the current
execution, and it is already offered by `offerClosure`. Rewiring would also
remove the only way to stop a running execution.

### Two controls rather than one contextual button
`Close` and `Next: <skill>` stay separate, each disabled when it does not apply.

Rejected: a single button swapping between "close the step" and "next step".
It would halve the toolbar's state surface but hides one of the two actions at
any moment, makes the control's meaning depend on polled run state, and would
require reworking `renderNextStep`, the two `#stop` disabled computations and
the closure dialog trigger. The gain does not pay for that coupling. This was
the ticket author's explicit call during clarification.

### Colours
The closing control takes the existing green accent already used for
affirmative controls (`#62d3be` / `#80e6ce` family, as `#start-agent` and the
setup form button). `#next-step`, currently teal (`#80e6ce` on `#49796e`), moves
to a distinct blue so the pair does not read as two shades of the same colour.
Both keep their `focus-visible` outline.

### Icon
A closing glyph is preferred over text to keep the toolbar compact. If no glyph
reads unambiguously at 20px, `#stop` becomes a text button labelled `Close`,
which the ticket explicitly allows. Either way the accessible name and the
tooltip continue to describe the effect on the execution, so assistive
technology never loses what the control really does.

### Accessible name kept literal
Implementation note: the accessible name and tooltip stay `Stop execution`. The
same control is used by free agent consoles (`desktop/tests/free-console.ui.cjs`,
`desktop/tests/console.ui.cjs` both address it by that name), where no next step
exists to unlock, so a workflow-flavoured name would be wrong there. The glyph
and colour carry the closing-step meaning; the name carries the literal effect,
which is what the requirement asks for.

### Order
New toolbar order: `#save-log`, `#stop` (closing), `#next-step`,
`#retry-next-step`. DOM order carries focus order, so no `tabindex` is needed.

## Risks
`desktop/tests/next-step.ui.cjs:80` asserts that `#retry-next-step`'s next
sibling is `#stop`; the reorder invalidates it and it must be updated to the new
order. `desktop/tests/task-closure.ui.cjs` exercises the unchanged behaviour and
must keep passing as is. Desktop UI tests load `dist/index.html`, so
`npx vite build` runs before them.
