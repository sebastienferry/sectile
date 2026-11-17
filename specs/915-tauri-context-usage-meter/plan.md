# Plan #915 - context usage meter

- Go: no production change. `internal/agent/agent_codex_conversation_test.go`
  asserts `"context":{"used":25,"window":200000}` (the Claude test already
  asserts its object).
- Rust: none (`read_conversation` passes JSON through).
- `desktop-tauri/src/lib/conversation.ts`: `context?: { used; window }` on
  `ConversationSnapshot`.
- `desktop-tauri/src/domain/context-usage.ts` (pure): `contextUsage(context)`
  returns `undefined` or `{ percent, warning, label }`.
- `desktop-tauri/src/components/ContextUsageMeter.tsx`: SVG 16×16, radius 6,
  stroke 2, round caps; background arc and fill arc share a 240° path from
  150° to 390°; fill length = `arc * percent / 100` through
  `stroke-dasharray`/`stroke-dashoffset`. `role="progressbar"`.
- `ProjectPrompt.tsx`: render it after `TurnSettings` in the conversation
  footer; CSS `order:1` keeps it between the model chip (order 1, earlier in
  the DOM) and the mode chip (order 2).
- `App.css`: `.context-meter`, `.context-meter.warning` (uses `--danger`).

## Test plan

- `tests/context-usage.test.ts`: percent rounding and clamping, hidden cases,
  label formatting, warning at 81 not at 80.
- `tests/context-meter.ui.test.tsx`: hidden without context, rendered beside
  the model chip with progressbar attributes and fill, warning class and
  `/compact` label.
