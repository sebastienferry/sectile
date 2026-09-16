# Design

## State derivation
The rendering decision moves out of the component into a pure helper, `web/src/lib/remoteRunIndicator.ts`, exporting `deriveRunIndicator(activities, taskId, now)`. It returns `null`, or `{ state, runs, cancelableRunIds, count }` where `state` is `running | queued | canceled`.

Rules:
- Only activities with `skillId === 'remote_run'` for the task are considered.
- Precedence is running, then queued, then cancelled.
- A cancelled run counts only while `now - completedAt <= 30 s` (`CANCELED_VISIBILITY_MS`). Without an end timestamp it is ignored, because an unbounded cancelled icon would never clear.
- A run is cancellable only when `action === 'Agent-owned remote execution'`, which is the existing rule for showing the Stop button.

A pure function is used because the repository's web test harness (`node --test` over `web/tests/*.test.mjs`, transpiling a single TypeScript module) tests logic modules, not rendered React trees. Rejected alternative: keeping the filtering inline in the component and adding a DOM-based test runner, which would introduce a testing dependency the project does not have.

## Presentation
One `<span role="status">` holding one Lucide icon: `Loader2` spinning for running, `Clock` for queued, `CircleSlash` for cancelled. Colour tokens keep their meaning: cyan running, amber queued, slate cancelled. The visible text is removed; `title` and `aria-label` carry the state, the skill names and the count.

When the derived state is cancellable, the span is rendered as a `<button>`: on hover or focus the glyph swaps to `CircleStop` and the label becomes the stop wording. Clicking cancels every cancellable run of the displayed state and stops event propagation, so a click on a board card does not also open the task. While a cancellation is in flight the control is disabled and shows the spinner.

Rejected alternative: a separate small stop button next to the icon. It reintroduces the width the ticket asks to remove and is hard to hit on a condensed card.

## Open points
None. The 30 s cancelled window is an implementation choice recorded here, reversible without changing behaviour elsewhere.
