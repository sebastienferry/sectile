# Present the execution stop as the closing step of the current stage

## Why
The toolbar control that ends the selected task's running execution is a red
cross (`#stop`). Red and a cross read as destructive, as if the work were being
thrown away. That is not what the button does in the task workflow: ending the
running execution is the normal way to declare the current stage finished, and
it is precisely what unblocks the next step — `Next: <skill>` is disabled for
exactly as long as an execution is active on the task.

The control also sits after `Next:`, at the far right of the toolbar, so the
sequence a user actually performs (close the current step, then launch the next
one) is displayed backwards. The two controls are additionally hard to tell
apart by colour: `Next:` is teal, and any green closing control would look the
same.

## What Changes
- Present `#stop` as the closing step of the current stage: green accent, a
  closing glyph or the `Close` label instead of the red cross.
- Move it before `Next: <skill>` in the toolbar, so the toolbar reads in the
  order the user acts.
- Recolour `Next: <skill>` to a distinct blue, so closing and continuing are
  never confused.
- Keep two separate controls, each disabled when it does not apply: closing only
  during an execution, `Next:` only outside one.

## Impact
Desktop renderer only: `desktop/src/main.js` (toolbar markup, `iconPaths`) and
`desktop/src/style.css`. No change to what the button does: it still stops the
selected execution and still offers the handoff closing step on a reviewed task.
Sectile still never transitions a stage from the desktop.
