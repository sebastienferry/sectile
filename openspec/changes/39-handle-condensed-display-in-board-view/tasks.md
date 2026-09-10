# Implementation checklist

- [x] 1. Read the clarification and capability scenarios; inspect TaskCard handlers and the three BoardView rendering paths before editing.
- [x] 2. Add the Compact-only key/menu header and single-line accessible title, remove metadata/footer space, and retain detailed rendering for other densities.
- [x] 3. Reuse the portal menu and add missing compact-mode operations: pin toggle, step/auto advance, conditional parent filter and existing PR/MR link. Preserve all guards, confirmations, event propagation and destinations.
- [x] 4. Ensure keyboard title/menu interaction, visible focus, Escape focus restoration, full-title hover text and existing activity/drag cues; cover any new labels in supported languages.
- [x] 5. Run `npm run lint` and `npm run build` from `web`; record results and distinguish any pre-existing failures.
- [x] 6. Verify density switching and reload persistence; rich/minimal/local tickets; long titles; all three board paths; light/dark themes and existing zoom settings. Confirm compact cards have no hidden-row gaps and detailed cards remain unchanged.
- [x] 7. Verify pointer and keyboard details, tracker link, menu clipping/scrolling/Escape, pinning, parent filter and PR links. On controlled tickets, verify single-step/auto actions, disabled states and drag-and-drop retain their behavior and issue no duplicate operation.
- [x] 8. Verify running/queued border cues and terminal access; record acceptance results mapped to the capability scenarios before advancing implementation status.

Implementation completed. See validation.md for executed checks and the distinction between isolated browser coverage and unchanged integration paths. Open requirements: none.
