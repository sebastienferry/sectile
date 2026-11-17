# Plan #913 - auto-expanding composer and prompt history

Stack: React 19 + TypeScript in `desktop-tauri/`. No agent, server or Rust
change. Tests: `npm test` (node test runner, pure modules) and
`npm run test:ui` (vitest + jsdom).

## 1. Sizing (`src/lib/composer-sizing.ts`)

- `composerMaxHeight(viewHeight)` returns `Math.max(150, viewHeight * 0.4)`.
- `useAutoGrowTextarea(ref, value, containerSelector)` (React hook):
  - on every `value` change (layout effect): `height = "auto"`, then
    `height = scrollHeight + "px"`; CSS `max-height` clamps it; toggles
    `overflow-y` to `auto` only when `scrollHeight` exceeds the cap.
  - on mount: finds `textarea.closest(containerSelector)`, sets
    `--composer-max-height` on the textarea from its `clientHeight`
    (fallback `window.innerHeight`), and observes it with `ResizeObserver`
    (when available), updating the property from `contentRect.height`.
- Containers: `.project-prompt` (new prompt), `.conversation-main-column`
  (conversation).

CSS (`src/App.css`): `.project-prompt .composer textarea` gets
`min-height:48px; max-height:var(--composer-max-height, 200px);
line-height:1.4; overflow-y:hidden; box-sizing:border-box`.

## 2. History (`src/domain/prompt-history.ts`, framework-free)

```ts
type HistoryKey = { key: string; shiftKey; ctrlKey; altKey; metaKey; isComposing };
type HistoryState = { index: number; text: string } | null;
navigatePromptHistory({ prompts, value, selectionStart, selectionEnd, state, event, isEdgeRow? })
  -> { handled: false, state } | { handled: true, value, state }
```

Rules from the reference (`PromptHistoryNavigation.ts`) and decisions D3-D6:
recall starts only from an empty value with `ArrowUp`; a recall is active only
while `value === state.text` and `prompts[state.index] === state.text`;
edge rows approximated by logical lines unless `isEdgeRow` is supplied.

`userPrompts(events)` extracts `kind: "user"` events cleaned with
`cleanUserEventText` (moved to a tiny shared helper so the domain module does
not import a component), empty ones dropped.

Edge rows: `caretOnEdgeRow(textarea, direction)` in
`src/lib/composer-sizing.ts` measures the caret's rendered row with a hidden
mirror element styled like the textarea (font, padding, width, `pre-wrap`),
so a soft-wrapped line counts as several rows. Without layout (jsdom, zero
line height or width) it returns `undefined` and the domain module falls back
to logical lines.

## 3. Wiring (`src/components/ProjectPrompt.tsx`)

- New prompt: `useAutoGrowTextarea(ref, draft, ".project-prompt")`.
- Conversation: same hook with `.conversation-main-column`; `historyState`
  ref; `onKeyDown` order: history (incl. `Escape` on recall) -> `Escape`
  interrupt -> `sendOnEnter`. `compositionstart/end` tracked in a ref.
  Reset on `id` change and after a successful submit.

## Test plan

- `tests/prompt-history.test.ts`: recall from empty, no recall on non-empty,
  step back/forward, stop at oldest, clear past newest, Escape clears,
  modifiers, IME, multi-line caret rows, edited recall becomes draft, stale
  index, `userPrompts` cleaning, `composerMaxHeight`.
- `tests/composer.ui.test.tsx`: conversation recall/step/clear, Escape clears
  without interrupting then interrupts, multi-line caret, IME, new-prompt view
  keeps native arrows, `--composer-max-height` set and updated by a stubbed
  `ResizeObserver` in both views, height follows `scrollHeight`.
