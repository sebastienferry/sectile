# Design

## Context
The board renders one component, `TaskCard.tsx`, for both display modes, switching on
`isCondensed = compact`. The `...` menu is built from three parts: a condensed-only block, the
copy-skill submenu, and the current workflow action. The mode override had been written inside
the condensed-only block because that is where it was first needed — the condensed card has no
inline chevrons, so the menu is its only launch surface.

## Decisions

### A shared fragment, not a duplicated block
`modeActions` is defined once as a JSX fragment and referenced from both branches. The
alternative — copying the two buttons into the `!isCondensed` branch — was rejected: two copies
of the same handler wiring is how the entries drifted out of one shape in the first place, and
the disabled conditions (`advancing !== null || isFinishedTask`) would have to be kept in sync
by hand.

### The expanded card gains nothing beyond the two entries
The inline chevrons on the expanded card keep launching in the resolved mode with no override.
Adding a mode picker to them would mean three inline controls per card, and the `...` menu
already is the documented surface for the one-off choice. The expanded branch therefore renders
`modeActions` plus a separator and nothing else.

### The condensed order is preserved exactly
`modeActions` is substituted in place, between `Avancer` and `Avancer toute la chaîne`, so the
condensed menu is byte-identical in behaviour and order. Muscle memory on the condensed board is
the thing most likely to be noticed if it breaks.

### The test asserts on the source, and on the sharing specifically
There is no exported function to call: the wiring lives in JSX handlers, and
`web/tests/skillLaunchMode.test.mjs` already accepts that trade-off in its header comment. The
existing assertion (`handleAdvance\(false, 'interactive'\)`) matches whether the fragment is
rendered once or twice, so it cannot fail on the bug this change is about. The new assertions
target the structure instead: the `modeActions` definition, a `{modeActions}` reference inside
the `isCondensed` block, and one inside a `!isCondensed` block.

Rejected: putting the regression test in `web/tests/condensed-card.browser.mjs`. It is a real
Playwright/Vite harness and would assert on rendered DOM, but it is not part of `npm test`
(`node --test tests/*.test.mjs`) and requires a Chrome install, so a regression there would not
fail CI — which is the entire point of the criterion.
