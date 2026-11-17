# ADR 0019: Use CodeMirror 6 for the conversation composer

Status: accepted

Date: 2026-11-17

## Context

Ticket #916 asks for inline chips in the conversation composer: a completed
`/command` or `$skill` shows as a badge (monospace for commands, an icon for
skills) and `Backspace` right after it deletes the whole token. Ticket #917
adds the same chips for `@` mentions anywhere in the message. A `<textarea>`
holds plain text only, so it cannot render a badge inside the text.

## Decision

The conversation composer is a CodeMirror 6 editor (`@codemirror/state`,
`@codemirror/view`, `@codemirror/commands`, MIT), wrapped by
`src/components/ComposerEditor.tsx`:

- the draft stays a plain string owned by React; chips are replace
  decorations computed from the text and registered as atomic ranges, so
  what is sent, queued and stored is exactly what was typed;
- the parent handles keys through one highest-precedence keymap binding
  (completion menu, prompt history, Escape, Enter), and CodeMirror keeps
  caret movement, `Shift+Enter`, undo and IME;
- the 32 000-character limit is a change filter.

The "New prompt" composer keeps its textarea: it has no command list.

## Alternatives rejected

- **Textarea with a highlight layer behind it.** No dependency, but it can
  only colour a token in place with the same glyph widths: no icon, no
  padding, no monospace badge. It reads as a highlighted word, not a chip.
- **Hand-written `contenteditable`.** Real chips without a dependency, but
  caret, selection, IME, undo and paste must be rebuilt by hand, with the
  highest regression risk.

## Consequences

- CodeMirror ships in the lazily loaded conversation chunk (`ProjectPrompt`, about 320 kB minified with it), not in the main bundle.
- UI tests drive the editor through `tests/composer-editor.ts` instead of
  `fireEvent.change` on a textarea; jsdom's missing range geometry is stubbed
  in `tests/vitest.setup.ts`.
- `THIRD_PARTY_NOTICES_JS.md` lists the new packages.
