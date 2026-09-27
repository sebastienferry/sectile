# Desktop execution status in the task footer

## Why
Run and skill status crowd the console title, while the current-step badge repeats the skill already named there.

## What Changes
- Move selected execution run state and skill result to the task footer.
- Hide the current-step badge during active/submitted work, retaining next-step labels and action accessibility.
- Preserve sidebar indicators, status semantics, animation, and responsive layout.

## Impact
Desktop renderer and Electron UI regression tests only. No API or infrastructure changes.
