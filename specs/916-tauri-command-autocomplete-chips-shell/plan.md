# Plan #916 - command autocomplete, chips and shell indicator

## Dependencies

`@codemirror/state`, `@codemirror/view`, `@codemirror/commands` (MIT;
`@codemirror/language` comes with `commands`). ADR
`desktop-tauri/docs/adrs/0019-codemirror-conversation-composer.md`.

## Modules

- `src/domain/command-completion.ts` (pure):
  - `completionContext(value, caret)` -> `{ trigger: "/" | "$", query, from, to }`
    when the caret is inside the first token starting with `/` or `$`.
  - `rankCommands(commands, trigger, query, limit = 50)` -> entries with
    `{ command, tier, match: { field: "name" | "description", from, to } }`.
  - `commandChip(value, commands)` -> `{ from, to, kind: "command" | "skill" }`
    for a known first token followed by whitespace.
  - `isShellDraft(value)`.
- `src/components/ComposerEditor.tsx`: React wrapper over `EditorView`.
  - Props: `value`, `onChange`, `ariaLabel`, `placeholder`, `maxLength`,
    `readOnly`, `chips(doc) => ChipRange[]`, `onKey(event, handle)` (a
    `Prec.highest` `keymap` `any` binding), `onCaret(head)`, `onBlur`,
    `contentAttributes` (combobox ARIA).
  - Controlled: an external `value` different from the document replaces it
    and puts the caret at the end; user transactions call `onChange`.
  - Chips: a `StateField` of `Decoration.replace` widgets, also provided as
    `EditorView.atomicRanges`, so `deleteCharBackward` removes a chip whole.
  - `maxLength`: `EditorState.changeFilter`.
  - Handle (ref): `focus()`, `selection()`, `setCaret()`, `caretOnEdgeRow()`
    (`coordsAtPos`, undefined without layout), `dom`.
- `src/components/CompletionMenu.tsx`: generic listbox (`role="listbox"`,
  options with ids, `aria-selected`, `<mark>` highlights, hover/click via
  `onMouseDown` so the editor keeps focus), placed above the composer box and
  flipped below when `boxTop < menuHeight`.

## Wiring (`ProjectPrompt.tsx` conversation composer)

- Replace the textarea by `ComposerEditor`; `useComposerMaxHeight` sets
  `--composer-max-height` on the editor wrapper (the textarea hook keeps its
  fit-to-content for the "New prompt" view).
- Key order in `onKey`: IME -> menu (arrows, Enter/Tab, Escape) -> prompt
  history (#913) -> Escape interrupt -> Enter submit. `Shift+Enter` falls
  through to CodeMirror's newline.
- Shell indicator: `composer-box shell-mode` class, `Shell` badge in the
  footer, `aria-description` on the editor.

## Tests

- `tests/command-completion.test.ts`.
- `tests/composer-editor.ts` helper (find the editor, set text, read text,
  move the caret) used by the UI tests that drove the textarea.
- `tests/command-menu.ui.test.tsx`.
- jsdom gaps (`Range.getClientRects`, `getBoundingClientRect`) stubbed in
  `tests/vitest.setup.ts`.
