# Design

## Context

`retireCheckout` calls `retireContextBlock` on every scaffold. The latter currently recognizes only the legacy marker pair and removes one block. The writer is already gone.

## Decisions

- Recognize both marker pairs and remove every complete managed block in one cleanup pass.
- Match starts and ends by spelling so unrelated text cannot become an accidental deletion range. Return the existing incomplete-block error for unmatched or mismatched markers.
- Keep the file when instructions remain; remove it when only managed content remains. Avoid rewriting files without managed markers.
- Remove only the tracked managed block in this repository's `AGENTS.md`.

## Rejected Alternatives

- Prevent future insertion only: the writer is already absent and existing `sectile` blocks would remain.
- Replace `AGENTS.md` wholesale: that would erase user-authored instructions.
- Introduce a new project-context file: dispatch and MCP already provide this context.
