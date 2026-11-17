# Specification #915 - Tauri: circular context usage meter in the composer toolbar

Ticket: https://github.com/sebastienferry/sectile/issues/915
Clarification: `docs/clarifications/915.md` (rounds 1 and 2).

## Scope

In: a context usage gauge in the Sectile Desktop (Tauri) conversation composer,
fed by the `context: {used, window}` object `GET /desktop/conversation`
already returns (D1, confirmed). Out: the Electron app (its ring is
unchanged), the web client, automatic compaction, persisting usage across
agent restarts, any change to the Go payload.

## User stories

### US1 - See how full the context is (P1)

1. **Given** a conversation snapshot with `context.used > 0` and
   `context.window > 0`, **then** the composer footer shows, right after the
   model selector, a 16×16 240° arc gauge filled in proportion to the usage,
   and its percentage label (e.g. `18%`).
2. The percentage is `round(used * 100 / window)`, clamped to `[0, 100]`.
3. **Given** no `context`, `used` of 0 or `window` of 0 (Antigravity always,
   Claude during its first turn), **then** nothing is shown (Q1 -> A).
4. The gauge is a `progressbar` with `aria-valuenow`, `aria-valuemin` 0 and
   `aria-valuemax` 100, and its accessible label and tooltip read
   `<pct>% · <used> / <window>` with en-US grouping
   (`18% · 36,000 / 200,000`).

### US2 - Warning when the context is nearly full (P1)

1. **Given** a percentage above 80, **then** the gauge is in a warning state
   (warning colour) and its label and tooltip end with ` · Consider /compact`.
2. **Given** 80% exactly, **then** no warning.

## Functional requirements

- FR1. `ConversationSnapshot.context?: { used: number; window: number }`.
- FR2. `ContextUsageMeter` component per US1, US2.
- FR3. Go tests keep asserting `context` serialization for Claude and Codex.
- FR4. One `Added` line in `CHANGELOG.md`.

## Success criteria

Vitest tests for visibility (hidden without window or usage), arc fill,
label, and the warning state; the Codex Go test asserts the full `context`
object.
