# Plan #922

Branch: `feat/922`. Stack: React, TypeScript, shared CSS; Spec Kit.

## Design

- In `desktop-tauri/src/components/ProjectPrompt.tsx`, use only `icon-button`
  for the next-step button, as Rename and Archive already do.
- Remove the redundant `.conversation-next-step-btn` rule from
  `desktop-tauri/src/App.css`, so padding, dimensions and interaction states
  come from the same shared rules. Keep the existing chevron Icon, whose
  default size is 18 pixels, matching Rename and Archive.
- Keep all handlers, availability conditions and accessibility attributes.
- Update `CHANGELOG.md`, `desktop-tauri/CHANGELOG.md` and
  `desktop-tauri/README.md`.

No new contracts, dependencies or architectural decisions. Avoid another
button-specific size override: the shared primitive already defines the desired
geometry and states.

## Verification

- Existing `tests/prompt.ui.test.tsx` verifies the next-step chevron, tooltip
  and eligible-step callback. Run the full Tauri typecheck, unit, UI, build and
  Rust checks required by `desktop-tauri/AGENTS.md`.
- Inspect the fixture conversation in a browser at 800, 1050 and 1280 pixels,
  including hover, keyboard focus and disabled appearance when available.
- No new automated tests for this reversible class/CSS-only adjustment:
  jsdom does not validate browser layout, and existing tests cover interaction.
- Fetch and verify the current repository base before publishing; local
  history forks from `origin/feat/tauri-monorepo`, as project memory documents.
  Preserve the sandbox refusals and await the requested decisions for retries.
