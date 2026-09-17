# The one-off mode override belongs to the card, not to its shape

## Why
The one-off interactive / autonomous override was only reachable on **condensed** cards. In
`web/src/components/TaskCard.tsx` the two entries `Avancer en interactif` and
`Avancer en autonome` sat inside the `{isCondensed && ( … )}` block, mixed with genuinely
condensed-only affordances (pin, parent filter, open PR — all of which the expanded card already
shows inline). On an expanded card the `...` menu fell through to `workflowAction`, which calls
the stage skill with no mode override.

A user working on expanded cards could therefore only ever run the skill in the mode resolved
from the skill setting and the project default: the top level of the precedence chain documented
in `docs/CAPABILITIES.md` was unreachable. Nothing in
`openspec/changes/123-autonomous-or-interactive-skill-runs/` restricts the override to one
display mode, so this was an oversight of the condensed-card work, not a decision.

Commit `74b9a9a` ("feat(web,desktop): offer the execution mode on every card and name the
project default") fixed the production code by extracting the two entries into a shared
`modeActions` fragment rendered from both the `isCondensed` and the `!isCondensed` branch. This
change records the requirement behind that code and adds the regression test that was missing,
so the same refactor cannot silently drop the expanded branch again.

## What Changes
- The board card's one-off mode override is specified as independent of the board display mode:
  both entries are in the `...` menu whether the card is condensed or expanded.
- The condensed menu keeps its existing entries and their order; the expanded menu gains only
  the two mode entries and a separator, nothing else.
- `web/tests/skillLaunchMode.test.mjs` gains assertions that pin the *shared* fragment: a single
  `modeActions` definition, referenced from the `isCondensed` branch **and** from the
  `!isCondensed` branch. This is exactly what the refactor could lose, and what the existing
  assertions — plain regexes on the handler calls — cannot distinguish.

## Impact
- Affected specs: `skill-execution-mode`
- Affected code: `web/tests/skillLaunchMode.test.mjs` (the production change is already merged
  in `74b9a9a`, `web/src/components/TaskCard.tsx`)
- No API change, no migration, no persisted state. The server-side refusal for a provider
  without a headless mode (`internal/agent/agent_config.go`, `internal/db/db.go`) is untouched
  and independent of the display mode by construction.
